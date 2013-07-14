/*
Package group implements the Group Management logic
*/
package group

import (
	"expvar"
	"log"
	"net/rpc"

	"mateusbraga/gotf/view"
)

var (
	CurrentView view.View
)

// GetCurrentViewClient asks process for the CurrentView
func GetCurrentView(process view.Process) {
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

	CurrentView.Set(newView)
	log.Println("Got new current view:", CurrentView)
}

func init() {
	//addr, err := view.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}

	//GetCurrentView(view.Process{addr})

	CurrentView = view.New()
	CurrentView.AddUpdate(view.Update{view.Join, view.Process{":5000"}})
	CurrentView.AddUpdate(view.Update{view.Join, view.Process{":5001"}})
	//CurrentView.AddUpdate(view.Update{view.Join, view.Process{":5002"}})

	expvar.Publish("CurrentView", CurrentView)
}
