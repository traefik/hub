package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadOpenAPISpec(t *testing.T) {
	a := api{}
	err := a.loadOpenAPISpec("fixtures/openapi.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, a.openAPISpec)
}

func Test_loadOpenAPISpec_nonExistingFile(t *testing.T) {
	a := api{}
	err := a.loadOpenAPISpec("fixtures/openapi.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, a.openAPISpec)
}

func Test_loadOpenAPISpec_invalidSpecs(t *testing.T) {
	a := api{}
	file, err := os.CreateTemp(t.TempDir(), "")
	t.Cleanup(func() {
		_ = os.Remove(file.Name())
	})
	require.NoError(t, err)

	_, err = file.Write([]byte(`test`))
	require.NoError(t, err)

	err = a.loadOpenAPISpec(file.Name())
	assert.Error(t, err)
	assert.Nil(t, a.openAPISpec)
}

func Test_loadData(t *testing.T) {
	a := api{}
	err := a.loadData("fixtures/data.json")
	assert.NoError(t, err)
	assert.NotNil(t, a.data)
}

func Test_loadData_invalidJSON(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "")
	t.Cleanup(func() {
		_ = os.Remove(file.Name())
	})
	require.NoError(t, err)

	a := api{}
	_, err = file.Write([]byte(`test`))
	require.NoError(t, err)

	err = a.loadData(file.Name())
	assert.Error(t, err)
	assert.Nil(t, a.data)
}

func Test_loadData_invalidData(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "")
	t.Cleanup(func() {
		_ = os.Remove(file.Name())
	})
	require.NoError(t, err)

	a := api{}
	_, err = file.Write([]byte(`{"weather": []}`))
	require.NoError(t, err)

	err = a.loadData(file.Name())
	assert.Error(t, err)
	assert.Nil(t, a.data)
}

func Test_loadData_invalidDocuments(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "")
	t.Cleanup(func() {
		_ = os.Remove(file.Name())
	})
	require.NoError(t, err)

	a := api{}
	_, err = file.Write([]byte(`{"weather": {"id": []}}`))
	require.NoError(t, err)

	err = a.loadData(file.Name())
	assert.Error(t, err)
	assert.Nil(t, a.data)
}

func Test_loadData_nonExistingFile(t *testing.T) {
	a := api{}

	err := a.loadData("non-existing-file")
	assert.Error(t, err)
	assert.Nil(t, a.data)
}

func Test_handleOpenAPISpec(t *testing.T) {
	a := api{}
	err := a.loadOpenAPISpec("fixtures/openapi.yaml")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/openapi.yaml", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	require.NoError(t, err)
	assert.Contains(t, string(body), "openapi: 3.0.0")
}

func Test_handleGetAll(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/weather", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	require.NoError(t, err)

	var docs []map[string]string
	err = json.Unmarshal(body, &docs)
	require.NoError(t, err)
	assert.ElementsMatch(t, docs, []map[string]string{
		{
			"id":      "0",
			"city":    "GopherCity",
			"weather": "Moderate rain",
		},
		{
			"id":      "1",
			"city":    "City of Gophers",
			"weather": "Sunny",
		},
		{
			"id":      "2",
			"city":    "GopherRocks",
			"weather": "Cloudy",
		},
	})
}

func Test_handleGetAll_unknownType(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/obj", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func Test_handleGet(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/weather/0", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	require.NoError(t, err)

	var doc map[string]string
	err = json.Unmarshal(body, &doc)
	require.NoError(t, err)
	assert.Equal(t, doc, map[string]string{
		"city":    "GopherCity",
		"weather": "Moderate rain",
	})
}

func Test_handleGet_unknown(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/weather/4", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func Test_handlePost(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/weather", bytes.NewBuffer([]byte(`{"data": "test"}`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	require.NoError(t, err)

	var ret struct {
		ID   string `json:"id"`
		Data string `json:"data"`
	}

	err = json.Unmarshal(body, &ret)
	require.NoError(t, err)
	assert.NoError(t, uuid.Validate(ret.ID))
	assert.Equal(t, ret.Data, "test")
}

func Test_handlePost_invalidJSON(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/weather", bytes.NewBuffer([]byte(`{"data`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func Test_handleDelete(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/weather/0", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func Test_handlePut(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/weather/0", bytes.NewBuffer([]byte(`{"data": "test"}`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, a.data["weather"]["0"], json.RawMessage(`{"data":"test"}`))
}

func Test_handlePut_invalidJSON(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/weather/0", bytes.NewBuffer([]byte(`{"data": "test"`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func Test_handlePatch(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/weather/0", bytes.NewBuffer([]byte(`[{"op": "add", "path": "/country", "value": "France"},{"op": "replace", "path": "/city", "value": "Lyon"}]`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, a.data["weather"]["0"], json.RawMessage(`{"city":"Lyon","country":"France","weather":"Moderate rain"}`))
}

func Test_handlePatch_invalidPatch(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/weather/0", bytes.NewBuffer([]byte(`[{"data": "test"}]`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func Test_handlePatch_invalidJSON(t *testing.T) {
	a := api{}

	err := a.loadData("fixtures/data.json")
	require.NoError(t, err)

	srv := httptest.NewServer(a.getRouter())

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/weather/0", bytes.NewBuffer([]byte(`[{"data": "test"]`)))
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
