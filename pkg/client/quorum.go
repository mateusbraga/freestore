package client

import (
	"errors"
	"log"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

// diffResultsErr is returned by readQuorum if not all answers from the servers
// were the same. It is used to indicate that Read() should do 2nd phase of the
// read protocol.
var diffResultsErr = errors.New("Read Divergence")

// readQuorum asks for the register value of all members from the current view.
// It returns the most recent value after it receives answers from a majority.
// If the client's view needs to be updated, it will update it and retry.  If
// values returned by the processes differ, it will return diffResultsErr.
func (thisClient *Client) readQuorum() (RegisterMsg, error) {
	destinationView := thisClient.View()

	// Send write request to all
	resultChan := make(chan RegisterMsg, destinationView.NumberOfMembers())
	go broadcastRead(destinationView, resultChan)

	// Wait for quorum
	var failedTotal int
	var resultArray []RegisterMsg
	var finalValue RegisterMsg

	countError := func(err error) bool {
		log.Println("+1 error to read:", err)
		failedTotal++

		allFailed := failedTotal == destinationView.NumberOfMembers()
		mostFailedInspiteSomeSuccess := len(resultArray) > 0 && failedTotal > destinationView.NumberOfToleratedFaults()

		if mostFailedInspiteSomeSuccess || allFailed {
			return false
		}

		return true
	}

	countSuccess := func(receivedValue RegisterMsg) bool {
		resultArray = append(resultArray, receivedValue)

		if receivedValue.Timestamp > finalValue.Timestamp {
			finalValue = receivedValue
		}

		if len(resultArray) == destinationView.QuorumSize() {
			return true
		}
		return false
	}

	for {
		receivedValue := <-resultChan

		if receivedValue.Err != nil {
			if oldViewError, ok := receivedValue.Err.(*view.OldViewError); ok {
				log.Println("View updated during read quorum")
				thisClient.updateCurrentView(oldViewError.NewView)
				return thisClient.readQuorum()
			}

			stillOk := countError(receivedValue.Err)
			if !stillOk {
				return RegisterMsg{}, errors.New("Failed to get read quorun")
			}
			continue
		}

		done := countSuccess(receivedValue)
		if done {
			// Look for divergence on values received
			for _, val := range resultArray {
				if finalValue.Timestamp != val.Timestamp {
					return finalValue, diffResultsErr
				}
			}
			return finalValue, nil
		}
	}

}

// writeQuorum tries to write the value on writeMsg in the register of all
// processes on client's current view. It returns when it gets confirmation
// from a majority.  If the client's view needs to be updated, it will update
// it and retry.
func (thisClient *Client) writeQuorum(writeMsg RegisterMsg) error {
	destinationView := thisClient.View()

	writeMsg.View = destinationView

	// Send write request to all
	resultChan := make(chan RegisterMsg, destinationView.NumberOfMembers())
	go broadcastWrite(destinationView, writeMsg, resultChan)

	// Wait for quorum
	var successTotal int
	var failedTotal int

	countError := func(err error) bool {
		log.Println("+1 error to write:", err)
		failedTotal++

		allFailed := failedTotal == destinationView.NumberOfMembers()
		mostFailedInspiteSomeSuccess := successTotal > 0 && failedTotal > destinationView.NumberOfToleratedFaults()

		if mostFailedInspiteSomeSuccess || allFailed {
			return false
		}

		return true
	}

	countSuccess := func() bool {
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return true
		}
		return false
	}

	for {
		receivedValue := <-resultChan

		if receivedValue.Err != nil {
			if oldViewError, ok := receivedValue.Err.(*view.OldViewError); ok {
				log.Println("View updated during write quorum")
				thisClient.updateCurrentView(oldViewError.NewView)
				return thisClient.writeQuorum(writeMsg)
			}

			stillOk := countError(receivedValue.Err)
			if !stillOk {
				return errors.New("Failed to get write quorun")
			}
			continue
		}

		done := countSuccess()
		if done {
			return nil
		}
	}
}

func sendRead(process view.Process, destinationView *view.View, resultChan chan RegisterMsg) {
	var result RegisterMsg
	err := comm.SendRPCRequest(process, "ClientRequest.Read", destinationView, &result)
	if err != nil {
		resultChan <- RegisterMsg{Err: err}
		return
	}

	resultChan <- result
}

func broadcastRead(destinationView *view.View, resultChan chan RegisterMsg) {
	for _, process := range destinationView.GetMembers() {
		go sendRead(process, destinationView, resultChan)
	}
}

func sendWrite(process view.Process, writeMsg *RegisterMsg, resultChan chan RegisterMsg) {
	var result RegisterMsg
	err := comm.SendRPCRequest(process, "ClientRequest.Write", writeMsg, &result)
	if err != nil {
		resultChan <- RegisterMsg{Err: err}
		return
	}
	resultChan <- result
}

func broadcastWrite(destinationView *view.View, writeMsg RegisterMsg, resultChan chan RegisterMsg) {
	for _, process := range destinationView.GetMembers() {
		go sendWrite(process, &writeMsg, resultChan)
	}
}
