package client

import (
	"log"
	"net/rpc"

	"mateusbraga/freestore/view"
)

var (
	currentView view.View
)

func init() {
	currentView = view.New()
	getCurrentView(view.Process{"[::]:5000"})

	// Option 1: Static initial view
	//currentView.AddUpdate(view.Update{view.Join, view.Process{":5000"}})
	//currentView.AddUpdate(view.Update{view.Join, view.Process{":5001"}})
	//currentView.AddUpdate(view.Update{view.Join, view.Process{":5002"}})

	// Option 2: Get view on predefined location
	//addr, err := view.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}
	//GetCurrentView(view.Process{addr})
}

// getCurrentView asks process for the currentView
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

	log.Printf("Updating view from %v to %v\n", &currentView, &newView)
	currentView.Set(&newView)
}
