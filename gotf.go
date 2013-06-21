package gotf

import (
	//"log"
	//"fmt"
)

type Process struct {
	Addr string
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
