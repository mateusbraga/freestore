//TODO make it a key value storage
package server

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"os"
	"sync"
	"time"

	"github.com/mateusbraga/freestore/pkg/view"
)

var register Value

//  ---------- RPC Requests -------------
type RegisterService int

func (r *RegisterService) Read(clientViewRef view.ViewRef, reply *Value) error {
	if clientViewRef != currentView.ViewRef() {
		log.Printf("Got old view with ViewRef: %v, sending new View %v with ViewRef: %v\n", clientViewRef, currentView.View(), currentView.ViewRef())
		reply.Err = view.OldViewError{NewView: currentView.View()}
		return nil
	}

	register.mu.RLock()
	defer register.mu.RUnlock()

	reply.Value = register.Value
	reply.Timestamp = register.Timestamp

	throughput++

	return nil
}

func (r *RegisterService) Write(value Value, reply *Value) error {
	if value.ViewRef != currentView.ViewRef() {
		log.Printf("Got old view with ViewRef: %v, sending new View %v with ViewRef: %v\n", value.ViewRef, currentView.View(), currentView.ViewRef())
		reply.Err = view.OldViewError{NewView: currentView.View()}
		return nil
	}

	register.mu.Lock()
	defer register.mu.Unlock()

	// Two writes with the same timestamp -> give preference to first one. This makes the Write operation idempotent and still read/write coherent.
	if value.Timestamp > register.Timestamp {
		register.Value = value.Value
		register.Timestamp = value.Timestamp
	}

	return nil
}

func (r *RegisterService) GetCurrentView(value int, reply *view.View) error {
	*reply = *currentView.View()
	log.Println("Done GetCurrentView request")
	return nil
}

// --------- Init ---------
func init() {
	register.mu.Lock() // The register starts locked
	register.Value = nil
	register.Timestamp = 0
}

func init() {
	rpc.Register(new(RegisterService))
}

// --------- Types ---------
type Value struct {
	Value     interface{}
	Timestamp int

	ViewRef view.ViewRef
	Err     error

	mu sync.RWMutex
}

var throughput uint64
var throughputBuffer = make(map[time.Time]uint64, 70)

func collectThroughputWorker() {
	writeLength := rand.Intn(20)
	var lastThroughput uint64

	for now := range time.Tick(time.Second) {
		aux := throughput
		throughputBuffer[now] = aux - lastThroughput
		lastThroughput = aux

		if len(throughputBuffer) > writeLength {
			writeLength = rand.Intn(20)
			saveThroughput()
		}
	}
	log.Fatalln("STOPPED COLLECTING THROUGHPUT!")
}

func saveThroughput() {
	filename := fmt.Sprintf("/proj/freestore/throughputs_%v.txt", currentView.View().GetProcessPosition(thisProcess))
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		//log.Println(err)
		for loopTime, loopThroughput := range throughputBuffer {
			fmt.Printf("%v %v %v\n", thisProcess, loopThroughput, loopTime.Format(time.RFC3339))
			delete(throughputBuffer, loopTime)
		}
		return
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	for loopTime, loopThroughput := range throughputBuffer {
		if _, err = w.Write([]byte(fmt.Sprintf("%v %v %v\n", thisProcess, loopThroughput, loopTime.Format(time.RFC3339)))); err != nil {
			log.Fatalln(err)
		}
		delete(throughputBuffer, loopTime)
	}
}

func init() {
	go collectThroughputWorker()
	rand.Seed(int64(time.Now().Nanosecond()))
}
