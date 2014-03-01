package server

import (
	"log"
	"net/rpc"
)

type ControllerRequest int

func (r *ControllerRequest) Terminate(anything bool, reply *bool) error {
	log.Println("Terminating...")

	// this causes an abrupt termination because rpc.Accept will do a log.Fatal
	err := listener.Close()
	if err != nil {
		return err
	}
	return nil
}

func (r *ControllerRequest) Ping(anything bool, reply *bool) error {
	return nil
}

func (r *ControllerRequest) Leave(anything bool, reply *bool) error {
	log.Println("ControllerRequest to leave view")
	Leave()
	return nil
}

func init() {
	rpc.Register(new(ControllerRequest))
}
