package main

import (
	"fmt"

	"github.com/andaru/afind"
)

func (c *ctx) Search(request afind.SearchRequest) []string {
	results := make([]string, 0)
	sr, err := c.rpcClient.Search(request)
	if err != nil {
		fmt.Errorf("Error: %v\n", err)
	}
	fmt.Println(sr)
	return results
}
