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
	currentView = initialView.NewCopy()

	useConsensus = useConsensusArg

	// Enable operations or join View
	if currentView.HasMember(thisProcess) {
		register.mu.Unlock() // Enable r/w operations
	} else {
		// try to update currentView with the first member
		getCurrentView(currentView.GetMembers()...)
		// join the view
		Join()
	}

	// Accept connections forever
	rpc.Accept(listener)
}

// GetCurrentView asks processes for the its current view and returns it.
func getCurrentView(processes ...view.Process) {
	for _, loopProcess := range processes {
		var receivedView *view.View
		err := comm.SendRPCRequest(loopProcess, "ClientRequest.GetCurrentView", 0, &receivedView)
		if err != nil {
			log.Println(err)
			continue
		}

		currentView = receivedView.NewCopy()
		return
	}

	log.Fatalln("Failed to get current view from processes")
}
