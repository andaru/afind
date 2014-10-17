package afind

import (
	"net/rpc"
	"strings"
	"sync"
)

type clientStats struct {
	callsIndex       int64
	callsSearch      int64
	callsGetRepo     int64
	callsGetAllRepos int64
	callsDeleteRepo  int64
}

type rpcClient struct {
	*rpc.Client
	rem   *Remotes
	stats clientStats
}

func NewRpcClient(server string) (*rpcClient, error) {
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	return &rpcClient{client, nil, clientStats{}}, nil
}

func newRpcClientFromRemotes(server string, remotes *Remotes) (*rpcClient, error) {
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	return &rpcClient{client, remotes, clientStats{}}, nil
}

func (client *rpcClient) Index(request IndexRequest) (*IndexResponse, error) {
	indexResponse := newIndexResponse()
	err := client.Call("Afind.Index", request, indexResponse)
	client.stats.callsIndex++
	return indexResponse, err
}

func (client *rpcClient) Search(request SearchRequest) (*SearchResponse, error) {
	sr := newSearchResponse()
	rpcCall := client.Go("Afind.Search", &request, sr, nil)
	call := <-rpcCall.Done
	client.stats.callsSearch++
	return sr, call.Error
}

func (client *rpcClient) GetRepo(key string) (*map[string]*Repo, error) {
	repos := make(map[string]*Repo)
	err := client.Call("Afind.GetRepo", key, &repos)
	client.stats.callsGetRepo++
	return &repos, err
}

func (client *rpcClient) GetAllRepos() (*map[string]*Repo, error) {
	repos := make(map[string]*Repo)
	rpcCall := client.Go("Afind.GetAllRepos", struct{}{}, &repos, nil)
	call := <-rpcCall.Done
	client.stats.callsGetAllRepos++
	return call.Reply.(*map[string]*Repo), call.Error
}

func (client *rpcClient) DeleteRepo(key string) error {
	rpcCall := client.Go("Afind.DeleteRepo", nil, nil, nil)
	call := <-rpcCall.Done
	client.stats.callsDeleteRepo++
	return call.Error
}

// Remote (RPC client) management

type remoteStats struct {
	clientNew      int64
	clientClose    int64
	clientCloseErr int64
}

// Remotes is a factory for RPC access to remote afindd
type Remotes struct {
	*sync.RWMutex
	afindd map[string]*remote
	stats  remoteStats
}

type remote struct {
	address string // address:port (or hostname:port)
	client  *rpcClient
}

func NewRemotes() Remotes {
	return Remotes{
		RWMutex: new(sync.RWMutex),
		afindd:  make(map[string]*remote),
		stats:   remoteStats{},
	}
}

func (r *Remotes) RegisterAddress(address string) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.afindd[address]; ok {
		return
	}
	r.afindd[address] = &remote{address: address}
}

func (r *Remotes) Register(host, rpcPort string) {
	if host == "" || rpcPort == "" {
		panic("must provide both host and rpcPort to Register")
	}
	r.RegisterAddress(host + ":" + rpcPort)
}

func (r *Remotes) Get(address string) (*rpcClient, error) {
	r.RLock()
	defer r.RUnlock()

	// Search for any endpoint matching the hostname
	// when no full endpoint address is provided.
	if !strings.Contains(address, ":") {
		// it's a host, get the first matching
		for endpoint, _ := range r.afindd {
			if strings.HasPrefix(endpoint, address) {
				// this'll do
				address = endpoint
				break
			}
		}
	}

	if remote, ok := r.afindd[address]; ok {
		// maybe there is no client yet, as manual
		// registration does not demand one.
		if remote.client == nil {
			newClient, err := newRpcClientFromRemotes(address, r)
			if newClient != nil && err == nil {
				remote.client = newClient
				r.stats.clientNew++
			}
		}
		r.afindd[address] = remote
		return remote.client, nil
	}

	newClient, err := newRpcClientFromRemotes(address, r)
	if newClient != nil {
		r.stats.clientNew++
		rem := remote{
			address: address,
			client:  newClient,
		}
		r.afindd[address] = &rem
	}
	return newClient, err
}

func (r *Remotes) Close(host, rpcPort string) error {
	r.Lock()
	defer r.Unlock()

	server := host + ":" + rpcPort
	if remote, ok := r.afindd[server]; ok {
		err := remote.client.Close()
		if err == nil {
			delete(r.afindd, server)
		}
		return err
	}
	return nil
}

func addressFromRepo(repo *Repo, defaultPort string) string {
	host := repo.Meta["host"]
	port, _ := repo.Meta["port"]
	if port == "" {
		port = defaultPort
	}
	return host + ":" + port
}
