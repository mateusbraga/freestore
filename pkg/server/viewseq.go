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
	leastUpdatedView := viewSeq[0]
	for _, loopView := range viewSeq[1:] {
		if loopView.LessUpdatedThan(leastUpdatedView) {
			leastUpdatedView.Set(loopView)
		}
	}

	return leastUpdatedView
}

func (viewSeq ViewSeq) GetMostUpdatedView() *view.View {
	mostUpdatedView := viewSeq[0]
	for _, loopView := range viewSeq[1:] {
		if mostUpdatedView.LessUpdatedThan(loopView) {
			mostUpdatedView.Set(loopView)
		}
	}

	return mostUpdatedView
}

func init() {
	gob.Register(new(ViewSeq))
}
