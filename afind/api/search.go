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

func (s *SearcherClient) Search(
	ctx context.Context,
	query afind.SearchQuery) (sr *afind.SearchResult, err error) {

	sr = afind.NewSearchResult()
	searchCall := s.client.Go(s.endpoint+".Search", query, sr, nil)
	select {
	case <-ctx.Done():
		err = errs.NewTimeoutError("search")
	case reply := <-searchCall.Done:
		err = reply.Error
	}
	return
}

// returns a slice of Repo relevant to this search query
func getRepos(
	rstore afind.KeyValueStorer,
	request afind.SearchQuery,
	max int) (repos []*afind.Repo) {

	// Either select specific requested repo keys...
	repos = []*afind.Repo{}
	for _, key := range request.RepoKeys {
		if value := rstore.Get(key); value != nil {
			if max > 0 && len(repos) > max {
				break
			}
			repo := value.(*afind.Repo)
			if repo.State == afind.OK {
				repos = append(repos, repo)
			}
		}
	}
	if len(request.RepoKeys) > 0 {
		return repos
	}
	// ...or select repos matching the provided metadata. All
	// repos are selected if no metadata is provided.
	return afind.ReposMatchingMeta(rstore, request.Meta, request.MetaRegexpMatch, max)
}

func getSearchQueries(
	s *searchServer,
	q afind.SearchQuery,
	chQuery chan par.RequestFunc,
	chResult chan *afind.SearchResult) {

	sw := stopwatch.New()
	sw.Start("*")
	repos := genGetReqpos(s.repos, q.RepoKeys, q.Meta, q.MetaRegexpMatch)

	count := 0
	countBe := 0
	maxBe := s.cfg.MaxSearchReqBe

	defer close(chQuery)
	if len(repos) < 1 {
		return
	}

	hosts := map[string][]string{}
	for _, repo := range repos {
		host := repo.Host()
		_, ok := hosts[host]
		if !ok {
			hosts[host] = []string{}
		}
		hosts[host] = append(hosts[host], repo.Key)
	}

	for host, keys := range hosts {
		if maxBe > 0 && countBe >= maxBe {
			log.Warning("%s max backend requests (%d)", logmsgSearch(q), maxBe)
			break
		}

		this := afind.SearchQuery(q)
		this.RepoKeys = []string{}
		this.Meta.SetHost(host)
		if isLocal(s.cfg, host) {
			// local requests are not concatenated
			for _, key := range keys {
				this.RepoKeys = []string{key}
				count++
				chQuery <- localSearch(s, this, chResult)
			}
			log.Debug("new local queries for keys=%v", this.RepoKeys)
		} else {
			this.RepoKeys = keys
			count++
			countBe++
			chQuery <- remoteSearch(s, this, chResult)
			log.Debug("new remote query host=%v keys=%v", host, this.RepoKeys)
		}
	}
	elapsed := sw.Stop("*")
	log.Debug("getSearchQueries count=%d (%d be) elapsed=%v", count, countBe, elapsed)
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
	if err != nil {
		sr.Error = err.Error()
	}
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
		}
		if err != nil {
			sr.Errors[req.Meta.Host()] = errs.NewStructError(err)
		}

		select {
		case <-ctx.Done():
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

func logmsgSearch(req afind.SearchQuery) string {
	msg := "search [" + req.Re + "]"
	if req.IgnoreCase {
		msg += " ignore-case"
	}
	if req.PathRe != "" {
		msg += " path-re [" + req.PathRe + "]"
	}
	return msg
}

func doSearch(s *searchServer, req afind.SearchQuery, timeout time.Duration) (
	resp *afind.SearchResult, err error) {

	sw := stopwatch.New()
	sw.Start("total")
	msg := logmsgSearch(req)
	log.Info("%s", msg)

	resp = afind.NewSearchResult()
	resp.MaxMatches = req.MaxMatches
	updateRepos := map[string]*afind.Repo{}

	// Start filling the query channel
	chQuery := make(chan par.RequestFunc, 100)
	chResult := make(chan *afind.SearchResult, 10)
	go getSearchQueries(s, req, chQuery, chResult)

	// Get a request context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = req.Normalize()
	if err != nil {
		resp.SetError(err)
		goto done
	}

	// Execute the requests concurrently
	go func() {
		_ = par.Requests(chQuery).WithConcurrency(s.cfg.MaxSearchC).DoWithContext(ctx)
		close(chResult)
	}()

	// Merge the incoming results until we have enough.
	// Updating first ensures we'll collect more than enough if
	// they're available.
	for in := range chResult {
		// Repositories with good and bad responses must error out
		for key, repo := range in.Repos {
			if r, ok := updateRepos[key]; ok && r.State == afind.ERROR {
				continue
			}
			updateRepos[key] = repo
		}
		resp.Update(in)
		if resp.EnoughResults() {
			log.Debug("%s finished early (%d matches)",
				msg, resp.NumMatches)
			cancel()
			break
		}
	}

done:
	// Update our knowledge about Repo found in the responses
	for key, repo := range updateRepos {
		if repo.State == afind.OK {
			_ = s.repos.Set(key, repo)
		} else if s.cfg.DeleteRepoOnError {
			_ = s.repos.Delete(key)
		}
	}

	resp.Durations.Search = sw.Stop("total")
	if resp.Error == "" {
		msg += " ok"
	} else {
		msg += " error"
	}
	log.Info("%s (%d matches) (%v)", msg, resp.NumMatches, resp.Durations.Search)
	return
}
