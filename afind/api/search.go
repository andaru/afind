package api

import (
	"encoding/json"
	"net/http"
	"net/rpc"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/stopwatch"
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

func (s *SearcherClient) Close() {
	_ = s.client.Close()
}

// returns a slice of Repo relevant to this search query
func getRepos(rstore afind.KeyValueStorer, request afind.SearchQuery) []*afind.Repo {
	repos := make([]*afind.Repo, 0)

	// Either select specific requested repo keys from the database
	for _, key := range request.RepoKeys {
		if value := rstore.Get(key); value != nil {
			repos = append(repos, value.(*afind.Repo))
		}
	}
	if len(request.RepoKeys) > 0 {
		return repos
	}

	// Otherwise, filter all available Repo against request
	// Metadata. Values in the Meta are considered regular
	// expressions, if request.MetaRegexpMatch is set. If not
	// set, Meta values are treated as exact strings to filter
	// for. Only matching keys are considered, so filters that do
	// not appear in the Repo pass the filter.
	rstore.ForEach(func(key string, value interface{}) bool {
		r := value.(*afind.Repo)
		if !request.MetaRegexpMatch && r.Meta.Matches(request.Meta) {
			// Exact string match
			repos = append(repos, r)
		} else if request.MetaRegexpMatch && r.Meta.MatchesRegexp(request.Meta) {
			// Regular expression match
			repos = append(repos, r)
		}
		return true
	})

	return repos
}

func (s *SearcherClient) Search(
	ctx context.Context,
	query afind.SearchQuery) (sr *afind.SearchResult, err error) {

	sr = afind.NewSearchResult()
	searchCall := s.client.Go(s.endpoint+".Search", query, sr, nil)
	select {
	case <-ctx.Done():
		err = errs.NewTimeoutError("search")
	case reply := <-searchCall.Done:
		if reply.Error != nil {
			err = reply.Error
		}
	}
	return
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
			results <- sr
		}
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
			client := NewSearcherClient(cl)
			defer client.Close()

			sr, err = client.Search(ctx, req)
			updateRepos(s.repos, sr.Repos)
		}
		if err != nil {
			sr.Errors[req.Meta.Host()] = errs.NewStructError(err)
		}

		select {
		case <-ctx.Done():
			return nil
		default:
			results <- sr
		}
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

	sw := stopwatch.New()
	sw.Start("total")
	resp = afind.NewSearchResult()
	resp.MaxMatches = req.MaxMatches
	// Get a request context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Determine which repos to search, then search concurrently
	sw.Start("get_repos")
	searchRepos := getRepos(s.repos, req)
	resp.Durations.GetRepos = sw.Stop("get_repos")

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
	resp.Durations.Search = sw.Stop("total")
	if resp.Error == "" {
		msg = "done"
	} else {
		msg = "error"
	}
	log.Info("search [%s] %s %v", req.Re, msg, resp.Durations.Search)
	return
}

func updateRepos(kv afind.KeyValueStorer, repos map[string]*afind.Repo) {
	for key, repo := range repos {
		_ = kv.Set(key, repo)
	}
}
