package backend

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"

	"mateusbraga/gotf/view"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	OldProposalNumberErr OldProposalNumberError
	consensusTable       ConsensusTable
)

func init() {
	consensusTable.Table = make(map[int]*Consensus)
}

type ConsensusTable struct {
	Table map[int]*Consensus
	mu    sync.RWMutex
}

func GetConsensusOrCreate(id int, callbackLearnChan chan interface{}) *Consensus {
	consensusTable.mu.Lock()
	defer consensusTable.mu.Unlock()

	if consensus, ok := consensusTable.Table[id]; ok {
		return consensus
	} else {
		log.Println("Creating consensus with id", id)
		consensus := &Consensus{Id: id, CallbackLearnChan: callbackLearnChan}
		consensusTable.Table[consensus.Id] = consensus
		return consensus
	}
}

func (consensusTable *ConsensusTable) GetConsensus(id int) *Consensus {
	consensusTable.mu.Lock()
	defer consensusTable.mu.Unlock()

	if consensus, ok := consensusTable.Table[id]; ok {
		return consensus
	} else {
		log.Println("Creating consensus with id", id)
		consensus := &Consensus{Id: id, CallbackLearnChan: make(chan interface{}, 1)}
		consensusTable.Table[consensus.Id] = consensus
		return consensus
	}
}

type Proposal struct {
	ConsensusId int
	N           int
	Value       interface{}
	Err         error
}

type Consensus struct {
	Id int

	acceptedProposal          Proposal // highest numbered accepted proposal
	lastPromiseProposalNumber int      // highest numbered prepare request
	learnCounter              int      // number of learn requests

	mu sync.RWMutex

	CallbackLearnChan chan interface{}
}

func (consensus *Consensus) SetLastPromiseProposalNumber(proposal *Proposal) {
	consensus.savePrepareRequest(proposal)
	consensus.lastPromiseProposalNumber = proposal.N
}

func (consensus *Consensus) SetAcceptedProposal(proposal *Proposal) {
	consensus.saveAcceptedProposal(proposal)
	consensus.acceptedProposal = *proposal
}

func (consensus *Consensus) NewPrepareRequest(proposal *Proposal, reply *Proposal) {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	if proposal.N > consensus.lastPromiseProposalNumber {
		consensus.SetLastPromiseProposalNumber(proposal)
		reply.N = consensus.acceptedProposal.N
		reply.Value = consensus.acceptedProposal.Value
	} else {
		reply.Err = OldProposalNumberErr
	}
	return
}

func (consensus *Consensus) NewAcceptRequest(proposal *Proposal, reply *Proposal) {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	if proposal.N >= consensus.lastPromiseProposalNumber {
		consensus.SetAcceptedProposal(proposal)
		go spreadAcceptance(*proposal)
	} else {
		reply.Err = OldProposalNumberErr
	}
	return
}

func (consensus *Consensus) NewLearnRequest(proposal Proposal) {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	//log.Println("learn", proposal)
	consensus.learnCounter++

	if consensus.learnCounter >= currentView.QuorunSize() {
		select { // Send just the size of the channel/Do not block
		case consensus.CallbackLearnChan <- proposal.Value:
			//log.Println("sent to callback")
		default:
		}
	}
}

func (consensus *Consensus) Propose(defaultValue interface{}) {
	proposalNumber := consensus.getNextProposalNumber()

	value, err := consensus.Prepare(proposalNumber)
	if err != nil {
		log.Println("WARN: Failed to propose. Could not pass prepare phase")
		return
	}

	if value == nil {
		value = defaultValue
	}

	proposal := Proposal{N: proposalNumber, Value: value}
	if err := consensus.Accept(proposal); err != nil {
		log.Println("WARN: Failed to propose. Could not pass accept phase")
		return
	}
}

func (consensus *Consensus) Prepare(proposalNumber int) (interface{}, error) {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())

	proposal := Proposal{N: proposalNumber, ConsensusId: consensus.Id}

	// Send read request to all
	for _, process := range currentView.GetMembers() {
		go prepareProcess(process, proposal, resultChan, errChan)
	}

	// Get quorum
	var failed int
	var success int
	var highestNumberedAcceptedProposal Proposal
	for {
		select {
		case receivedProposal := <-resultChan:
			if receivedProposal.Err != nil {
				errChan <- receivedProposal.Err
				break
			}

			success++
			if highestNumberedAcceptedProposal.N < receivedProposal.N {
				highestNumberedAcceptedProposal = receivedProposal
			}

			if success >= currentView.QuorunSize() {
				return highestNumberedAcceptedProposal.Value, nil
			}
		case err := <-errChan:
			log.Println("+1 failure to prepare:", err)
			failed++

			if failed > currentView.F() {
				return Value{}, errors.New("Failed to get prepare quorun")
			}
		}
	}
}

