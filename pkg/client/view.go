package client

import (
	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

// GetCurrentView asks process for the its current view and returns it.
func GetCurrentView(process ...view.Process) (*view.View, error) {
	var newView *view.View
	var err error
	for _, loopProcess := range process {
		newView, err = sendGetCurrentView(loopProcess)
		if err != nil {
			continue
		}
		return newView, nil
	}
	return nil, err
}

// sendGetCurrentView asks process for its currentView
func sendGetCurrentView(process view.Process) (*view.View, error) {
	var processCurrentView view.View
	err := comm.SendRPCRequest(process, "ClientRequest.GetCurrentView", 0, &processCurrentView)
	if err != nil {
		return nil, err
	}

	return &processCurrentView, nil
}
