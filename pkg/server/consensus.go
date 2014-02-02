package server

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"

	"github.com/cznic/kv"
)

var (
	consensusTable   = make(map[int]consensusInstance)
	consensusTableMu sync.RWMutex

	oldProposalNumberErr OldProposalNumberError

	database *kv.DB
)

type consensusInstance struct {
	id                int
	taskChan          chan consensusTask
	callbackLearnChan chan interface{}
}

type consensusTask interface{}

func getConsensus(consensusId int) consensusInstance {
	consensusTableMu.Lock()
	defer consensusTableMu.Unlock()

	ci, ok := consensusTable[consensusId]
	if !ok {
		ci = consensusInstance{id: consensusId, taskChan: make(chan consensusTask, CHANNEL_DEFAULT_SIZE), callbackLearnChan: make(chan interface{}, 1)}
		consensusTable[ci.id] = ci
		log.Println("Created consensus instance:", ci)

		go consensusWorker(ci)
	}
	return ci
}

func consensusWorker(ci consensusInstance) {
	var acceptedProposal Proposal     // highest numbered accepted proposal
	var lastPromiseProposalNumber int // highest numbered prepare request
	var learnCounter int              // number of learn requests received

	for {
		taskInterface, ok := <-ci.taskChan
		if !ok {
			log.Printf("consensus instance %v done\n", ci)
			return
		}

		switch task := taskInterface.(type) {
		case *Prepare:
			//log.Println("Processing prepare request")
			receivedPrepareRequest := task

			if receivedPrepareRequest.N > lastPromiseProposalNumber {
				//setLastPromiseProposalNumber
				savePrepareRequest(ci.id, receivedPrepareRequest)
				lastPromiseProposalNumber = receivedPrepareRequest.N

				receivedPrepareRequest.reply.N = acceptedProposal.N
				receivedPrepareRequest.reply.Value = acceptedProposal.Value
			} else {
				receivedPrepareRequest.reply.Err = oldProposalNumberErr
			}

			receivedPrepareRequest.returnChan <- true
		case *Accept:
			//log.Println("Processing accept request")
			receivedAcceptRequest := task

			if receivedAcceptRequest.N >= lastPromiseProposalNumber {
				//setAcceptedProposal
				saveAcceptedProposal(ci.id, receivedAcceptRequest.Proposal)
				acceptedProposal = *receivedAcceptRequest.Proposal

				go broadcastLearnRequest(currentView.NewCopy(), *receivedAcceptRequest.Proposal)
			} else {
				receivedAcceptRequest.reply.Err = oldProposalNumberErr
			}

			receivedAcceptRequest.returnChan <- true
		case *Learn:
			//log.Println("Processing learn request")
			receivedLearnRequest := task

			learnCounter++
			if learnCounter == currentView.QuorumSize() {
				ci.callbackLearnChan <- receivedLearnRequest.Value
			}
		default:
			log.Fatalf("BUG in the ConsensusWorker switch, got %T %v\n", task, task)
		}
	}
}

// Propose proposes the value to be agreed upon on this consensus instance. It should be run only by the leader process to guarantee termination.
func Propose(ci consensusInstance, defaultValue interface{}) {
	log.Println("Running propose with:", defaultValue)

	proposalNumber := getNextProposalNumber(ci.id)
	proposal := Proposal{N: proposalNumber, ConsensusId: ci.id}

	value, err := prepare(proposal)
	if err != nil {
		// Could not get quorum or old proposal number
		log.Println("Failed to propose. Could not pass prepare phase: ", err)
		return
	}

	if value == nil {
		value = defaultValue
	}

	proposal.Value = value
	if err := accept(proposal); err != nil {
		// Could not get quorum or old proposal number
		log.Println("Failed to propose. Could not pass accept phase: ", err)
		return
	}
}

// prepare is a stage of the Propose funcion
func prepare(proposal Proposal) (interface{}, error) {
	immutableCurrentView := currentView.NewCopy()

	// Send read request to all
	resultChan := make(chan Proposal, immutableCurrentView.N())
	go broadcastPrepareRequest(immutableCurrentView, proposal, resultChan)

	// Wait for quorum
	var successTotal int
	var failedTotal int
	var highestNumberedAcceptedProposal Proposal

	countError := func(err error) bool {
		log.Println("+1 error to prepare:", err)
		failedTotal++

		allFailed := failedTotal == immutableCurrentView.N()
		mostFailedInspiteSomeSuccess := successTotal > 0 && failedTotal > immutableCurrentView.F()

		if mostFailedInspiteSomeSuccess || allFailed {
			return false
		}

		return true
	}

	countSuccess := func(receivedProposal Proposal) bool {
		successTotal++
		if highestNumberedAcceptedProposal.N < receivedProposal.N {
			highestNumberedAcceptedProposal = receivedProposal
		}

		if successTotal == immutableCurrentView.QuorumSize() {
			return true
		}

		return false
	}

	for {
		receivedProposal := <-resultChan

		if receivedProposal.Err != nil {
			ok := countError(receivedProposal.Err)
			if !ok {
				return nil, errors.New("Failed to get prepare quorun")
			}
			continue
		}

		done := countSuccess(receivedProposal)
		if done {
			return highestNumberedAcceptedProposal, nil
		}
	}
}

