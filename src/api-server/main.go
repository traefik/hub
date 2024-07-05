package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-chi/chi/v5"
	"github.com/go-openapi/loads"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type api struct {
	openAPISpec *loads.Document
	data        map[string]map[string]json.RawMessage
	latency     time.Duration
	errorRate   int
}

type apiError struct {
	Message string `json:"error"`
}

func main() {
	openapispec := flag.String("openapi", "", "openapispec")
	datafile := flag.String("data", "", "file to put data in")
	latency := flag.Duration("latency", 0, "latency to add")
	errorrate := flag.Int("errorrate", 0, "latency to add")
	flag.Parse()

	a := api{}

	if openapispec != nil && *openapispec != "" {
		err := a.loadOpenAPISpec(*openapispec)
		if err != nil {
			log.Fatal(err)
		}
	}

	if datafile != nil && *datafile != "" {
		err := a.loadData(*datafile)
		if err != nil {
			log.Fatal(err)
		}
	}

	if latency != nil && *latency > 0 {
		a.latency = *latency
	}

	if errorrate != nil && *errorrate > 0 {
		a.errorRate = *errorrate
	}

	server := &http.Server{Addr: ":3000", Handler: a.getRouter()}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func (a *api) loadOpenAPISpec(path string) error {
	openAPISpec, err := loads.Spec(path)
	if err != nil {
		return err
	}

	a.openAPISpec = openAPISpec
	return nil
}

func (a *api) loadData(path string) error {
	rawData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	data := map[string]map[string]json.RawMessage{}
	err = json.Unmarshal(rawData, &data)
	if err != nil {
		return err
	}

	for _, obj := range data {
		for _, v := range obj {
			var doc map[string]interface{}
			err := json.Unmarshal(v, &doc)
			if err != nil {
				return err
			}
		}
	}

	a.data = data
	return nil
}

func (a *api) getRouter() http.Handler {
	router := chi.NewRouter()

	router.With(a.errorRateMiddleWare()).With(a.latencyMiddleWare()).Get("/openapi.y{[a]?}ml", a.handleOpenAPISpec)
	router.Get("/{objType}", a.handleGetAll)
	router.Get("/{objType}/{objId}", a.handleGet)
	router.Post("/{objType}", a.handlePost)
	router.Delete("/{objType}/{objId}", a.handleDelete)
	router.Put("/{objType}/{objId}", a.handlePut)
	router.Patch("/{objType}/{objId}", a.handlePatch)

	return router
}

func (a *api) latencyMiddleWare() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(a.latency)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (a *api) errorRateMiddleWare() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if a.errorRate > 0 {
				if rand.IntN(100) < a.errorRate {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (a *api) handleOpenAPISpec(rw http.ResponseWriter, req *http.Request) {
	var jsonObj interface{}
	err := yaml.Unmarshal(a.openAPISpec.Raw(), &jsonObj)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	out, err := yaml.Marshal(jsonObj)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	_, _ = rw.Write(out)
}

func (a *api) handleGetAll(rw http.ResponseWriter, req *http.Request) {
	objType := chi.URLParam(req, "objType")

	if val, ok := a.data[objType]; ok {
		var allDocs []map[string]interface{}
		for k, v := range val {
			var doc map[string]interface{}
			err := json.Unmarshal(v, &doc)
			if err != nil {
				JSONError(rw, http.StatusInternalServerError, err.Error())
				return
			}
			doc["id"] = k
			allDocs = append(allDocs, doc)
		}
		body, err := json.Marshal(allDocs)
		if err != nil {
			JSONError(rw, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = rw.Write(body)
		if err != nil {
			JSONError(rw, http.StatusInternalServerError, err.Error())
			return
		}

		return
	}

	rw.WriteHeader(http.StatusNotFound)
}

func (a *api) handleGet(rw http.ResponseWriter, req *http.Request) {
	objType := chi.URLParam(req, "objType")
	objId := chi.URLParam(req, "objId")

	objRaw, err := a.getObject(objType, objId)
	if err != nil {
		JSONError(rw, http.StatusNotFound, err.Error())
		return
	}

	obj, err := json.Marshal(objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = rw.Write(obj)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}
}

func (a *api) handlePost(rw http.ResponseWriter, req *http.Request) {
	objType := chi.URLParam(req, "objType")

	var objRaw map[string]interface{}

	err := json.NewDecoder(req.Body).Decode(&objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	objId := uuid.New().String()
	objRaw["id"] = objId

	output, err := json.Marshal(objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	delete(objRaw, "id")

	data, err := json.Marshal(objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	a.data[objType][objId] = data

	rw.WriteHeader(http.StatusCreated)
	_, err = rw.Write(output)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}
}

func (a *api) handleDelete(rw http.ResponseWriter, req *http.Request) {
	objType := chi.URLParam(req, "objType")
	objId := chi.URLParam(req, "objId")

	rw.WriteHeader(http.StatusNoContent)

	_, err := a.getObject(objType, objId)
	if err != nil {
		return
	}

	delete(a.data[objType], objId)
}

func (a *api) handlePut(rw http.ResponseWriter, req *http.Request) {
	objType := chi.URLParam(req, "objType")
	objId := chi.URLParam(req, "objId")

	_, err := a.getObject(objType, objId)
	if err != nil {
		JSONError(rw, http.StatusNotFound, err.Error())
		return
	}

	var objRaw map[string]interface{}

	err = json.NewDecoder(req.Body).Decode(&objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	delete(objRaw, "id")

	data, err := json.Marshal(objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	a.data[objType][objId] = data
}

func (a *api) handlePatch(rw http.ResponseWriter, req *http.Request) {
	objType := chi.URLParam(req, "objType")
	objId := chi.URLParam(req, "objId")

	origObj, err := a.getObject(objType, objId)
	if err != nil {
		JSONError(rw, http.StatusNotFound, err.Error())
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	patch, err := jsonpatch.DecodePatch(body)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	modifiedObj, err := patch.Apply(origObj)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	var objRaw map[string]interface{}
	err = json.Unmarshal(modifiedObj, &objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	delete(objRaw, "id")

	data, err := json.Marshal(objRaw)
	if err != nil {
		JSONError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	a.data[objType][objId] = data

	rw.WriteHeader(http.StatusNoContent)
}

func (a *api) getObject(objType, objId string) (json.RawMessage, error) {
	if objs, ok := a.data[objType]; ok {
		obj, ok := objs[objId]
		if ok {
			return obj, nil
		}
	}

	return nil, fmt.Errorf("%s/%s not found", objType, objId)
}

func JSONError(rw http.ResponseWriter, code int, errMsg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.WriteHeader(code)

	msg := apiError{
		Message: errMsg,
	}

	content, err := json.Marshal(msg)
	if err != nil {
		_, _ = rw.Write([]byte(`{"error": "Internal Server Error"}`))
		return
	}

	_, _ = rw.Write(content)
}
