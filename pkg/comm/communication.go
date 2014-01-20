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

func getClient(process view.Process) (*rpc.Client, error) {
	openConnectionsMu.Lock()
	defer openConnectionsMu.Unlock()

	var err error
	client, ok := openConnections[process]
	if !ok {
		client, err = rpc.Dial("tcp", process.Addr)
		if err != nil {
			return nil, err
		}
		openConnections[process] = client
	}

	return client, nil
}

func SendRPCRequest(process view.Process, serviceMethod string, args interface{}, reply interface{}) error {
	client, err := getClient(process)
	if err != nil {
		return nil
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
