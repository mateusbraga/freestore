/*
client is how clients should access the system


IMPROV: Because we send so many requests at once using threads, we may have problems having buffer overflow on the socket buffer.  We could limit the number of concurrent threads sending a request.
*/
package client

import (
	"errors"
	"log"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	diffResultsErr = errors.New("Read Divergence")
)

type RegisterMsg struct {
	Value     interface{}
	Timestamp int

	View *view.View

	Err error
}

// Write v on the system by running the quorum write protocol.
func Write(v interface{}) error {
	immutableCurrentView := currentView.NewCopy()

	readValue, err := basicReadQuorum(immutableCurrentView)
	if err != nil {
		// Expected: oldViewError or diffResultsErr
		if oldViewError, ok := err.(*view.OldViewError); ok {
			updateCurrentView(oldViewError.NewView)
			Write(v)
		} else if err == diffResultsErr {
			// Do nothing - we will write a new value anyway
		} else {
			return err
		}
	}

	writeMsg := RegisterMsg{}
	writeMsg.Value = v
	writeMsg.Timestamp = readValue.Timestamp + 1

	err = basicWriteQuorum(immutableCurrentView, writeMsg)
	if err != nil {
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic write quorum")
			updateCurrentView(oldViewError.NewView)
			return Write(v)
		} else {
			return err
		}
	}

	return nil
}

// basicWriteQuorum writes v to all processes on the view and returns when it gets confirmation from a quorum
//
// If the view needs to be updated, it will return the new view in a *view.OldViewError.
func basicWriteQuorum(view *view.View, writeMsg RegisterMsg) error {
	resultChan := make(chan RegisterMsg, view.N())
	errChan := make(chan error, view.N())

	writeMsg.View = view

	// Send write request to all
	for _, process := range view.GetMembers() {
		go writeProcess(process, writeMsg, resultChan, errChan)
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

			// view.F() needs an updated View, and we know we have an updated view when successTotal > 0
			if successTotal > 0 {
				if failedTotal > view.F() {
					return errors.New("failedTotal to get write quorun")
				}
			} else {
				if failedTotal == view.N() {
					return errors.New("failedTotal to get write quorun")
				}
			}
		}
	}
}

// writeProcess sends a write request with writeMsg to process and return the result through resultChan or an error through errChan
func writeProcess(process view.Process, writeMsg RegisterMsg, resultChan chan RegisterMsg, errChan chan error) {
	var reply RegisterMsg
	err := comm.SendRPCRequest(process, "ClientRequest.Write", writeMsg, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}

// Read executes the quorum read protocol.
func Read() (interface{}, error) {
	immutableCurrentView := currentView.NewCopy()

	readMsg, err := basicReadQuorum(immutableCurrentView)
	if err != nil {
		// Expected: oldViewError (will retry) or diffResultsErr (will write most current value to view).
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic read quorum")
			updateCurrentView(oldViewError.NewView)
			return Read()
		} else if err == diffResultsErr {
			log.Println("Found divergence: Going to 2nd phase of read protocol")
			return read2ndPhase(immutableCurrentView, readMsg)
		} else {
			return 0, err
		}
	}

	return readMsg.Value, nil
}

func read2ndPhase(immutableCurrentView *view.View, readMsg RegisterMsg) (interface{}, error) {
	err := basicWriteQuorum(immutableCurrentView, readMsg)
	if err != nil {
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic write quorum")
			updateCurrentView(oldViewError.NewView)
			return Read()
		} else {
			return 0, err
		}
	}
	return readMsg.Value, nil
}

// basicReadQuorum reads a RegisterMsg from all members of the view, returning the most recent one. It decides which is the most recent value as soon as it gets a quorum
//
// If the view needs to be updated, it will update the view in a *view.OldViewError.
// If values returned by the processes differ, it will return diffResultsErr
func basicReadQuorum(view *view.View) (RegisterMsg, error) {
	resultChan := make(chan RegisterMsg, view.N())
	errChan := make(chan error, view.N())

	// Send read request to all
	for _, process := range view.GetMembers() {
		go readProcess(process, view, resultChan, errChan)
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

			// view.F() needs an updated View, and we know we have an updated view when len(resultArray) > 0
			if len(resultArray) > 0 {
				if failedTotal > view.F() {
					return RegisterMsg{}, errors.New("Failed to get read quorun")
				}
			} else {
				if failedTotal == view.N() {
					return RegisterMsg{}, errors.New("Failed to get read quorun")
				}
			}
		}
	}
}

// readProcess sends a read request to process and return an err through errChan or a result through resultChan
func readProcess(process view.Process, immutableCurrentView *view.View, resultChan chan RegisterMsg, errChan chan error) {
	var reply RegisterMsg
	err := comm.SendRPCRequest(process, "ClientRequest.Read", immutableCurrentView, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}
