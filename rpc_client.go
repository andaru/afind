package afind

import (
	"fmt"
	"net/rpc"
)

type RpcClient struct {
	*rpc.Client
}

func NewRpcClient(address string) (*RpcClient, error) {
	fmt.Println(address)
	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		return nil, err
	}
	return &RpcClient{client}, nil
}

func (client *RpcClient) Index(request IndexRequest) (*IndexResponse, error) {
	indexResponse := newIndexResponse()
	err := client.Call("Index", request, indexResponse)
	return indexResponse, err
}

func (client *RpcClient) SearchRepo(key string, request SearchRequest) (
	*SearchRepoResponse, error) {
	sr := newSearchRepoResponse()
	err := client.Call("SearchRepo", request, sr)
	return sr, err
}

func (client *RpcClient) Search(request SearchRequest) (*SearchResponse, error) {
	sr := newSearchResponse()
	err := client.Call("Search", request, sr)
	return sr, err
}
