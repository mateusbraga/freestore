package client

import (
	"log"
	"os"

	"github.com/mateusbraga/freestore/comm"
	"github.com/mateusbraga/freestore/view"
)

var (
	currentView view.View
)

func init() {
	currentView = view.New()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	if hostname == "MateusPc" {
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

	log.Printf("Updating view from %v to %v\n", &currentView, &newView)
	currentView.Set(&newView)
}
