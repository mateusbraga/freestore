package server

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/comm"
	"github.com/mateusbraga/freestore/view"
)

var (
	consensusTable   map[int]consensusInstance
	consensusTableMu sync.RWMutex

	oldProposalNumberErr OldProposalNumberError
)

type consensusInstance struct {
	id                int
	taskChan          chan consensusTask
	callbackLearnChan chan interface{}
}

type consensusTask interface{}

func init() {
	consensusTable = make(map[int]consensusInstance)
}

func getConsensus(id int) consensusInstance {
	consensusTableMu.Lock()
	defer consensusTableMu.Unlock()

	if ci, ok := consensusTable[id]; ok {
		return ci
	} else {
		ci := consensusInstance{id: id, taskChan: make(chan consensusTask, CHANNEL_DEFAULT_SIZE), callbackLearnChan: make(chan interface{}, 1)}
		go consensusWorker(ci)

		consensusTable[ci.id] = ci
		log.Println("Created consensusInstance:", ci)
		return ci
	}
}

func consensusWorker(ci consensusInstance) {
	var acceptedProposal Proposal     // highest numbered accepted proposal
	var lastPromiseProposalNumber int // highest numbered prepare request
	var learnCounter int              // number of learn requests received

	for {
		taskInterface, ok := <-ci.taskChan
		if !ok {
			log.Printf("consensusInstance %v done\n", ci)
			return
		}

		switch task := taskInterface.(type) {
		case *Prepare:
			log.Println("Processing prepare request")
			if task.N > lastPromiseProposalNumber {
				//setLastPromiseProposalNumber
				savePrepareRequest(ci.id, task)
				lastPromiseProposalNumber = task.N

				task.reply.N = acceptedProposal.N
				task.reply.Value = acceptedProposal.Value
			} else {
				task.reply.Err = oldProposalNumberErr
			}
			task.returnChan <- true
		case *Accept:
			log.Println("Processing accept request")
			if task.N >= lastPromiseProposalNumber {
				//setAcceptedProposal
				saveAcceptedProposal(ci.id, task.Proposal)
				acceptedProposal = *task.Proposal

				go spreadAcceptance(*task.Proposal)
			} else {
				task.reply.Err = oldProposalNumberErr
			}
			task.returnChan <- true
		case *Learn:
			log.Println("Processing learn request")
			learnCounter++
			if learnCounter == currentView.QuorumSize() {
				ci.callbackLearnChan <- task.Value
			}
		default:
			log.Fatalf("BUG in the ConsensusWorker switch, got %T %v\n", task, task)
		}
	}
}

// Propose proposes the value to be agreed upon on this consensus instance. It should be run only by the leader process to guarantee termination.
func Propose(ci consensusInstance, defaultValue interface{}) {
	log.Println("Got in propose")
	proposalNumber := getNextProposalNumber(ci.id)

	proposal := Proposal{N: proposalNumber, ConsensusId: ci.id}
	value, err := prepare(proposal)
	if err != nil {
		// Could not get quorum or old proposal number
		log.Println("WARN: Failed to propose. Could not pass prepare phase: ", err)
		return
	}

	if value == nil {
		value = defaultValue
	}

	proposal.Value = value
	if err := accept(proposal); err != nil {
		// Could not get quorum or old proposal number
		log.Println("WARN: Failed to propose. Could not pass accept phase: ", err)
		return
	}
}

// prepare is a stage of the Propose funcion
func prepare(proposal Proposal) (interface{}, error) {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())

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
func accept(proposal Proposal) error {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())

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
func CheckForChosenValue(ci consensusInstance) (interface{}, error) {
	proposalNumber := getNextProposalNumber(ci.id)

	proposal := Proposal{N: proposalNumber, ConsensusId: ci.id}
	value, err := prepare(proposal)
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
	if currentView.N() == 0 {
		log.Fatalln("currentView is empty")
	}

	thisProcessPosition := currentView.GetProcessPosition(thisProcess)

	lastProposalNumber, err := getLastProposalNumber(consensusId)
	if err != nil {
		proposalNumber = currentView.N() + thisProcessPosition
	} else {
		proposalNumber = (lastProposalNumber - (lastProposalNumber % currentView.N()) + currentView.N()) + thisProcessPosition
	}

	saveProposalNumber(consensusId, proposalNumber)
	return
}

