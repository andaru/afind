package main

import "github.com/andaru/afind"

func (c *ctx) Search(request afind.SearchRequest) (*afind.SearchResponse, error) {
	return c.rpcClient.Search(request)
}
