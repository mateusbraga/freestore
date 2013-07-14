package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"runtime"
	"sync"
	"time"

	"mateusbraga/gotf/view"
)

var currentView view.View

var register Value

var port uint
var ln net.Listener

type Value struct {
	// Read
	Value     int
	Timestamp int

	View view.View
	Err  error

	mu sync.RWMutex
}

type ClientRequest int

func (r *ClientRequest) GetCurrentView(anything *int, reply *view.View) error {
	reply.Set(currentView)
	return nil
}

func (r *ClientRequest) Read(clientView view.View, reply *Value) error {
	register.mu.RLock()
	defer register.mu.RUnlock()

	if !clientView.Equal(currentView) {
		err := view.OldViewError{}
		err.OldView.Set(clientView)
		err.NewView.Set(currentView)
		reply.Err = err
	}

	reply.Value = register.Value
	reply.Timestamp = register.Timestamp

	return nil
}

func (r *ClientRequest) Write(value Value, reply *Value) error {
	register.mu.Lock()
	defer register.mu.Unlock()

	if !value.View.Equal(currentView) {
		err := view.OldViewError{}
		err.OldView.Set(value.View)
		err.NewView.Set(currentView)

		reply.Err = err
		return nil
	}

	if value.Timestamp > register.Timestamp {
		register.Value = value.Value
		register.Timestamp = value.Timestamp
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
	runtime.Gosched() // Just to reduce the chance of showing errors because it terminated too early
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

	err = view.PublishAddr(ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	currentView = view.New()
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5000"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5001"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5002"}})

	//currentView = view.New()
	//currentView.AddUpdate(view.Update{view.Join, view.Process{ln.Addr().String()}})

	register = Value{}
	register.Value = 3
	register.Timestamp = 1

	clientRequest := new(ClientRequest)
	controllerRequest := new(ControllerRequest)
	rpc.Register(clientRequest)
	rpc.Register(controllerRequest)
	rpc.Accept(ln)

	time.Sleep(3 * time.Second)
}
