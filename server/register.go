package server

import (
	"log"
	"net/rpc"
	"sync"

	"mateusbraga/freestore/view"
)

// --------- External State ---------

// --------- Internal State ---------
var register Value

//  ---------- Interface -------------
type ClientRequest int

func (r *ClientRequest) Read(clientView view.View, reply *Value) error {
	if !clientView.Equal(&currentView) {
		reply.Err = view.OldViewError{NewView: currentView.NewCopy()}
	}

	register.mu.RLock()
	defer register.mu.RUnlock()

	reply.Value = register.Value
	reply.Timestamp = register.Timestamp

	log.Println("Done read request")
	return nil
}

func (r *ClientRequest) Write(value Value, reply *Value) error {
	if !value.View.Equal(&currentView) {
		reply.Err = view.OldViewError{NewView: currentView.NewCopy()}
		return nil
	}

	register.mu.Lock()
	defer register.mu.Unlock()

	if value.Timestamp > register.Timestamp {
		register.Value = value.Value
		register.Timestamp = value.Timestamp
	}

	log.Println("Done write request")
	return nil
}

func (r *ClientRequest) GetCurrentView(value int, reply *view.View) error {
	*reply = currentView.NewCopy()
	log.Println("Done GetCurrentView request")
	return nil
}

// --------- Bootstrapping ---------
func init() {
	register.mu.Lock() // The register starts locked
	register.Value = 3
	register.Timestamp = 1

	clientRequest := new(ClientRequest)
	rpc.Register(clientRequest)
}

// --------- Types ---------
// TODO mudar para interface{}
type Value struct {
	Value     int
	Timestamp int

	View view.View
	Err  error

	mu sync.RWMutex
}
