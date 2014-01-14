package view

import (
	"testing"
)

func TestViewEqual(t *testing.T) {
	v1 := New()
	v2 := New()

	if !v1.Equal(&v2) {
		t.Fatalf("Empty views v1 and v2 should be equal")
	}

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})
	v1.AddUpdate(Update{Join, Process{"3"}})
	v1.AddUpdate(Update{Leave, Process{"1"}})

	if v1.Equal(&v2) {
		t.Fatalf("Views v1 and v2 should be different")
	}

	v2.AddUpdate(Update{Join, Process{"1"}})
	v2.AddUpdate(Update{Join, Process{"2"}})
	v2.AddUpdate(Update{Join, Process{"3"}})
	v2.AddUpdate(Update{Leave, Process{"1"}})

	if !v1.Equal(&v2) {
		t.Fatalf("Views v1 and v2 should be equal")
	}

	v2.AddUpdate(Update{Leave, Process{"2"}})

	if v1.Equal(&v2) {
		t.Fatalf("Views v1 and v2 should be different")
	}
}

func TestViewSet(t *testing.T) {
	v1 := New()
	v2 := New()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})
	v1.AddUpdate(Update{Join, Process{"3"}})
	v1.AddUpdate(Update{Leave, Process{"1"}})

	if v1.N() != 2 {
		t.Fatalf("v1 should have 2 members")
	}

	v2.Set(&v1)

	if !v1.Equal(&v2) {
		t.Fatalf("Views v1 and v2 should be equal")
	}
}

func TestViewGetMembers(t *testing.T) {
	v1 := New()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})
	v1.AddUpdate(Update{Join, Process{"3"}})
	v1.AddUpdate(Update{Leave, Process{"1"}})

	processes := v1.GetMembers()
	processes2 := []Process{Process{"2"}, Process{"3"}}

	for i, p := range processes {
		if p != processes2[i] {
			t.Fatalf("Array of processes should be equal")
		}
	}
}

func TestViewSize(t *testing.T) {
	v1 := New()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})

	if n := v1.N(); n != 2 {
		t.Errorf("v1 should have 2 members, not %d", n)
	}

	if q := v1.QuorumSize(); q != 2 {
		t.Errorf("Quorum of 2 processes should be 2, not %d", q)
	}

	if f := v1.F(); f != 0 {
		t.Errorf("Number of tolerable failures of 2 processes should be 0, not %d", f)
	}

	v1.AddUpdate(Update{Join, Process{"3"}})

	if q := v1.QuorumSize(); q != 2 {
		t.Errorf("Quorum of 3 processes should be 2, not %d", q)
	}

	if f := v1.F(); f != 1 {
		t.Errorf("Number of tolerable failures of 3 processes should be 1, not %d", f)
	}

	v1.AddUpdate(Update{Join, Process{"4"}})

	if q := v1.QuorumSize(); q != 3 {
		t.Errorf("Quorum of 4 processes should be 3, not %d", q)
	}

	if f := v1.F(); f != 1 {
		t.Errorf("Number of tolerable failures of 4 processes should be 1, not %d", f)
	}
}

func TestViewLessUpdatedThan(t *testing.T) {
	v1 := New()
	v2 := New()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})

	v2.AddUpdate(Update{Join, Process{"1"}})

	if v1.LessUpdatedThan(&v2) {
		t.Errorf("v1 is not less updated than v2!")
	}

	if !v2.LessUpdatedThan(&v1) {
		t.Errorf("v2 is less updated than v1!")
	}

	v2.AddUpdate(Update{Join, Process{"3"}})

	if !v1.LessUpdatedThan(&v2) {
		t.Errorf("v1 is less updated than v2!")
	}

	if !v2.LessUpdatedThan(&v1) {
		t.Errorf("v2 is less updated than v1!")
	}

	v1.AddUpdate(Update{Join, Process{"3"}})
	v2.AddUpdate(Update{Join, Process{"2"}})

	if v2.LessUpdatedThan(&v1) {
		t.Errorf("v2 is not less updated than v1!")
	}
}

func TestNewCopy(t *testing.T) {
	v1 := New()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})
	v1.AddUpdate(Update{Join, Process{"3"}})
	v1.AddUpdate(Update{Leave, Process{"1"}})

	v2 := v1.NewCopy()
	if !v1.Equal(&v2) {
		t.Errorf("v2 is not a copy of v1!")
	}
}
