package api

import (
	"testing"

	"github.com/loopfz/gadgeto/iffy"

	"github.com/magiconair/properties/assert"
	"github.com/ovh/cds/engine/api/application"
	"github.com/ovh/cds/engine/api/keys"
	"github.com/ovh/cds/engine/api/test/assets"
	"github.com/ovh/cds/sdk"
)

func Test_getKeysInApplicationHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//Create a fancy httptester
	tester := iffy.NewTester(t, router.Mux)

	//Insert Project
	pkey := sdk.RandomString(10)
	proj := assets.InsertTestProject(t, db, pkey, pkey, u)

	//Insert Application
	app := &sdk.Application{
		Name: sdk.RandomString(10),
	}
	if err := application.Insert(api.MustDB(), proj, app, u); err != nil {
		t.Fatal(err)
	}

	k := &sdk.ApplicationKey{
		Key: sdk.Key{
			Name: "mykey",
			Type: "pgp",
		},
		ApplicationID: app.ID,
	}

	kid, pub, priv, err := keys.GeneratePGPKeyPair(k.Name)
	if err != nil {
		t.Fatal(err)
	}
	k.Public = pub
	k.Private = priv
	k.KeyID = kid

	if err := application.InsertKey(api.MustDB(), k); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{
		"key": proj.Key,
		"permApplicationName": app.Name,
		"name":                k.Name,
	}

	route := router.GetRoute("GET", api.getKeysInApplicationHandler, vars)
	headers := assets.AuthHeaders(t, u, pass)

	var keys []sdk.ApplicationKey
	tester.AddCall("Test_getKeysInApplicationHandler", "GET", route, nil).Headers(headers).Checkers(iffy.ExpectStatus(200), iffy.ExpectListLength(1), iffy.DumpResponse(t), iffy.UnmarshalResponse(&keys))
	tester.Run()
}

func Test_deleteKeyInApplicationHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//Create a fancy httptester
	tester := iffy.NewTester(t, router.Mux)

	//Insert Project
	pkey := sdk.RandomString(10)
	proj := assets.InsertTestProject(t, db, pkey, pkey, u)

	//Insert Application
	app := &sdk.Application{
		Name: sdk.RandomString(10),
	}
	if err := application.Insert(api.MustDB(), proj, app, u); err != nil {
		t.Fatal(err)
	}

	k := &sdk.ApplicationKey{
		Key: sdk.Key{
			Name:    "mykey",
			Type:    "pgp",
			Public:  "pub",
			Private: "priv",
		},
		ApplicationID: app.ID,
	}

	if err := application.InsertKey(api.MustDB(), k); err != nil {
		t.Fatal(err)
	}

	vars := map[string]string{
		"key": proj.Key,
		"permApplicationName": app.Name,
		"name":                k.Name,
	}

	route := router.GetRoute("DELETE", api.deleteKeyInApplicationHandler, vars)
	headers := assets.AuthHeaders(t, u, pass)

	var keys []sdk.ApplicationKey
	tester.AddCall("Test_deleteKeyInApplicationHandler", "DELETE", route, nil).Headers(headers).Checkers(iffy.ExpectStatus(200), iffy.ExpectListLength(0), iffy.DumpResponse(t), iffy.UnmarshalResponse(&keys))
	tester.Run()
}

func Test_addKeyInApplicationHandler(t *testing.T) {
	api, db, router := newTestAPI(t)

	api.InitRouter()

	//Create admin user
	u, pass := assets.InsertAdminUser(api.MustDB())

	//Create a fancy httptester
	tester := iffy.NewTester(t, router.Mux)

	//Insert Project
	pkey := sdk.RandomString(10)
	proj := assets.InsertTestProject(t, db, pkey, pkey, u)

	//Insert Application
	app := &sdk.Application{
		Name: sdk.RandomString(10),
	}
	if err := application.Insert(api.MustDB(), proj, app, u); err != nil {
		t.Fatal(err)
	}

	k := &sdk.ApplicationKey{
		Key: sdk.Key{
			Name: "mykey",
			Type: "pgp",
		},
	}

	vars := map[string]string{
		"key": proj.Key,
		"permApplicationName": app.Name,
	}

	route := router.GetRoute("POST", api.addKeyInApplicationHandler, vars)
	headers := assets.AuthHeaders(t, u, pass)

	var key sdk.ApplicationKey
	tester.AddCall("Test_addKeyInApplicationHandler", "POST", route, k).Headers(headers).Checkers(iffy.ExpectStatus(200), iffy.UnmarshalResponse(&key))
	tester.Run()

	assert.Equal(t, app.ID, key.ApplicationID)
}
