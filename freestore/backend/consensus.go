package backend

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"

	"mateusbraga/gotf/freestore/view"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	oldProposalNumberErr OldProposalNumberError
	consensusTable       ConsensusTable
)

func init() {
	consensusTable.Table = make(map[int]*Consensus)
}

type ConsensusTable struct {
	Table map[int]*Consensus
	mu    sync.RWMutex
}

// GetConsensusOrCreate return the consensus instance with id. Create if it does not exist.
func GetConsensusOrCreate(id int, callbackLearnChan chan interface{}) *Consensus {
	consensusTable.mu.Lock()
	defer consensusTable.mu.Unlock()

	if consensus, ok := consensusTable.Table[id]; ok {
		return consensus
	} else {
		log.Println("Creating consensus with id", id)
		consensus := &Consensus{id: id, CallbackLearnChan: callbackLearnChan}
		consensusTable.Table[consensus.id] = consensus
		return consensus
	}
}

type Proposal struct {
	ConsensusId int //ConsensusId makes possible multiples consensus to run at the same time

	N int // N is the proposal number

	Value interface{} // Value proposed

	Err error // Err is used to return an error related to the proposal
}

type Consensus struct {
	id int // id makes possible multiples consensus to run at the same time

	acceptedProposal          Proposal // highest numbered accepted proposal
	lastPromiseProposalNumber int      // highest numbered prepare request
	learnCounter              int      // number of learn requests received

	mu sync.RWMutex

	CallbackLearnChan chan interface{}
}

// setLastPromiseProposalNumber save prepare proposal to permanent storage and then sets consensus.lastPromiseProposalNumber.
//
// This function must not be executed without concorrency control
func (consensus *Consensus) setLastPromiseProposalNumber(proposal *Proposal) {
	savePrepareRequest(consensus.id, proposal)
	consensus.lastPromiseProposalNumber = proposal.N
}

// setAcceptedProposal save accepted proposal to permanent storage and then sets consensus.acceptedProposal.
//
// This function must not be executed without concorrency control
func (consensus *Consensus) setAcceptedProposal(proposal *Proposal) {
	saveAcceptedProposal(consensus.id, proposal)
	consensus.acceptedProposal = *proposal
}

// newPrepareRequest processes the proposal prepare request and returns reply. Like a rpc funcion.
func (consensus *Consensus) newPrepareRequest(proposal *Proposal, reply *Proposal) {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	if proposal.N > consensus.lastPromiseProposalNumber {
		consensus.setLastPromiseProposalNumber(proposal)
		reply.N = consensus.acceptedProposal.N
		reply.Value = consensus.acceptedProposal.Value
	} else {
		reply.Err = oldProposalNumberErr
	}
	return
}

// newAcceptRequest processes the proposal accept request and returns reply. Like a rpc funcion.
func (consensus *Consensus) newAcceptRequest(proposal *Proposal, reply *Proposal) {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	if proposal.N >= consensus.lastPromiseProposalNumber {
		consensus.setAcceptedProposal(proposal)
		go spreadAcceptance(*proposal)
	} else {
		reply.Err = oldProposalNumberErr
	}
	return
}

// newLearnRequest processes the proposal learn request.
func (consensus *Consensus) newLearnRequest(proposal Proposal) {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	consensus.learnCounter++
	if consensus.learnCounter == currentView.QuorumSize() {
		consensus.CallbackLearnChan <- proposal.Value
	}
}

// Propose proposes the value to be agreed upon on this consensus instance. It should be run only by the leader process to guarantee termination.
//
// Has concurrency control
func (consensus *Consensus) Propose(defaultValue interface{}) {
	consensusId := consensus.Id()

	proposalNumber := getNextProposalNumber(consensusId)

	value, err := consensus.prepare(proposalNumber)
	if err != nil {
		// Could not get quorum or old proposal number
		log.Println("WARN: Failed to propose. Could not pass prepare phase: ", err)
		return
	}

	if value == nil {
		value = defaultValue
	}

	proposal := Proposal{N: proposalNumber, Value: value}
	if err := consensus.accept(proposal); err != nil {
		// Could not get quorum or old proposal number
		log.Println("WARN: Failed to propose. Could not pass accept phase: ", err)
		return
	}
}

// Propose proposes the value to be agreed upon on this consensus instance. It should be run only by the leader process to guarantee termination.
func (consensus *Consensus) Id() int {
	consensus.mu.Lock()
	defer consensus.mu.Unlock()

	return consensus.id
}