// saveProposalNumber to permanent storage.
func saveProposalNumber(consensusId int, proposalNumber int) {
	proposalNumberBuffer := new(bytes.Buffer)
	enc := gob.NewEncoder(proposalNumberBuffer)
	err := enc.Encode(proposalNumber)
	if err != nil {
		log.Fatalln("enc.Encode failed:", err)
	}

	err = db.Set([]byte(fmt.Sprintf("lastProposalNumber_%v", consensusId)), proposalNumberBuffer.Bytes())
	if err != nil {
		log.Fatalln("db.Set failed:", err)
	}
}

// getLastProposalNumber from permanent storage.
func getLastProposalNumber(consensusId int) (int, error) {
	lastProposalNumberBytes, err := db.Get(nil, []byte(fmt.Sprintf("lastProposalNumber_%v", consensusId)))
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

// saveAcceptedProposal to permanent storage.
// needs to be tested
func saveAcceptedProposal(consensusId int, proposal *Proposal) {
	proposalBuffer := new(bytes.Buffer)
	enc := gob.NewEncoder(proposalBuffer)

	err := enc.Encode(proposal)
	if err != nil {
		log.Fatalln("enc.Encode failed:", err)
	}

	err = db.Set([]byte(fmt.Sprintf("acceptedProposal_%v", consensusId)), proposalBuffer.Bytes())
	if err != nil {
		log.Fatalln("ERROR to save acceptedProposal:", err)
	}
}

// savePrepareRequest to permanent storage
// needs to be tested
func savePrepareRequest(consensusId int, proposal *Prepare) {
	proposalBuffer := new(bytes.Buffer)
	enc := gob.NewEncoder(proposalBuffer)
	err := enc.Encode(proposal)
	if err != nil {
		log.Fatalln("enc.Encode failed:", err)
	}

	err = db.Set([]byte(fmt.Sprintf("prepareRequest_%v", consensusId)), proposalBuffer.Bytes())
	if err != nil {
		log.Fatalln("ERROR to save prepareRequest:", err)
	}
}

// spreadAcceptance to all processes on the current view
func spreadAcceptance(proposal Proposal) {
	// Send acceptances to all
	// Can be improved: it is currently send too many messages at once
	for _, process := range currentView.GetMembers() {
		go spreadAcceptanceProcess(process, proposal)
	}
}

// prepareProcess sends a prepare proposal to process.
func prepareProcess(process view.Process, proposal Proposal, resultChan chan Proposal, errChan chan error) {
	var reply Proposal
	err := comm.SendRPCRequest(process, "ConsensusRequest.Prepare", proposal, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}

// acceptProcess sends a prepare proposal to process.
func acceptProcess(process view.Process, proposal Proposal, resultChan chan Proposal, errChan chan error) {
	var reply Proposal
	err := comm.SendRPCRequest(process, "ConsensusRequest.Accept", proposal, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}

// spreadAcceptance sends acceptance to process.
func spreadAcceptanceProcess(process view.Process, proposal Proposal) {
	err := comm.SendRPCRequest(process, "ConsensusRequest.Learn", proposal, Proposal{})
	if err != nil {
		log.Println("WARN: spreadAcceptanceProcess:", err)
		return
	}
}

// -------- REQUESTS -----------
type ConsensusRequest int

type Proposal struct {
	ConsensusId int //ConsensusId makes possible multiples consensus to run at the same time

	N int // N is the proposal number

	Value interface{} // Value proposed

	Err error // Err is used to return an error related to the proposal
}

type Prepare struct {
	*Proposal
	reply      *Proposal
	returnChan chan bool
}

type Accept struct {
	*Proposal
	reply      *Proposal
	returnChan chan bool
}

type Learn struct {
	*Proposal
}

// Prepare Request
func (r *ConsensusRequest) Prepare(arg Proposal, reply *Proposal) error {
	log.Println("New Prepare Request")
	ci := getConsensus(arg.ConsensusId)

	var prepare Prepare
	prepare.Proposal = &arg
	prepare.returnChan = make(chan bool)
	prepare.reply = reply

	ci.taskChan <- &prepare

	<-prepare.returnChan

	return nil
}

// Accept Request
func (r *ConsensusRequest) Accept(arg Proposal, reply *Proposal) error {
	log.Println("New Accept Request")
	ci := getConsensus(arg.ConsensusId)

	var accept Accept
	accept.Proposal = &arg
	accept.returnChan = make(chan bool)
	accept.reply = reply

	ci.taskChan <- &accept

	<-accept.returnChan

	return nil
}

// Learn Request
func (r *ConsensusRequest) Learn(arg Proposal, reply *Proposal) error {
	log.Println("New Learn Request")
	ci := getConsensus(arg.ConsensusId)

	var learn Learn
	learn.Proposal = &arg

	ci.taskChan <- &learn

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
