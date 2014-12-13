package api

import (
	"net/rpc"
)

func NewRpcClient(addr string) (c *rpc.Client, err error) {
	return rpc.Dial("tcp", addr)
}
