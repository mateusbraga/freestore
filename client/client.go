package main

import (
    "log"
    "net/rpc"
    "fmt"

    "mateusbraga/gotf"
)

var currentView gotf.View

type Value struct {
	Value  int
	Timestamp int
}

func ReadQuorum() Value {
    results := make(chan Value)

    for process, _ := range currentView.Members {
        go func () {
            client, err := rpc.Dial("tcp", process.Addr)
            if err != nil {
                log.Fatal(err)
            }
            defer client.Close()

            fmt.Println("Connected to", process.Addr)

            var i Value
            client.Call("Request.Read", 0, &i)
            if err != nil {
                log.Fatal(err)
            }
            fmt.Println("Read value:", i)

            results <- i
        }()
    }

    n := 0
    var finalValue Value

    for {
        select {
        case i := <-results:
            n++
            fmt.Println("got result number:", n)

            if i.Timestamp > finalValue.Timestamp {
                finalValue = i
            }

            if n >= currentView.QuorunSize() {
                fmt.Println("got quorum:")
                return finalValue
            }
        }
    }
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

    client, err := rpc.Dial("tcp", addr)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    fmt.Println("Connected to", addr)

    client.Call("Request.GetCurrentView", 0, &currentView)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Initial View:", currentView)
}
