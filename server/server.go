package main

import (
	"fmt"
	"log"
	"net"

    "github.com/ha/doozer"
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
	//var port int

	//if len(os.Args) != 2 {
	//log.Fatal("USAGE: server port")
	//}
	//port, err := strconv.Atoi(os.Args[1])
	//if err != nil {
	//log.Fatal("Invalid port: ", err)
	//}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Using address:", ln.Addr())
	publishAddr(ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Failed to accept connection: ", err)
			continue
		}
		go handleConnection(conn)
	}
}

func publishAddr(addr string) {
	doozerConn, err := doozer.Dial("localhost:8046")
	if err != nil {
		log.Fatal(err)
	}

    rev, err := doozerConn.Rev()
    if err != nil {
        log.Fatal(err)
    }

    files, err := doozerConn.Getdir("/goft/servers/", rev, 0, -1)
	if err != nil {
        if err.Error() != "NOENT" {
            log.Fatal(err)
        }
	}

    id := len(files)
    _, err = doozerConn.Set(fmt.Sprintf("/goft/servers/%d", id), 0,  []byte(addr)) 
    for err != nil && err.Error() == "REV_MISMATCH" {
        id++
        _, err = doozerConn.Set(fmt.Sprintf("/goft/servers/%d", id), 0,  []byte(addr)) 
    }
    if err != nil {
        log.Fatal(err)
    }

    //log.Println(id)
    //log.Println(newRev)
}
