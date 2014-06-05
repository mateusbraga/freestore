package server

import (
	"log"
	"net"
	"net/rpc"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
	//TODO count faults, faults masked, errors, failures and put in expvar
)

var (
	listener     net.Listener
	thisProcess  view.Process
	useConsensus bool
	currentView = view.NewCurrentView()
)

func Run(bindAddr string, initialView *view.View, useConsensusArg bool) {
	// init global variables
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Panicln(err)
	}

	thisProcess = view.Process{listener.Addr().String()}

	currentView.Update(initialView)

	useConsensus = useConsensusArg

	// Enable operations or join View
	if currentView.View().HasMember(thisProcess) {
		register.mu.Unlock() // Enable r/w operations
	} else {
		// try to update currentView
		getCurrentView(currentView.View().GetMembers()...)

		if currentView.View().HasMember(thisProcess) {
			register.mu.Unlock() // Enable r/w operations
		} else {
			// join the view
			Join()
		}
	}

	// Accept connections forever
	log.Println("Listening on address:", listener.Addr())
	rpc.Accept(listener)
}

// GetCurrentView asks processes for the its current view and returns it.
func getCurrentView(processes ...view.Process) {
	for _, loopProcess := range processes {
		var receivedView *view.View
		err := comm.SendRPCRequest(loopProcess, "RegisterService.GetCurrentView", 0, &receivedView)
		if err != nil {
			continue
		}

		if receivedView.Equal(currentView.View()) {
			return
		}

		currentView.Update(receivedView)
		return
	}

	log.Fatalln("Failed to get current view from processes", processes)
}
