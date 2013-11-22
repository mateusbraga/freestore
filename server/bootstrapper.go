package server

import (
	"log"
	"net"
	"net/rpc"

	"github.com/cznic/kv"

	"mateusbraga/freestore/view"
)

var (
	listener    net.Listener
	thisProcess view.Process
	currentView view.View

	db *kv.DB

	useConsensus bool
)

func Run(bindAddr string, join bool, master string, useConsensusArg bool) {
	var err error

	//listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	listener, err = net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on address:", listener.Addr())

	initThisProcess()
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

func initThisProcess() {
	thisProcess = view.Process{listener.Addr().String()}
}

func init() {
	currentView = view.New()
}

func initCurrentView(master string) {
	if thisProcess.Addr == "[::]:5000" || thisProcess.Addr == "[::]:5001" || thisProcess.Addr == "[::]:5002" {
		currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5000"}})
		currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5001"}})
		currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5002"}})
	} else {
		getCurrentView(view.Process{master})
	}
	log.Println("Init current view:", currentView)
}

// GetCurrentViewClient asks process for the currentView
func getCurrentView(process view.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	var newView view.View
	client.Call("ClientRequest.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatal(err)
	}

	currentView.Set(&newView)
}
