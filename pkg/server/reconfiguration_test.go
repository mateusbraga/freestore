package server

import (
	"testing"

	"github.com/mateusbraga/freestore/pkg/view"
)

func TestFindLeastUpdatedView(t *testing.T) {
	v := view.New()
	v.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}})

	v2 := v.NewCopy()
	v2.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}})

	v3 := v2.NewCopy()
	v3.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}})

	var seq []view.View

	seq = append(seq, v2)

	if view := findLeastUpdatedView(seq); !view.Equal(&v2) {
		t.Errorf("got %v, expected %v\n", &view, &v2)
	}

	seq = append(seq, v3)

	if view := findLeastUpdatedView(seq); !view.Equal(&v2) {
		t.Errorf("got %v, expected %v\n", &view, &v2)
	}

	seq = append(seq, v)

	if view := findLeastUpdatedView(seq); !view.Equal(&v) {
		t.Errorf("got %v, expected %v\n", &view, &v)
	}
}
