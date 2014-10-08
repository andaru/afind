package afind

import (
	"net"
	"net/rpc"
	"time"
)

func (rs *rpcService) Search(req SearchRequest, resp *SearchResponse) error {
	start := time.Now()
	r, err := rs.Searcher.Search(req)
	if r != nil {
		resp.Files = r.Files
		resp.NumLinesMatched = r.NumLinesMatched
	}
	resp.Elapsed = time.Since(start)
	return err
}

func (rs *rpcService) Index(req IndexRequest, response *IndexResponse) error {
	start := time.Now()
	req = newIndexRequestWithMeta(req.Key, req.Root, req.Dirs, req.Meta)
	repos, err := rs.Indexer.Index(req)
	if err != nil {
		return err
	}
	response.Repo = repos.Repo
	response.Elapsed = time.Since(start)
	return err
}

func (rs *rpcService) GetRepo(key string, response *Repo) error {
	repo := rs.Service.repos.Get(key)
	if repo != nil {
		r := repo.(*Repo)
		response = &(*r)
		return nil
	}
	return newNoRepoAvailableError()
}

func (rs *rpcService) GetRepos(keys []string, response *Repos) error {
	repos := make(map[string]*Repo)
	for _, key := range keys {
		repo := rs.Service.repos.Get(key)
		if repo != nil {
			repos[key] = repo.(*Repo)
		}
	}
	response.Repos = repos
	return nil
}

func (rs *rpcService) GetAllRepos(_ bool, response *map[string]*Repo) error {
	repos := make(map[string]*Repo)
	rs.repos.ForEach(func(key string, value interface{}) bool {
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

type rpcService struct {
	*Service
	svr *rpc.Server
}

func (rs *rpcService) start() error {
	if config.RpcBind == "" {
		return nil
	}

	var err error
	if err = rs.svr.RegisterName("Afind", rs); err != nil {
		log.Fatal("RPC server error:", err)
	}
	l, err := net.Listen("tcp", config.RpcBind)
	if err == nil {
		go rs.svr.Accept(l)
		log.Info("Started RPC server at %s", config.RpcBind)
	}
	return err
}

func newRpcService(service *Service) *rpcService {
	return &rpcService{service, rpc.NewServer()}
}
