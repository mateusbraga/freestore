package main

import (
    "log"
    "net"
    "fmt"
    "os"
    "strconv"
)

var currentView interface{}

var register int

func newRequest() {

}

func read() {

}

func handleConnection(conn net.Conn) {

}

func main() {
    var port int

    if len(os.Args) != 2 {
        log.Fatal("USAGE: server port")
    }
    port, err := strconv.Atoi(os.Args[1])
    if err != nil {
        log.Fatal("Invalid port: ", err)
    }

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
	    log.Fatal(err);
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
            log.Println("Failed to accept connection: ", err)
			continue
		}
		go handleConnection(conn)
	}
}
