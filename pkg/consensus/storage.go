package consensus

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"

	"github.com/cznic/kv"
)

var (
	storage *kv.DB
)

// ------- Init storage -----------
func initStorage() {
	var err error
	storage, err = kv.CreateMem(new(kv.Options))
	if err != nil {
		log.Fatalln("initStorage error:", err)
	}
}

func init() {
	initStorage()
}

// saveProposalNumberOnStorage to permanent storage.
func saveProposalNumberOnStorage(consensusId int, proposalNumber int) {
	proposalNumberBuffer := new(bytes.Buffer)
	enc := gob.NewEncoder(proposalNumberBuffer)
	err := enc.Encode(proposalNumber)
	if err != nil {
		log.Fatalln("enc.Encode failed:", err)
	}

	err = storage.Set([]byte(fmt.Sprintf("lastProposalNumber_%v", consensusId)), proposalNumberBuffer.Bytes())
	if err != nil {
		log.Fatalln("storage.Set failed:", err)
	}
}

// getLastProposalNumber from permanent storage.
func getLastProposalNumber(consensusId int) (int, error) {
	lastProposalNumberBytes, err := storage.Get(nil, []byte(fmt.Sprintf("lastProposalNumber_%v", consensusId)))
	if err != nil {
		log.Fatalln(err)
	} else if lastProposalNumberBytes == nil {
		return 0, errors.New("Last proposal number not found")
	} else {
		var lastProposalNumber int

		lastProposalNumberBuffer := bytes.NewBuffer(lastProposalNumberBytes)
		dec := gob.NewDecoder(lastProposalNumberBuffer)

		err := dec.Decode(&lastProposalNumber)
		if err != nil {
			log.Fatalln("dec.Decode failed:", err)
		}

		return lastProposalNumber, nil
	}

	log.Fatalln("BUG! Should never execute this command on getLastProposalNumber")
	return 0, nil
}

// saveAcceptedProposalOnStorage to permanent storage.
// needs to be tested
func saveAcceptedProposalOnStorage(ci consensusInstance, proposal *Proposal) {
	proposalBuffer := new(bytes.Buffer)
	enc := gob.NewEncoder(proposalBuffer)

	err := enc.Encode(proposal)
	if err != nil {
		log.Fatalln("enc.Encode failed:", err)
	}

	err = storage.Set([]byte(fmt.Sprintf("acceptedProposal_%v", ci.Id())), proposalBuffer.Bytes())
	if err != nil {
		log.Fatalln("ERROR to save acceptedProposal:", err)
	}
}

// savePrepareRequestOnStorage to permanent storage
// needs to be tested
func savePrepareRequestOnStorage(ci consensusInstance, proposal *Prepare) {
	proposalBuffer := new(bytes.Buffer)
	enc := gob.NewEncoder(proposalBuffer)
	err := enc.Encode(proposal)
	if err != nil {
		log.Fatalln("enc.Encode failed:", err)
	}

	err = storage.Set([]byte(fmt.Sprintf("prepareRequest_%v", ci.Id())), proposalBuffer.Bytes())
	if err != nil {
		log.Fatalln("ERROR to save prepareRequest:", err)
	}
}
