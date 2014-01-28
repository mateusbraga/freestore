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

func (thisProcess Process) Less(otherProcess Process) bool {
	return thisProcess.Addr < otherProcess.Addr
}

type Update struct {
	Type    updateType
	Process Process
}

func (thisUpdate Update) Less(otherUpdate Update) bool {
	if thisUpdate.Type < otherUpdate.Type {
		return true
	} else if thisUpdate.Type == otherUpdate.Type {
		return thisUpdate.Process.Less(otherUpdate.Process)
	} else {
		return false
	}
}

type View struct {
	Entries map[Update]bool
	mu      sync.RWMutex

	Members map[Process]bool // Cache
}

func New() *View {
	v := View{}
	v.Entries = make(map[Update]bool)
	v.Members = make(map[Process]bool)
	return &v
}

func (v *View) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var b bytes.Buffer
	fmt.Fprintf(&b, "{")

	first := true
	for process, _ := range v.Members {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%v", process.Addr)
		first = false
	}

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
	v.mu.RLock()
	defer v.mu.RUnlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	if len(v.Entries) != len(v2.Entries) {
		return false
	}

	for k2, _ := range v2.Entries {
		if _, ok := v.Entries[k2]; !ok {
			return false
		}
	}
	return true
}

func (v *View) AddUpdate(updates ...Update) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, newUpdate := range updates {
		v.Entries[newUpdate] = true

		switch newUpdate.Type {
		case Join:
			if !v.Entries[Update{Leave, newUpdate.Process}] {
				v.Members[newUpdate.Process] = true
			}
		case Leave:
			delete(v.Members, newUpdate.Process)
		}
	}
}

func (v *View) NewCopy() *View {
	newCopy := New()
	newCopy.Set(v)
	return newCopy
}

func (v *View) Merge(v2 *View) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	for update, _ := range v2.Entries {
		v.Entries[update] = true

		switch update.Type {
		case Join:
			if !v.Entries[Update{Leave, update.Process}] {
				v.Members[update.Process] = true
			}
		case Leave:
			delete(v.Members, update.Process)
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

	var entries []Update
	for update, _ := range v.Entries {
		entries = append(entries, update)
	}
	return entries
}

func (v *View) GetMembers() []Process {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var members []Process
	for process, _ := range v.Members {
		members = append(members, process)
	}
	return members
}

func (v *View) GetMembersAlsoIn(v2 *View) []Process {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	var members []Process
	for process, _ := range v.Members {
		members = append(members, process)
	}
	for process, _ := range v2.Members {
		if !v.Members[process] {
			members = append(members, process)
		}
	}

	return members
}

func (v *View) GetMembersNotIn(v2 *View) []Process {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	var members []Process
	for process, _ := range v.Members {
		if !v2.Members[process] {
			members = append(members, process)
		}
	}

	return members
}

// GetProcessPosition returns an unique number for the process in the view. Returns -1 if process is not a member of the view.
func (v *View) GetProcessPosition(process Process) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if _, ok := v.Members[process]; !ok {
		return -1
	}

	// Position will be the position of the process in an ordered list of the members.
	position := 0
	for proc, _ := range v.Members {
		if proc.Addr < process.Addr {
			position++
		}
	}
	return position
}

func (v *View) NumberOfEntries() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.Entries)
}

func (v *View) QuorumSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.quorumSize()
}

func (v *View) quorumSize() int {
	membersTotal := len(v.Members)
	return (membersTotal+1)/2 + (membersTotal+1)%2
}

func (v *View) N() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.Members)
}

func (v *View) F() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	membersTotal := len(v.Members)
	return membersTotal - v.quorumSize()
}

// ----- ERRORS -----

type OldViewError struct {
	NewView *View
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
