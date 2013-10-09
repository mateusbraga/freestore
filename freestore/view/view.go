package view

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
)

type updateType string

const (
	Join  updateType = "+"
	Leave updateType = "-"
)

type Process struct {
	Addr string
}

type Update struct {
	Type    updateType
	Process Process
}

type View struct {
	Entries map[Update]bool
	mu      sync.RWMutex

	// Cache
	Members map[Process]bool
}

func New() View {
	v := View{}
	v.Entries = make(map[Update]bool)
	v.Members = make(map[Process]bool)
	return v
}

func (v *View) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var b bytes.Buffer
	fmt.Fprintf(&b, "{")
	first := true
	//fmt.Fprintf(&b, "{")
	//for k, _ := range v.Entries {
	//if !first {
	//fmt.Fprintf(&b, ", ")
	//}
	//fmt.Fprintf(&b, "<%v, %v>", k.Type, k.Process)
	//first = false
	//}
	//fmt.Fprintf(&b, "}")
	//fmt.Fprintf(&b, "{")
	first = true
	for k, _ := range v.Members {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%v", k.Addr)
		first = false
	}
	//fmt.Fprintf(&b, "}")
	fmt.Fprintf(&b, "}")
	return b.String()
}

func (v *View) Set(v2 *View) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	v.Entries = make(map[Update]bool, len(v2.Entries))
	v.Members = make(map[Process]bool, len(v2.Members))

	for update, _ := range v2.Entries {
		v.Entries[update] = true
	}
	for process, _ := range v2.Members {
		v.Members[process] = true
	}
}

func (v *View) Contains(v2 *View) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	if len(v.Entries) < len(v2.Entries) {
		return false
	}

	for k2, _ := range v2.Entries {
		if _, ok := v.Entries[k2]; !ok {
			return false
		}
	}
	return true
}

func (v *View) LessUpdatedThan(v2 *View) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	if len(v2.Entries) > len(v.Entries) {
		return true
	}

	for k2, _ := range v2.Entries {
		if _, ok := v.Entries[k2]; !ok {
			return true
		}
	}
	return false

}

func (v *View) Equal(v2 *View) bool {
	return v.Contains(v2) && v2.Contains(v)
}

func (v *View) AddUpdate(u Update) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.Entries[u] = true

	switch u.Type {
	case Join:
		if !v.Entries[Update{Leave, u.Process}] {
			v.Members[u.Process] = true
		}
	case Leave:
		delete(v.Members, u.Process)
	}
}

func (v *View) Merge(v2 *View) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	for u, _ := range v2.Entries {
		v.Entries[u] = true

		switch u.Type {
		case Join:
			if !v.Entries[Update{Leave, u.Process}] {
				v.Members[u.Process] = true
			}
		case Leave:
			delete(v.Members, u.Process)
		}
	}
}

func (v *View) HasUpdate(u Update) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.Entries[u]
}

func (v *View) HasMember(p Process) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.Members[p]
}

func (v *View) GetEntries() []Update {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var l []Update
	for k, _ := range v.Entries {
		l = append(l, k)
	}
	return l
}

func (v *View) GetMembers() []Process {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var l []Process
	for k, _ := range v.Members {
		l = append(l, k)
	}
	return l
}

func (v *View) GetMembersAlsoIn(v2 *View) []Process {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	var l []Process
	for k, _ := range v.Members {
		l = append(l, k)
	}
	for k, _ := range v2.Members {
		if !v.Members[k] {
			l = append(l, k)
		}
	}

	return l
}

func (v *View) GetMembersNotIn(v2 *View) []Process {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	var l []Process
	for k, _ := range v.Members {
		if !v2.Members[k] {
			l = append(l, k)
		}
	}

	return l
}

func (v *View) GetProcessPosition(process Process) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	i := 0
	for k, _ := range v.Members {
		if k.Addr < process.Addr {
			i++
		}
	}
	return i
}

func (v *View) NumberOfEntries() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.Entries)
}

func (v *View) QuorumSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	n := len(v.Members) + 1
	return n/2 + n%2
}

func (v *View) N() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.Members)
}

func (v *View) F() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	n := len(v.Members) + 1
	// N() - QuorumSize()
	return (n - 1) - (n/2 + n%2)
}

// ----- ERRORS -----

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
	gob.Register(new([]View))
}
