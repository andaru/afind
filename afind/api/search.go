package api

import (
	"encoding/json"
	"net/http"
	"net/rpc"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/errs"
	"github.com/julienschmidt/httprouter"
	"github.com/savaki/par"
)

type SearcherClient struct {
	endpoint string
	client   *rpc.Client
}

func NewSearcherClient(client *rpc.Client) *SearcherClient {
	return &SearcherClient{endpoint: EPSearcher, client: client}
}

func (s *SearcherClient) Close() error {
	return s.client.Close()
}

// returns a list of repos relevant to this search given a service
func getRepos(rstore afind.KeyValueStorer, request afind.SearchQuery) []*afind.Repo {

	// Select repos to search
	repos := make([]*afind.Repo, 0)

	// Select requested repo keys from the database
	for _, key := range request.RepoKeys {
		if value := rstore.Get(key); value != nil {
			repos = append(repos, value.(*afind.Repo))
		}
	}

	// Otherwise, select all repos (optionally matching the metadata)
	if len(request.RepoKeys) == 0 {
		rstore.ForEach(func(key string, value interface{}) bool {
			r := value.(*afind.Repo)
			if r.Meta.Matches(request.Meta) {
				repos = append(repos, r)
			}
			return true
		})
	}

	return repos
}

func (s *SearcherClient) Search(ctx context.Context, query afind.SearchQuery) (
	*afind.SearchResult, error) {
	// todo: use context to cancel long running tasks
	resp := afind.NewSearchResult()
	err := s.client.Call(s.endpoint+".Search", query, resp)
	return resp, err
}

// Common HTTP/GobRPC search server
type searchServer struct {
	cfg      *afind.Config
	repos    afind.KeyValueStorer
	searcher afind.Searcher
}

func (s *searchServer) Search(args afind.SearchQuery,
	reply *afind.SearchResult) (err error) {
	timeout := timeoutSearch(args, s.cfg)
	sr, err := doSearch(s, args, timeout)
	sr.SetError(err)
	*reply = *sr
	return
}

// Search HTTP handler

func (s *searchServer) webSearch(rw http.ResponseWriter, req *http.Request,
	ps httprouter.Params) {

	setJson(rw)
	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)
	sr := afind.SearchQuery{}
	sr.Meta = make(afind.Meta)

	// Parse the query
	if err := dec.Decode(&sr); err != nil {
		rw.WriteHeader(403)
		_ = enc.Encode(
			errs.NewStructError(errs.InvalidRequestError(err.Error())))
		return
	}
	// Allow single recursive query to perform master->backend resolution
	sr.Recurse = true

	// Perform the search
	if resp, err := doSearch(s, sr, timeoutSearch(sr, s.cfg)); err == nil {
		rw.WriteHeader(200)
		_ = enc.Encode(resp)
	} else {
		rw.WriteHeader(500)
		_ = enc.Encode(errs.StructError{T: "search", M: err.Error()})
	}
}

func localSearch(s *searchServer, req afind.SearchQuery,
	results chan *afind.SearchResult) par.RequestFunc {

	return func(ctx context.Context) error {
		sr, err := s.searcher.Search(ctx, req)
		if err != nil {
			if len(req.RepoKeys) > 0 {
				sr.Errors[req.RepoKeys[0]] = errs.NewStructError(err)
			} else {
				sr.Error = err.Error()
			}
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		results <- sr
		return nil
	}
}

func remoteSearch(s *searchServer, req afind.SearchQuery,
	results chan *afind.SearchResult) par.RequestFunc {

	addr := getAddress(req.Meta, s.cfg.PortRpc())
	return func(ctx context.Context) error {
		sr := afind.NewSearchResult()
		cl, err := NewRpcClient(addr)
		if err == nil {
			sr, err = NewSearcherClient(cl).Search(ctx, req)
		}
		if err != nil {
			sr.Errors[req.Meta.Host()] = errs.NewStructError(err)
		}
		results <- sr
		return nil
	}
}

func timeoutSearch(req afind.SearchQuery, cfg *afind.Config) time.Duration {
	if req.Timeout == 0 {
		return cfg.GetTimeoutSearch()
	}
	return req.Timeout
}

func doSearch(s *searchServer, req afind.SearchQuery, timeout time.Duration) (
	resp *afind.SearchResult, err error) {

	msg := "search [" + req.Re + "]"
	if req.IgnoreCase {
		msg += " ignore-case"
	}
	if req.PathRe != "" {
		msg += " path-re [" + req.PathRe + "]"
	}
	log.Info(msg)

	start := time.Now()
	resp = afind.NewSearchResult()
	// Get a request context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Determine which repos to search, then search concurrently
	searchRepos := getRepos(s.repos, req)
	ch := make(chan *afind.SearchResult, len(searchRepos))
	reqch := make(chan par.RequestFunc, len(searchRepos))
	for _, repo := range searchRepos {
		req.RepoKeys = []string{repo.Key}
		req.Meta.SetHost(repo.Host())
		if isLocal(s.cfg, repo.Host()) {
			reqch <- localSearch(s, req, ch)
		} else {
			reqch <- remoteSearch(s, req, ch)
		}
	}
	close(reqch)

	err = par.Requests(reqch).WithConcurrency(s.cfg.MaxSearchC).DoWithContext(ctx)
	close(ch)

	// Merge the incoming results
	for in := range ch {
		resp.Update(in)
	}
	resp.Elapsed = time.Since(start)
	if resp.Error == "" {
		msg = "done"
	} else {
		msg = "error"
	}
	log.Info("search [%s] %s %v", req.Re, msg, resp.Elapsed)
	return
}
