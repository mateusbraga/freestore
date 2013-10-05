package frontend

import (
	//"expvar"
	"log"
	"net/rpc"

	"mateusbraga/gotf/freestore/view"
)

var (
	currentView view.View
)

func init() {
	//addr, err := view.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}

	//GetCurrentView(view.Process{addr})

	currentView = view.New()
	getCurrentView(view.Process{"[::]:5000"})

	//currentView.AddUpdate(view.Update{view.Join, view.Process{":5000"}})
	//currentView.AddUpdate(view.Update{view.Join, view.Process{":5001"}})
	//currentView.AddUpdate(view.Update{view.Join, view.Process{":5002"}})

	//expvar.Publish("currentView", currentView)
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

	currentView.Set(&newView)
	log.Println("Got new current view:", currentView)
}
