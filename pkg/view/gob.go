package view

import (
	"bytes"
	"encoding/gob"
	"log"
	"sync"
)

const freeBufferListSize = 100

var (
	freeBufferList = make(chan *bytes.Buffer, freeBufferListSize)
)

func getEmptyBuffer() *bytes.Buffer {
	var buf *bytes.Buffer
	select {
	case buf = <-freeBufferList:
		// Got buffer; clean it up
		buf.Reset()
	default:
		// None free, so allocate a new one
		buf = new(bytes.Buffer)
	}
	return buf
}

func doneWithBuffer(buf *bytes.Buffer) {
	select {
	case freeBufferList <- buf:
		// Buffer on free list; nothing more to do.
	default:
		// Free list full, just carry on.
	}
}

// GobEncode encodes only the entries of the view.
func (v *View) GobEncode() ([]byte, error) {
	// Do not lock if not initialized: nil view will be sent
	if v.mu != nil {
		v.mu.RLock()
		defer v.mu.RUnlock()
	}

	w := getEmptyBuffer()
	defer doneWithBuffer(w)

	encoder := gob.NewEncoder(w)
	err := encoder.Encode(&v.entries)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// GobDecode decodes the entries of the view and then initializes a new mutex and the members of the view.
func (v *View) GobDecode(data []byte) error {
	// init buffer
	r := getEmptyBuffer()
	defer doneWithBuffer(r)

	n, err := r.Write(data)
	if n != len(data) {
		log.Fatalf("assert failed: write to buffer: got %v expected %v\n", n, len(data))
	}

	// get the entries
	decoder := gob.NewDecoder(r)
	err = decoder.Decode(&v.entries)
	if err != nil {
		return err
	}

	// initialize mutex and members
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
