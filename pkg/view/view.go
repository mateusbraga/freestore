// Package view implements the view type. A view is a set of processes, and it is used to put together all the system's processes.
package view

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

// View represents a server's view of the members of the distributed system. A View once created is immutable, for a mutable View, see type CurrentView.
type View struct {
	Entries map[Update]bool
	Members map[Process]bool // Cache, can be rebuilt from Entries
	ViewRef
}

func newView() *View {
	v := View{}
	v.Entries = make(map[Update]bool)
	v.Members = make(map[Process]bool)
	return &v
}

func NewWithUpdates(updates ...Update) *View {
	newCopy := newView()
	newCopy.addUpdate(updates...)
	return newCopy
}

func NewWithProcesses(processes ...Process) *View {
	updates := []Update{}
	for _, process := range processes {
		updates = append(updates, Update{Join, process})
	}
	return NewWithUpdates(updates...)
}

func (v *View) NewCopyWithUpdates(updates ...Update) *View {
	newViewUpdates := []Update{}

	for update, _ := range v.Entries {
		newViewUpdates = append(newViewUpdates, update)
	}
    newViewUpdates = append(newViewUpdates, updates...)

	return NewWithUpdates(newViewUpdates...)
}

func (v *View) addUpdate(updates ...Update) {
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
	v.ViewRef = ViewToViewRef(v)
}

func (v *View) String() string {
	buf := new(bytes.Buffer)

	fmt.Fprintf(buf, "{")

	first := true
	for process, _ := range v.Members {
		if !first {
			fmt.Fprintf(buf, ", ")
		}
		fmt.Fprintf(buf, "%v", process.Addr)
		first = false
	}

	fmt.Fprintf(buf, "}")
	return buf.String()
}

func (v *View) LessUpdatedThan(v2 *View) bool {
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

func (v *View) MoreUpdatedThan(v2 *View) bool {
	if len(v2.Entries) > len(v.Entries) {
		return false
	}

	for k2, _ := range v2.Entries {
		if _, ok := v.Entries[k2]; !ok {
			return false
		}
	}

	if len(v2.Entries) == len(v.Entries) {
		// they are equal
		return false
	}

	return true
}

func (v *View) Equal(v2 *View) bool {
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

func (v *View) HasUpdate(u Update) bool {
	return v.Entries[u]
}

func (v *View) HasMember(p Process) bool {
	return v.Members[p]
}

func (v *View) GetUpdates() []Update {
	var updates []Update
	for update, _ := range v.Entries {
		updates = append(updates, update)
	}
	return updates
}

func (v *View) GetMembers() []Process {
	var members []Process
	for process, _ := range v.Members {
		members = append(members, process)
	}
	return members
}

func (v *View) GetMembersNotIn(v2 *View) []Process {
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

func (v *View) NumberOfMembers() int {
	return len(v.Members)
}

func (v *View) NumberOfUpdates() int {
	return len(v.Entries)
}

func (v *View) QuorumSize() int {
	membersTotal := len(v.Members)
	return (membersTotal+1)/2 + (membersTotal+1)%2
}

func (v *View) NumberOfToleratedFaults() int {
	membersTotal := len(v.Members)
	return membersTotal - v.QuorumSize()
}

// ----- ERRORS -----

type OldViewError struct {
	NewView *View
}

func (e OldViewError) Error() string {
	return fmt.Sprintf("Client's current view is old, update to '%v'", e.NewView)
}

func init() { gob.Register(new(OldViewError)) }

// ----- Auxiliary types ---------

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
