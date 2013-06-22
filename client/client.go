package main

import (
    "log"
    "net/rpc"
    "fmt"

    "mateusbraga/gotf"
)

var currentView gotf.View

type Value struct {
    // Read
	Value  int
	Timestamp int

    // Write
    Result bool

	View gotf.View
	Process gotf.Process
}

// WriteValue writes value to process and return the result through resultChan or an error through errChan
func WriteValue(process gotf.Process, value Value, resultChan chan Value, errChan chan error) {
    client, err := rpc.Dial("tcp", process.Addr)
    if err != nil {
        errChan <- err
        return
    }
    defer client.Close()

    fmt.Println("Connected to", process.Addr)

    var result bool
    err = client.Call("Request.Write", value, &result)
    if err != nil {
        errChan <- err
        return
    }

    fmt.Println("Result", result)

    resultChan <- Value{Result:result, Process:process}
}

// WriteQuorumValue writes value to all processes on the currentView and return as soon as it gets the confirmation from a quorum
func WriteQuorumValue(value Value) {
    value.View = currentView
    resultChan := make(chan Value, len(currentView.Members))
    errChan := make(chan error, len(currentView.Members))

    // Send write request to all
    for process, _ := range currentView.Members {
        go WriteValue(process, value, resultChan, errChan)
    }

    // Get quorum
    var n int
    for {
        select {
        case i := <-resultChan:
            if i.Result {
                n++
                fmt.Println("got result number:", n)

                if n >= currentView.QuorunSize() {
                    return
                }
            }
        case err := <-errChan:
            log.Println("ERROR: ", err);
            if err.Error() == "OLD_VIEW" {
                go WriteQuorumValue(value)
                return
            }
        }
    }
}

// Read reads the value on process and return an err through errChan or a result through resultChan
func Read(process gotf.Process, resultChan chan Value, errChan chan error) {
    client, err := rpc.Dial("tcp", process.Addr)
    if err != nil {
        errChan <- err
        return
    }
    defer client.Close()

    fmt.Println("Connected to", process.Addr)

    var value Value
    err = client.Call("Request.Read", currentView, &value)
    if err != nil {
        errChan <- err
        return
    }

    value.Process = process
    fmt.Println("Read value:", value)

    resultChan <- value
}

// ReadQuorum reads a Value from all members of the most updated currentView returning the most recent one. It decides which is the most recent one as soon as it gets a quorum
// It writes back the most recent value if any process is not updated
func ReadQuorum() Value {
    resultChan := make(chan Value, len(currentView.Members))
    errChan := make(chan error, len(currentView.Members))

    // Send read request to all
    for process, _ := range currentView.Members {
        go Read(process, resultChan, errChan)
    }

    // Get quorum
    var finalValue Value
    finalValue.Timestamp = -1 // Make it negative to force i.Timestamp > finalValue.Timestamp
    resultArray := make([]Value, 0)
    for {
        select {
        case i := <-resultChan:
            resultArray = append(resultArray, i)
            fmt.Println("got result number:", len(resultArray))

            if i.Timestamp > finalValue.Timestamp {
                finalValue = i
            }

            if len(resultArray) >= currentView.QuorunSize() {
                fmt.Println("got quorum:")

                for _, val := range resultArray {
                    if finalValue.Timestamp != val.Timestamp { // There are divergence on the processes
                        fmt.Println("divergence: 2nd phase")
                        WriteQuorumValue(finalValue)
                        break
                    }
                }
                return finalValue
            }
        case err := <-errChan:
            log.Println("ERROR: ", err);
            if err.Error() == "OLD_VIEW" {
                return ReadQuorum()
            }
        }
    }
}

// GetCurrentViewClient asks process for the currentView
func GetCurrentView(process gotf.Process) {
    client, err := rpc.Dial("tcp", process.Addr)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    fmt.Println("Connected to", process.Addr)

    client.Call("Request.GetCurrentView", 0, &currentView)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("New CurrentView:", currentView)
}

func main() {
    finalValue := ReadQuorum()
    fmt.Println("Final Read value:", finalValue)
}

func init() {
    addr, err := gotf.GetRunningServer()
    if err != nil {
        log.Fatal(err)
    }

    GetCurrentView(gotf.Process{addr})
}
