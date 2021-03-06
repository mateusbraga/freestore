package client

import (
	"errors"
	"fmt"
	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

// GetCurrentView asks processes for the its current view and returns it.
func GetCurrentView(processes ...view.Process) (*view.View, error) {
	for _, loopProcess := range processes {
		var receivedView *view.View
		err := comm.SendRPCRequest(loopProcess, "RegisterService.GetCurrentView", struct{}{}, &receivedView)
		if err != nil {
			continue
		}

		return receivedView, nil
	}
	return nil, errors.New(fmt.Sprintf("Failed to get current view from any of the processes: %v", processes))
}
