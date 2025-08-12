package ethrpc

import (
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/geoffreyhinton/goeth_self_training/ethpub"
	"github.com/geoffreyhinton/goeth_self_training/ethutil"
)

type JsonRpcServer struct {
	quit     chan bool
	listener net.Listener
	ethp     *ethpub.PEthereum
}

func (s *JsonRpcServer) exitHandler() {
out:
	for {
		select {
		case <-s.quit:
			s.listener.Close()
			break out
		}
	}

	ethutil.Config.Log.Infoln("[JSON] Shutdown JSON-RPC server")
}

func (s *JsonRpcServer) Stop() {
	close(s.quit)
}

func (s *JsonRpcServer) Start() {
	ethutil.Config.Log.Infoln("[JSON] Starting JSON-RPC server")
	go s.exitHandler()
	rpc.Register(&EthereumApi{ethp: s.ethp})
	rpc.HandleHTTP()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			ethutil.Config.Log.Infoln("[JSON] Error starting JSON-RPC:", err)
			break
		}
		ethutil.Config.Log.Debugln("[JSON] Incoming request.")
		go jsonrpc.ServeConn(conn)
	}
}

func NewJsonRpcServer(ethp *ethpub.PEthereum, port int) (*JsonRpcServer, error) {
	sport := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp", sport)
	if err != nil {
		return nil, err
	}

	return &JsonRpcServer{
		listener: l,
		quit:     make(chan bool),
		ethp:     ethp,
	}, nil
}
