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
	EPFinder   = "Finder"
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
	s.Quit()
	return s.l.Close()
}

func (s *RpcServer) CloseNoErr() {
	_ = s.Close()
}

func (s *RpcServer) Register() {
	if s.repos == nil || s.indexer == nil || s.searcher == nil {
		panic("server must be setup prior to Register being called")
	}
	_ = s.server.RegisterName(EPRepos, &reposServer{s.repos})
	_ = s.server.RegisterName(EPIndexer, &indexServer{&s.config, s.repos, s.indexer})
	_ = s.server.RegisterName(EPSearcher, &searchServer{&s.config, s.repos, s.searcher})
	_ = s.server.RegisterName(EPFinder, &findServer{&s.config, s.repos, s.finder})
}

func (s *RpcServer) Serve() error {
	defer s.CloseNoErr()
	startDelay := time.Duration(3 * time.Millisecond)
	delay := startDelay
	max := 5 * time.Second
	for {
		select {
		case <-s.quit:
			return nil
		default:
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
			delay = startDelay
			go s.server.ServeConn(rwc)
		}

	}
}
