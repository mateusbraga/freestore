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
	Process   view.Process
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
		commLink = communicationLink{Process: process}

		newRpcClient, err := rpc.Dial("tcp", process.Addr)
		if err != nil {
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
}

func deleteCommLink(process view.Process) {
	commLinkTableMu.Lock()
	defer commLinkTableMu.Unlock()

	delete(commLinkTable, process)
}

func SendRPCRequest(process view.Process, serviceMethod string, arg interface{}, result interface{}) error {
	commLink := getCommLink(process)
	if commLink.isFaulty() {
		return errors.New(fmt.Sprintf("SendRPCRequest: Process %v is currently unreachable", process))
	}

	err := commLink.rpcClient.Call(serviceMethod, arg, result)
	if err != nil {
		setCommLinkFaulty(commLink.Process)
		repairLinkChan <- commLink
		return errors.New(fmt.Sprintf("SendRPCRequest: %v call to process %v failed: %v", serviceMethod, commLink.Process, err))
	}

	return nil
}

func TryBroadcastRPCRequest(destinationView *view.View, serviceMethod string, arg interface{}) {
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
				log.Printf("WARN: failed to send %v to a quorum\n", serviceMethod)
				return
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}

func MustBroadcastRPCRequest(destinationView *view.View, serviceMethod string, arg interface{}) {
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
				log.Fatalf("FATAL: failed to send %v to a quorum\n", serviceMethod)
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}
