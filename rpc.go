package afind

import (
	"fmt"
)

func (self *RpcService) Search(req SearchRequest, resp *SearchResponse) error {
	// resp.Files = make(map[string]map[string]map[string]string)
	r, err := self.Searcher.Search(req)
	resp.Files = r.Files
	resp.NumLinesMatched = r.NumLinesMatched
	resp.ElapsedNs = r.ElapsedNs
	return err
}

func (self *RpcService) Index(req IndexRequest, response *IndexResponse) error {
	response.Repos = make(map[string]*Repo)
	repos, err := self.Indexer.Index(req)
	if err != nil {
		return err
	}
	fmt.Println("rpc index repos=", repos)
	for k, v := range repos.Repos {
		response.Repos[k] = v
	}
	return err
}

func (self *RpcService) GetRepo(key string, response *Repos) error {
	if response.Repos == nil {
		response.Repos = make(map[string]*Repo)
	}
	repo := self.Service.repos.Get(key)
	if repo != nil {
		response.Repos[key] = repo.(*Repo)
	}
	return nil
}

type Repos struct {
	Repos map[string]*Repo
}

type Keys []string

func newKeys(keys ...string) Keys {
	return keys
}

func (self *RpcService) GetRepos(keys Keys, response *Repos) error {
	if response.Repos == nil {
		response.Repos = make(map[string]*Repo)
	}
	for _, key := range keys {
		repo := self.Service.repos.Get(key)
		if repo != nil {
			response.Repos[key] = repo.(*Repo)
		}
	}
	return nil
}

func (self *RpcService) GetAllRepos(_ interface{}, response *Repos) error {
	if response.Repos == nil {
		response.Repos = make(map[string]*Repo)
	}
	self.repos.ForEach(func(key string, value interface{}) bool {
		if v, ok := value.(*Repo); ok {
			response.Repos[key] = v
		} else {
			return false
		}
		return true
	})
	return nil
}

type RpcService struct {
	*Service
}

func newRpcService(service *Service) *RpcService {
	return &RpcService{service}
}
