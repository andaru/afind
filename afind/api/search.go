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

func getRequests(
	s *searchServer,
	q afind.SearchQuery) (
	count int,
	chQuery chan par.RequestFunc,
	chResult chan *afind.SearchResult) {

	sw := stopwatch.New()
	sw.Start("*")

	maxBe := s.cfg.MaxSearchReqBe
	repos := getRepos(s.repos, q, s.cfg.MaxSearchRepo)
	numrepos := len(repos)
	countBe := 0

	chQuery = make(chan par.RequestFunc, numrepos+1)
	chResult = make(chan *afind.SearchResult, numrepos+1)
	// whatever the case, we always close the request channel
	// upon completion, or we will deadlock
	defer close(chQuery)

	if numrepos < 1 {
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

	// A single request is built for each remote host, containing
	// all of the repo keys to search. For local requests, the
	// query for each repo is added as a separate request, to
	// parallelize local work.
	for host, keys := range hosts {
		if maxBe > 0 && countBe >= maxBe {
			log.Warning("search [%v] max backend requests (%d)", q, maxBe)
			break
		}

		this := afind.SearchQuery(q)
		this.Meta.SetHost(host)
		if isLocal(s.cfg, host) {
			// local requests are not concatenated
			log.Debug("new local queries for keys=%v", keys)
			for _, key := range keys {
				this.RepoKeys = []string{key}
				count++
				chQuery <- localSearch(s, this, chResult)
			}
		} else {
			log.Debug("new remote query host=%v keys=%v", host, keys)
			this.RepoKeys = keys
			count++
			countBe++
			chQuery <- remoteSearch(s, this, chResult)
		}
	}
	elapsed := sw.Stop("*")
	log.Debug("getRequests count=%d (%d be) elapsed=%v", count, countBe, elapsed)
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
		sw := stopwatch.New()
		sw.Start("*")
		sr := afind.NewSearchResult()
		cl, err := NewRpcClient(addr)
		if err == nil {
			client := NewSearcherClient(cl)
			defer client.Close()

			sr, err = client.Search(ctx, req)
			if sr != nil {
				updateRepos(s.repos, sr.Repos)
			}
		}
		if err != nil {
			sr.Errors[req.Meta.Host()] = errs.NewStructError(err)
		}

		select {
		case <-ctx.Done():
		default:
			if sr.NumMatches > 0 {
				log.Debug("search backend %v (%d matches) (%v)",
					addr, sr.NumMatches, sw.Stop("*"))
			}
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

	// Determine which repos to search, flatten requests per host,
	// then execute the searches concurrently.
	sw.Start("get_requests")
	numreq, reqch, ch := getRequests(s, req)
	resp.Durations.GetRepos = sw.Stop("get_requests")
	log.Debug("getRequests numRequests=%d elapsed=%v",
		numreq, resp.Durations.GetRepos)

	// Get a request context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute the requests
	go func() {
		_ = par.Requests(reqch).WithConcurrency(s.cfg.MaxSearchC).DoWithContext(ctx)
		close(ch)
	}()

	// Merge the incoming results until we have enough.
	// Updating first ensures we'll collect more than enough if
	// they're available.
	for in := range ch {
		resp.Update(in)
		if resp.EnoughResults() {
			log.Debug("%s finished early (%d matches)",
				msg, resp.NumMatches)
			cancel()
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

func updateRepos(kv afind.KeyValueStorer, repos map[string]*afind.Repo) {
	for key, repo := range repos {
		_ = kv.Set(key, repo)
	}
}
