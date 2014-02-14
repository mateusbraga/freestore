// +build ignore
package view

import (
	"bytes"
	"encoding/gob"
)

func (v *View) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(&v.Entries)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *View) GobDecode(b []byte) error {
	buf := bytes.NewReader(b)
	decoder := gob.NewDecoder(buf)
	err := decoder.Decode(&v.Entries)
	if err != nil {
		return err
	}

	v.Members = make(map[Process]bool)

	for u, _ := range v.Entries {
		switch u.Type {
		case Join:
			if !v.Entries[Update{Leave, u.Process}] {
				v.Members[u.Process] = true
			}
		case Leave:
			delete(v.Members, u.Process)
		}
	}

	return nil
}
