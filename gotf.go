package gotf

import (
	//"log"
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
	//"errors"
)

type Process struct {
	Addr string
}

type OldViewError struct {
	OldView View
	NewView View
}

func (e OldViewError) Error() string {
	return fmt.Sprint("OLD_VIEW")
}

type WriteOlderError struct {
	WriteTimestamp  int
	ServerTimestamp int
}

func (e WriteOlderError) Error() string {
	return fmt.Sprintf("error: write request has timestamp %v but server has more updated timestamp %v", e.WriteTimestamp, e.ServerTimestamp)
}

func init() {
	gob.Register(new(OldViewError))
	gob.Register(new(WriteOlderError))
}

type updateType string

const (
	Join  updateType = "+"
	Leave updateType = "-"
)

type Update struct {
	Type    updateType
	Process Process
}

type View struct {
	Entries map[Update]bool
	Members map[Process]bool
	mu      sync.RWMutex
}

func NewView() View {
	v := View{}
	v.Entries = make(map[Update]bool)
	v.Members = make(map[Process]bool)
	return v
}

func (v View) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var b bytes.Buffer
	fmt.Fprintf(&b, "{")
	first := true
	fmt.Fprintf(&b, "{")
	for k, _ := range v.Entries {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "<%v, %v>", k.Type, k.Process)
		first = false
	}
	fmt.Fprintf(&b, "}")
	fmt.Fprintf(&b, "{")
	first = true
	for k, _ := range v.Members {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%v", k.Addr)
		first = false
	}
	fmt.Fprintf(&b, "}")
	fmt.Fprintf(&b, "}")
	return b.String()
}

func (v *View) Set(v2 View) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	v.Entries = v2.Entries
	v.Members = v2.Members
}

func (v View) Contains(v2 View) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	for k2, _ := range v2.Entries {
		if _, ok := v.Entries[k2]; !ok {
			return false
		}
	}
	return true
}

func (v View) Equal(v2 View) bool {
	return v.Contains(v2) && v2.Contains(v)
}

func (v *View) AddUpdate(u Update) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.Entries[u] = true

	switch u.Type {
	case Join:
		v.Members[u.Process] = true
	case Leave:
		delete(v.Members, u.Process)
	}
}

func (v View) GetMembers() []Process {
	v.mu.RLock()
	defer v.mu.RUnlock()

	l := make([]Process, 0)
	for k, _ := range v.Members {
		l = append(l, k)
	}
	return l
}

func (v View) QuorunSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	n := len(v.Members) + 1
	return n/2 + n%2
}

func (v View) N() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.Members)
}
