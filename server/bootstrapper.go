package server

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"

	"github.com/cznic/kv"

	"github.com/mateusbraga/freestore/comm"
	"github.com/mateusbraga/freestore/view"
)

var (
	listener    net.Listener
	thisProcess view.Process
	currentView view.View

	db *kv.DB

	useConsensus bool
)

func Run(bindAddr string, join bool, master string, useConsensusArg bool, numberOfServers int) {
	var err error

	listener, err = net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on address:", listener.Addr())

	initThisProcess()
	initCurrentView(master, numberOfServers)
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

func initThisProcess() {
	thisProcess = view.Process{listener.Addr().String()}
}

func init() {
	currentView = view.New()
}

func initCurrentView(master string, numberOfServers int) {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	initialView := make(map[view.Process]bool)
	if hostname == "MateusPc" {
		for i := 0; i < numberOfServers; i++ {
			initialView[view.Process{fmt.Sprintf("[::]:500%d", i)}] = true
		}
	} else {
		for i := 0; i < numberOfServers; i++ {
			initialView[view.Process{fmt.Sprintf("10.1.1.%d:5000", i+2)}] = true
		}
	}

	if initialView[thisProcess] {
		for process, _ := range initialView {
			currentView.AddUpdate(view.Update{view.Join, process})
		}
	} else {
		getCurrentView(view.Process{master})
	}

	log.Println("Init current view:", currentView)
}

// GetCurrentViewClient asks process for the currentView
func getCurrentView(process view.Process) {
	var newView view.View
	err := comm.SendRPCRequest(process, "ClientRequest.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatalln("ERROR: getCurrentView:", err)
		return
	}

	currentView.Set(&newView)
}
