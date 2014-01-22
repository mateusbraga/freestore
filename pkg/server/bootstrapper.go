package server

import (
	"log"
	"net"
	"net/rpc"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"

	"github.com/cznic/kv"
)

var (
	currentView = view.New()

	listener    net.Listener
	thisProcess view.Process

	db *kv.DB

	useConsensus bool
)

func Run(bindAddr string, initialView *view.View, useConsensusArg bool) {
	// init listener
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on address:", listener.Addr())

	// init thisProcess
	thisProcess = view.Process{listener.Addr().String()}

	// init currentView
	currentView.Set(initialView)
	log.Println("Initial View:", currentView)

	// init storage
	initStorage()

	// init useConsensus
	useConsensus = useConsensusArg

	// Enable operations or join View
	if currentView.HasMember(thisProcess) {
		register.mu.Unlock() // Enable r/w operations
	} else {
		// try to update currentView with the first member
		getCurrentView(currentView.GetMembers()[0])
		// join the view
		Join()
	}

	// Start server
	rpc.Accept(listener)
}

func initStorage() {
	var err error
	db, err = kv.CreateMem(new(kv.Options))
	if err != nil {
		log.Fatalln("initStorage error:", err)
	}
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
