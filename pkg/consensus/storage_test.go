package consensus

import (
	"testing"

	"github.com/mateusbraga/freestore/pkg/view"
)

func TestDatabaseFunctions(t *testing.T) {
	initStorage()

	updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}},
		view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}},
		view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}}}

	associatedView := view.NewWithUpdates(updates...)

	thisProcess := view.Process{"[::]:5001"}

	if key, value, _ := storage.First(); key != nil && value != nil {
		t.Errorf("storage is not empty or unitinialized")
	}

	if proposalNumber, err := getLastProposalNumber(associatedView.NumberOfUpdates()); proposalNumber != 0 || err == nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 0 and err != nil, got %v and %v", proposalNumber, err)
	}

	saveProposalNumberOnStorage(associatedView.NumberOfUpdates(), 1)
	if proposalNumber, err := getLastProposalNumber(associatedView.NumberOfUpdates()); proposalNumber != 1 || err != nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 1 and err == nil, got %v and %v", proposalNumber, err)
	}

	if proposalNumber := getNextProposalNumber(associatedView, thisProcess); proposalNumber != 4 {
		t.Errorf("getNextProposalNumber: expected proposalNumber == 4, got %v", proposalNumber)
	}

	if proposalNumber, err := getLastProposalNumber(associatedView.NumberOfUpdates()); proposalNumber != 4 || err != nil {
		t.Errorf("getNextProposalNumber: expted proposalNumber == 4 and err == <nil>, got %v and %v", proposalNumber, err)
	}
}
