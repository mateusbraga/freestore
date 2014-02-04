package view

import (
	"bytes"
	"encoding/gob"
)

func (v *View) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(&v.entries)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *View) GobDecode(b []byte) error {
	buf := bytes.NewReader(b)
	decoder := gob.NewDecoder(buf)
	err := decoder.Decode(&v.entries)
	if err != nil {
		return err
	}

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
