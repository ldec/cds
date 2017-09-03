package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-gorp/gorp"
	"github.com/golang/protobuf/ptypes"
	"github.com/ovh/venom"
	"github.com/stretchr/testify/assert"

	"github.com/ovh/cds/engine/api/group"
	"github.com/ovh/cds/engine/api/hatchery"
	"github.com/ovh/cds/engine/api/objectstore"
	"github.com/ovh/cds/engine/api/pipeline"
	"github.com/ovh/cds/engine/api/test"
	"github.com/ovh/cds/engine/api/test/assets"
	"github.com/ovh/cds/engine/api/token"
	"github.com/ovh/cds/engine/api/worker"
	"github.com/ovh/cds/engine/api/workflow"
	"github.com/ovh/cds/sdk"
)

type test_runWorkflowCtx struct {
	user        *sdk.User
	password    string
	project     *sdk.Project
	workflow    *sdk.Workflow
	run         *sdk.WorkflowRun
	job         *sdk.WorkflowNodeJobRun
	worker      *sdk.Worker
	workerToken string
	hatchery    *sdk.Hatchery
}

func test_runWorkflow(t *testing.T, api *API, router *Router, db *gorp.DbMap) test_runWorkflowCtx {
	u, pass := assets.InsertAdminUser(api.MustDB())
	key := sdk.RandomString(10)
	proj := assets.InsertTestProject(t, db, key, key, u)
	group.InsertUserInGroup(api.MustDB(), proj.ProjectGroups[0].Group.ID, u.ID, true)
	u.Groups = append(u.Groups, proj.ProjectGroups[0].Group)

	//First pipeline
	pip := sdk.Pipeline{
		ProjectID:  proj.ID,
		ProjectKey: proj.Key,
		Name:       "pip1",
		Type:       sdk.BuildPipeline,
	}
	test.NoError(t, pipeline.InsertPipeline(api.MustDB(), proj, &pip, u))

	s := sdk.NewStage("stage 1")
	s.Enabled = true
	s.PipelineID = pip.ID
	pipeline.InsertStage(api.MustDB(), s)
	j := &sdk.Job{
		Enabled: true,
		Action: sdk.Action{
			Enabled: true,
			Actions: []sdk.Action{
				sdk.NewScriptAction("echo lol"),
			},
		},
	}
	pipeline.InsertJob(api.MustDB(), j, s.ID, &pip)
	s.Jobs = append(s.Jobs, *j)

	pip.Stages = append(pip.Stages, *s)

	w := sdk.Workflow{
		Name:       "test_1",
		ProjectID:  proj.ID,
		ProjectKey: proj.Key,
		Root: &sdk.WorkflowNode{
			Pipeline: pip,
		},
	}

	test.NoError(t, workflow.Insert(api.MustDB(), &w, u))
	w1, err := workflow.Load(api.MustDB(), key, "test_1", u)
	test.NoError(t, err)

	// Init router

	api.InitRouter()
	//Prepare request
	vars := map[string]string{
		"permProjectKey": proj.Key,
		"workflowName":   w1.Name,
	}
	uri := router.GetRoute("POST", api.postWorkflowRunHandler, vars)
	test.NotEmpty(t, uri)

	opts := &postWorkflowRunHandlerOption{}
	req := assets.NewAuthentifiedRequest(t, u, pass, "POST", uri, opts)

	//Do the request
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	wr := &sdk.WorkflowRun{}
	test.NoError(t, json.Unmarshal(rec.Body.Bytes(), wr))
	assert.Equal(t, int64(1), wr.Number)

	if t.Failed() {
		t.FailNow()
	}

	c, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	workflow.Scheduler(c, func() *gorp.DbMap { return db })
	time.Sleep(1 * time.Second)

	return test_runWorkflowCtx{
		user:     u,
		password: pass,
		project:  proj,
		workflow: w1,
		run:      wr,
	}
}

