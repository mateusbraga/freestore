package view

import (
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
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}},
	}

	v1 := NewWithUpdates(updates...)

	if n := v1.N(); n != 2 {
		t.Errorf("v1 should have 2 members, not %d", n)
	}

	if q := v1.QuorumSize(); q != 2 {
		t.Errorf("Quorum of 2 processes should be 2, not %d", q)
	}

	if f := v1.F(); f != 0 {
		t.Errorf("Number of tolerable failures of 2 processes should be 0, not %d", f)
	}

	v1 = v1.NewCopyWithUpdates(Update{Join, Process{"3"}})

	if q := v1.QuorumSize(); q != 2 {
		t.Errorf("Quorum of 3 processes should be 2, not %d", q)
	}

	if f := v1.F(); f != 1 {
		t.Errorf("Number of tolerable failures of 3 processes should be 1, not %d", f)
	}

	v1 = v1.NewCopyWithUpdates(Update{Join, Process{"4"}})

	if q := v1.QuorumSize(); q != 3 {
		t.Errorf("Quorum of 4 processes should be 3, not %d", q)
	}

	if f := v1.F(); f != 1 {
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
