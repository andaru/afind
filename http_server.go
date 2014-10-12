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

func (ws *webService) start() error {
	if ws.router == nil {
		panic("HTTP URL router not yet setup")
	}

	var err error
	addr := config.HttpBind
	if addr != "" {
		go func() {
			err = http.ListenAndServe(addr, ws.router)
		}()
		if err == nil {
			log.Info("Started HTTP server at %s", addr)
		}
	}
	return err
}

func (ws *webService) setupHandlers() {
	ws.router.GET("/repo/:key", ws.GetRepo)
	ws.router.GET("/repos", ws.GetAllRepos)
	ws.router.POST("/repo/:key", ws.PostRepo)
	// ws.router.POST("/search", )
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

// Request Handlers

func (ws *webService) GetRepo(
	rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	key := ps.ByName("key")
	enc := json.NewEncoder(rw)
	if repo := ws.repos.Get(key); repo != nil {
		rw.WriteHeader(200)
		_ = enc.Encode(repo)
	} else {
		rw.WriteHeader(404)
		// error
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
	rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	key := ps.ByName("key")

	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)

	// Parse the request
	var ir IndexRequest
	if err := dec.Decode(&ir); err != nil {
		rw.WriteHeader(403)
		_ = enc.Encode(
			httpError("invalid_request", "bad request format",
				"Fix the request format"))
		return
	}

	// Generate the index
	indexResponse, err := ws.Indexer.Index(ir)

	if serr := ws.repos.Set(key, indexResponse); serr == nil {
		if err == nil {
			rw.WriteHeader(200)
			_ = enc.Encode(indexResponse)
		} else {
			rw.WriteHeader(500)
			_ = enc.Encode(
				httpError("indexing_error", err.Error(), ""))
		}
	} else {
		rw.WriteHeader(501)
		_ = enc.Encode(
			httpError("store_set_error", "repos", "Try again"))
	}
}
