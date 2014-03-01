package consensus

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

const CHANNEL_DEFAULT_BUFFER_SIZE = 20

var oldProposalNumberErr OldProposalNumberError

var (
	consensusTable   = make(map[int]consensusInstance)
	consensusTableMu sync.RWMutex
)

type consensusInstance struct {
	associatedView    *view.View
	taskChan          chan consensusTask
	callbackLearnChan chan interface{}
}

func (ci consensusInstance) Id() int {
	return ci.associatedView.NumberOfEntries()
}

type consensusTask interface{}

func GetConsensusResultChan(associatedView *view.View) chan interface{} {
	ci := getOrCreateConsensus(associatedView)
	return ci.callbackLearnChan
}

func getOrCreateConsensus(associatedView *view.View) consensusInstance {
	consensusTableMu.Lock()
	defer consensusTableMu.Unlock()

	ci, ok := consensusTable[associatedView.NumberOfEntries()]
	if !ok {
		ci = consensusInstance{associatedView: associatedView, taskChan: make(chan consensusTask, CHANNEL_DEFAULT_BUFFER_SIZE), callbackLearnChan: make(chan interface{}, 1)}
		consensusTable[associatedView.NumberOfEntries()] = ci
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
				savePrepareRequestOnStorage(ci, receivedPrepareRequest)
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
				saveAcceptedProposalOnStorage(ci, receivedAcceptRequest.Proposal)
				acceptedProposal = *receivedAcceptRequest.Proposal

				go broadcastLearnRequest(receivedAcceptRequest.AssociatedView, *receivedAcceptRequest.Proposal)
			} else {
				receivedAcceptRequest.reply.Err = oldProposalNumberErr
			}

			receivedAcceptRequest.returnChan <- true
		case *Learn:
			//log.Println("Processing learn request")
			receivedLearnRequest := task

			learnCounter++
			if learnCounter == receivedLearnRequest.AssociatedView.QuorumSize() {
				ci.callbackLearnChan <- receivedLearnRequest.Value
			}
		default:
			log.Fatalf("BUG in the ConsensusWorker switch, got %T %v\n", task, task)
		}
	}
}

// Propose proposes the value to be agreed upon on this consensus instance. It should be run only by the leader process to guarantee termination.
func Propose(associatedView *view.View, thisProcess view.Process, defaultValue interface{}) {
	log.Println("Running propose with:", defaultValue)

	proposalNumber := getNextProposalNumber(associatedView, thisProcess)
	proposal := Proposal{AssociatedView: associatedView, N: proposalNumber}

	value, err := prepare(proposal)
	if err != nil {
		// Could not get quorum or old proposal number
		log.Fatalf("Failed to propose. Could not pass prepare phase: %v\n", err)
		return
	}

	if value == nil {
		value = defaultValue
	}

	proposal.Value = value
	if err := accept(proposal); err != nil {
		// Could not get quorum or old proposal number
		log.Fatalf("Failed to propose. Could not pass accept phase: %v\n", err)
		return
	}
}

// prepare is a stage of the Propose funcion
func prepare(proposal Proposal) (interface{}, error) {
	// Send read request to all
	resultChan := make(chan Proposal, proposal.AssociatedView.N())
	go broadcastPrepareRequest(proposal.AssociatedView, proposal, resultChan)

	// Wait for quorum
	var successTotal int
	var failedTotal int
	var highestNumberedAcceptedProposal Proposal

	countError := func(err error) bool {
		log.Println("+1 error to prepare:", err)
		failedTotal++

		allFailed := failedTotal == proposal.AssociatedView.N()
		mostFailedInspiteSomeSuccess := successTotal > 0 && failedTotal > proposal.AssociatedView.F()

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

		if successTotal == proposal.AssociatedView.QuorumSize() {
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
			return highestNumberedAcceptedProposal.Value, nil
		}
	}
}

