/*
client is how clients should access the system


IMPROV: Because we send so many requests at once using threads, we may have problems having buffer overflow on the socket buffer.  We could limit the number of concurrent threads sending a request.
*/
package client

import (
	"errors"
	"log"
	"net/rpc"

	"mateusbraga/freestore/view"
)

var (
	diffResultsErr = errors.New("Read Divergence")
	viewUpdatedErr = errors.New("View Updated")
)

type RegisterMsg struct {
	Value     interface{}
	Timestamp int

	View view.View

	Err error
}

// Write v on the system by running the quorum write protocol.
func Write(v interface{}) error {
	readValue, err := basicReadQuorum()
	if err != nil {
		switch err {
		case viewUpdatedErr:
			Write(v)
		case diffResultsErr:
			// Do nothing - we will write a new value anyway
		default:
			return err
		}
	}

	writeMsg := RegisterMsg{}
	writeMsg.Value = v
	writeMsg.Timestamp = readValue.Timestamp + 1

	err = basicWriteQuorum(writeMsg)
	if err != nil {
		switch err {
		case viewUpdatedErr:
			Write(v)
		default:
			return err
		}
	}

	return nil
}

// basicWriteQuorum writes v to all processes on the currentView and return as soon as it gets the confirmation from a quorum
//
// If the view needs to be updated, it will update the view and return viewUpdatedErr. Otherwise, returns nil
func basicWriteQuorum(writeMsg RegisterMsg) error {
	resultChan := make(chan RegisterMsg, currentView.N())
	errChan := make(chan error, currentView.N())
	stopChan := make(chan bool, currentView.N())
	defer fillStopChan(stopChan, currentView.N())

	writeMsg.View = currentView.NewCopy()

	// Send write request to all
	for _, process := range currentView.GetMembers() {
		go writeProcess(process, writeMsg, resultChan, errChan, stopChan)
	}

	// Get quorum
	var success int
	var failed int
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				switch err := resultValue.Err.(type) {
				case *view.OldViewError:
					log.Println("View updated during basic write quorum")
					currentView.Set(&err.NewView)
					return viewUpdatedErr
				default:
					log.Fatalf("resultValue from writeProcess returned unexpected error: %v (%T)", err, err)
				}
			}

			success++
			if success == currentView.QuorumSize() {
				return nil
			}

		case err := <-errChan:
			log.Println("+1 error on write:", err)
			failed++

			// currentView.F() needs an updated View, and we know we have an updated view when success > 0
			if success > 0 {
				if failed > currentView.F() {
					return errors.New("Failed to get write quorun")
				}
			} else {
				if failed == currentView.N() {
					return errors.New("Failed to get write quorun")
				}
			}
		}
	}
}

// writeProcess sends a write request with writeMsg to process and return the result through resultChan or an error through errChan if stopChan is empty
func writeProcess(process view.Process, writeMsg RegisterMsg, resultChan chan RegisterMsg, errChan chan error, stopChan chan bool) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var result RegisterMsg
	call := client.Go("ClientRequest.Write", writeMsg, &result, nil)
	select {
	case <-call.Done:
		if call.Error != nil {
			errChan <- call.Error
		} else {
			resultChan <- result
		}
	case <-stopChan:
	}
}

// Read executes the quorum read protocol.
func Read() (interface{}, error) {
	readMsg, err := basicReadQuorum()
	if err != nil {
		switch err {
		case diffResultsErr:
			log.Println("Found divergence: Going to 2nd phase of read protocol")

			err := basicWriteQuorum(readMsg)
			if err != nil {
				switch err {
				case viewUpdatedErr:
					return Read()
				default:
					return 0, err
				}
			}

		case viewUpdatedErr:
			return Read()
		default:
			return 0, err
		}
	}

	return readMsg.Value, nil
}

// basicReadQuorum reads a RegisterMsg from all members of the most updated currentView returning the most recent one. It decides which is the most recent one as soon as it gets a quorum
//
// RegisterMsg is only valid if err == nil
// If the view needs to be updated, it will update the view and return viewUpdatedErr.
// If any value returned by a process differ, it will return diffResultsErr
func basicReadQuorum() (RegisterMsg, error) {
	resultChan := make(chan RegisterMsg, currentView.N())
	errChan := make(chan error, currentView.N())
	stopChan := make(chan bool, currentView.N())
	defer fillStopChan(stopChan, currentView.N())

	// Send read request to all
	for _, process := range currentView.GetMembers() {
		go readProcess(process, resultChan, errChan, stopChan)
	}

	// Get quorum
	var failed int
	var resultArray []RegisterMsg
	var finalValue RegisterMsg
	finalValue.Timestamp = -1 // Make it negative to force value.Timestamp > finalValue.Timestamp
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				switch err := resultValue.Err.(type) {
				case *view.OldViewError:
					log.Println("View updated during basic read quorum")
					currentView.Set(&err.NewView)
					return RegisterMsg{}, viewUpdatedErr
				default:
					log.Fatalf("resultValue from readProcess returned unexpected error: %v (%T)", err, err)
				}
			}

			resultArray = append(resultArray, resultValue)

			if resultValue.Timestamp > finalValue.Timestamp {
				finalValue = resultValue
			}

			if len(resultArray) == currentView.QuorumSize() {
				for _, val := range resultArray {
					if finalValue.Timestamp != val.Timestamp { // There are divergence on the processes
						return finalValue, diffResultsErr
					}
				}
				return finalValue, nil
			}
		case err := <-errChan:
			log.Println("+1 error on read:", err)
			failed++

			// currentView.F() needs an updated View, and we know we have an updated view when len(resultArray) > 0
			if len(resultArray) > 0 {
				if failed > currentView.F() {
					return RegisterMsg{}, errors.New("Failed to get read quorun")
				}
			} else {
				if failed == currentView.N() {
					return RegisterMsg{}, errors.New("Failed to get read quorun")
				}
			}
		}
	}
}

// readProcess sends a read request to process and return an err through errChan or a result through resultChan if stopChan is empty
func readProcess(process view.Process, resultChan chan RegisterMsg, errChan chan error, stopChan chan bool) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var readMsg RegisterMsg
	call := client.Go("ClientRequest.Read", currentView, &readMsg, nil)
	select {
	case <-call.Done:
		if call.Error != nil {
			errChan <- call.Error
		} else {
			resultChan <- readMsg
		}
	case <-stopChan:
	}
}

// fillStopChan send times * 'true' on stopChan. It is used to signal completion to readProcess and writeProcess.
func fillStopChan(stopChan chan bool, times int) {
	for i := 0; i < times; i++ {
		stopChan <- true
	}
}
