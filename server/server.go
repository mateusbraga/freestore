package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	"mateusbraga/gotf"
)

var currentView gotf.View

var register Value

var port uint
var ln net.Listener

type Value struct {
	// Read
	Value     int
	Timestamp int

	View gotf.View

	Err error

	mu sync.RWMutex
}

type ClientRequest int

func (r *ClientRequest) GetCurrentView(anything *int, reply *gotf.View) error {
	reply.Set(currentView)
	return nil
}

func (r *ClientRequest) Read(view gotf.View, reply *Value) error {
	register.mu.RLock()
	defer register.mu.RUnlock()

	if !view.Equal(currentView) {
		err := gotf.OldViewError{}
		err.OldView.Set(view)
		err.NewView.Set(currentView)
		reply.Err = err
	}

	*reply = register

	return nil
}

func (r *ClientRequest) Write(value Value, reply *Value) error {
	register.mu.Lock()
	defer register.mu.Unlock()

	if !value.View.Equal(currentView) {
		err := gotf.OldViewError{}
		err.OldView.Set(value.View)
		err.NewView.Set(currentView)

		reply.Err = err
		return nil
	}

	if value.Timestamp > register.Timestamp {
		register = value
	}

	return nil
}

type ControllerRequest int

func (r *ControllerRequest) Terminate(anything bool, reply *bool) error {
	log.Println("Terminating...")
	err := ln.Close()
	if err != nil {
		return err
	}
	return nil
}

func (r *ControllerRequest) Ping(anything bool, reply *bool) error {
	return nil
}

func init() {
	flag.UintVar(&port, "port", 0, "Set port to listen to. Default is a random port")
	flag.UintVar(&port, "p", 0, "Set port to listen to. Default is a random port")
}

func main() {
	flag.Parse()

	var err error

	ln, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Listening on address:", ln.Addr())

	err = gotf.PublishAddr(ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	currentView = gotf.NewView()
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5000"}})
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5001"}})
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5002"}})

	//currentView = gotf.NewView()
	//currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{ln.Addr().String()}})

	register = *new(Value)
	register.Value = 3
	register.Timestamp = 1

	clientRequest := new(ClientRequest)
	controllerRequest := new(ControllerRequest)
	rpc.Register(clientRequest)
	rpc.Register(controllerRequest)
	rpc.Accept(ln)

	time.Sleep(3 * time.Second)
}
