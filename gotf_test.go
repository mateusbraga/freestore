package gotf

import (
	"testing"
)

func TestViewEqual(t *testing.T) {
	v1 := NewView()
	v2 := NewView()

	if !v1.Equal(v2) {
		t.Fatalf("Empty views v1 and v2 should be equal")
	}

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})
	v1.AddUpdate(Update{Join, Process{"3"}})
	v1.AddUpdate(Update{Leave, Process{"1"}})

	if v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be different")
	}

	v2.AddUpdate(Update{Join, Process{"1"}})
	v2.AddUpdate(Update{Join, Process{"2"}})
	v2.AddUpdate(Update{Join, Process{"3"}})
	v2.AddUpdate(Update{Leave, Process{"1"}})

	if !v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be equal")
	}

	v2.AddUpdate(Update{Leave, Process{"2"}})

	if v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be different")
	}
}

func TestViewSet(t *testing.T) {
	v1 := NewView()
	v2 := NewView()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})
	v1.AddUpdate(Update{Join, Process{"3"}})
	v1.AddUpdate(Update{Leave, Process{"1"}})

	if v1.N() != 2 {
		t.Fatalf("v1 should have 2 members")
	}

	v2.Set(v1)

	if !v1.Equal(v2) {
		t.Fatalf("Views v1 and v2 should be equal")
	}
}

func TestViewGetMembers(t *testing.T) {
	v1 := NewView()

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
	v1 := NewView()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})

	if n := v1.N(); n != 2 {
		t.Errorf("v1 should have 2 members, not %d", n)
	}

	if q := v1.QuorunSize(); q != 2 {
		t.Errorf("Quorum of 2 processes should be 2, not %d", q)
	}

	if f := v1.F(); f != 0 {
		t.Errorf("Number of tolerable failures of 2 processes should be 0, not %d", f)
	}

	v1.AddUpdate(Update{Join, Process{"3"}})

	if q := v1.QuorunSize(); q != 2 {
		t.Errorf("Quorum of 3 processes should be 2, not %d", q)
	}

	if f := v1.F(); f != 1 {
		t.Errorf("Number of tolerable failures of 3 processes should be 1, not %d", f)
	}
}
