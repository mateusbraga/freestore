package view

import (
	"testing"
)

func TestViewRef(t *testing.T) {
	updates := []Update{Update{Type: Join, Process: Process{"1"}},
		Update{Type: Join, Process: Process{"2"}}}

	v1 := NewWithUpdates(updates...)
	v2 := NewWithUpdates(updates...)
	v4 := NewWithUpdates(updates...)

	if ViewToViewRef(v1) != ViewToViewRef(v2) {
		t.Errorf("v1 and v2 should have same ViewRef\n")
	}
	if ViewToViewRef(v1) != ViewToViewRef(v4) {
		t.Errorf("v1 and v4 should have same ViewRef\n")
	}

	v3 := v1.NewCopyWithUpdates(Update{Join, Process{"3"}})
	v5 := v1.NewCopyWithUpdates(Update{Join, Process{"3"}})

	if ViewToViewRef(v3) == ViewToViewRef(v1) {
		t.Errorf("v1 and v3 should have different ViewRefs\n")
	}
	if ViewToViewRef(v3) != ViewToViewRef(v5) {
		t.Errorf("v1 and v3 should have same ViewRefs\n")
	}
}
