package client

import (
	"log"
	"os"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	currentView = view.New()
)

// get current view
func init() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	if hostname == "mabr" {
		getCurrentView(view.Process{"[::]:5000"})
	} else {
		getCurrentView(view.Process{"10.1.1.2:5000"})
	}
}

// getCurrentView asks process for the currentView
func getCurrentView(process view.Process) {
	var newView view.View
	err := comm.SendRPCRequest(process, "ClientRequest.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatalln("ERROR getCurrentView:", err)
		return
	}

	updateCurrentView(&newView)
}

func updateCurrentView(newView *view.View) {
	log.Printf("Updating view from %v to %v\n", currentView, newView)
	currentView.Set(newView)
}