func test_getWorkflowJob(t *testing.T, api *API, router *Router, ctx *test_runWorkflowCtx) {
	uri := router.GetRoute("GET", api.getWorkflowJobQueueHandler, nil)
	test.NotEmpty(t, uri)

	req := assets.NewAuthentifiedRequest(t, ctx.user, ctx.password, "GET", uri, nil)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	jobs := []sdk.WorkflowNodeJobRun{}
	test.NoError(t, json.Unmarshal(rec.Body.Bytes(), &jobs))
	assert.Len(t, jobs, 1)

	if t.Failed() {
		t.FailNow()
	}

	ctx.job = &jobs[0]
}

func test_registerWorker(t *testing.T, api *API, router *Router, ctx *test_runWorkflowCtx) {
	var err error
	//Generate token
	ctx.workerToken, err = token.GenerateToken()
	test.NoError(t, err)
	//Insert token
	test.NoError(t, token.InsertToken(api.MustDB(), ctx.user.Groups[0].ID, ctx.workerToken, sdk.Persistent))
	//Register the worker
	params := &worker.RegistrationForm{
		Name:  sdk.RandomString(10),
		Token: ctx.workerToken,
	}
	ctx.worker, err = worker.RegisterWorker(api.MustDB(), params.Name, params.Token, params.Model, nil, params.BinaryCapabilities)
	test.NoError(t, err)
}

func test_registerHatchery(t *testing.T, api *API, router *Router, ctx *test_runWorkflowCtx) {
	//Generate token
	tk, err := token.GenerateToken()
	test.NoError(t, err)
	//Insert token
	test.NoError(t, token.InsertToken(api.MustDB(), ctx.user.Groups[0].ID, tk, sdk.Persistent))

	ctx.hatchery = &sdk.Hatchery{
		UID:      tk,
		LastBeat: time.Now(),
		Name:     sdk.RandomString(10),
		GroupID:  ctx.user.Groups[0].ID,
	}

	err = hatchery.InsertHatchery(api.MustDB(), ctx.hatchery)
	test.NoError(t, err)
}

func Test_getWorkflowJobQueueHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)
}

func Test_postWorkflowJobRequirementsErrorHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)

	uri := router.GetRoute("POST", api.postWorkflowJobRequirementsErrorHandler, nil)
	test.NotEmpty(t, uri)

	//This will check the needWorker() auth
	req := assets.NewAuthentifiedRequest(t, ctx.user, ctx.password, "POST", uri, "This is a requirement log error")
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 403, rec.Code)

	//Register the worker
	test_registerWorker(t, api, router, &ctx)

	//This call must work
	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, "This is a requirement log error")
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

}
func Test_postTakeWorkflowJobHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)

	takeForm := worker.TakeForm{
		BookedJobID: ctx.job.ID,
		Time:        time.Now(),
	}

	//Prepare request
	vars := map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"id":             fmt.Sprintf("%d", ctx.job.ID),
	}

	//Register the worker
	test_registerWorker(t, api, router, &ctx)

	uri := router.GetRoute("POST", api.postTakeWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	//This will check the needWorker() auth
	req := assets.NewAuthentifiedRequest(t, ctx.user, ctx.password, "POST", uri, takeForm)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 403, rec.Code)

	//This call must work
	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, takeForm)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	run, err := workflow.LoadNodeJobRun(api.MustDB(), ctx.job.ID)
	test.NoError(t, err)
	assert.Equal(t, "Building", run.Status)

}
func Test_postBookWorkflowJobHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)

	//Prepare request
	vars := map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"id":             fmt.Sprintf("%d", ctx.job.ID),
	}

	//Register the hatchery
	test_registerHatchery(t, api, router, &ctx)

	//TakeBook
	uri := router.GetRoute("POST", api.postBookWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	req := assets.NewAuthentifiedRequestFromHatchery(t, ctx.hatchery, "POST", uri, nil)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

}

func Test_postWorkflowJobResultHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)

	//Prepare request
	vars := map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"id":             fmt.Sprintf("%d", ctx.job.ID),
	}

	//Register the worker
	test_registerWorker(t, api, router, &ctx)

	//Take
	uri := router.GetRoute("POST", api.postTakeWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	takeForm := worker.TakeForm{
		BookedJobID: ctx.job.ID,
		Time:        time.Now(),
	}

	req := assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, takeForm)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	vars = map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"permID":         fmt.Sprintf("%d", ctx.job.ID),
	}

	//Send logs
	logs := sdk.Log{
		Val: "This is a log",
	}

	uri = router.GetRoute("POST", api.postWorkflowJobLogsHandler, vars)
	test.NotEmpty(t, uri)

	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, logs)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	now, _ := ptypes.TimestampProto(time.Now())

	//Send result
	res := sdk.Result{
		Duration:   "10",
		Status:     sdk.StatusSuccess.String(),
		RemoteTime: now,
	}

	uri = router.GetRoute("POST", api.postWorkflowJobResultHandler, vars)
	test.NotEmpty(t, uri)

	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, res)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

}

func Test_postWorkflowJobTestsResultsHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)

	//Prepare request
	vars := map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"id":             fmt.Sprintf("%d", ctx.job.ID),
	}

	//Register the worker
	test_registerWorker(t, api, router, &ctx)
	//Register the hatchery
	test_registerHatchery(t, api, router, &ctx)

	//Send spawninfo
	info := []sdk.SpawnInfo{}
	uri := router.GetRoute("POST", api.postSpawnInfosWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	req := assets.NewAuthentifiedRequestFromHatchery(t, ctx.hatchery, "POST", uri, info)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	//spawn
	uri = router.GetRoute("POST", api.postTakeWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	takeForm := worker.TakeForm{
		BookedJobID: ctx.job.ID,
		Time:        time.Now(),
	}

	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, takeForm)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	vars = map[string]string{
		"permID": fmt.Sprintf("%d", ctx.job.ID),
	}

	//Send test
	tests := venom.Tests{
		Total:        2,
		TotalKO:      1,
		TotalOK:      1,
		TotalSkipped: 0,
		TestSuites: []venom.TestSuite{
			{
				Total: 1,
				Name:  "TestSuite1",
				TestCases: []venom.TestCase{
					{
						Name:   "TestCase1",
						Status: "OK",
					},
				},
			},
			{
				Total: 1,
				Name:  "TestSuite2",
				TestCases: []venom.TestCase{
					{
						Name:   "TestCase1",
						Status: "KO",
						Failures: []venom.Failure{
							{
								Value:   "Fail",
								Type:    "Assertion error",
								Message: "Error occured",
							},
						},
					},
				},
			},
		},
	}

	uri = router.GetRoute("POST", api.postWorkflowJobTestsResultsHandler, vars)
	test.NotEmpty(t, uri)

	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, tests)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	step := sdk.StepStatus{
		Status:    sdk.StatusSuccess.String(),
		StepOrder: 0,
	}

	uri = router.GetRoute("POST", api.postWorkflowJobStepStatusHandler, vars)
	test.NotEmpty(t, uri)

	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, step)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	wNodeJobRun, errJ := workflow.LoadNodeJobRun(api.MustDB(), ctx.job.ID)
	test.NoError(t, errJ)
	nodeRun, errN := workflow.LoadNodeRunByID(api.MustDB(), wNodeJobRun.WorkflowNodeRunID)
	test.NoError(t, errN)

	assert.NotNil(t, nodeRun.Tests)
	assert.Equal(t, 2, nodeRun.Tests.Total)
}
func Test_postWorkflowJobVariableHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)

	//Prepare request
	vars := map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"id":             fmt.Sprintf("%d", ctx.job.ID),
	}

	//Register the worker
	test_registerWorker(t, api, router, &ctx)

	//Take
	uri := router.GetRoute("POST", api.postTakeWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	takeForm := worker.TakeForm{
		BookedJobID: ctx.job.ID,
		Time:        time.Now(),
	}

	req := assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, takeForm)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	vars = map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"permID":         fmt.Sprintf("%d", ctx.job.ID),
	}

	//Send result
	v := sdk.Variable{
		Name:  "var",
		Value: "value",
	}

	uri = router.GetRoute("POST", api.postWorkflowJobVariableHandler, vars)
	test.NotEmpty(t, uri)

	req = assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, v)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

}
func Test_postWorkflowJobArtifactHandler(t *testing.T) {
	api, db, router := newTestAPI(t)
	ctx := test_runWorkflow(t, api, router, db)
	test_getWorkflowJob(t, api, router, &ctx)
	assert.NotNil(t, ctx.job)

	// Init store
	cfg := objectstore.Config{
		Kind: objectstore.Filesystem,
		Options: objectstore.ConfigOptions{
			Filesystem: objectstore.ConfigOptionsFilesystem{
				Basedir: path.Join(os.TempDir(), "store"),
			},
		},
	}

	errO := objectstore.Initialize(context.Background(), cfg)
	test.NoError(t, errO)

	//Prepare request
	vars := map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"id":             fmt.Sprintf("%d", ctx.job.ID),
	}

	//Register the worker
	test_registerWorker(t, api, router, &ctx)

	//Take
	uri := router.GetRoute("POST", api.postTakeWorkflowJobHandler, vars)
	test.NotEmpty(t, uri)

	takeForm := worker.TakeForm{
		BookedJobID: ctx.job.ID,
		Time:        time.Now(),
	}

	req := assets.NewAuthentifiedRequestFromWorker(t, ctx.worker, "POST", uri, takeForm)
	rec := httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	vars = map[string]string{
		"tag":    "latest",
		"permID": fmt.Sprintf("%d", ctx.job.ID),
	}

	uri = router.GetRoute("POST", api.postWorkflowJobArtifactHandler, vars)
	test.NotEmpty(t, uri)

	myartifact, errF := os.Create(path.Join(os.TempDir(), "myartifact"))
	defer os.RemoveAll(path.Join(os.TempDir(), "myartifact"))
	test.NoError(t, errF)
	_, errW := myartifact.Write([]byte("Hi, I am foo"))
	test.NoError(t, errW)

	errClose := myartifact.Close()
	test.NoError(t, errClose)

	params := map[string]string{}
	params["size"] = "12"
	params["perm"] = "7"
	params["md5sum"] = "123"
	req = assets.NewAuthentifiedMultipartRequestFromWorker(t, ctx.worker, "POST", uri, "/tmp/myartifact", "myartifact", params)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	wNodeJobRun, errJ := workflow.LoadNodeJobRun(api.MustDB(), ctx.job.ID)
	test.NoError(t, errJ)

	updatedNodeRun, errN2 := workflow.LoadNodeRunByID(api.MustDB(), wNodeJobRun.WorkflowNodeRunID)
	test.NoError(t, errN2)

	assert.NotNil(t, updatedNodeRun.Artifacts)
	assert.Equal(t, 1, len(updatedNodeRun.Artifacts))

	//Prepare request
	vars = map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"number":         fmt.Sprintf("%d", updatedNodeRun.Number),
		"id":             fmt.Sprintf("%d", wNodeJobRun.WorkflowNodeRunID),
	}
	uri = router.GetRoute("GET", api.getWorkflowNodeRunArtifactsHandler, vars)
	test.NotEmpty(t, uri)
	req = assets.NewAuthentifiedRequest(t, ctx.user, ctx.password, "GET", uri, nil)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	var arts []sdk.WorkflowNodeRunArtifact
	test.NoError(t, json.Unmarshal(rec.Body.Bytes(), &arts))
	assert.Equal(t, 1, len(arts))
	assert.Equal(t, "myartifact", arts[0].Name)

	// Download artifact
	//Prepare request
	vars = map[string]string{
		"permProjectKey": ctx.project.Key,
		"workflowName":   ctx.workflow.Name,
		"artifactId":     fmt.Sprintf("%d", arts[0].ID),
	}
	uri = router.GetRoute("GET", api.getDownloadArtifactHandler, vars)
	test.NotEmpty(t, uri)
	req = assets.NewAuthentifiedRequest(t, ctx.user, ctx.password, "GET", uri, nil)
	rec = httptest.NewRecorder()
	router.Mux.ServeHTTP(rec, req)

	resp := rec.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "Hi, I am foo", string(body))
}
func Test_getWorkflowJobArtifactsHandler(t *testing.T) {
	//api, db, router := newTestAPI(t)
	//ctx := runWorkflow(t, db, "Test_postWorkflowJobRequirementsErrorHandler")
}
func Test_getDownloadArtifactHandler(t *testing.T) {
	//api, db, router := newTestAPI(t)
	//ctx := runWorkflow(t, db, "Test_postWorkflowJobRequirementsErrorHandler")
}
