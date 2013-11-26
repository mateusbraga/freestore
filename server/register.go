package server

import (
	"log"
	"net/rpc"
	"sync"
	"time"
	//"io/ioutil"
	//"fmt"

	"mateusbraga/freestore/view"
)

// --------- External State ---------

// --------- Internal State ---------
var register Value

var (
	numberOfOperations      int
	startNumberOfOperations map[int]int
	startTime               map[int]time.Time
	statsMutex              sync.Mutex
)

//  ---------- Interface -------------
type ClientRequest int

func (r *ClientRequest) Read(clientView view.View, reply *Value) error {
	if !clientView.Equal(&currentView) {
		reply.Err = view.OldViewError{NewView: currentView.NewCopy()}
		return nil
	}

	register.mu.RLock()
	defer register.mu.RUnlock()

	numberOfOperations++

	reply.Value = register.Value
	reply.Timestamp = register.Timestamp

	//log.Println("Done read request")
	return nil
}

func (r *ClientRequest) Write(value Value, reply *Value) error {
	if !value.View.Equal(&currentView) {
		reply.Err = view.OldViewError{NewView: currentView.NewCopy()}
		return nil
	}

	register.mu.Lock()
	defer register.mu.Unlock()

	numberOfOperations++

	if value.Timestamp > register.Timestamp {
		register.Value = value.Value
		register.Timestamp = value.Timestamp
	}

	//log.Println("Done write request")
	return nil
}

func (r *ClientRequest) GetCurrentView(value int, reply *view.View) error {
	*reply = currentView.NewCopy()
	log.Println("Done GetCurrentView request")
	return nil
}

func (r *ClientRequest) StartMeasurements(value bool, id *int) error {
	register.mu.Lock()
	defer register.mu.Unlock()

	*id = len(startNumberOfOperations) + 1
	startNumberOfOperations[*id] = numberOfOperations
	startTime[*id] = time.Now()
	return nil
}

func (r *ClientRequest) EndMeasurements(id int, stats *ServerStats) error {
	register.mu.Lock()
	defer register.mu.Unlock()

	endTime := time.Now()
	stats.Duration = endTime.Sub(startTime[id])
	stats.NumberOfOperations = numberOfOperations - startNumberOfOperations[id]
	return nil
}

// --------- Bootstrapping ---------
func init() {
	register.mu.Lock() // The register starts locked
	register.Value = nil
	register.Timestamp = 0

	clientRequest := new(ClientRequest)
	rpc.Register(clientRequest)
}

func init() {
	numberOfOperations = 0
	startNumberOfOperations = make(map[int]int)
	startTime = make(map[int]time.Time)
}

// --------- Types ---------
type Value struct {
	Value     interface{}
	Timestamp int

	View view.View
	Err  error

	mu sync.RWMutex
}

type ServerStats struct {
	NumberOfOperations int
	Duration           time.Duration
}
