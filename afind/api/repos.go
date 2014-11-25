package api

import (
	"encoding/json"
	"net/http"
	"net/rpc"

	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/errs"
	"github.com/julienschmidt/httprouter"
)

type ReposClient struct {
	endpoint string
	client   *rpc.Client
}

func NewReposClient(client *rpc.Client) *ReposClient {
	return &ReposClient{endpoint: EPRepos, client: client}
}

func (r *ReposClient) Get(key string) (resp map[string]*afind.Repo, err error) {
	err = r.client.Call(r.endpoint+".Get", key, &resp)
	return
}

func (r *ReposClient) GetAll() (resp map[string]*afind.Repo, err error) {
	err = r.client.Call(r.endpoint+".GetAll", struct{}{}, &resp)
	return
}

func (r *ReposClient) Delete(key string) (err error) {
	err = r.client.Call(r.endpoint+".Delete", key, nil)
	return
}

type reposServer struct {
	repos afind.KeyValueStorer
}

func (s *reposServer) Get(args string, reply *map[string]*afind.Repo) error {
	rs := make(map[string]*afind.Repo)
	if r := s.repos.Get(args); r != nil {
		repo := r.(*afind.Repo)
		rs[repo.Key] = repo
	}
	*reply = rs
	return nil
}

func (s *reposServer) GetAll(args struct{}, reply *map[string]*afind.Repo) error {
	replyv := make(map[string]*afind.Repo)
	s.repos.ForEach(func(key string, value interface{}) bool {
		replyv[key] = value.(*afind.Repo)
		return true
	})
	*reply = replyv
	return nil
}

func (s *reposServer) Delete(args string, _ *interface{}) error {
	return s.repos.Delete(args)
}

func (s *reposServer) webDelete(rw http.ResponseWriter, req *http.Request,
	ps httprouter.Params) {

	setJson(rw)
	key := ps.ByName("key")
	if err := s.repos.Delete(key); err == nil {
		rw.WriteHeader(200)
	} else {
		enc := json.NewEncoder(rw)
		rw.WriteHeader(500)
		_ = enc.Encode(errs.StructError{T: "delete_repo", M: err.Error()})
	}
}

func (s *reposServer) webGet(rw http.ResponseWriter, req *http.Request,
	ps httprouter.Params) {

	setJson(rw)
	enc := json.NewEncoder(rw)
	repos := make(map[string]*afind.Repo)
	key := ps.ByName("key")
	if key != "" {
		// get one repo
		if v := s.repos.Get(key); v != nil {
			repos[key] = v.(*afind.Repo)
		}
	} else {
		// get all repos
		s.repos.ForEach(func(key string, value interface{}) bool {
			if value != nil {
				repos[key] = value.(*afind.Repo)
			}
			return true
		})
	}

	if len(repos) > 0 {
		rw.WriteHeader(200)
		_ = enc.Encode(repos)
	} else {
		rw.WriteHeader(404)
	}
}
