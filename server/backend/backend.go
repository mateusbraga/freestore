package backend

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"time"

	"mateusbraga/gotf/view"
)

var currentView view.View

var (
	listener net.Listener
)

func Run(port uint) {
	flag.Parse()

	var err error

	listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Listening on address:", listener.Addr())

	err = view.PublishAddr(listener.Addr().String())
	if err != nil {
		log.Fatal(err)
	}

	currentView = view.New()
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5000"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5001"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{":5002"}})

	//currentView = view.New()
	//currentView.AddUpdate(view.Update{view.Join, view.Process{listener.Addr().String()}})

	rpc.Accept(listener)

	time.Sleep(3 * time.Second)
}
