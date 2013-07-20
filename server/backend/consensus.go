package backend

import (
	"errors"
	"log"
	"net/rpc"

	"mateusbraga/gotf/view"
)

var (
	OldProposalNumberErr = errors.New("Promised to a higher numbered prepare request")
)

var (
	highestNumberedAcceptedProposal Proposal
	highestNumberedPrepareRequest   uint

	learnChannel chan Proposal
)

type Proposal struct {
	Value interface{}
	N     uint
	Err   error
}

func propose() {
	// TODO get proposal number
	proposalNumber := uint(0)

	value, err := prepare(proposalNumber)
	if err != nil {
		log.Println("ERROR: Could not pass prepare phase")
	}

	if value == nil {
		// TODO Propor um valor "qualquer"
		value = 1
	}

	proposal := Proposal{N: proposalNumber, Value: value}
	if err := accept(proposal); err != nil {
		log.Println("ERROR: Could not pass prepare phase")
	}
}

func prepare(proposalNumber uint) (interface{}, error) {
	resultChan := make(chan Proposal, currentView.N())
	errChan := make(chan error, currentView.N())

	proposal := Proposal{N: proposalNumber}

	// Send read request to all
	for _, process := range currentView.GetMembers() {
		go prepareProcess(process, proposal, resultChan, errChan)
	}

	// Get quorum
	var failed int
	var resultArray []Proposal
	for {
		select {
		case receivedProposal := <-resultChan:
			if receivedProposal.Err != nil {
				errChan <- receivedProposal.Err
			}

			resultArray = append(resultArray, receivedProposal)

			if len(resultArray) >= currentView.QuorunSize() {
				return resultArray[0].Value, nil
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

func learn(proposal Proposal) {
	// Send acceptances to all
	for _, process := range currentView.GetMembers() {
		go learnProcess(process, proposal)
	}
}

func learner() {

	for {
		select {
		case proposal := <-learnChannel:
			//TODO
			_ = proposal
		}
	}
}

type ConsensusRequest int

func (r *ConsensusRequest) Prepare(arg Proposal, reply *Proposal) error {
	if arg.N > highestNumberedPrepareRequest {
		// TODO save this change before doing the next lines
		highestNumberedPrepareRequest = arg.N
		reply.Value = highestNumberedAcceptedProposal.Value
	} else {
		reply.Err = OldProposalNumberErr
	}

	return nil
}

func (r *ConsensusRequest) Accept(arg Proposal, reply *Proposal) error {
	if arg.N > highestNumberedPrepareRequest {
		// TODO save this change before doing the next lines
		highestNumberedAcceptedProposal = arg
		learn(arg)
	} else {
		reply.Err = OldProposalNumberErr
	}

	return nil
}

func (r *ConsensusRequest) Learn(arg Proposal, reply *Proposal) error {
	learnChannel <- arg
	return nil
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

func learnProcess(process view.Process, proposal Proposal) {
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

func init() {
	learnChannel = make(chan Proposal, 5)

	go learner()

	consensusRequest := new(ConsensusRequest)
	rpc.Register(consensusRequest)
}
