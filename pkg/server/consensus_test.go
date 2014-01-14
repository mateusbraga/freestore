package server

import (
	"testing"

	"github.com/mateusbraga/freestore/pkg/view"
)

func TestDatabaseFunctions(t *testing.T) {
	initStorage()

	currentView.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}})
	currentView.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}})
	currentView.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}})

	thisProcess = view.Process{"[::]:5001"}

	if key, value, _ := db.First(); key != nil && value != nil {
		t.Errorf("Database is not empty or unitinialized")
	}

	if proposalNumber, err := getLastProposalNumber(0); proposalNumber != 0 || err == nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 0 and err != nil, got %v and %v", proposalNumber, err)
	}

	saveProposalNumber(0, 1)
	if proposalNumber, err := getLastProposalNumber(0); proposalNumber != 1 || err != nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 1 and err == nil, got %v and %v", proposalNumber, err)
	}

	if proposalNumber := getNextProposalNumber(0); proposalNumber != 4 {
		t.Errorf("getLastProposalNumber should return proposalNumber == 4, got %v", proposalNumber)
	}

	if proposalNumber, err := getLastProposalNumber(0); proposalNumber != 4 || err != nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 4 and err == nil, got %v and %v", proposalNumber, err)
	}
}
