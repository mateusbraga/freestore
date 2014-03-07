package server

import (
	"log"
	"net/rpc"
	"os"
)

type AdminService int

func init() { rpc.Register(new(AdminService)) }

func (r *AdminService) Terminate(anything bool, reply *bool) error {
	log.Println("AdminService request to terminate")
	os.Exit(0)
	return nil
}

func (r *AdminService) Ping(anything bool, reply *bool) error {
	return nil
}

func (r *AdminService) Leave(anything bool, reply *bool) error {
	log.Println("AdminService request to leave view")
	Leave()
	return nil
}
