/*
This is a Quorum client

*/
package main

import (
	"errors"
	"expvar"
	"fmt"
	"log"
	"net/rpc"

	"mateusbraga/gotf"
)

var currentView gotf.View

var (
	DiffResultsErr = errors.New("DIFFERENT RESULTS")
	ViewUpdatedErr = errors.New("View Updated")
)

type Value struct {
	// Read
	Value     int
	Timestamp int

	// Write
	Result bool

	View gotf.View

	Err error
}

// Write implements the quorum write protocol.
func Write(value Value) {
	readValue, err := basicReadQuorum()
	if err != nil {
		switch err {
		case ViewUpdatedErr:
			Write(value)
		case DiffResultsErr:
			// Ignore - we will write a new value
		default:
			log.Fatal(err)
		}
	}

	value.Timestamp = readValue.Timestamp + 1

	err = basicWriteQuorum(value)
	if err != nil {
		switch err {
		case ViewUpdatedErr:
			Write(value)
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

	v.View.Set(currentView)

	// Send write request to all
	for _, process := range currentView.GetMembers() {
		go writeProcess(process, v, resultChan, errChan)
	}

	// Get quorum
	var n int
	for {
		select {
		case resultValue := <-resultChan:
			if resultValue.Err != nil {
				switch err := resultValue.Err.(type) {
				default:
					log.Fatal("resultValue returned error of type: %T", err)
				case *gotf.OldViewError:
					log.Println("VIEW UPDATED")
					currentView.Set(err.NewView)
					return ViewUpdatedErr
				}
			}

			if resultValue.Result {
				n++
				if n >= currentView.QuorunSize() {
					return nil
				}
			}
		case err := <-errChan:
			log.Fatal(err)
		}
	}
}

// writeProcess writes value to process and return the result through resultChan or an error through errChan
func writeProcess(process gotf.Process, value Value, resultChan chan Value, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var result Value
	err = client.Call("Request.Write", value, &result)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- result
}

// Read executes the quorum read protocol.
func Read() Value {
	value, err := basicReadQuorum()
	if err != nil {
		switch err {
		case DiffResultsErr:
			fmt.Println("Going to 2nd phase - read")

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

	return value
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
	var finalValue Value
	finalValue.Timestamp = -1 // Make it negative to force value.Timestamp > finalValue.Timestamp
	resultArray := make([]Value, 0)
	for {
		select {
		case value := <-resultChan:
			if value.Err != nil {
				switch err := value.Err.(type) {
				default:
					log.Fatal("Results value returned error of type: %T", err)
				case *gotf.OldViewError:
					log.Println("VIEW UPDATED")
					currentView.Set(err.NewView)
					return Value{}, ViewUpdatedErr
				}
			}

			resultArray = append(resultArray, value)

			if value.Timestamp > finalValue.Timestamp {
				finalValue = value
			}

			if len(resultArray) >= currentView.QuorunSize() {
				for _, val := range resultArray {
					if finalValue.Timestamp != val.Timestamp { // There are divergence on the processes
						return finalValue, DiffResultsErr
					}
				}
				return finalValue, nil
			}
		case err := <-errChan:
			log.Fatal(err)
		}
	}
}

// readProcess reads the value on process and return an err through errChan or a result through resultChan
func readProcess(process gotf.Process, resultChan chan Value, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var value Value

	err = client.Call("Request.Read", currentView, &value)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- value
}

// GetCurrentViewClient asks process for the currentView
func GetCurrentView(process gotf.Process) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	var newView gotf.View
	client.Call("Request.GetCurrentView", 0, &newView)
	if err != nil {
		log.Fatal(err)
	}

	currentView.Set(newView)
	fmt.Println("New Current View:", currentView)
}

func main() {
	fmt.Println(" ---- Start ---- ")
	finalValue := Read()
	fmt.Println("Final Read value:", finalValue)

	fmt.Println(" ---- Start 2 ---- ")
	finalValue = Value{}
	finalValue.Value = 5
	Write(finalValue)

	fmt.Println(" ---- Start 3 ---- ")
	finalValue = Read()
	fmt.Println("Final Read value:", finalValue)
}

func init() {
	//addr, err := gotf.GetRunningServer()
	//if err != nil {
	//log.Fatal(err)
	//}

	//GetCurrentView(gotf.Process{addr})

	currentView = gotf.NewView()
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5000"}})
	currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5001"}})
	//currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{":5002"}})

	expvar.Publish("CurrentView", currentView)
}
