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

// updateCurrentView sets newView as the client's view.
func (thisClient *Client) updateCurrentView(newView *view.View) {
	thisClient.view.Update(newView)
}

// Write v to the system's register. Can be run concurrently.
func (thisClient *Client) Write(v interface{}) error {
	readValue, err := thisClient.readQuorum()
	if err != nil {
		// Special cases:
		//  diffResultsErr: can be ignored
		if err == diffResultsErr {
			// Do nothing - we will write a new value anyway
		} else {
			return err
		}
	}

	//TODO append writer id to timestamp? or test if values and timestamps are equal instead of just the timestamps
	writeMsg := RegisterMsg{}
	writeMsg.Value = v
	writeMsg.Timestamp = readValue.Timestamp + 1
	writeMsg.View = thisClient.View()

	err = thisClient.writeQuorum(writeMsg)
	if err != nil {
		return err
	}

	return nil
}

// Read executes the quorum read protocol.
func (thisClient *Client) Read() (interface{}, error) {
	readMsg, err := thisClient.readQuorum()
	if err != nil {
		// Expected: diffResultsErr (will write most current value to view).
		if err == diffResultsErr {
			log.Println("Found divergence: Going to 2nd phase of read protocol")
			return thisClient.read2ndPhase(readMsg)
		} else {
			return nil, err
		}
	}

	return readMsg.Value, nil
}

func (thisClient *Client) read2ndPhase(readMsg RegisterMsg) (interface{}, error) {
	destinationView := thisClient.View()

	readMsg.View = destinationView

	err := thisClient.writeQuorum(readMsg)
	if err != nil {
		return 0, err
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
