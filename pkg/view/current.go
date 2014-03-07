package view

import (
	"log"
	"sync"
)

type CurrentView struct {
	viewRef ViewRef
	view    *View
	mu      *sync.RWMutex
}

func NewCurrentView() CurrentView {
	newCurrentView := CurrentView{}
	newCurrentView.view = newView()
	newCurrentView.mu = &sync.RWMutex{}
	return newCurrentView
}

func (currentView CurrentView) String() string {
	currentView.mu.RLock()
	defer currentView.mu.RUnlock()

	return currentView.view.String()
}

func (currentView *CurrentView) Update(newView *View) {
	currentView.mu.Lock()
	defer currentView.mu.Unlock()

	if newView.LessUpdatedThan(currentView.view) || newView.Equal(currentView.view) {
		if newView.LessUpdatedThan(currentView.view) {
			log.Println("Tried to Update current view with a less updated view")
		} else {
			log.Println("Tried to Update current view with the same view")
		}
		return
	}

	currentView.view = newView
	currentView.viewRef = ViewToViewRef(newView)
	log.Println("CurrentView updated to:", currentView.view)
}

func (currentView *CurrentView) View() *View {
	currentView.mu.RLock()
	defer currentView.mu.RUnlock()

	return currentView.view
}

func (currentView *CurrentView) ViewRef() ViewRef {
	currentView.mu.RLock()
	defer currentView.mu.RUnlock()

	return currentView.viewRef
}

func (currentView *CurrentView) ViewAndViewRef() (*View, ViewRef) {
	currentView.mu.RLock()
	defer currentView.mu.RUnlock()

	return currentView.view, currentView.viewRef
}