func (consensus *Consensus) Accept(proposal Proposal) error {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())

	proposal.ConsensusId = consensus.Id

	// Send accept request to all
	for _, process := range currentView.GetMembers() {
		go acceptProcess(process, proposal, resultChan, errChan)
	}

	// Get quorum
	var failed int
	var success int
	for {
		select {
		case receivedProposal := <-resultChan:
			if receivedProposal.Err != nil {
				errChan <- receivedProposal.Err
				break
			}

			success++

			if success >= currentView.QuorunSize() {
				return nil
			}
		case err := <-errChan:
			log.Println("+1 failure to accept:", err)
			failed++

			if failed > currentView.F() {
				return errors.New("Failed to get accept quorun")
			}
		}
	}
}

func (consensus *Consensus) checkForChosenValue() (interface{}, error) {
	proposalNumber := consensus.getNextProposalNumber()

	value, err := consensus.Prepare(proposalNumber)
	if err != nil {
		return nil, errors.New("Could not read prepare consensus")
	}
	if value == nil {
		return nil, errors.New("No value has been chosen")
	}

	return value, nil
}

func (consensus Consensus) getNextProposalNumber() (proposalNumber int) {
	thisProcessPosition := currentView.GetProcessPosition(thisProcess)

	lastProposalNumber, err := consensus.getLastProposalNumber()
	if err != nil {
		if err == sql.ErrNoRows {
			proposalNumber = currentView.N() + thisProcessPosition
		} else {
			log.Fatal(err)
		}
	} else {
		proposalNumber = ((lastProposalNumber / currentView.N()) + 1) + thisProcessPosition
	}

	consensus.saveProposalNumber(proposalNumber)
	return
}

func (consensus Consensus) saveProposalNumber(proposalNumber int) {
	// MODEL
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("insert into proposal_number(consensus_id, last_proposal_number) values (?, ?)", consensus.Id, proposalNumber)
	if err != nil {
		log.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func (consensus Consensus) getLastProposalNumber() (int, error) {
	// MODEL
	var proposalNumber int
	err := db.QueryRow("select last_proposal_number from proposal_number WHERE consensus_id = ? ORDER BY last_proposal_number DESC", consensus.Id).Scan(&proposalNumber)
	if err != nil {
		return 0, err
	}

	return proposalNumber, nil
}

func (consensus Consensus) saveAcceptedProposal(proposal *Proposal) {
	// TODO make this work with slices
	//tx, err := db.Begin()
	//if err != nil {
	//log.Fatal(err)
	//}
	//_, err = tx.Exec("insert into accepted_proposal(consensus_id, accepted_proposal) values (?, ?)", consensus.Id, proposal.Value)
	//if err != nil {
	//log.Fatal(err)
	//}
	//err = tx.Commit()
	//if err != nil {
	//log.Fatal(err)
	//}
}

func (consensus Consensus) savePrepareRequest(proposal *Proposal) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("insert into prepare_request(consensus_id, highest_proposal_number) values (?, ?)", consensus.Id, proposal.N)
	if err != nil {
		log.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func spreadAcceptance(proposal Proposal) {
	// Send acceptances to all
	// TODO can improve: it will send too many messages
	for _, process := range currentView.GetMembers() {
		go spreadAcceptanceProcess(process, proposal)
	}
}

func prepareProcess(process view.Process, proposal Proposal, resultChan chan Proposal, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var reply Proposal

	err = client.Call("ConsensusRequest.Prepare", proposal, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}

func acceptProcess(process view.Process, proposal Proposal, resultChan chan Proposal, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var reply Proposal
	err = client.Call("ConsensusRequest.Accept", proposal, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}

func spreadAcceptanceProcess(process view.Process, proposal Proposal) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Println("WARN: learnProcess:", err)
		return
	}
	defer client.Close()

	var reply Proposal
	err = client.Call("ConsensusRequest.Learn", proposal, &reply)
	if err != nil {
		log.Println("WARN: learnProcess:", err)
		return
	}
}

// -------- REQUESTS -----------
type ConsensusRequest int

func (r *ConsensusRequest) Prepare(arg Proposal, reply *Proposal) error {
	consensus := consensusTable.GetConsensus(arg.ConsensusId)
	consensus.NewPrepareRequest(&arg, reply)
	return nil
}

func (r *ConsensusRequest) Accept(arg Proposal, reply *Proposal) error {
	consensus := consensusTable.GetConsensus(arg.ConsensusId)
	consensus.NewAcceptRequest(&arg, reply)
	return nil
}

func (r *ConsensusRequest) Learn(arg Proposal, reply *Proposal) error {
	consensus := consensusTable.GetConsensus(arg.ConsensusId)
	go consensus.NewLearnRequest(arg)
	return nil
}

func init() {
	consensusRequest := new(ConsensusRequest)
	rpc.Register(consensusRequest)
}

// ------- ERRORS -----------
type OldProposalNumberError struct{}

func (e OldProposalNumberError) Error() string {
	return fmt.Sprint("Promised to a higher numbered prepare request")
}

func init() {
	gob.Register(new(OldProposalNumberError))
}
