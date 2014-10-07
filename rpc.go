package afind

import (
	"time"
)

func (self *RpcService) Search(req SearchRequest, resp *SearchResponse) error {
	// resp.Files = make(map[string]map[string]map[string]string)
	r, err := self.Searcher.Search(req)
	if r != nil {
		resp.Files = r.Files
		resp.NumLinesMatched = r.NumLinesMatched
		resp.ElapsedNs = r.ElapsedNs
	}
	return err
}

func (self *RpcService) Index(req IndexRequest, response *IndexResponse) error {
	start := time.Now()
	repos, err := self.Indexer.Index(req)
	if err != nil {
		return err
	}
	response.Repos = repos.Repos
	response.Elapsed = time.Since(start)
	return err
}

func (self *RpcService) GetRepo(key string, response *Repos) error {
	repos := make(map[string]*Repo)
	repo := self.Service.repos.Get(key)
	if repo != nil {
		repos[key] = repo.(*Repo)
	}
	response.Repos = repos
	return nil
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

func (self *RpcService) GetAllRepos(_ bool, response *Repos) error {
	repos := make(map[string]*Repo)
	self.repos.ForEach(func(key string, value interface{}) bool {
		log.Debug("key=%v value=%v", key, value)
		if v, ok := value.(*Repo); ok {
			repos[key] = v
		} else {
			return false
		}
		return true
	})
	response.Repos = repos
	return nil
}

type RpcService struct {
	*Service
}

func newRpcService(service *Service) *RpcService {
	return &RpcService{service}
}
