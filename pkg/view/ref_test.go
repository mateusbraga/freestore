package view

import (
	"testing"
)

func TestViewRef(t *testing.T) {
	v1 := New()

	v1.AddUpdate(Update{Join, Process{"1"}})
	v1.AddUpdate(Update{Join, Process{"2"}})

	viewRef := viewToViewRef(v1)
	otherViewRef := viewToViewRef(v1)

	if viewRef != otherViewRef {
		t.Errorf("viewRef and otherViewRef should be equal, but %v != %v\n", viewRef, otherViewRef)
	}

	v1.AddUpdate(Update{Join, Process{"3"}})

	otherViewRef = viewToViewRef(v1)

	if viewRef == otherViewRef {
		t.Errorf("viewRef and otherViewRef should be different, got both %v\n", viewRef)
	}

	v2 := New()
	v2.AddUpdate(Update{Join, Process{"1"}})
	v2.AddUpdate(Update{Join, Process{"2"}})
	v2.AddUpdate(Update{Join, Process{"3"}})

	viewRef = viewToViewRef(v1)
	otherViewRef = viewToViewRef(v2)

	if viewRef != otherViewRef {
		t.Errorf("viewRef and otherViewRef should be equal, but %v != %v\n", viewRef, otherViewRef)
	}
}
