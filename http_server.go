package afind

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Web service definition

type webService struct {
	*Service
	router *httprouter.Router
}

func (ws *webService) start() (err error) {
	if ws.router == nil {
		panic("HTTP URL router not yet setup")
	}

	if ws.config.HttpBind != "" {
		go func() {
			err = http.ListenAndServe(ws.config.HttpBind, ws.router)
		}()
		if err == nil {
			log.Info("Started HTTP server at %s", ws.config.HttpBind)
		}
	}
	return
}

func (ws *webService) setupHandlers() {
	ws.router.GET("/repo/:key", ws.GetRepo)
	ws.router.GET("/repo", ws.GetAllRepos)
	ws.router.POST("/repo", ws.PostRepo)
	ws.router.POST("/search", ws.Search)
}

func newWebService(service *Service) *webService {
	ws := &webService{service, httprouter.New()}
	ws.setupHandlers()
	return ws
}

// helper

func httpError(t, msg, remedy string) *map[string]string {
	n := make(map[string]string)
	n["error"] = t
	n["message"] = msg
	n["remedy"] = remedy
	return &n
}

// Webservice Request Handlers

func (ws *webService) GetRepo(
	rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	key := ps.ByName("key")
	enc := json.NewEncoder(rw)
	if repo := ws.repos.Get(key); repo != nil {
		rw.WriteHeader(200)
		_ = enc.Encode(repo)
	} else {
		rw.WriteHeader(404)
	}
}

func (ws *webService) GetAllRepos(
	rw http.ResponseWriter, req *http.Request, _ httprouter.Params) {

	repos := make(map[string]*Repo)
	enc := json.NewEncoder(rw)
	ws.repos.ForEach(func(key string, value interface{}) bool {
		if value != nil {
			repos[key] = value.(*Repo)
		}
		return true
	})
	rw.WriteHeader(200)
	_ = enc.Encode(repos)
}

func (ws *webService) PostRepo(
	rw http.ResponseWriter, req *http.Request, _ httprouter.Params) {

	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)

	// Parse the request
	var ir IndexRequest
	if err := dec.Decode(&ir); err != nil {
		rw.WriteHeader(403)
		_ = enc.Encode(
			httpError("invalid_request", "badly formatted JSON request",
				"Provide a valid JSON IndexRequest"))
		return
	}

	// Generate the index
	indexResponse, err := ws.Indexer.Index(ir)
	if err == nil {
		rw.WriteHeader(200)
		_ = enc.Encode(indexResponse)
	} else {
		rw.WriteHeader(500)
		_ = enc.Encode(
			httpError("indexing_error", err.Error(), ""))
	}
}

func (ws *webService) Search(
	rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	_ = ps.ByName("key")
	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)

	sr := SearchRequest{}
	if err := dec.Decode(&sr); err != nil {
		rw.WriteHeader(403)
		_ = enc.Encode(
			httpError("invalid_request", "bad request format",
				"Fix the request format"))
		return
	}

	sresp, err := ws.Searcher.Search(sr)
	if err == nil {
		if err == nil {
			rw.WriteHeader(200)
			_ = enc.Encode(sresp)
		} else {
			rw.WriteHeader(500)
			_ = enc.Encode(
				httpError("search_error", err.Error(), ""))
		}
	}
}
