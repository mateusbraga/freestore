/*
Package client implements a Freestore client
*/
package client

import (
	"log"

	"github.com/mateusbraga/freestore/pkg/view"
)

type Client struct {
	view view.CurrentView
}

// New returns a new Client with initialView.
func New(initialView *view.View) *Client {
	newClient := &Client{}
	newClient.view = view.NewCurrentView()
	newClient.view.Update(initialView)
	return newClient
}

func (thisClient Client) View() *view.View {
	return thisClient.view.View()
}

// UpdateCurrentView sets newView as the client's view.
func (thisClient *Client) UpdateCurrentView(newView *view.View) {
	thisClient.view.Update(newView)
}

// Write v to the system's register.
func (thisClient *Client) Write(v interface{}) error {
	readValue, err := readQuorum(thisClient.View())
	if err != nil {
		// Special cases:
		//  oldViewError: thisClient.view is old, update it and retry
		//  diffResultsErr: can be ignored
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic read quorum of Write op")
			thisClient.UpdateCurrentView(oldViewError.NewView)
			return thisClient.Write(v)
		} else if err == diffResultsErr {
			// Do nothing - we will write a new value anyway
		} else {
			return err
		}
	}

	writeMsg := RegisterMsg{}
	writeMsg.Value = v
	writeMsg.Timestamp = readValue.Timestamp + 1
	writeMsg.View = thisClient.View()

	err = writeQuorum(thisClient.View(), writeMsg)
	if err != nil {
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic write quorum of Write op")
			thisClient.UpdateCurrentView(oldViewError.NewView)
			return thisClient.Write(v)
		} else {
			return err
		}
	}

	return nil
}

// Read executes the quorum read protocol.
func (thisClient *Client) Read() (interface{}, error) {
	readMsg, err := readQuorum(thisClient.View())
	if err != nil {
		// Expected: oldViewError (will retry) or diffResultsErr (will write most current value to view).
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic read quorum of Read op")
			thisClient.UpdateCurrentView(oldViewError.NewView)
			return thisClient.Read()
		} else if err == diffResultsErr {
			log.Println("Found divergence: Going to 2nd phase of read protocol")

			readMsg.View = thisClient.View()

			return thisClient.read2ndPhase(thisClient.View(), readMsg)
		} else {
			return 0, err
		}
	}

	return readMsg.Value, nil
}

func (thisClient *Client) read2ndPhase(destinationView *view.View, readMsg RegisterMsg) (interface{}, error) {
	err := writeQuorum(destinationView, readMsg)
	if err != nil {
		if oldViewError, ok := err.(*view.OldViewError); ok {
			log.Println("View updated during basic write quorum of Read op (2nd phase)")
			thisClient.UpdateCurrentView(oldViewError.NewView)
			return thisClient.Read()
		} else {
			return 0, err
		}
	}
	return readMsg.Value, nil
}

type RegisterMsg struct {
	Value     interface{} // Value of the register
	Timestamp int         // Timestamp of the register

	// View is used to send the client's view on client requests.
	View *view.View

	// Err is set by the server if any error occurs
	Err error
}
