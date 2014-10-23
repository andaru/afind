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
	ir, err := rs.Indexer.Index(req)
	if err != nil {
		return err
	}
	response.Repo = ir.Repo
	response.Elapsed = time.Since(start)
	return nil
}

func (rs *rpcService) GetRepo(key string, response *map[string]*Repo) error {
	repos := make(map[string]*Repo, 1)
	repo := rs.Service.repos.Get(key)
	if repo != nil {
		repos[key] = repo.(*Repo)
		*response = repos
		return nil
	}
	return newNoRepoFoundError()
}

func (rs *rpcService) GetRepos(keys []string, response *map[string]*Repo) error {
	repos := make(map[string]*Repo)
	for _, key := range keys {
		repo := rs.Service.repos.Get(key)
		if repo != nil {
			repos[key] = repo.(*Repo)
		}
	}
	*response = repos
	return nil
}

func (rs *rpcService) GetAllRepos(_ struct{}, response *map[string]*Repo) error {
	repos := make(map[string]*Repo)
	rs.repos.ForEach(func(key string, value interface{}) bool {
		if v, ok := value.(*Repo); ok {
			repos[key] = v
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
	if rs.config.RpcBind == "" {
		return nil
	}

	var err error
	if err = rs.svr.RegisterName("Afind", rs); err != nil {
		return err
	}
	l, err := net.Listen("tcp", rs.config.RpcBind)
	if err == nil {
		go rs.svr.Accept(l)
		log.Info("Started RPC server at %s", rs.config.RpcBind)
	}
	return err
}

func newRpcService(service *Service) *rpcService {
	return &rpcService{service, rpc.NewServer()}
}
