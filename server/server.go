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
var port uint
var register Value


type Value struct {
    // Read
	Value  int
	Timestamp int

    // Write
    Result bool

	View gotf.View

	Err error
}

type Request int

func (r *Request) GetCurrentView(anything *int, reply *gotf.View) error {
	*reply = currentView // May need locking
	return nil
}

func (r *Request) Read(view gotf.View, reply *Value) error {
    *reply = register // May need locking

    if !view.Equal(currentView) {
        reply.Err = gotf.OldViewError{view, currentView}
    }

	return nil
}

func (r *Request) Write(value Value, reply *Value) error {
    if !value.View.Equal(currentView) {
        reply = new(Value)
        reply.Err = gotf.OldViewError{value.View, currentView}
        return nil
    }

    if value.Timestamp > register.Timestamp {
        register = value // May need locking
    } else {
        reply = new(Value)
        reply.Err = gotf.WriteOlderError{value.Timestamp, register.Timestamp}
        return nil
    }

    reply.Result = true

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
    currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5000"}})
    currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5001"}})
    currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5002"}})

	//currentView = gotf.NewView()
	//currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{ln.Addr().String()}})

	register = *new(Value)
	register.Value = 3
	register.Timestamp = 1

	request := new(Request)
	rpc.Register(request)
	rpc.Accept(ln)
}
