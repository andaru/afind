package api

import (
	"net"
	"net/rpc"

	"time"
)

const (
	EPIndexer  = "Indexer"
	EPSearcher = "Searcher"
	EPRepos    = "Repos"
)

type RpcServer struct {
	*baseServer

	l      net.Listener
	server *rpc.Server
}

func NewRpcServer(l net.Listener, b *baseServer) *RpcServer {
	return &RpcServer{b, l, rpc.NewServer()}
}

func (s *RpcServer) Close() error {
	return s.l.Close()
}

func (s *RpcServer) CloseNoErr() {
	_ = s.l.Close()
}

func (s *RpcServer) Register() {
	if s.repos == nil || s.indexer == nil || s.searcher == nil {
		panic("server must be setup prior to Register being called")
	}
	_ = s.server.RegisterName(EPRepos, &reposServer{s.repos})
	_ = s.server.RegisterName(EPIndexer, &indexServer{&s.config, s.repos, s.indexer})
	_ = s.server.RegisterName(EPSearcher, &searchServer{&s.config, s.repos, s.searcher})
}

func (s *RpcServer) Serve() error {
	defer s.CloseNoErr()
	delay := time.Duration(3 * time.Millisecond)
	max := 1 * time.Second
	for {
		rwc, err := s.l.Accept()
		if err != nil {
			e, ok := err.(net.Error)
			if ok && e.Temporary() {
				if delay *= 2; delay > max {
					delay = max
				}
				time.Sleep(delay)
				continue
			}
			return e
		}
		delay = 0
		go s.server.ServeConn(rwc)
	}
}
