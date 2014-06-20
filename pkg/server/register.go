//TODO make it a key value storage

package server

import (
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

func init() {
	register.mu.Lock() // The register starts locked
	register.Value = nil
	register.Timestamp = 0
}

type Value struct {
	Value     interface{}
	Timestamp int

	ViewRef view.ViewRef
	Err     error

	mu sync.RWMutex
}

var register Value

//  ---------- RPC Requests -------------

type RegisterService int

func init() { rpc.Register(new(RegisterService)) }

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

	//throughput++

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

type RegisterValue struct {
	Value     interface{}
	Timestamp uint64
}

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
