// +build !staticRegister

//TODO make it a key value storage
package server

import (
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

var register Value

//  ---------- RPC Requests -------------
type ClientRequest int

func (r *ClientRequest) Read(clientView *view.View, reply *Value) error {
	if !clientView.Equal(currentView) {
		reply.Err = view.OldViewError{NewView: currentView}
		return nil
	}

	register.mu.RLock()
	defer register.mu.RUnlock()

	reply.Value = register.Value
	reply.Timestamp = register.Timestamp

	return nil
}

func (r *ClientRequest) Write(value Value, reply *Value) error {
	if !value.View.Equal(currentView) {
		reply.Err = view.OldViewError{NewView: currentView}
		return nil
	}

	register.mu.Lock()
	defer register.mu.Unlock()

	// Two writes with the same timestamp -> give preference to later one.
	if value.Timestamp >= register.Timestamp {
		register.Value = value.Value
		register.Timestamp = value.Timestamp
	}

	return nil
}

func (r *ClientRequest) GetCurrentView(value int, reply *view.View) error {
	*reply = *currentView
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
	rpc.Register(new(ClientRequest))
}

// --------- Types ---------
type Value struct {
	Value     interface{}
	Timestamp int

	View *view.View
	Err  error

	mu sync.RWMutex
}
