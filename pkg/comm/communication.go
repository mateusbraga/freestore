package comm

import (
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"

	"github.com/mateusbraga/freestore/pkg/view"
)

const commLinkRepairPeriod = 3 * time.Second

var (
	commLinkTable   = make(map[view.Process]communicationLink)
	commLinkTableMu sync.Mutex

	repairLinkChan = make(chan communicationLink)
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

func SendRPCRequest(process view.Process, serviceMethod string, arg interface{}, result interface{}) error {
	commLink := getCommLink(process)
	if commLink.isFaulty() {
		return errors.New(fmt.Sprintf("process %v is unreachable", process))
	}

	err := commLink.rpcClient.Call(serviceMethod, arg, result)
	if err != nil {
		setCommLinkFaulty(commLink.Process)
		repairLinkChan <- commLink
		return errors.New(fmt.Sprintf("sendRPCRequest to process %v failed: %v", commLink.Process, err))
	}

	return nil
}

func BroadcastRPCRequest(destinationView *view.View, serviceMethod string, arg interface{}) {
	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult struct{}
			_ = SendRPCRequest(process, serviceMethod, arg, &discardResult)
		}(process)
	}
}

func BroadcastQuorumRPCRequest(destinationView *view.View, serviceMethod string, arg interface{}) {
	errorChan := make(chan error, destinationView.N())

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
			if failedTotal > destinationView.F() {
				log.Fatalf("Failed to send %v to a quorum\n", serviceMethod)
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}

func repairCommLinkFunc(process view.Process) error {
	newRpcClient, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return err
	}

	commLinkTableMu.Lock()
	defer commLinkTableMu.Unlock()

	commLink := commLinkTable[process]
	commLink.rpcClient = newRpcClient
	commLinkTable[process] = commLink
	return nil
}

func repairCommLinkLoop() {
	faultyCommLinks := make(map[view.Process]time.Time)
	repairTicker := time.NewTicker(commLinkRepairPeriod)

	for {
		select {
		case commLink := <-repairLinkChan:
			err := repairCommLinkFunc(commLink.Process)
			if err != nil {
				log.Printf("Failed to repair communication link to process %v: %v\n", commLink.Process, err)
				faultyCommLinks[commLink.Process] = time.Now()
			}
		case tickTime := <-repairTicker.C:
			for process, loopTime := range faultyCommLinks {
				err := repairCommLinkFunc(process)
				if err != nil {
					log.Printf("Failed to repair communication link to process %v: %v\n", process, err)

					if tickTime.Sub(loopTime) < 10*time.Second {
						continue
					}
				}

				delete(faultyCommLinks, process)
			}
		}

	}
}

func init() {
	go repairCommLinkLoop()
}