// accept is a stage of the Propose funcion.
func accept(proposal Proposal) error {
	immutableCurrentView := currentView.NewCopy()

	// Send accept request to all
	resultChan := make(chan Proposal, immutableCurrentView.N())
	go broadcastAcceptRequest(immutableCurrentView, proposal, resultChan)

	// Wait for quorum
	var successTotal int
	var failedTotal int

	countError := func(err error) bool {
		log.Println("+1 error to accept:", err)
		failedTotal++

		allFailed := failedTotal == immutableCurrentView.N()
		mostFailedInspiteSomeSuccess := successTotal > 0 && failedTotal > immutableCurrentView.F()

		if mostFailedInspiteSomeSuccess || allFailed {
			return false
		}

		return true
	}

	countSuccess := func() bool {
		successTotal++

		if successTotal == immutableCurrentView.QuorumSize() {
			return true
		}

		return false
	}

	for {
		receivedProposal := <-resultChan

		if receivedProposal.Err != nil {
			ok := countError(receivedProposal.Err)
			if !ok {
				return errors.New("Failed to get accept quorun")
			}
			continue
		}

		done := countSuccess()
		if done {
			return nil
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

	err = database.Set([]byte(fmt.Sprintf("lastProposalNumber_%v", consensusId)), proposalNumberBuffer.Bytes())
	if err != nil {
		log.Fatalln("database.Set failed:", err)
	}
}

// getLastProposalNumber from permanent storage.
func getLastProposalNumber(consensusId int) (int, error) {
	lastProposalNumberBytes, err := database.Get(nil, []byte(fmt.Sprintf("lastProposalNumber_%v", consensusId)))
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

	err = database.Set([]byte(fmt.Sprintf("acceptedProposal_%v", consensusId)), proposalBuffer.Bytes())
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

	err = database.Set([]byte(fmt.Sprintf("prepareRequest_%v", consensusId)), proposalBuffer.Bytes())
	if err != nil {
		log.Fatalln("ERROR to save prepareRequest:", err)
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
func (r *ConsensusRequest) Learn(arg Proposal, reply *int) error {
	log.Println("New Learn Request")
	ci := getConsensus(arg.ConsensusId)

	var learn Learn
	learn.Proposal = &arg

	ci.taskChan <- &learn

	return nil
}

func init() {
	rpc.Register(new(ConsensusRequest))
}

// ------- Init database -----------
func initDatabase() {
	var err error
	database, err = kv.CreateMem(new(kv.Options))
	if err != nil {
		log.Fatalln("initDatabase error:", err)
	}
}

func init() {
	initDatabase()
}

// ------- ERRORS -----------
type OldProposalNumberError struct{}

func (e OldProposalNumberError) Error() string {
	return fmt.Sprint("Promised to a higher numbered prepare request")
}

func init() {
	gob.Register(new(OldProposalNumberError))
}

// ------- Broadcast functions -----------
func broadcastPrepareRequest(destinationView *view.View, proposal Proposal, resultChan chan Proposal) {
	for _, process := range destinationView.GetMembers() {
		go func() {
			var result Proposal
			err := comm.SendRPCRequest(process, "ConsensusRequest.Prepare", proposal, &result)
			if err != nil {
				resultChan <- Proposal{Err: err}
			}
			resultChan <- result
		}()
	}
}

func broadcastAcceptRequest(destinationView *view.View, proposal Proposal, resultChan chan Proposal) {
	for _, process := range destinationView.GetMembers() {
		go func() {
			var result Proposal
			err := comm.SendRPCRequest(process, "ConsensusRequest.Accept", proposal, &result)
			if err != nil {
				resultChan <- Proposal{Err: err}
			}
			resultChan <- result
		}()
	}
}

func broadcastLearnRequest(destinationView *view.View, proposal Proposal) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		var discardResult error
		go comm.SendRPCRequestWithErrorChan(process, "ConsensusRequest.Learn", proposal, &discardResult, errorChan)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan
		if err != nil {
			failedTotal++
			if failedTotal > destinationView.F() {
				log.Fatalln("Failed to send Reconfig to a quorum")
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}
