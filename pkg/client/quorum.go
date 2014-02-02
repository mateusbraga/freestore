package client

import (
	"errors"
	"log"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

// diffResultsErr is returned by readQuorum if not all answers from the servers were the same. Returned to indicate that Read() should do 2nd phase of the read protocol.
var diffResultsErr = errors.New("Read Divergence")

// readQuorum asks for the register value from all members of the view, returning the most recent one after it receives answers from a majority.
// If the view needs to be updated, it will return a *view.OldViewError.  If values returned by the processes differ, it will return diffResultsErr.
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

// writeQuorum tries to write the value in writeMsg to the register of all processes on the view, returning when it gets confirmation from a majority.
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
