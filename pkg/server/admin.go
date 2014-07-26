package server

import (
	"log"
	"net/rpc"
)

type AdminService struct{}

func init() { rpc.Register(new(AdminService)) }

func (r *AdminService) Leave(anything struct{}, reply *struct{}) error {
    log.Println("AdminService requested to leave view")
	globalServer.leave()
	return nil
}
