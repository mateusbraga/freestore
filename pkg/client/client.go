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

func (thisClient Client) View() *view.View                      { return thisClient.view.View() }
func (thisClient *Client) updateCurrentView(newView *view.View) { thisClient.view.Update(newView) }

// Write v to the system's register. Can be run concurrently.
func (thisClient *Client) Write(v interface{}) error {
	readValue, err := thisClient.readQuorum()
	if err != nil {
		// Special case: diffResultsErr
		if err == diffResultsErr {
			// Do nothing - we will write a new value anyway
		} else {
			return err
		}
	}

	writeMsg := RegisterMsg{}
	writeMsg.Value = v
	//TODO append writer id to timestamp
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
		// Special case: diffResultsErr
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
	err := thisClient.writeQuorum(readMsg)
	if err != nil {
		return nil, err
	}

	return readMsg.Value, nil
}

type RegisterMsg struct {
	Value     interface{} // Value of the register
	Timestamp int         // Timestamp of the register
	View      *view.View  // Current client's view
	Err       error       // Any RPC or register service errors
}
