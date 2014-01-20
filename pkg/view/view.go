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
	entries map[Update]bool
	mu      *sync.RWMutex

	members map[Process]bool // Cache
}

func New() View {
	v := View{}
	v.entries = make(map[Update]bool)
	v.members = make(map[Process]bool)
	v.mu = new(sync.RWMutex)
	return v
}

func (v View) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var b bytes.Buffer
	fmt.Fprintf(&b, "{")

	first := true
	for process, _ := range v.members {
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%v", process.Addr)
		first = false
	}

	fmt.Fprintf(&b, "}")
	return b.String()
}

func (v *View) Set(v2 View) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	v.entries = make(map[Update]bool, len(v2.entries))
	v.members = make(map[Process]bool, len(v2.members))

	for update, _ := range v2.entries {
		v.entries[update] = true
	}
	for process, _ := range v2.members {
		v.members[process] = true
	}
}

func (v View) LessUpdatedThan(v2 View) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	if len(v2.entries) > len(v.entries) {
		return true
	}

	for k2, _ := range v2.entries {
		if _, ok := v.entries[k2]; !ok {
			return true
		}
	}
	return false

}

func (v View) Equal(v2 View) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	if len(v.entries) != len(v2.entries) {
		return false
	}

	for k2, _ := range v2.entries {
		if _, ok := v.entries[k2]; !ok {
			return false
		}
	}
	return true
}

func (v *View) AddUpdate(updates ...Update) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, newUpdate := range updates {
		v.entries[newUpdate] = true

		switch newUpdate.Type {
		case Join:
			if !v.entries[Update{Leave, newUpdate.Process}] {
				v.members[newUpdate.Process] = true
			}
		case Leave:
			delete(v.members, newUpdate.Process)
		}
	}
}

func (v View) NewCopy() View {
	newCopy := New()
	newCopy.Set(v)
	return newCopy
}

func (v *View) Merge(v2 View) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	for update, _ := range v2.entries {
		v.entries[update] = true

		switch update.Type {
		case Join:
			if !v.entries[Update{Leave, update.Process}] {
				v.members[update.Process] = true
			}
		case Leave:
			delete(v.members, update.Process)
		}
	}
}

func (v View) HasUpdate(u Update) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.entries[u]
}

func (v View) HasMember(p Process) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.members[p]
}

func (v View) GetEntries() []Update {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var entries []Update
	for update, _ := range v.entries {
		entries = append(entries, update)
	}
	return entries
}

func (v View) GetMembers() []Process {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var members []Process
	for process, _ := range v.members {
		members = append(members, process)
	}
	return members
}

func (v View) GetMembersAlsoIn(v2 View) []Process {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	var members []Process
	for process, _ := range v.members {
		members = append(members, process)
	}
	for process, _ := range v2.members {
		if !v.members[process] {
			members = append(members, process)
		}
	}

	return members
}

func (v View) GetMembersNotIn(v2 View) []Process {
	v.mu.Lock()
	defer v.mu.Unlock()

	v2.mu.RLock()
	defer v2.mu.RUnlock()

	var members []Process
	for process, _ := range v.members {
		if !v2.members[process] {
			members = append(members, process)
		}
	}

	return members
}

func (v View) GetProcessPosition(process Process) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if _, ok := v.members[process]; !ok {
		return -1
	}

	position := 0
	for proc, _ := range v.members {
		fmt.Println(proc, process, position)
		if proc.Addr < process.Addr {
			position++
		}
	}
	return position
}

func (v View) NumberOfEntries() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.entries)
}

func (v View) QuorumSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	n := len(v.members) + 1
	return n/2 + n%2
}

func (v View) N() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return len(v.members)
}

func (v View) F() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	n := len(v.members) + 1
	// N() - QuorumSize()
	return (n - 1) - (n/2 + n%2)
}

// ----- Gob -----

// GobEncode encodes only the entries of the view.
func (v *View) GobEncode() ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	err := encoder.Encode(&v.entries)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// GobDecode decodes the entries of the view and then initializes a new mutex and the members of the view.
func (v *View) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&v.entries)
	if err != nil {
		return err
	}

	v.mu = new(sync.RWMutex)
	v.members = make(map[Process]bool)

	for u, _ := range v.entries {
		switch u.Type {
		case Join:
			if !v.entries[Update{Leave, u.Process}] {
				v.members[u.Process] = true
			}
		case Leave:
			delete(v.members, u.Process)
		}
	}

	return nil
}

func init() {
	gob.Register(new(OldViewError))
	gob.Register(new(WriteOlderError))
	gob.Register(new([]View))
}

// ----- ERRORS -----

type OldViewError struct {
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
