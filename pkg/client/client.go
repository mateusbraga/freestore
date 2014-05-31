/*
Package client implements a Freestore client
*/
package client

import (
	"log"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

// Client represents a freestore client. Client may be used by multiple goroutines simultaneously.
type Client struct {
	view                view.CurrentView
	getFurtherViewsFunc GetViewFunc
	err                 error
	mutex               sync.Mutex
}

type GetViewFunc func() (*view.View, error)

// New returns a new Client with initialView.
func New(getInitialViewFunc GetViewFunc, getFurtherViewsFunc GetViewFunc) (*Client, error) {
	newClient := &Client{}
	newClient.view = view.NewCurrentView()

	initialView, err := getInitialViewFunc()
	if err != nil {
		return nil, err
	}

	newClient.view.Update(initialView)

	newClient.getFurtherViewsFunc = getFurtherViewsFunc

	return newClient, nil
}

func (cl Client) View() *view.View      { return cl.view.View() }
func (cl Client) ViewRef() view.ViewRef { return cl.view.ViewRef() }
func (cl Client) ViewAndViewRef() (*view.View, view.ViewRef) {
	return cl.view.ViewAndViewRef()
}
func (cl *Client) updateCurrentView(newView *view.View) { cl.view.Update(newView) }

// Write v to the system's register. Can be run concurrently.
func (cl *Client) Write(v interface{}) error {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	// Stop using the system if it is known to be broken (fail-fast)
	if cl.err != nil {
		return cl.err
	}

	readValue, err := cl.readQuorum()
	if err != nil {
		// Special case: diffResultsErr
		if err == diffResultsErr {
			// Do nothing - we will write a new value anyway
		} else {
			cl.err = err
			return err
		}
	}

	writeMsg := RegisterMsg{}
	writeMsg.Value = v
	//TODO append writer id to timestamp
	writeMsg.Timestamp = readValue.Timestamp + 1
	writeMsg.ViewRef = cl.ViewRef()

	err = cl.writeQuorum(writeMsg)
	if err != nil {
		cl.err = err
		return err
	}

	return nil
}

// Read executes the quorum read protocol.
func (cl *Client) Read() (interface{}, error) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	// Stop using the system if it is known to be broken (fail-fast)
	if cl.err != nil {
		return nil, cl.err
	}

	readMsg, err := cl.readQuorum()
	if err != nil {
		// Special case: diffResultsErr
		if err == diffResultsErr {
			log.Println("Found divergence: Going to 2nd phase of read protocol")
			return cl.read2ndPhase(readMsg)
		} else {
			cl.err = err
			return nil, err
		}
	}

	return readMsg.Value, nil
}

func (cl *Client) read2ndPhase(readMsg RegisterMsg) (interface{}, error) {
	err := cl.writeQuorum(readMsg)
	if err != nil {
		cl.err = err
		return nil, err
	}

	return readMsg.Value, nil
}

type RegisterMsg struct {
	Value     interface{}  // Value of the register
	Timestamp int          // Timestamp of the register
	ViewRef   view.ViewRef // Current client's view
	Err       error        // Any RPC or register service errors
}
