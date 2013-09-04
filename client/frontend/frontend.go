/*
frontend is the Front End to the servers and contain the quorum logic

TODO:
    An old view can make read/write fail with no necessity
        Add logic to update view according to the view reconfiguration

    Because we send so many requests at once using threads, we may have problems having buffer overflow on the socket buffer.
        We could limit the number of concurrent threads sending a request.
*/
package frontend

import (
	"errors"
	"log"
	"net/rpc"

	"mateusbraga/gotf/view"
)

var (
	DiffResultsErr = errors.New("Read divergence")
	ViewUpdatedErr = errors.New("View Updated")
)

type Value struct {
	// Read
	Value     int
	Timestamp int

	View view.View

	Err error
}

// Write implements the quorum write protocol.
func Write(v int) {
	readValue, err := basicReadQuorum()
	if err != nil {
		switch err {
		case ViewUpdatedErr:
			Write(v)
		case DiffResultsErr:
			// Ignore - we will write a new value anyway
		default:
			log.Fatal(err)
		}
	}

	writeValue := Value{}
	writeValue.Value = v
	writeValue.Timestamp = readValue.Timestamp + 1

	err = basicWriteQuorum(writeValue)
	if err != nil {
		switch err {
		case ViewUpdatedErr:
			Write(v)
		default:
			log.Fatal(err)
		}
	}
}

// basicWriteQuorum writes v to all processes on the currentView and return as soon as it gets the confirmation from a quorum
//
// If the view needs to be updated, it will update the view and return ViewUpdatedErr. Otherwise, returns nil
func basicWriteQuorum(v Value) error {
	resultChan := make(chan Value, currentView.N())
	errChan := make(chan error, currentView.N())

	v.View = view.New()
	v.View.Set(currentView)

	// Send write request to all
	for _, process := range currentView.GetMembers() {
		go writeProcess(process, v, resultChan, errChan)
	}

	// Get quorum
	var success int
	var failed int
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				switch err := resultValue.Err.(type) {
				default:
					log.Fatal("resultValue from writeProcess returned unexpected error of type: %T", err)
				case *view.OldViewError:
					log.Println("View updated during basic write quorum")
					currentView.Set(err.NewView)
					return ViewUpdatedErr
				}
			}

			success++
			if success == currentView.QuorumSize() {
				return nil
			}

		case err := <-errChan:
			log.Println("+1 failure to write:", err)
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

// writeProcess writes value to process and return the result through resultChan or an error through errChan
func writeProcess(process view.Process, value Value, resultChan chan Value, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var result Value
	err = client.Call("ClientRequest.Write", value, &result)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- result
}

// Read executes the quorum read protocol.
func Read() int {
	value, err := basicReadQuorum()
	if err != nil {
		switch err {
		case DiffResultsErr:
			log.Println("Found divergence: Going to 2nd phase of read protocol")

			err := basicWriteQuorum(value)
			if err != nil {
				switch err {
				case ViewUpdatedErr:
					return Read()
				default:
					log.Fatal(err)
				}
			}

		case ViewUpdatedErr:
			return Read()
		default:
			log.Fatal(err)
		}
	}

	return value.Value
}

// basicReadQuorum reads a Value from all members of the most updated currentView returning the most recent one. It decides which is the most recent one as soon as it gets a quorum
//
// Value is only valid if err == nil
// If the view needs to be updated, it will update the view and return ViewUpdatedErr.
// If any value returned by a process differ, it will return DiffResultsErr
func basicReadQuorum() (Value, error) {
	resultChan := make(chan Value, currentView.N())
	errChan := make(chan error, currentView.N())

	// Send read request to all
	for _, process := range currentView.GetMembers() {
		go readProcess(process, resultChan, errChan)
	}

	// Get quorum
	var failed int
	var resultArray []Value
	var finalValue Value
	finalValue.Timestamp = -1 // Make it negative to force value.Timestamp > finalValue.Timestamp
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				switch err := resultValue.Err.(type) {
				default:
					log.Fatal("resultValue from writeProcess returned unexpected error of type: %T", err)
				case *view.OldViewError:
					log.Println("View updated during basic read quorum")
					currentView.Set(err.NewView)
					return Value{}, ViewUpdatedErr
				}
			}

			resultArray = append(resultArray, resultValue)

			if resultValue.Timestamp > finalValue.Timestamp {
				finalValue = resultValue
			}

			if len(resultArray) == currentView.QuorumSize() {
				for _, val := range resultArray {
					if finalValue.Timestamp != val.Timestamp { // There are divergence on the processes
						return finalValue, DiffResultsErr
					}
				}
				return finalValue, nil
			}
		case err := <-errChan:
			log.Println("+1 failure to read:", err)
			failed++

			// currentView.F() needs an updated View, and we know we have an updated view when len(resultArray) > 0
			if len(resultArray) > 0 {
				if failed > currentView.F() {
					return Value{}, errors.New("Failed to get read quorun")
				}
			} else {
				if failed == currentView.N() {
					return Value{}, errors.New("Failed to get read quorun")
				}
			}
		}
	}
}

// readProcess reads the value on process and return an err through errChan or a result through resultChan
func readProcess(process view.Process, resultChan chan Value, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var value Value

	err = client.Call("ClientRequest.Read", currentView, &value)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- value
}
