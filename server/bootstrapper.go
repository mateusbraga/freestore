package server

import (
	"fmt"
	"log"
	"net"
	"net/rpc"

	"github.com/cznic/kv"

	"mateusbraga/gotf/freestore/view"
)

var (
	listener    net.Listener
	thisProcess view.Process
	db          *kv.DB

	useConsensus bool
)

func Run(port uint, join bool, master string, useConsensusArg bool) {
	var err error

	listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on address:", listener.Addr())

	thisProcess = view.Process{listener.Addr().String()}

	initCurrentView(master)
	initStorage()
	useConsensus = useConsensusArg

	if currentView.HasMember(thisProcess) {
		register.mu.Unlock() // Enable r/w operations
	} else {
		if join {
			Join()
		}
	}

	rpc.Accept(listener)
}

func initStorage() {
	var err error
	db, err = kv.CreateMem(new(kv.Options))
	if err != nil {
		log.Fatalln("initStorage error:", err)
	}
}
