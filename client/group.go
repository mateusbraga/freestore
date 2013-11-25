package client

import (
	"log"
	"net/rpc"
	"os"

	"mateusbraga/freestore/view"
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
