package server

import (
	"log"
	"net"
	"net/rpc"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	listener     net.Listener
	thisProcess  view.Process
	useConsensus bool

	// currentView of this server cluster.
	currentView = view.New()
)

func Run(bindAddr string, initialView *view.View, useConsensusArg bool) {
	// init global variables
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on address:", listener.Addr())

	thisProcess = view.Process{listener.Addr().String()}

	log.Println("Initial View:", initialView)
	currentView.Set(initialView)

	useConsensus = useConsensusArg

	// Enable operations or join View
	if currentView.HasMember(thisProcess) {
		register.mu.Unlock() // Enable r/w operations
	} else {
		// try to update currentView with the first member
		getCurrentView(currentView.GetMembers()[0])
		// join the view
		err := Join()
		if err != nil {
			log.Println("Join:", err)
		}
	}

	// Accept connections forever
	rpc.Accept(listener)
}

// GetCurrentViewClient asks process for the currentView
func getCurrentView(process view.Process) {
	var newView *view.View
	err := comm.SendRPCRequest(process, "ClientRequest.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatalln("ERROR: getCurrentView:", err)
		return
	}

	currentView.Set(newView)
}
