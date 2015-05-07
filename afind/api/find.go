package api

import (
	"encoding/json"
	"net/http"
	"net/rpc"
	"time"

	"code.google.com/p/go.net/context"
	"fmt"
	"github.com/andaru/afind/afind"
	"github.com/andaru/afind/errs"
	"github.com/andaru/afind/stopwatch"
	"github.com/julienschmidt/httprouter"
	"github.com/savaki/par"
)

// FindClient is an implementation of afind.Finder making Find calls
// on a remote afindd instance.
type finderClient struct {
	endpoint string
	client   *rpc.Client
}

// FinderCloser is a Finder plus the local Close function
type FinderCloser interface {
	afind.Finder
	Close()
}

// NewFindClient returns a new Finder from an existing rpc.Client
func NewFindClient(client *rpc.Client) FinderCloser {
	return &finderClient{endpoint: EPFinder, client: client}
}

func (f *finderClient) Close() {
	_ = f.client.Close()
}

func (f *finderClient) Find(
	ctx context.Context,
	query afind.FindQuery) (fr *afind.FindResult, err error) {

	fr = afind.NewFindResult()
	findCall := f.client.Go(f.endpoint+".Find", query, fr, nil)
	select {
	case <-ctx.Done():
	case reply := <-findCall.Done:
		err = reply.Error
	}
	return
}

type findServer struct {
	cfg    *afind.Config
	repos  afind.KeyValueStorer
	finder afind.Finder
}

func (s *findServer) Find(args afind.FindQuery, reply *afind.FindResult) error {
	timeout := timeoutFind(args, s.cfg)
	fr, err := doFind(s, args, timeout)
	fr.SetError(err)
	*reply = *fr
	return nil
}

func timeoutFind(q afind.FindQuery, cfg *afind.Config) time.Duration {
	if q.Timeout == 0 {
		return cfg.GetTimeoutFind()
	}
	return q.Timeout
}

func (s *findServer) webFind(
	rw http.ResponseWriter,
	req *http.Request,
	ps httprouter.Params) {

	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)
	setJson(rw)

	q := afind.NewFindQuery()
	if err := dec.Decode(&q); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_ = enc.Encode(
			errs.NewStructError(errs.InvalidRequestError(err.Error())))
		return
	}
	q.Recurse = true
	fr, err := doFind(s, q, timeoutFind(q, s.cfg))

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_ = enc.Encode(errs.NewStructError(err))
	} else {
		rw.WriteHeader(http.StatusOK)
		_ = enc.Encode(fr)
	}

}

func genGetReqpos(kvs afind.KeyValueStorer, keys []string, meta afind.Meta, metaRegexpMatch bool) (repos []*afind.Repo) {
	repos = []*afind.Repo{}
	if len(keys) > 0 {
		for _, repo := range getReposForKeys(kvs, keys, afind.OK) {
			repos = append(repos, repo)
		}
	} else {
		repos = afind.ReposMatchingMeta(kvs, meta, metaRegexpMatch, 0)
	}
	return
}

func logmsgFind(q afind.FindQuery) string {
	return fmt.Sprintf("find [%v]", q.PathRe)
}

func doFind(s *findServer, q afind.FindQuery, timeout time.Duration) (
	fr *afind.FindResult, err error) {
	sw := stopwatch.New()
	sw.Start("*")
	msg := logmsgFind(q)
	local := isLocal(s.cfg, q.Meta.Host())
	log.Debug("%s local=%v", msg, local)

	fr = afind.NewFindResult()
	fr.MaxMatches = q.MaxMatches
	repos := genGetReqpos(s.repos, q.RepoKeys, q.Meta, q.MetaRegexpMatch)
	chQuery := make(chan par.RequestFunc, 100)
	chResult := make(chan *afind.FindResult, 100)

	sw.Start("getFindRequests")
	getFindRequests(s, q, chQuery, chResult)
	log.Debug("%s getFindRequests elapsed=%v", msg, sw.Stop("getFindRequests"))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if len(repos) == 0 {
		err = errs.NewRepoUnavailableError()
	} else {
		err = q.Normalize()
	}
	if err != nil {
		fr.SetError(err)
		goto done
	}

	// Execute the requests
	sw.Start("queryFind")
	go func() {
		_ = par.Requests(chQuery).WithConcurrency(s.cfg.MaxSearchC).DoWithContext(ctx)
		close(chResult)
	}()
	for in := range chResult {
		fr.Update(in)
		if fr.EnoughResults() {
			log.Debug("%s finished early (%d matches)",
				logmsgFind(q), fr.NumMatches)
			cancel()
		}
	}
	sw.Stop("queryFind")
done:
	log.Info("find [%v] done (%v matches in %v files) (%v)",
		q.PathRe, fr.NumMatches,
		len(fr.Matches), sw.Stop("*"))
	return
}