// accept is a stage of the Propose funcion.
func accept(proposal Proposal) error {
	// Send accept request to all
	resultChan := make(chan Proposal, proposal.AssociatedView.N())
	go broadcastAcceptRequest(proposal.AssociatedView, proposal, resultChan)

	// Wait for quorum
	var successTotal int
	var failedTotal int

	countError := func(err error) bool {
		log.Println("+1 error to accept:", err)
		failedTotal++

		allFailed := failedTotal == proposal.AssociatedView.N()
		mostFailedInspiteSomeSuccess := successTotal > 0 && failedTotal > proposal.AssociatedView.F()

		if mostFailedInspiteSomeSuccess || allFailed {
			return false
		}

		return true
	}

	countSuccess := func() bool {
		successTotal++

		if successTotal == proposal.AssociatedView.QuorumSize() {
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
//func CheckForChosenValue(ci consensusInstance) (interface{}, error) {
//proposalNumber := getNextProposalNumber(ci.associatedView)

//proposal := Proposal{N: proposalNumber, AssociatedView: ci.associatedView}
//value, err := prepare(proposal)
//if err != nil {
//return nil, errors.New("Could not read prepare consensus")
//}
//if value == nil {
//return nil, errors.New("No value has been chosen")
//}

//return value, nil
//}

// getNextProposalNumber to be used by this process. This function is a stage of the Propose funcion.
func getNextProposalNumber(associatedView *view.View, thisProcess view.Process) (proposalNumber int) {
	if associatedView.N() == 0 {
		log.Fatalln("associatedView is empty")
	}

	thisProcessPosition := associatedView.GetProcessPosition(thisProcess)

	lastProposalNumber, err := getLastProposalNumber(associatedView.NumberOfEntries())
	if err != nil {
		proposalNumber = associatedView.N() + thisProcessPosition
	} else {
		proposalNumber = (lastProposalNumber - (lastProposalNumber % associatedView.N()) + associatedView.N()) + thisProcessPosition
	}

	saveProposalNumberOnStorage(associatedView.NumberOfEntries(), proposalNumber)
	return
}

// -------- REQUESTS -----------
type ConsensusRequest int

type Proposal struct {
	AssociatedView *view.View

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
	ci := getOrCreateConsensus(arg.AssociatedView)

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
	ci := getOrCreateConsensus(arg.AssociatedView)

	var accept Accept
	accept.Proposal = &arg
	accept.returnChan = make(chan bool)
	accept.reply = reply

	ci.taskChan <- &accept

	<-accept.returnChan

	return nil
}

// Learn Request
func (r *ConsensusRequest) Learn(arg Proposal, reply *struct{}) error {
	log.Println("New Learn Request")
	ci := getOrCreateConsensus(arg.AssociatedView)

	var learn Learn
	learn.Proposal = &arg

	ci.taskChan <- &learn

	return nil
}

func init() {
	rpc.Register(new(ConsensusRequest))
}

// ------- ERRORS -----------
type OldProposalNumberError struct{}

func (e OldProposalNumberError) Error() string {
	return fmt.Sprint("Promised to a higher numbered prepare request")
}

func init() {
	gob.Register(new(OldProposalNumberError))
	gob.Register(new(Proposal))
}

// ------- Broadcast functions -----------
func broadcastPrepareRequest(destinationView *view.View, proposal Proposal, resultChan chan Proposal) {
	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var result Proposal
			err := comm.SendRPCRequest(process, "ConsensusRequest.Prepare", proposal, &result)
			if err != nil {
				resultChan <- Proposal{Err: err}
			}
			resultChan <- result
		}(process)
	}
}

func broadcastAcceptRequest(destinationView *view.View, proposal Proposal, resultChan chan Proposal) {
	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var result Proposal
			err := comm.SendRPCRequest(process, "ConsensusRequest.Accept", proposal, &result)
			if err != nil {
				resultChan <- Proposal{Err: err}
			}
			resultChan <- result
		}(process)
	}
}

func broadcastLearnRequest(destinationView *view.View, proposal Proposal) {
	comm.BroadcastQuorumRPCRequest(destinationView, "ConsensusRequest.Learn", proposal)
}
