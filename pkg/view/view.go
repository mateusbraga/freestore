package view

import (
	"bytes"
	"encoding/gob"
	"fmt"
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

// -------- View type -----------

type View struct {
	entries map[Update]bool

	members map[Process]bool // Cache
	//TODO viewRef may need lock or be set on the 'New' functions
	viewRef *ViewRef // Cache
}

func newView() *View {
	v := View{}
	v.entries = make(map[Update]bool)
	v.members = make(map[Process]bool)
	v.viewRef = nil
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
	newCopy := newView()

	for update, _ := range v.entries {
		newCopy.entries[update] = true
	}
	for process, _ := range v.members {
		newCopy.members[process] = true
	}

	newCopy.addUpdate(updates...)

	return newCopy
}

func (v *View) String() string {
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

func (v *View) LessUpdatedThan(v2 *View) bool {
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

func (v *View) Equal(v2 *View) bool {
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

func (v *View) addUpdate(updates ...Update) {
	v.viewRef = nil

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

func (v *View) HasUpdate(u Update) bool {
	return v.entries[u]
}

func (v *View) HasMember(p Process) bool {
	return v.members[p]
}

func (v *View) GetEntries() []Update {
	return v.getEntries()
}

func (v *View) getEntries() []Update {
	var entries []Update
	for update, _ := range v.entries {
		entries = append(entries, update)
	}
	return entries
}

func (v *View) GetMembers() []Process {
	var members []Process
	for process, _ := range v.members {
		members = append(members, process)
	}
	return members
}

func (v *View) GetMembersNotIn(v2 *View) []Process {
	var members []Process
	for process, _ := range v.members {
		if !v2.members[process] {
			members = append(members, process)
		}
	}

	return members
}

// GetProcessPosition returns an unique number for the process in the view. Returns -1 if process is not a member of the view.
func (v *View) GetProcessPosition(process Process) int {
	if _, ok := v.members[process]; !ok {
		return -1
	}

	// Position will be the position of the process in an ordered list of the members.
	position := 0
	for proc, _ := range v.members {
		if proc.Addr < process.Addr {
			position++
		}
	}
	return position
}

func (v *View) NumberOfEntries() int {
	return len(v.entries)
}

func (v *View) QuorumSize() int {
	return v.quorumSize()
}

func (v *View) quorumSize() int {
	membersTotal := len(v.members)
	return (membersTotal+1)/2 + (membersTotal+1)%2
}

func (v *View) N() int {
	return len(v.members)
}

func (v *View) F() int {
	membersTotal := len(v.members)
	return membersTotal - v.quorumSize()
}

func (v *View) getViewRef() ViewRef {
	if v.viewRef == nil {
		v.viewRef = viewToViewRef(v)
	}
	return *v.viewRef
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
