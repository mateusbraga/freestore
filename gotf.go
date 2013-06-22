package gotf

import (
	//"log"
    "fmt"
    "encoding/gob"
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
    WriteTimestamp int
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
}

func NewView() View {
    return View{make(map[Update]bool), make(map[Process]bool)}
}

func (v View) Contains(v2 View) bool {
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

func (v View) AddUpdate(u Update) {
    v.Entries[u] = true

    switch u.Type {
    case Join:
        v.Members[u.Process] = true
    case Leave:
        delete(v.Members, u.Process)
    }
}

func (v View) QuorunSize() int {
    n := len(v.Members) + 1
    return n/2 + n%2
}

func (v View) N() int {
    return len(v.Members)
}
