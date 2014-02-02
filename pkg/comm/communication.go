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

const commLinkRepairPeriod = 20 * time.Second

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

func SendRPCRequest(process view.Process, serviceMethod string, arg interface{}, result interface{}) error {
	commLink := getCommLink(process)
	if commLink.isFaulty() {
		return errors.New(fmt.Sprintf("process %v is unreachable", process))
	}

	err := commLink.rpcClient.Call(serviceMethod, arg, result)
	if err != nil {
		commLink.rpcClient = nil
		repairLinkChan <- commLink
		return errors.New(fmt.Sprintf("sendRPCRequest to process %v failed: %v", commLink.Process, err))
	}

	return nil
}

func SendRPCRequestWithErrorChan(process view.Process, serviceMethod string, arg interface{}, result interface{}, errorChan chan error) {
	errorChan <- SendRPCRequest(process, serviceMethod, arg, result)
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
	faultyCommLinks := make(map[view.Process]bool)
	repairTicker := time.NewTicker(commLinkRepairPeriod)

	for {
		select {
		case commLink := <-repairLinkChan:
			err := repairCommLinkFunc(commLink.Process)
			if err != nil {
				log.Printf("Failed to repair communication link to process %v: %v\n", commLink.Process, err)
				faultyCommLinks[commLink.Process] = true
			}
		case _ = <-repairTicker.C:
			var repairedSucessfully []view.Process

			for process, _ := range faultyCommLinks {
				err := repairCommLinkFunc(process)
				if err != nil {
					log.Printf("Failed to repair communication link to process %v: %v\n", process, err)
				}

				repairedSucessfully = append(repairedSucessfully, process)
			}

			for _, process := range repairedSucessfully {
				delete(faultyCommLinks, process)
			}
		}

	}
}

func init() {
	go repairCommLinkLoop()
}
