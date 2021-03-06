package view

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func TestViewEqual(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}},
		Update{Type: Join, Process: Process{"3"}},
		Update{Type: Leave, Process: Process{"1"}},
	}

	v1 := newView()
	v2 := newView()

	if !v1.Equal(v2) {
		t.Fatalf("Empty views v1 and v2 should be equal")
	}

	v1 = v1.NewCopyWithUpdates(updates...)

	if v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be different")
	}

	v2 = v2.NewCopyWithUpdates(updates...)

	if !v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be equal")
	}

	v2 = v2.NewCopyWithUpdates(Update{Leave, Process{"2"}})

	if v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be different")
	}
}

func TestViewGetMembers(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}},
		Update{Type: Join, Process: Process{"3"}},
		Update{Type: Leave, Process: Process{"1"}},
	}

	v1 := NewWithUpdates(updates...)

	processes := v1.GetMembers()
	processes2 := []Process{Process{"2"}, Process{"3"}}

	if len(processes) != len(processes2) {
		t.Fatalf("Array of processes should be equal")
	}

	for _, p := range processes {
		found := false
		for _, p2 := range processes2 {
			if p == p2 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Array of processes should be equal")
		}
	}
}

func TestViewSize(t *testing.T) {
	v1 := NewWithUpdates(Update{Type: Join, Process: Process{"1"}})

	if n := v1.NumberOfMembers(); n != 1 {
		t.Errorf("v1 should have 1 member, not %d", n)
	}

	if q := v1.QuorumSize(); q != 1 {
		t.Errorf("Quorum of 1 processes should be 1, not %d", q)
	}

	if f := v1.NumberOfToleratedFaults(); f != 0 {
		t.Errorf("Number of tolerable failures of 1 process should be 0, not %d", f)
	}

	v1 = v1.NewCopyWithUpdates(Update{Type: Join, Process: Process{"2"}})

	if n := v1.NumberOfMembers(); n != 2 {
		t.Errorf("v1 should have 2 members, not %d", n)
	}

	if q := v1.QuorumSize(); q != 2 {
		t.Errorf("Quorum of 2 processes should be 2, not %d", q)
	}

	if f := v1.NumberOfToleratedFaults(); f != 0 {
		t.Errorf("Number of tolerable failures of 2 processes should be 0, not %d", f)
	}

	v1 = v1.NewCopyWithUpdates(Update{Join, Process{"3"}})

	if q := v1.QuorumSize(); q != 2 {
		t.Errorf("Quorum of 3 processes should be 2, not %d", q)
	}

	if f := v1.NumberOfToleratedFaults(); f != 1 {
		t.Errorf("Number of tolerable failures of 3 processes should be 1, not %d", f)
	}

	v1 = v1.NewCopyWithUpdates(Update{Join, Process{"4"}})

	if q := v1.QuorumSize(); q != 3 {
		t.Errorf("Quorum of 4 processes should be 3, not %d", q)
	}

	if f := v1.NumberOfToleratedFaults(); f != 1 {
		t.Errorf("Number of tolerable failures of 4 processes should be 1, not %d", f)
	}
}

func TestViewLessUpdatedThan(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}},
	}

	v1 := NewWithUpdates(updates...)
	v2 := NewWithUpdates(updates[0])

	if v1.LessUpdatedThan(v2) {
		t.Errorf("v1 is not less updated than v2!")
	}

	if !v2.LessUpdatedThan(v1) {
		t.Errorf("v2 is less updated than v1!")
	}

	v2 = v2.NewCopyWithUpdates(Update{Join, Process{"3"}})

	if !v1.LessUpdatedThan(v2) {
		t.Errorf("v1 is less updated than v2!")
	}

	if !v2.LessUpdatedThan(v1) {
		t.Errorf("v2 is less updated than v1!")
	}

	v1 = v1.NewCopyWithUpdates(Update{Join, Process{"3"}})
	v2 = v2.NewCopyWithUpdates(Update{Join, Process{"2"}})

	if v2.LessUpdatedThan(v1) {
		t.Errorf("v2 is not less updated than v1!")
	}
}

func TestViewMoreUpdatedThan(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}},
	}

	v1 := NewWithUpdates(updates...)
	v2 := NewWithUpdates(updates[0])

	if !v1.MoreUpdatedThan(v2) {
		t.Errorf("v1 is more updated than v2!")
	}

	if v2.MoreUpdatedThan(v1) {
		t.Errorf("v2 is not more updated than v1!")
	}

	v2 = v2.NewCopyWithUpdates(Update{Join, Process{"3"}})

	if v1.MoreUpdatedThan(v2) {
		t.Errorf("v1 is not more updated than v2!")
	}

	if v2.MoreUpdatedThan(v1) {
		t.Errorf("v2 is not more updated than v1!")
	}

	v1 = v1.NewCopyWithUpdates(Update{Join, Process{"3"}})
	v2 = v2.NewCopyWithUpdates(Update{Join, Process{"2"}})

	if v2.MoreUpdatedThan(v1) {
		t.Errorf("v2 is not more updated than v1!")
	}
}

func TestGetProcessPosition(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}},
		Update{Type: Join, Process: Process{"3"}},
	}

	v1 := NewWithUpdates(updates...)

	if position := v1.GetProcessPosition(Process{"1"}); position != 0 {
		t.Errorf("GetProcessPosition: expected 0, got %v", position)
	}

	if position := v1.GetProcessPosition(Process{"2"}); position != 1 {
		t.Errorf("GetProcessPosition: expected 1, got %v", position)
	}

	v1 = v1.NewCopyWithUpdates(Update{Leave, Process{"1"}})

	if position := v1.GetProcessPosition(Process{"1"}); position != -1 {
		t.Errorf("GetProcessPosition: expected -1, got %v", position)
	}

	if position := v1.GetProcessPosition(Process{"2"}); position != 0 {
		t.Errorf("GetProcessPosition: expected 0, got %v", position)
	}
}

func TestViewGob(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"10.1.1.2:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.3:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.4:5000"}},
	}
	v1 := NewWithUpdates(updates...)

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(v1)
	if err != nil {
		t.Errorf(err.Error())
	}

	var v2 *View
	buf2 := bytes.NewReader(buf.Bytes())
	decoder := gob.NewDecoder(buf2)
	err = decoder.Decode(&v2)
	if err != nil {
		t.Errorf(err.Error())
	}

	if !v1.Equal(v2) {
		t.Errorf("View gob encoder got it wrong")
	}
}

func BenchmarkViewGobEncode(b *testing.B) {
	updates := []Update{Update{Type: Join, Process: Process{"10.1.1.2:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.3:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.4:5000"}},
	}
	v1 := NewWithUpdates(updates...)

	buf := new(bytes.Buffer)

	for i := 0; i < b.N; i++ {
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(v1)
		if err != nil {
			b.Errorf(err.Error())
		}
		buf.Reset()
	}
}

func BenchmarkViewGobDecode(b *testing.B) {
	updates := []Update{Update{Type: Join, Process: Process{"10.1.1.2:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.3:5000"}},
		Update{Type: Join, Process: Process{"10.1.1.4:5000"}},
	}
	v1 := NewWithUpdates(updates...)
	buf := new(bytes.Buffer)
	var v2 *View

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		buf.Reset()
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(v1)
		if err != nil {
			b.Errorf(err.Error())
		}
		buf2 := bytes.NewReader(buf.Bytes())
		decoder := gob.NewDecoder(buf2)
		b.StartTimer()

		err = decoder.Decode(&v2)
		if err != nil {
			b.Errorf(err.Error())
		}
	}
}
