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
	destinationView, viewRef := thisClient.ViewAndViewRef()

	// Send write request to all
	resultChan := make(chan RegisterMsg, destinationView.NumberOfMembers())
	go broadcastRead(destinationView, viewRef, resultChan)

	// Wait for quorum
	var failedTotal int
	var resultArray []RegisterMsg
	var finalValue RegisterMsg
	for {
		receivedValue := <-resultChan

		// count success or fail
		if receivedValue.Err != nil {
			if oldViewError, ok := receivedValue.Err.(*view.OldViewError); ok {
				if oldViewError.NewView.MoreUpdatedThan(destinationView) {
					log.Println("View updated during read quorum by process", receivedValue.process)
					thisClient.updateCurrentView(oldViewError.NewView)
					return thisClient.readQuorum()
				}
				// oldViewError.NewView is actually not more updated than current view, try again
				go sendRead(receivedValue.process, viewRef, resultChan)
				log.Printf("Process %v has old view %v\n", receivedValue.process, oldViewError.NewView)
				continue
			}

			//log.Println("+1 error to read:", err)
			failedTotal++
		} else {
			resultArray = append(resultArray, receivedValue)

			if receivedValue.Timestamp > finalValue.Timestamp {
				finalValue = receivedValue
			}
		}

		// check conditions. this is done here to handle when a quorum leaves
		// the system. In this case, most processes would fail but we should
		// wait for one (or all) that will tell the client the updated view.
		everyProcessReturned := len(resultArray)+failedTotal == destinationView.NumberOfMembers()
		systemFailed := everyProcessReturned && failedTotal > destinationView.NumberOfToleratedFaults()
		if systemFailed {
			// maybe all processes from the view left, try to get the new view
			if thisClient.couldGetNewView() {
				return thisClient.readQuorum()
			} else {
				return RegisterMsg{}, errors.New("Failed to get read quorum")
			}
		}

		if len(resultArray) == destinationView.QuorumSize() {
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
	destinationView, viewRef := thisClient.ViewAndViewRef()

	writeMsg.ViewRef = viewRef

	// Send write request to all
	resultChan := make(chan RegisterMsg, destinationView.NumberOfMembers())
	go broadcastWrite(destinationView, writeMsg, resultChan)

	// Wait for quorum
	var successTotal int
	var failedTotal int
	for {
		receivedValue := <-resultChan

		// count success or fail
		if receivedValue.Err != nil {
			if oldViewError, ok := receivedValue.Err.(*view.OldViewError); ok {
				if oldViewError.NewView.MoreUpdatedThan(destinationView) {
					log.Println("View updated during write quorum by process", receivedValue.process)
					thisClient.updateCurrentView(oldViewError.NewView)
					return thisClient.writeQuorum(writeMsg)
				}
				// oldViewError.NewView is actually not more updated than current view, try again
				go sendWrite(receivedValue.process, &writeMsg, resultChan)
				log.Printf("Process %v has old view %v\n", receivedValue.process, oldViewError.NewView)
				continue
			}

			//log.Println("+1 error to write:", err)
			failedTotal++
		} else {
			successTotal++
		}

		// check conditions. this is done here to handle when a quorum leaves
		// the system. In this case, most processes would fail but we should
		// wait for one (or all) that will tell the client the updated view.
		everyProcessReturned := successTotal+failedTotal == destinationView.NumberOfMembers()
		systemFailed := everyProcessReturned && failedTotal > destinationView.NumberOfToleratedFaults()
		if systemFailed {
			// maybe all processes from the view left, try to get the new view
			if thisClient.couldGetNewView() {
				return thisClient.writeQuorum(writeMsg)
			} else {
				return errors.New("Failed to get write quorum")
			}
		}

		if successTotal == destinationView.QuorumSize() {
			return nil
		}
	}
}

func (thisClient *Client) couldGetNewView() bool {
	view, err := thisClient.getFurtherViewsFunc()
	if err != nil {
		return false
	}

	if !view.MoreUpdatedThan(thisClient.View()) {
		return false
	}

	thisClient.updateCurrentView(view)
	return true
}

func sendRead(process view.Process, viewRef view.ViewRef, resultChan chan RegisterMsg) {
	var result RegisterMsg
	err := comm.SendRPCRequest(process, "RegisterService.Read", viewRef, &result)
	if err != nil {
		resultChan <- RegisterMsg{Err: err}
		return
	}
	result.process = process
	resultChan <- result
}

func broadcastRead(destinationView *view.View, viewRef view.ViewRef, resultChan chan RegisterMsg) {
	for _, process := range destinationView.GetMembers() {
		go sendRead(process, viewRef, resultChan)
	}
}

func sendWrite(process view.Process, writeMsg *RegisterMsg, resultChan chan RegisterMsg) {
	var result RegisterMsg
	err := comm.SendRPCRequest(process, "RegisterService.Write", writeMsg, &result)
	if err != nil {
		resultChan <- RegisterMsg{Err: err}
		return
	}
	result.process = process
	resultChan <- result
}

func broadcastWrite(destinationView *view.View, writeMsg RegisterMsg, resultChan chan RegisterMsg) {
	for _, process := range destinationView.GetMembers() {
		go sendWrite(process, &writeMsg, resultChan)
	}
}
