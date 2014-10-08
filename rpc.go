package afind

import (
	"time"
)

func (self *RpcService) Search(req SearchRequest, resp *SearchResponse) error {
	start := time.Now()
	r, err := self.Searcher.Search(req)
	if r != nil {
		resp.Files = r.Files
		resp.NumLinesMatched = r.NumLinesMatched
	}
	resp.Elapsed = time.Since(start)
	return err
}

func (self *RpcService) Index(req IndexRequest, response *IndexResponse) error {
	start := time.Now()
	req = newIndexRequestWithMeta(req.Key, req.Root, req.Dirs, req.Meta)
	repos, err := self.Indexer.Index(req)
	if err != nil {
		return err
	}
	response.Repo = repos.Repo
	response.Elapsed = time.Since(start)
	return err
}

func (self *RpcService) GetRepo(key string, response *Repo) error {
	repo := self.Service.repos.Get(key)
	if repo != nil {
		r := repo.(*Repo)
		response = &(*r)
		return nil
	}
	return newNoRepoAvailableError()
}

func (self *RpcService) GetRepos(keys []string, response *Repos) error {
	repos := make(map[string]*Repo)
	for _, key := range keys {
		repo := self.Service.repos.Get(key)
		if repo != nil {
			repos[key] = repo.(*Repo)
		}
	}
	response.Repos = repos
	return nil
}

func (self *RpcService) GetAllRepos(_ bool, response *map[string]*Repo) error {
	repos := make(map[string]*Repo)
	self.repos.ForEach(func(key string, value interface{}) bool {
		if v, ok := value.(*Repo); ok {
			repos[key] = v
		} else {
			return false
		}
		return true
	})
	*response = repos
	return nil
}

type RpcService struct {
	*Service
}

func newRpcService(service *Service) *RpcService {
	return &RpcService{service}
}
