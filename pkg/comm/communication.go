package comm

import (
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	openConnections   map[view.Process]*rpc.Client
	openConnectionsMu sync.Mutex
)

func init() {
	openConnections = make(map[view.Process]*rpc.Client)
}

func SendRPCRequest(process view.Process, serviceMethod string, args interface{}, reply interface{}) error {
	openConnectionsMu.Lock()
	defer openConnectionsMu.Unlock()

	var err error
	client, ok := openConnections[process]
	if !ok {
		client, err = rpc.Dial("tcp", process.Addr)
		if err != nil {
			return err
		}
		openConnections[process] = client
	}

	err = client.Call(serviceMethod, args, reply)
	if err != nil {
		if err == rpc.ErrShutdown {
			return SendRPCRequest(process, serviceMethod, args, reply)
		} else {
			return err
		}
	}

	return nil
}
