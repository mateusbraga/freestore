package server

import (
	"log"
	"mateusbraga/gotf/freestore/view"
	"net/rpc"
)

var currentView view.View

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