func getReposForKeys(
	kvs afind.KeyValueStorer,
	keys []string,
	state string) (repos []*afind.Repo) {

	repos = []*afind.Repo{}
	for _, key := range keys {
		if v := kvs.Get(key); v != nil {
			repo := v.(*afind.Repo)
			if repo.State == state {
				repos = append(repos, repo)
			}
		}
	}
	return
}

func getFindRequests(
	s *findServer,
	q afind.FindQuery,
	chQuery chan par.RequestFunc,
	chResult chan *afind.FindResult) {

	repos := genGetReqpos(s.repos, q.RepoKeys, q.Meta, q.MetaRegexpMatch)
	log.Debug("%s getFindRequests %d repos", logmsgFind(q), len(repos))
	numrepos := len(repos)
	count := 0
	var countBe uint64
	maxBe := q.MaxMatches

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

	for host, keys := range hosts {
		if maxBe > 0 && countBe >= maxBe {
			log.Warning("search [%v] max backend requests (%d)", q, maxBe)
			break
		}

		this := afind.FindQuery(q)
		this.RepoKeys = []string{}
		this.Meta.SetHost(host)
		for _, key := range keys {
			this.RepoKeys = append(this.RepoKeys, key)
		}
		if isLocal(s.cfg, host) {
			// local requests are not concatenated
			count++
			chQuery <- localFind(s, this, chResult)
		} else {
			this.RepoKeys = keys
			count++
			countBe++
			chQuery <- remoteFind(s, this, chResult)
		}
	}

}

func localFind(
	s *findServer,
	q afind.FindQuery,
	results chan *afind.FindResult) par.RequestFunc {

	log.Debug("localFind query %v", q)
	return func(ctx context.Context) error {
		sw := stopwatch.New()
		sw.Start("*")

		fr, err := s.finder.Find(ctx, q)
		if err != nil {
			fr.Error = errs.NewStructError(err)
			log.Debug("find local [%v] error %v", q.PathRe, err)
		}
		select {
		case <-ctx.Done():
			return nil
		default:
			results <- fr
			log.Debug("find local (%d matches) (%v)", fr.NumMatches, sw.Stop("*"))
		}
		return nil
	}
}

func remoteFind(
	s *findServer,
	q afind.FindQuery,
	results chan *afind.FindResult) par.RequestFunc {

	addr := getAddress(q.Meta, s.cfg.PortRpc())
	return func(ctx context.Context) error {
		sw := stopwatch.New()
		sw.Start("*")
		var numMatches uint64

		fr := afind.NewFindResult()
		cl, err := NewRpcClient(addr)
		if err != nil {
			fr.Errors[q.Meta.Host()] = errs.NewStructError(err)
			return err
		}
		client := NewFindClient(cl)
		defer client.Close()
		fr, err = client.Find(ctx, q)
		numMatches = fr.NumMatches
		if err != nil {
			fr.Errors[q.Meta.Host()] = errs.NewStructError(err)
		}
		select {
		case <-ctx.Done():
		default:
			results <- fr
		}
		if numMatches > 0 {
			log.Debug("find backend %v (%d matches) (%v)",
				addr, numMatches, sw.Stop("*"))
		}
		return nil
	}
}
