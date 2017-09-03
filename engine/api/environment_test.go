package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loopfz/gadgeto/iffy"
	"github.com/stretchr/testify/assert"

	"github.com/ovh/cds/engine/api/environment"
	"github.com/ovh/cds/engine/api/test"
	"github.com/ovh/cds/engine/api/test/assets"
	"github.com/ovh/cds/sdk"
)

func TestAddEnvironmentHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//1. Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//2. Create project
	proj := assets.InsertTestProject(t, db, sdk.RandomString(10), sdk.RandomString(10), nil)
	test.NotNil(t, proj)

	//3. Prepare the request
	addEnvRequest := sdk.Environment{
		Name: "Production",
	}
	jsonBody, _ := json.Marshal(addEnvRequest)
	body := bytes.NewBuffer(jsonBody)

	vars := map[string]string{
		"permProjectKey": proj.Key,
	}

	uri := router.GetRoute("POST", api.addEnvironmentHandler, vars)
	test.NotEmpty(t, uri)

	req, err := http.NewRequest("POST", uri, body)
	assets.AuthentifyRequest(t, req, u, pass)

	//4. Do the request
	w := httptest.NewRecorder()
	router.Mux.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	res, _ := ioutil.ReadAll(w.Body)
	projectResult := &sdk.Project{}
	json.Unmarshal(res, &projectResult)
	assert.Equal(t, len(projectResult.Environments), 1)

	env, err := environment.LoadEnvironmentByName(api.MustDB(), proj.Key, "Production")
	if err != nil {
		t.Fail()
		return
	}
	assert.Equal(t, env.Name, "Production")
}

func TestUpdateEnvironmentHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//1. Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//2. Create project
	proj := assets.InsertTestProject(t, db, sdk.RandomString(10), sdk.RandomString(10), nil)
	test.NotNil(t, proj)

	//3. Create env
	env := sdk.Environment{
		ProjectID: proj.ID,
		Name:      "Preproduction",
	}
	if err := environment.InsertEnvironment(api.MustDB(), &env); err != nil {
		t.Fail()
		return
	}

	//4. Prepare the request
	env.Name = "Production"

	jsonBody, _ := json.Marshal(env)
	body := bytes.NewBuffer(jsonBody)

	vars := map[string]string{
		"key": proj.Key,
		"permEnvironmentName": "Preproduction",
	}

	uri := router.GetRoute("PUT", api.updateEnvironmentHandler, vars)
	test.NotEmpty(t, uri)

	req, err := http.NewRequest("PUT", uri, body)
	assets.AuthentifyRequest(t, req, u, pass)

	//5. Do the request
	w := httptest.NewRecorder()
	router.Mux.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	res, _ := ioutil.ReadAll(w.Body)
	t.Logf(string(res))
	projectResult := &sdk.Project{}
	json.Unmarshal(res, &projectResult)
	assert.Equal(t, len(projectResult.Environments), 1)

	envDb, err := environment.LoadEnvironmentByName(api.MustDB(), proj.Key, "Production")
	if err != nil {
		t.Fail()
		return
	}
	assert.Equal(t, envDb.Name, "Production")
}

func TestDeleteEnvironmentHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//1. Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//2. Create project
	proj := assets.InsertTestProject(t, db, sdk.RandomString(10), sdk.RandomString(10), nil)
	test.NotNil(t, proj)

	//3. Create env
	env := sdk.Environment{
		ProjectID: proj.ID,
		Name:      "Preproduction",
	}
	if err := environment.InsertEnvironment(api.MustDB(), &env); err != nil {
		t.Fail()
		return
	}

	vars := map[string]string{
		"key": proj.Key,
		"permEnvironmentName": "Preproduction",
	}

	uri := router.GetRoute("DELETE", api.deleteEnvironmentHandler, vars)
	test.NotEmpty(t, uri)

	req, err := http.NewRequest("DELETE", uri, nil)
	assets.AuthentifyRequest(t, req, u, pass)

	//4. Do the request
	w := httptest.NewRecorder()
	router.Mux.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	res, _ := ioutil.ReadAll(w.Body)
	projectResult := &sdk.Project{}
	json.Unmarshal(res, &projectResult)
	assert.Equal(t, len(projectResult.Environments), 0)

	_, err = environment.LoadEnvironmentByName(api.MustDB(), proj.Key, "Preproduction")
	if err == sdk.ErrNoEnvironment {
		return
	}
	t.Fail()
}

func TestGetEnvironmentsHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//1. Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//2. Create project
	proj := assets.InsertTestProject(t, db, sdk.RandomString(10), sdk.RandomString(10), nil)
	test.NotNil(t, proj)

	//3. Create env
	env := sdk.Environment{
		ProjectID: proj.ID,
		Name:      "Preproduction",
	}
	if err := environment.InsertEnvironment(api.MustDB(), &env); err != nil {
		t.Fail()
		return
	}

	vars := map[string]string{
		"permProjectKey": proj.Key,
	}

	uri := router.GetRoute("GET", api.getEnvironmentsHandler, vars)
	test.NotEmpty(t, uri)

	req, _ := http.NewRequest("GET", uri, nil)
	assets.AuthentifyRequest(t, req, u, pass)

	//4. Do the request
	w := httptest.NewRecorder()
	router.Mux.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	res, _ := ioutil.ReadAll(w.Body)
	envsResults := []sdk.Environment{}
	json.Unmarshal(res, &envsResults)
	assert.Equal(t, len(envsResults), 1)
}

func TestGetEnvironmentHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//1. Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//2. Create project
	proj := assets.InsertTestProject(t, db, sdk.RandomString(10), sdk.RandomString(10), nil)
	test.NotNil(t, proj)

	//3. Create env
	env := sdk.Environment{
		ProjectID: proj.ID,
		Name:      "Preproduction",
	}
	if err := environment.InsertEnvironment(api.MustDB(), &env); err != nil {
		t.Fail()
		return
	}

	vars := map[string]string{
		"key": proj.Key,
		"permEnvironmentName": env.Name,
	}

	uri := router.GetRoute("GET", api.getEnvironmentHandler, vars)
	test.NotEmpty(t, uri)

	req, _ := http.NewRequest("GET", uri, nil)
	assets.AuthentifyRequest(t, req, u, pass)

	//4. Do the request
	w := httptest.NewRecorder()
	router.Mux.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	res, _ := ioutil.ReadAll(w.Body)
	envResults := sdk.Environment{}
	json.Unmarshal(res, &envResults)
	assert.Equal(t, envResults.Name, "Preproduction")
}

func Test_cloneEnvironmentHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//1. Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//2. Create project
	proj := assets.InsertTestProject(t, db, sdk.RandomString(10), sdk.RandomString(10), nil)
	test.NotNil(t, proj)

	//3. Create env
	env := sdk.Environment{
		ProjectID: proj.ID,
		Name:      "Preproduction",
	}
	test.NoError(t, environment.InsertEnvironment(api.MustDB(), &env))

	v := &sdk.Variable{
		Name:  "var1",
		Type:  sdk.StringVariable,
		Value: "val1",
	}
	test.NoError(t, environment.InsertVariable(api.MustDB(), env.ID, v, u))

	vars := map[string]string{
		"key": proj.Key,
		"permEnvironmentName": env.Name,
	}

	envPost := sdk.Environment{
		Name: "Production2",
	}

	uri := router.GetRoute("POST", api.cloneEnvironmentHandler, vars)
	tester := iffy.NewTester(t, router.Mux)
	headers := assets.AuthHeaders(t, u, pass)
	tester.AddCall("Test_cloneEnvironmentHandler", "POST", uri, &envPost).Headers(headers).Checkers(iffy.ExpectStatus(200), iffy.DumpResponse(t))
	tester.Run()
}
