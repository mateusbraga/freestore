package client

import (
	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

func GetCurrentView(process view.Process) (*view.View, error) {
	return sendGetCurrentView(process)
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
