package comm

import (
	"net/rpc"
	"time"

	"github.com/mateusbraga/freestore/pkg/view"
)

const initialCommLinkRepairPeriod = 1 * time.Second
const maxCommLinkRepairPeriod = 3 * time.Second

var (
	repairLinkChan = make(chan communicationLink, 20)
)

func init() {
	go repairCommLinkLoop()
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
	commLinkRepairPeriod := initialCommLinkRepairPeriod
	faultyCommLinks := make(map[view.Process]bool)
	repairTimer := time.NewTimer(commLinkRepairPeriod)

	for {
		select {
		case commLink := <-repairLinkChan:
			err := repairCommLinkFunc(commLink.Process)
			if err != nil {
				faultyCommLinks[commLink.Process] = true

				commLinkRepairPeriod = initialCommLinkRepairPeriod
				repairTimer.Reset(commLinkRepairPeriod)

			}
		case _ = <-repairTimer.C:
			for process, _ := range faultyCommLinks {
				err := repairCommLinkFunc(process)
				if err != nil {
					continue
				}

				delete(faultyCommLinks, process)
			}

			// exponential backoff
			commLinkRepairPeriod = 2 * commLinkRepairPeriod

			if commLinkRepairPeriod > maxCommLinkRepairPeriod {
				for process, _ := range faultyCommLinks {
					deleteCommLink(process)
				}
			}

			if len(faultyCommLinks) == 0 {
				repairTimer.Stop()
			} else {
				repairTimer.Reset(commLinkRepairPeriod)
			}
		}
	}
}
