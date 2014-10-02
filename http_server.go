package afind

import (
	"encoding/json"
	"net"

	"github.com/gocraft/web"
)

type Context struct {
	repos    KeyValueStorer
	indexer  Indexer
	searcher Searcher
}

func startHttpServer(addr string, be *Service) (*web.Router, error) {
	if addr == "" {
		panic("no bind address passed")
	}
	app := web.New(be)
	app.Get("/repos/:key", GetRepo)
	app.Post("/repos/:key", PostRepo)
	app.Get("/repos", GetAllRepos)

	l, err := net.Listen("tcp", addr)
	if err == nil {
		for {
			go func() {
				l.Accept()
			}()
		}
	}

	return nil, err
}

func httpError(t, msg, remedy string) *map[string]string {
	n := make(map[string]string)
	n["error"] = t
	n["message"] = msg
	n["remedy"] = remedy
	return &n
}

// Request Handlers

func GetRepo(c *Context, rw web.ResponseWriter, req *web.Request) {
	key := req.PathParams["key"]
	enc := json.NewEncoder(rw)
	if repo := c.repos.Get(key); repo != nil {
		rw.WriteHeader(200)
		enc.Encode(repo)
	} else {
		rw.WriteHeader(404)
		// error
	}
}

func GetAllRepos(c *Context, rw web.ResponseWriter, req *web.Request) {
	repos := make(map[string]*Repo)
	enc := json.NewEncoder(rw)
	c.repos.ForEach(func(key string, value interface{}) bool {
		if value != nil {
			repos[key] = value.(*Repo)
		}
		return true
	})
	rw.WriteHeader(200)
	enc.Encode(repos)
}

func PostRepo(c *Context, rw web.ResponseWriter, req *web.Request) {
	key := req.PathParams["key"]
	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)

	// Parse the request
	var ir IndexRequest
	if err := dec.Decode(&ir); err != nil {
		rw.WriteHeader(403)
		enc.Encode(
			httpError("invalid_request", "bad request format",
				"Fix the request format"))
		return
	}

	// Generate the index
	indexResponse, err := c.indexer.Index(ir)

	if serr := c.repos.Set(key, indexResponse); serr == nil {
		if err == nil {
			rw.WriteHeader(200)
			enc.Encode(indexResponse)
		} else {
			rw.WriteHeader(500)
			enc.Encode(
				httpError("indexing_error", err.Error(), ""))
		}
	} else {
		rw.WriteHeader(501)
		enc.Encode(
			httpError("store_set_error", "repos", "Try again"))
	}
}
