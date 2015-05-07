package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type webServer struct {
	*baseServer

	svr *http.Server
	rtr *httprouter.Router
}

func NewWebServer(b *baseServer) *webServer {
	rtr := httprouter.New()
	return &webServer{b, &http.Server{Handler: rtr}, rtr}
}

func (s *webServer) Register() {
	if s.repos == nil || s.indexer == nil || s.searcher == nil {
		panic("server must be setup prior to Register being called")
	}

	svrRepos := &reposServer{s.repos}
	svrIndex := &indexServer{&s.config, s.repos, s.indexer}
	svrSearch := &searchServer{&s.config, s.repos, s.searcher}
	svrFind := &findServer{&s.config, s.repos, s.finder}

	s.rtr.GET("/api/v1/repo", svrRepos.webGet)
	s.rtr.GET("/api/v1/repo/:key", svrRepos.webGet)
	s.rtr.DELETE("/api/v1/repo/:key", svrRepos.webDelete)

	s.rtr.POST("/api/v1/index", svrIndex.webIndex)
	s.rtr.POST("/api/v1/search", svrSearch.webSearch)
	s.rtr.POST("/api/v1/find", svrFind.webFind)
}

func (s *webServer) HttpServer(addr string) *http.Server {
	return &http.Server{Addr: addr, Handler: s.rtr}
}

func setJson(rw http.ResponseWriter) {
	rw.Header().Add("Content-Type", "application/json; charset=utf-8")
}
