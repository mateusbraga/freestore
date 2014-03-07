package comm

import (
	"net/rpc"
	"time"

	"github.com/mateusbraga/freestore/pkg/view"
)

const (
	// initialCommLinkRepairPeriod is how long it will take to first try to
	// repair a commLink
	initialCommLinkRepairPeriod = 1 * time.Second

	// maxCommLinkRepairPeriod is the max period on which higher periods will
	// not be retried. When repairPeriod gets higher then this because of the
	// exponential backoff, we will stop trying to repair the current faulty
	// links.  this is used to clean up old communication links that will not
	// be used anymore.
	maxCommLinkRepairPeriod = 3 * time.Second
)

var (
	// chan which new faulty links arrive
	repairLinkChan = make(chan communicationLink, 20)
)

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

func init() { go repairCommLinkLoop() }

func repairCommLinkLoop() {
	commLinkRepairPeriod := initialCommLinkRepairPeriod
	faultyCommLinks := make(map[view.Process]bool)
	repairTimer := time.NewTimer(commLinkRepairPeriod)

	for {
		select {
		case commLink := <-repairLinkChan:
			err := repairCommLinkFunc(commLink.process)
			if err != nil {
				faultyCommLinks[commLink.process] = true

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
