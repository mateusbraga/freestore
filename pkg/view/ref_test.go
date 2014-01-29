package view

import (
	"testing"
)

func TestViewRef(t *testing.T) {
	v1 := New()
	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})

	v2 := New()
	v2.AddUpdate(Update{Join, Process{"1"}})
	v2.AddUpdate(Update{Join, Process{"2"}})

	if v1.GetViewRef() != v2.GetViewRef() {
		t.Errorf("v1 and v2 should have same ViewRef\n")
	}

	v1.AddUpdate(Update{Join, Process{"3"}})

	if v1.GetViewRef() == v2.GetViewRef() {
		t.Errorf("v1 and v2 should have different ViewRefs\n")
	}

	v2.AddUpdate(Update{Join, Process{"3"}})

	if v1.GetViewRef() != v2.GetViewRef() {
		t.Errorf("v1 and v2 should have same ViewRef\n")
	}
}
