package view

import (
	"bytes"
	"encoding/gob"
	"sync"
)

// GobEncode encodes only the entries of the view.
func (v *View) GobEncode() ([]byte, error) {
	// Do not lock if not initialized: nil view will be sent
	if v.mu != nil {
		v.mu.RLock()
		defer v.mu.RUnlock()
	}

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(&v.entries)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode decodes the entries of the view and then initializes a new mutex and the members of the view.
func (v *View) GobDecode(data []byte) error {
	// get the entries
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	err := decoder.Decode(&v.entries)
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
