package backend

import (
	"net/rpc"
	"sync"

	"mateusbraga/gotf/view"
)

var register Value

type Value struct {
	Value     int
	Timestamp int

	View view.View
	Err  error

	mu sync.RWMutex
}

type ClientRequest int

func (r *ClientRequest) GetCurrentView(anything *int, reply *view.View) error {
	reply.Set(currentView)
	return nil
}

func (r *ClientRequest) Read(clientView view.View, reply *Value) error {
	register.mu.RLock()
	defer register.mu.RUnlock()

	if !clientView.Equal(currentView) {
		err := view.OldViewError{}
		err.OldView.Set(clientView)
		err.NewView.Set(currentView)
		reply.Err = err
	}

	reply.Value = register.Value
	reply.Timestamp = register.Timestamp

	return nil
}

func (r *ClientRequest) Write(value Value, reply *Value) error {
	register.mu.Lock()
	defer register.mu.Unlock()

	if !value.View.Equal(currentView) {
		err := view.OldViewError{}
		err.OldView.Set(value.View)
		err.NewView.Set(currentView)

		reply.Err = err
		return nil
	}

	if value.Timestamp > register.Timestamp {
		register.Value = value.Value
		register.Timestamp = value.Timestamp
	}

	return nil
}

func init() {
	register.Value = 3
	register.Timestamp = 1

	clientRequest := new(ClientRequest)
	rpc.Register(clientRequest)
}
