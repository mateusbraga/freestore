package view

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func TestViewRef(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"10.1.1.2:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.3:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.4:5000"}},
	}

	v1 := NewWithUpdates(updates...)
	v2 := NewWithUpdates(updates...)

	if ViewToViewRef(v1) != ViewToViewRef(v2) {
		t.Errorf("v1 and v2 should have same ViewRef\n")
	}

	v3 := v1.NewCopyWithUpdates(Update{Join, Process{"3"}})
	v5 := v1.NewCopyWithUpdates(Update{Join, Process{"3"}})

	if ViewToViewRef(v3) == ViewToViewRef(v1) {
		t.Errorf("v1 and v3 should have different ViewRefs\n")
	}
	if ViewToViewRef(v3) != ViewToViewRef(v5) {
		t.Errorf("v1 and v3 should have same ViewRefs\n")
	}

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(v1)
	if err != nil {
		t.Errorf(err.Error())
	}

	buf2 := bytes.NewReader(buf.Bytes())
	decoder := gob.NewDecoder(buf2)

	var v6 *View
	err = decoder.Decode(&v6)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ViewToViewRef(v1) != ViewToViewRef(v6) {
		t.Errorf("V1 and v6 should have same ViewRefs\n")
	}
}
