package comm

import (
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	commLinkTable   = make(map[view.Process]communicationLink)
	commLinkTableMu sync.Mutex
)

type communicationLink struct {
	process   view.Process
	rpcClient *rpc.Client
}

func (commLink communicationLink) isFaulty() bool {
	return commLink.rpcClient == nil
}

func getCommLink(process view.Process) communicationLink {
	commLinkTableMu.Lock()
	defer commLinkTableMu.Unlock()

	commLink, ok := commLinkTable[process]
	if !ok {
		commLink = communicationLink{process: process}

		newRpcClient, err := rpc.Dial("tcp", process.Addr)
		if err != nil {
			newRpcClient = nil
			repairLinkChan <- commLink
		}
		commLink.rpcClient = newRpcClient

		commLinkTable[process] = commLink
	}

	return commLink
}

func setCommLinkFaulty(process view.Process) {
	commLinkTableMu.Lock()
	defer commLinkTableMu.Unlock()

	commLink := commLinkTable[process]
	commLink.rpcClient = nil
	commLinkTable[process] = commLink

	repairLinkChan <- commLink
}

func deleteCommLink(process view.Process) {
	commLinkTableMu.Lock()
	defer commLinkTableMu.Unlock()

	delete(commLinkTable, process)
}

// SendRPCRequest invokes serviceMethod at process with arg and puts the result at result. Any communication error that occurs is returned.
func SendRPCRequest(process view.Process, serviceMethod string, arg interface{}, result interface{}) error {
	commLink := getCommLink(process)
	if commLink.isFaulty() {
		return errors.New(fmt.Sprintf("SendRPCRequest: Process %v is currently unreachable", process))
	}

	err := commLink.rpcClient.Call(serviceMethod, arg, result)
	if err != nil {
		setCommLinkFaulty(commLink.process)
		return errors.New(fmt.Sprintf("SendRPCRequest: %v call to process %v failed: %v", serviceMethod, commLink.process, err))
	}

	return nil
}

// BroadcastRPCRequest invokes serviceMethod at all members of the
// destinationView with arg. It returns an error if it fails to receive
// a response from a quorum of processes.
func BroadcastRPCRequest(destinationView *view.View, serviceMethod string, arg interface{}) error {
	errorChan := make(chan error, destinationView.NumberOfMembers())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult struct{}
			errorChan <- SendRPCRequest(process, serviceMethod, arg, &discardResult)
		}(process)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan

		if err != nil {
			failedTotal++
			if failedTotal > destinationView.NumberOfToleratedFaults() {
				log.Printf("WARN: BroadcastRPCRequest failed to send %v to a quorum\n", serviceMethod)
				return err
			}
		} else {
			successTotal++
			if successTotal == destinationView.QuorumSize() {
				return nil
			}
		}
	}
}
