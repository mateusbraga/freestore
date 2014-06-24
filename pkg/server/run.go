package server

import (
	"log"
	"net"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
	//TODO count faults, faults masked, errors, failures and put in expvar
)

var (
	listener     net.Listener
	thisProcess  view.Process
	useConsensus bool

	currentView *view.View
	currentViewMu sync.RWMutex
)

func Run(bindAddr string, initialView *view.View, useConsensusArg bool) {
    currentViewMu.Lock()

	// init global variables
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Panicln(err)
	}

	thisProcess = view.Process{listener.Addr().String()}

	currentView = initialView

	useConsensus = useConsensusArg

	// Enable operations or join View
	if currentView.HasMember(thisProcess) {
		register.mu.Unlock() // Enable r/w operations
	} else {
		// try to update currentView
		getCurrentViewLocked(currentView.GetMembers()...)

		if currentView.HasMember(thisProcess) {
			register.mu.Unlock() // Enable r/w operations
		} else {
			// join the view
			joinLocked()
		}
	}

    currentViewMu.Unlock()

	// Accept connections forever
	log.Println("Listening on address:", listener.Addr())
	rpc.Accept(listener)
}

// getCurrentViewLocked asks processes for the its current view and returns it.
func getCurrentViewLocked(processes ...view.Process) {
	for _, loopProcess := range processes {
		var receivedView *view.View
		err := comm.SendRPCRequest(loopProcess, "RegisterService.GetCurrentView", 0, &receivedView)
		if err != nil {
			continue
		}

		if receivedView.Equal(currentView) {
			return
		}

		updateCurrentViewLocked(receivedView)
		return
	}

	log.Fatalln("Failed to get current view from processes", processes)
}

func updateCurrentViewLocked(newView *view.View) {
	if !newView.MoreUpdatedThan(currentView) {
	    // comment these log messages; they are just for debugging
        if newView.LessUpdatedThan(currentView) {
            log.Println("WARNING: Tried to Update current view with a less updated view")
        } else {
            log.Println("WARNING: Tried to Update current view with the same view")
        }
		return
	}

	currentView = newView
	log.Printf("CurrentView updated to: %v, ref: %v\n", currentView, currentView.ViewRef)
}
