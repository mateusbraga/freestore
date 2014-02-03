package consensus

import (
	"testing"

	"github.com/mateusbraga/freestore/pkg/view"
)

func TestDatabaseFunctions(t *testing.T) {
	initStorage()

	associatedView := view.New()
	associatedView.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}})
	associatedView.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}})
	associatedView.AddUpdate(view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}})

	thisProcess := view.Process{"[::]:5001"}

	if key, value, _ := storage.First(); key != nil && value != nil {
		t.Errorf("storage is not empty or unitinialized")
	}

	if proposalNumber, err := getLastProposalNumber(associatedView.NumberOfEntries()); proposalNumber != 0 || err == nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 0 and err != nil, got %v and %v", proposalNumber, err)
	}

	saveProposalNumberOnStorage(associatedView.NumberOfEntries(), 1)
	if proposalNumber, err := getLastProposalNumber(associatedView.NumberOfEntries()); proposalNumber != 1 || err != nil {
		t.Errorf("getLastProposalNumber should return proposalNumber == 1 and err == nil, got %v and %v", proposalNumber, err)
	}

	if proposalNumber := getNextProposalNumber(associatedView, thisProcess); proposalNumber != 4 {
		t.Errorf("getNextProposalNumber: expected proposalNumber == 4, got %v", proposalNumber)
	}

	if proposalNumber, err := getLastProposalNumber(associatedView.NumberOfEntries()); proposalNumber != 4 || err != nil {
		t.Errorf("getNextProposalNumber: expted proposalNumber == 4 and err == <nil>, got %v and %v", proposalNumber, err)
	}
}
