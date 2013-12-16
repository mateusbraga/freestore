package comm

import (
	"mateusbraga/freestore/view"
	"net/rpc"
)

var (
	openConnections map[view.Process]*rpc.Client
)

func init() {
	openConnections = make(map[view.Process]*rpc.Client)
}

func SendRPCRequest(process view.Process, serviceMethod string, args interface{}, reply interface{}) error {
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
