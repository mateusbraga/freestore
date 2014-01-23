package comm

import (
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	rpcClientWorkers   = make(map[view.Process]rpcClientInstance)
	rpcClientWorkersMu sync.Mutex
)

type rpcClientJob struct {
	serviceMethod string
	args          interface{}
	reply         interface{}
	errorCh       chan error
}

type rpcClientInstance struct {
	process view.Process
	jobChan chan rpcClientJob
}

// rpcClientWorker is responsable for maintaining a rpc.Client.
// Currently, rpcClientWorker never ends so we can keep it simpler for now.
func rpcClientWorker(rci rpcClientInstance) {
	process := rci.process
	jobChan := rci.jobChan

	var client *rpc.Client
	var err error

	for {
		job := <-jobChan

		// make sure client is not null
		if client == nil {
			client, err = rpc.Dial("tcp", process.Addr)
			if err != nil {
				job.errorCh <- err
				continue
			}
		}

		err = client.Call(job.serviceMethod, job.args, job.reply)
		if err != nil {
			if err == rpc.ErrShutdown {
				client, err = rpc.Dial("tcp", process.Addr)
				if err != nil {
					job.errorCh <- err
					continue
				}

				// retry this job
				go func() { jobChan <- job }()

			} else {
				job.errorCh <- err
			}
		}
		job.errorCh <- nil
	}
}

func getRpcClientInstance(process view.Process) rpcClientInstance {
	rpcClientWorkersMu.Lock()
	defer rpcClientWorkersMu.Unlock()

	rci, ok := rpcClientWorkers[process]
	if !ok {
		// worker does not exist, create one
		rci = rpcClientInstance{process: process, jobChan: make(chan rpcClientJob, 20)}
		rpcClientWorkers[process] = rci
		go rpcClientWorker(rci)
	}
	return rci
}

func SendRPCRequest(process view.Process, serviceMethod string, args interface{}, reply interface{}) error {
	rci := getRpcClientInstance(process)

	errorCh := make(chan error)
	rcj := rpcClientJob{serviceMethod: serviceMethod, args: args, reply: reply, errorCh: errorCh}
	rci.jobChan <- rcj

	err := <-errorCh
	return err
}
