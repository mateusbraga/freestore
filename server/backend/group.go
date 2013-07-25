package backend

import (
	"mateusbraga/gotf/view"
)

var currentView view.View

func init() {
	currentView = view.New()
	currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5000"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5001"}})
	currentView.AddUpdate(view.Update{view.Join, view.Process{"[::]:5002"}})

	//currentView = view.New()
	//currentView.AddUpdate(view.Update{view.Join, view.Process{listener.Addr().String()}})
}
