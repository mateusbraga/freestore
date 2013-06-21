package main

import (
    "log"
    "net/rpc"
    "fmt"

    "mateusbraga/gotf"
)

var currentView gotf.View

//var servers map[int]


func write(val int) {

}

func main() {
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
    fmt.Println("Connected to", addr)

    client.Call("Request.GetCurrentView", 0, &currentView)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Initial View:", currentView)
}
