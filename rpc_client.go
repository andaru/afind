package afind

import (
	"net/rpc"
)

type RpcClient struct {
	*rpc.Client
}

func NewRpcClient(server string) (*RpcClient, error) {
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	return &RpcClient{client}, nil
}

func (client *RpcClient) Index(request IndexRequest) (*IndexResponse, error) {
	indexResponse := newIndexResponse()
	err := client.Call("Afind.Index", request, indexResponse)
	return indexResponse, err
}

func (client *RpcClient) Search(request SearchRequest) (*SearchResponse, error) {
	sr := newSearchResponse()
	err := client.Call("Afind.Search", request, sr)
	return sr, err
}

func (client *RpcClient) GetRepo(key string) (*map[string]*Repo, error) {
	repos := make(map[string]*Repo)
	err := client.Call("Afind.GetRepo", key, &repos)
	return &repos, err
}

func (client *RpcClient) GetAllRepos() (*map[string]*Repo, error) {
	repos := make(map[string]*Repo)
	rpcCall := client.Go("Afind.GetAllRepos", struct{}{}, &repos, nil)
	call := <-rpcCall.Done
	return call.Reply.(*map[string]*Repo), call.Error
}

func (client *RpcClient) DeleteRepo(key string) error {
	rpcCall := client.Go("Afind.DeleteRepo", nil, nil, nil)
	call := <-rpcCall.Done
	return call.Error
}
