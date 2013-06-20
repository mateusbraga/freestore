package main

import (
    "github.com/ha/doozer"
    "log"
    "net"
    "fmt"
)

//var doozerConn *doozer.Conn
var currentView interface{}

//var servers map[int]


func write(val int) {

}

func main() {
}

func init() {
    doozerConn, err := doozer.Dial("localhost:8046")
    if err != nil { log.Fatal(err)
    }

    rev, err := doozerConn.Rev()
    if err != nil {
        log.Fatal(err)
    }

    files, err := doozerConn.Getdir("/goft/servers", rev, 0, -1)
	if err != nil {
        log.Fatal(err)
	}
	log.Println(files)

	for _, file := range files {
        path := fmt.Sprintf("/goft/servers/%v", file)
        addr, rev, err := doozerConn.Get(path, nil)
        if err != nil {
            log.Fatal(err)
        }

        fmt.Println(string(addr))
        conn, err := net.Dial("tcp", string(addr))
        if err != nil {
            fmt.Println("falhou dial", err)
            err := doozerConn.Del(path, rev)
            if err != nil {
                log.Fatal(err)
            }
            continue
        }
        log.Println("Did it!")
        // do something with conn to get currentView
        err = conn.Close()
        if err != nil {
            log.Fatal(err)
        }
	}

    //get currentView from servers
}
