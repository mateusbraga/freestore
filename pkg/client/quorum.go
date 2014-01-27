package client

import (
	"errors"
	"log"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

// readQuorum reads a RegisterMsg from all members of the view, returning the most recent one. It decides which is the most recent value as soon as it gets a quorum
//
// If the view needs to be updated, it will update the view in a *view.OldViewError.
// If values returned by the processes differ, it will return diffResultsErr
func readQuorum(view *view.View) (RegisterMsg, error) {
	resultChan := make(chan RegisterMsg, view.N())
	errChan := make(chan error, view.N())

	// Send read request to all
	for _, process := range view.GetMembers() {
		go sendReadRequest(process, view, resultChan, errChan)
	}

	// Get quorum
	var failedTotal int
	var resultArray []RegisterMsg
	var finalValue RegisterMsg
	finalValue.Timestamp = -1 // Make it negative to force value.Timestamp > finalValue.Timestamp
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				return RegisterMsg{}, resultValue.Err
			}

			resultArray = append(resultArray, resultValue)

			if resultValue.Timestamp > finalValue.Timestamp {
				finalValue = resultValue
			}

			if len(resultArray) == view.QuorumSize() {
				for _, val := range resultArray {
					if finalValue.Timestamp != val.Timestamp { // There are divergence on the processes
						return finalValue, diffResultsErr
					}
				}
				return finalValue, nil
			}
		case err := <-errChan:
			log.Println("+1 error on read:", err)
			failedTotal++

			allFailed := failedTotal == view.N()
			mostFailedInspiteSomeSuccess := len(resultArray) > 0 && failedTotal > view.F()

			if mostFailedInspiteSomeSuccess || allFailed {
				return RegisterMsg{}, errors.New("Failed to get read quorun")
			}
		}
	}
}

// writeQuorum writes v to all processes on the view and returns when it gets confirmation from a quorum
//
// If the view needs to be updated, it will return the new view in a *view.OldViewError.
func writeQuorum(view *view.View, writeMsg RegisterMsg) error {
	resultChan := make(chan RegisterMsg, view.N())
	errChan := make(chan error, view.N())

	// Send write request to all
	for _, process := range view.GetMembers() {
		go sendWriteRequest(process, writeMsg, resultChan, errChan)
	}

	// Get quorum
	var successTotal int
	var failedTotal int
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				return resultValue.Err
			}

			successTotal++
			if successTotal == view.QuorumSize() {
				return nil
			}

		case err := <-errChan:
			log.Println("+1 error on write:", err)
			failedTotal++

			allFailed := failedTotal == view.N()
			mostFailedInspiteSomeSuccess := successTotal > 0 && failedTotal > view.F()

			if mostFailedInspiteSomeSuccess || allFailed {
				return errors.New("failedTotal to get write quorun")
			}
		}
	}
}

// ---------- Send functions -------------

// sendWriteRequest sends a write request with writeMsg to process and return the result through resultChan or an error through errChan
func sendWriteRequest(process view.Process, writeMsg RegisterMsg, resultChan chan RegisterMsg, errChan chan error) {
	var reply RegisterMsg
	err := comm.SendRPCRequest(process, "ClientRequest.Write", writeMsg, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}

// sendReadRequest sends a read request to process and return an err through errChan or a result through resultChan
func sendReadRequest(process view.Process, immutableCurrentView *view.View, resultChan chan RegisterMsg, errChan chan error) {
	var reply RegisterMsg
	err := comm.SendRPCRequest(process, "ClientRequest.Read", immutableCurrentView, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}
