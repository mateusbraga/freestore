package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"

    "mateusbraga/gotf"
)


var currentView gotf.View
var register int

func newRequest() {

}

func read() {

}

func handleConnection(conn net.Conn) {

}

type Request int

func (r *Request) GetCurrentView(anything *int, reply *gotf.View) error {
    *reply = currentView // May need locking
    return nil
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

    log.Println("Listening on address:", ln.Addr())

    err = gotf.PublishAddr(ln.Addr().String())
    if err != nil {
        log.Fatal(err)
    }

    currentView = gotf.NewView()
    currentView.AddUpdate(gotf.Update{gotf.Join, gotf.Process{ln.Addr().String()}})

    request := new(Request)
    rpc.Register(request)
    rpc.Accept(ln)
}

