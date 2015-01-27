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

type IndexerClient struct {
	endpoint string
	client   *rpc.Client
}

func NewIndexerClient(client *rpc.Client) *IndexerClient {
	return &IndexerClient{endpoint: EPIndexer, client: client}
}

func (i *IndexerClient) Close() error {
	return i.client.Close()
}

func (i *IndexerClient) Index(ctx context.Context, req afind.IndexQuery) (
	*afind.IndexResult, error) {
	// todo: use context
	resp := afind.NewIndexResult()
	err := i.client.Call(i.endpoint+".Index", req, resp)
	return resp, err
}

type indexServer struct {
	cfg     *afind.Config
	repos   afind.KeyValueStorer
	indexer afind.Indexer
}

func (s *indexServer) Index(args afind.IndexQuery, reply *afind.IndexResult) error {
	timeout := timeoutIndex(args, s.cfg)
	ir, err := doIndex(s, args, timeout)
	ir.SetError(err)
	*reply = *ir
	return nil
}

func (s *indexServer) webIndex(rw http.ResponseWriter, req *http.Request,
	ps httprouter.Params) {

	dec := json.NewDecoder(req.Body)
	enc := json.NewEncoder(rw)
	setJson(rw)

	// parse and validate the JSON request
	var q afind.IndexQuery
	if err := dec.Decode(&q); err != nil {
		rw.WriteHeader(400)
		_ = enc.Encode(
			errs.NewStructError(errs.InvalidRequestError(err.Error())))
		return
	}
	// Enable request recursion
	q.Recurse = true

	// Execute the request
	timeout := timeoutIndex(q, s.cfg)
	ir, err := doIndex(s, q, timeout)

	if ir.Error != nil {
		rw.WriteHeader(500)
		_ = enc.Encode(ir.Error)
	} else if err != nil {
		rw.WriteHeader(500)
		_ = enc.Encode(errs.NewStructError(err))
	} else {
		rw.WriteHeader(200)
		_ = enc.Encode(ir)
	}
}

func localIndex(s *indexServer, req afind.IndexQuery,
	results chan *afind.IndexResult) par.RequestFunc {

	return func(ctx context.Context) error {
		ir, err := s.indexer.Index(ctx, req)
		ir.SetError(err)
		results <- ir
		return nil
	}
}

func remoteIndex(s *indexServer, req afind.IndexQuery,
	results chan *afind.IndexResult) par.RequestFunc {

	addr := getAddress(req.Meta, s.cfg.PortRpc())
	return func(ctx context.Context) error {
		ir := afind.NewIndexResult()
		cl, err := NewRpcClient(addr)
		defer cl.Close()

		if err == nil {
			ir, err = NewIndexerClient(cl).Index(ctx, req)
		}
		ir.SetError(err)

		select {
		case <-ctx.Done():
			return nil
		default:
			results <- ir
		}
		return nil
	}
}

func timeoutIndex(req afind.IndexQuery, cfg *afind.Config) time.Duration {
	if req.Timeout == 0 {
		return cfg.GetTimeoutIndex()
	}
	return req.Timeout
}

func doIndex(s *indexServer, req afind.IndexQuery, timeout time.Duration) (
	resp *afind.IndexResult, err error) {

	local := isLocal(s.cfg, req.Meta.Host())
	resp = afind.NewIndexResult()
	log.Debug("index [%s] request %#v local=%v", req.Key, req, local)
	resp = afind.NewIndexResult()

	// Duplicate request handling: if this is a remote indexing
	// request and we've got a recent matching Repo for that key
	// already, save RPC bandwith and latency and return our copy.
	if r := s.repos.Get(req.Key); r != nil {
		resp.Repo = r.(*afind.Repo)
		if local || !resp.Repo.Stale(s.cfg.TimeoutRepoStale) {
			return
		}
	}

	// Set a marker repo in the store, indicating we're presently indexing
	tmprepo := afind.NewRepo()
	tmprepo.Key = req.Key
	tmprepo.State = afind.INDEXING
	tmprepo.Root = req.Root
	tmprepo.Meta.Update(req.Meta)
	_ = s.repos.Set(req.Key, tmprepo)

	// setup a request context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if local || req.Recurse {
		// we have a local or a remote indexing request to make.
		// recursion is disabled on any request relayed to a remote afindd.
		req.Recurse = false
		ch := make(chan *afind.IndexResult, 1)
		reqch := make(chan par.RequestFunc, 1)
		if local {
			reqch <- localIndex(s, req, ch)
		} else {
			reqch <- remoteIndex(s, req, ch)
		}
		close(reqch)
		err = par.Requests(reqch).DoWithContext(ctx)
		close(ch)
		resp = <-ch
		if resp.Error != nil {
			err = resp.Error
		}
		// Set the repo if we have one in the response
		if resp.Repo != nil {
			_ = s.repos.Set(resp.Repo.Key, resp.Repo)
		} else {
			// Delete the temporary indexing repo
			_ = s.repos.Delete(req.Key)
		}
	} else {
		// neither a local query or a recursive query,
		// so this presumably once recursive query
		// has looped, meaning we cannot resolve an
		// appropriate backend.
		log.Debug("unservicable IndexQuery %#v local=%v", req, local)
		err = errs.NewNoRpcClientError()
		_ = s.repos.Delete(req.Key)
	}
	return
}
