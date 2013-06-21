package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"

	"mateusbraga/gotf"
)

var currentView gotf.View

var register Value

type Value struct {
	Value  int
	Timestamp int
}

var port uint

type Request int

func (r *Request) GetCurrentView(anything *int, reply *gotf.View) error {
	*reply = currentView // May need locking
	return nil
}

func (r *Request) Read(anything *int, reply *Value) error {
	*reply = register // May need locking
	return nil
}

func init() {
	flag.UintVar(&port, "port", 0, "Set port to listen to. Default is a random port")
	flag.UintVar(&port, "p", 0, "Set port to listen to. Default is a random port")
}

func main() {
	flag.Parse()

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Listening on address:", ln.Addr())

	err = gotf.PublishAddr(ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	currentView = gotf.NewView()
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{ln.Addr().String()}})

	register = Value{3, 1}

	request := new(Request)
	rpc.Register(request)
	rpc.Accept(ln)
}