// prepare is a stage of the Propose funcion
func (consensus *Consensus) prepare(proposalNumber int) (interface{}, error) {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())
	stopChan := make(chan bool, currentView.N())
	defer fillStopChan(stopChan, currentView.N())

	proposal := Proposal{N: proposalNumber, ConsensusId: consensus.Id()}

	// Send read request to all
	for _, process := range currentView.GetMembers() {
		go prepareProcess(process, proposal, resultChan, errChan, stopChan)
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

			if success == currentView.QuorumSize() {
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

// accept is a stage of the Propose funcion.
func (consensus *Consensus) accept(proposal Proposal) error {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())
	stopChan := make(chan bool, currentView.N())
	defer fillStopChan(stopChan, currentView.N())

	proposal.ConsensusId = consensus.Id()

	// Send accept request to all
	for _, process := range currentView.GetMembers() {
		go acceptProcess(process, proposal, resultChan, errChan, stopChan)
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

			if success == currentView.QuorumSize() {
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

// CheckForChosenValue checks to see if any value has already been agreed upon on this consensus instance.
func (consensus *Consensus) CheckForChosenValue() (interface{}, error) {
	consensusId := consensus.Id()

	proposalNumber := getNextProposalNumber(consensusId)

	value, err := consensus.prepare(proposalNumber)
	if err != nil {
		return nil, errors.New("Could not read prepare consensus")
	}
	if value == nil {
		return nil, errors.New("No value has been chosen")
	}

	return value, nil
}

// getNextProposalNumber to be used by this process. This function is a stage of the Propose funcion.
func getNextProposalNumber(consensusId int) (proposalNumber int) {
	thisProcessPosition := currentView.GetProcessPosition(thisProcess)

	lastProposalNumber, err := getLastProposalNumber(consensusId)
	if err != nil {
		if err == sql.ErrNoRows {
			proposalNumber = currentView.N() + thisProcessPosition
		} else {
			log.Fatal(err)
		}
	} else {
		proposalNumber = ((lastProposalNumber / currentView.N()) + 1) + thisProcessPosition
	}

	saveProposalNumber(consensusId, proposalNumber)
	return
}

// saveProposalNumber to permanent storage.
func saveProposalNumber(consensusId int, proposalNumber int) {
	// MODEL
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("insert into proposal_number(consensus_id, last_proposal_number) values (?, ?)", consensusId, proposalNumber)
	if err != nil {
		log.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

// getLastProposalNumber from permanent storage.
func getLastProposalNumber(consensusId int) (int, error) {
	// MODEL
	var proposalNumber int
	err := db.QueryRow("select last_proposal_number from proposal_number WHERE consensus_id = ? ORDER BY last_proposal_number DESC", consensusId).Scan(&proposalNumber)
	if err != nil {
		return 0, err
	}

	return proposalNumber, nil
}

// saveAcceptedProposal to permanent storage.
func saveAcceptedProposal(consensusId int, proposal *Proposal) {
	// TODO make this work with slices
	//tx, err := db.Begin()
	//if err != nil {
	//log.Fatal(err)
	//}
	//_, err = tx.Exec("insert into accepted_proposal(consensus_id, accepted_proposal) values (?, ?)", consensusId, proposal.Value)
	//if err != nil {
	//log.Fatal(err)
	//}
	//err = tx.Commit()
	//if err != nil {
	//log.Fatal(err)
	//}
}

// savePrepareRequest to permanent storage
func savePrepareRequest(consensusId int, proposal *Proposal) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("insert into prepare_request(consensus_id, highest_proposal_number) values (?, ?)", consensusId, proposal.N)
	if err != nil {
		log.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

// spreadAcceptance to all processes on the current view
func spreadAcceptance(proposal Proposal) {
	// Send acceptances to all
	// TODO can improve: it will send too many messages
	for _, process := range currentView.GetMembers() {
		go spreadAcceptanceProcess(process, proposal)
	}
}

// prepareProcess sends a prepare proposal to process.
func prepareProcess(process view.Process, proposal Proposal, resultChan chan Proposal, errChan chan error, stopChan chan bool) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		select {
		case errChan <- err:
		case <-stopChan:
			log.Println("Failure masked:", err)
		}
		return
	}
	defer client.Close()

	var reply Proposal

	err = client.Call("ConsensusRequest.Prepare", proposal, &reply)
	if err != nil {
		select {
		case errChan <- err:
		case <-stopChan:
			log.Println("Failure masked:", err)
		}
		return
	}

	select {
	case resultChan <- reply:
	case <-stopChan:
	}
}

// acceptProcess sends a prepare proposal to process.
func acceptProcess(process view.Process, proposal Proposal, resultChan chan Proposal, errChan chan error, stopChan chan bool) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		select {
		case errChan <- err:
		case <-stopChan:
			log.Println("Failure masked:", err)
		}
		return
	}
	defer client.Close()

	var reply Proposal
	err = client.Call("ConsensusRequest.Accept", proposal, &reply)
	if err != nil {
		select {
		case errChan <- err:
		case <-stopChan:
			log.Println("Failure masked:", err)
		}
		return
	}

	select {
	case resultChan <- reply:
	case <-stopChan:
	}
}

// spreadAcceptance sends acceptance to process.
func spreadAcceptanceProcess(process view.Process, proposal Proposal) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		log.Println("WARN: spreadAcceptanceProcess:", err)
		return
	}
	defer client.Close()

	var reply Proposal
	err = client.Call("ConsensusRequest.Learn", proposal, &reply)
	if err != nil {
		log.Println("WARN: spreadAcceptanceProcess:", err)
		return
	}
}

// -------- REQUESTS -----------
type ConsensusRequest int

// Prepare Request
func (r *ConsensusRequest) Prepare(arg Proposal, reply *Proposal) error {
	consensus := GetConsensusOrCreate(arg.ConsensusId, make(chan interface{}, 1))
	consensus.newPrepareRequest(&arg, reply)
	return nil
}

// Accept Request
func (r *ConsensusRequest) Accept(arg Proposal, reply *Proposal) error {
	consensus := GetConsensusOrCreate(arg.ConsensusId, make(chan interface{}, 1))
	consensus.newAcceptRequest(&arg, reply)
	return nil
}

// Learn Request
func (r *ConsensusRequest) Learn(arg Proposal, reply *Proposal) error {
	consensus := GetConsensusOrCreate(arg.ConsensusId, make(chan interface{}, 1))
	go consensus.newLearnRequest(arg)
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

// ------- Auxiliary functions -----------
// fillStopChan send times * 'true' on stopChan. It is used to signal completion to readProcess and writeProcess.
func fillStopChan(stopChan chan bool, times int) {
	for i := 0; i < times; i++ {
		stopChan <- true
	}
}
