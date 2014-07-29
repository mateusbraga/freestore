//TODO make it a key value storage

package server

import (
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

type Value struct {
	Value     interface{}
	Timestamp int

	ViewRef view.ViewRef
	Err     error

	mu sync.RWMutex
}

type RegisterService struct{}

func init() { rpc.Register(new(RegisterService)) }

func (r *RegisterService) Read(clientViewRef view.ViewRef, reply *Value) error {
	globalServer.currentViewMu.RLock()
	defer globalServer.currentViewMu.RUnlock()

	if clientViewRef != globalServer.currentView.ViewRef {
		log.Printf("Got old view with ViewRef: %v, sending new View %v with ViewRef: %v\n", clientViewRef, globalServer.currentView, globalServer.currentView.ViewRef)
		reply.Err = view.OldViewError{NewView: globalServer.currentView}
		return nil
	}

	globalServer.register.mu.RLock()
	defer globalServer.register.mu.RUnlock()

	reply.Value = globalServer.register.Value
	reply.Timestamp = globalServer.register.Timestamp

	return nil
}

func (r *RegisterService) Write(value Value, reply *Value) error {
	globalServer.currentViewMu.RLock()
	defer globalServer.currentViewMu.RUnlock()

	if value.ViewRef != globalServer.currentView.ViewRef {
		log.Printf("Got old view with ViewRef: %v, sending new View %v with ViewRef: %v\n", value.ViewRef, globalServer.currentView, globalServer.currentView.ViewRef)
		reply.Err = view.OldViewError{NewView: globalServer.currentView}
		return nil
	}

	globalServer.register.mu.Lock()
	defer globalServer.register.mu.Unlock()

	// Two writes with the same timestamp -> give preference to first one. This makes the Write operation idempotent and still read/write coherent.
	if value.Timestamp > globalServer.register.Timestamp {
		globalServer.register.Value = value.Value
		globalServer.register.Timestamp = value.Timestamp
	}

	return nil
}

func (r *RegisterService) GetCurrentView(anything struct{}, reply **view.View) error {
	globalServer.currentViewMu.RLock()
	defer globalServer.currentViewMu.RUnlock()

	*reply = globalServer.currentView
	log.Println("Done GetCurrentView request")
	return nil
}

type RegisterValue struct {
	Value     interface{}
	Timestamp uint64
}

// TODO Add state synchronization logic to Storage
type Storage interface {
	Read(name string) (RegisterValue, error)
	Write(name string, value RegisterValue) error
	LockAll()
	UnlockAll()
}

type memoryStorage struct {
	keyvalues map[string]RegisterValue
	kvMu      sync.RWMutex
	storageMu sync.RWMutex
}

var _ Storage = new(memoryStorage)

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{
		keyvalues: make(map[string]RegisterValue),
	}
}

func (s *memoryStorage) Read(name string) (RegisterValue, error) {
	s.storageMu.RLock()
	defer s.storageMu.RUnlock()

	s.kvMu.RLock()
	defer s.kvMu.RUnlock()

	v, ok := s.keyvalues[name]
	if !ok {
		return RegisterValue{}, nil
	}

	return v, nil
}

func (s *memoryStorage) Write(name string, newValue RegisterValue) error {
	s.storageMu.RLock()
	defer s.storageMu.RUnlock()

	s.kvMu.Lock()
	defer s.kvMu.Unlock()

	currentValue := s.keyvalues[name]

	if currentValue.Timestamp < newValue.Timestamp {
		s.keyvalues[name] = newValue
	}

	return nil
}

func (s *memoryStorage) LockAll()   { s.storageMu.Lock() }
func (s *memoryStorage) UnlockAll() { s.storageMu.Unlock() }
