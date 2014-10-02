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

func (client *RpcClient) SearchRepo(key string, request SearchRequest) (
	*SearchRepoResponse, error) {
	sr := newSearchRepoResponse()
	err := client.Call("Afind.SearchRepo", request, sr)
	return sr, err
}

func (client *RpcClient) Search(request SearchRequest) (*SearchResponse, error) {
	sr := newSearchResponse()
	err := client.Call("Afind.Search", request, sr)
	return sr, err
}

func (client *RpcClient) GetPrefixRepos(prefix string) (*Repos, error) {
	r := Repos{Repos: make(map[string]*Repo)}
	err := client.Call("Afind.GetPrefixRepos", prefix, &r)
	return &r, err
}
