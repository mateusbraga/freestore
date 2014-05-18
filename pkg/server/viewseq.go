package server

import (
	"encoding/gob"

	"github.com/mateusbraga/freestore/pkg/view"
)

type ViewSeq []*view.View

func (viewSeq ViewSeq) Equal(otherViewSeq ViewSeq) bool {
	if len(otherViewSeq) != len(viewSeq) {
		return false
	}

	for _, loopView := range viewSeq {
		if !otherViewSeq.HasView(loopView) {
			return false
		}
	}

	return true
}

func (viewSeq ViewSeq) HasView(view *view.View) bool {
	for _, loopView := range viewSeq {
		if loopView.Equal(view) {
			return true
		}
	}
	return false
}

func (viewSeq ViewSeq) GetLeastUpdatedView() *view.View {
	leastUpdatedViewIndex := 0
	for loopIndex, loopView := range viewSeq {
		if loopView.LessUpdatedThan(viewSeq[leastUpdatedViewIndex]) {
			leastUpdatedViewIndex = loopIndex
		}
	}

	return viewSeq[leastUpdatedViewIndex]
}

func (viewSeq ViewSeq) GetMostUpdatedView() *view.View {
	if len(viewSeq) == 0 {
		return view.NewWithUpdates()
	}

	mostUpdatedViewIndex := 0
	for loopIndex, loopView := range viewSeq {
		if viewSeq[mostUpdatedViewIndex].LessUpdatedThan(loopView) {
			mostUpdatedViewIndex = loopIndex
		}
	}

	return viewSeq[mostUpdatedViewIndex]
}

func (viewSeq ViewSeq) HasViewMoreUpdatedThan(otherView *view.View) bool {
	for _, v := range viewSeq {
		if otherView.LessUpdatedThan(v) && !otherView.Equal(v) {
			return true
		}
	}
	return false
}

func (viewSeq ViewSeq) Append(views ...*view.View) ViewSeq {
	newViewSeq := viewSeq
	for _, v := range views {
		if viewSeq.HasView(v) {
			continue
		}
		newViewSeq = append(newViewSeq, v)
	}

	return newViewSeq
}

func init() {
	gob.Register(new(ViewSeq))
}
