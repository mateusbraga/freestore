package server

import (
	"container/list"
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

const (
	CHANNEL_DEFAULT_SIZE              int = 20
	reconfigurationPeriod                 = 1 * time.Minute
	firstReconfigurationTimerDuration     = 10 * time.Second
)

var (
	recv      = make(map[view.Update]bool)
	recvMutex sync.RWMutex

	generatedViewSeqChan       = make(chan generatedViewSeq)
	installSeqProcessingChan   = make(chan InstallSeqMsg, CHANNEL_DEFAULT_SIZE)
	stateUpdateProcessingChan  = make(chan StateUpdateMsg, CHANNEL_DEFAULT_SIZE)
	stateUpdateChanRequestChan = make(chan stateUpdateChanRequest, CHANNEL_DEFAULT_SIZE)
	newViewInstalledChan       = make(chan ViewInstalledMsg, CHANNEL_DEFAULT_SIZE)

	resetReconfigurationTimer = make(chan bool, 5)
)

// ---------- Bootstrapping ------------
func init() {
	go generatedViewSeqProcessingLoop()
	go installSeqProcessingLoop()
	go stateUpdateProcessingLoop()
	go resetTimerLoop()
}

func resetTimerLoop() {
	timer := time.AfterFunc(firstReconfigurationTimerDuration, startReconfiguration)
	for {
		<-resetReconfigurationTimer
		timer.Reset(reconfigurationPeriod)
	}
}

func startReconfiguration() {
	recvMutex.Lock()
	defer recvMutex.Unlock()

	if len(recv) == 0 {
		// No configuration changes, restart reconfiguration timer
		resetReconfigurationTimer <- true
		return
	}

	log.Println("Start reconfiguration of currentView:", currentView)

	updates := []view.Update{}
	for update, _ := range recv {
		updates = append(updates, update)
	}
	newView := currentView.NewCopyWithUpdates(updates...)
	initialViewSeq := ViewSeq{newView}
	currentViewCopy := currentView.NewCopy()

	if useConsensus {
		go generateViewSequenceWithConsensus(currentViewCopy, initialViewSeq)
	} else {
		go generateViewSequenceWithoutConsensus(currentViewCopy, initialViewSeq)
	}
}

// ---------- Others ------------

type generatedViewSeq struct {
	ViewSeq        ViewSeq
	AssociatedView *view.View
}

func generatedViewSeqProcessingLoop() {
	for {
		newGeneratedViewSeq := <-generatedViewSeqChan
		log.Println("New generated view sequence:", newGeneratedViewSeq)

		leastUpdatedView := newGeneratedViewSeq.ViewSeq.GetLeastUpdatedView()

		installSeqMsg := InstallSeqMsg{}
		installSeqMsg.AssociatedView = newGeneratedViewSeq.AssociatedView
		installSeqMsg.InstallView = leastUpdatedView
		installSeqMsg.ViewSeq = newGeneratedViewSeq.ViewSeq
		installSeqMsg.Sender = thisProcess

		// Send install-seq to all from old and new view
		processes := append(newGeneratedViewSeq.AssociatedView.GetMembers(), leastUpdatedView.GetMembers()...)
		mergedView := view.NewWithProcesses(processes...)

		go broadcastInstallSeq(mergedView, installSeqMsg)
	}
}

type installSeqQuorumCounterType struct {
	list    []*InstallSeq
	counter []int
}

func (quorumCounter *installSeqQuorumCounterType) count(newInstallSeq *InstallSeq, quorumSize int) bool {
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newInstallSeq) {
			quorumCounter.counter[i]++

			return quorumCounter.counter[i] == quorumSize
		}
	}

	quorumCounter.list = append(quorumCounter.list, newInstallSeq)
	quorumCounter.counter = append(quorumCounter.counter, 1)

	return (1 == quorumSize)
}

func installSeqProcessingLoop() {
	processToInstallSeqMsgMap := make(map[view.Process]*InstallSeqMsg)
	// ENHANCEMENT: clean up quorum counter old views
	var installSeqQuorumCounter installSeqQuorumCounterType

	for {
		installSeqMsg := <-installSeqProcessingChan

		// Check for duplicate
		previousInstallSeq, ok := processToInstallSeqMsgMap[installSeqMsg.Sender]
		processToInstallSeqMsgMap[installSeqMsg.Sender] = &installSeqMsg
		if ok && previousInstallSeq.Equal(installSeqMsg) {
			// It's a duplicate
			continue
		}

		// Re-send install-seq to all
		processes := append(installSeqMsg.AssociatedView.GetMembers(), installSeqMsg.InstallView.GetMembers()...)
		mergedView := view.NewWithProcesses(processes...)
		go broadcastInstallSeq(mergedView, installSeqMsg)

		// Quorum check
		if installSeqQuorumCounter.count(&installSeqMsg.InstallSeq, installSeqMsg.AssociatedView.QuorumSize()) {
			gotInstallSeqQuorum(installSeqMsg.InstallSeq)
		}
	}
}

func gotInstallSeqQuorum(installSeq InstallSeq) {
	log.Println("Running gotInstallSeqQuorum")

	startTime := time.Now()

	cvIsLessUpdatedThanInstallView := currentView.LessUpdatedThan(installSeq.InstallView)

	// don't matter if installView is old, send state if server was a member of the associated view
	if installSeq.AssociatedView.HasMember(thisProcess) {
		// if installView is not old, disable r/w
		if cvIsLessUpdatedThanInstallView {
			register.mu.Lock()
			log.Println("R/W operations disabled")
		}

		stateMsg := StateUpdateMsg{}
		stateMsg.Value = register.Value
		stateMsg.Timestamp = register.Timestamp
		recvMutex.RLock()
		stateMsg.Recv = make(map[view.Update]bool)
		for update, _ := range recv {
			stateMsg.Recv[update] = true
		}
		recvMutex.RUnlock()
		stateMsg.AssociatedView = installSeq.AssociatedView

		// Send state-update request to all
		go broadcastStateUpdate(installSeq.InstallView.NewCopy(), stateMsg)

		log.Println("State sent!")
	}

	// stop here if installView is old
	if !cvIsLessUpdatedThanInstallView {
		return
	}

	if installSeq.InstallView.HasMember(thisProcess) {
		// Process is on the new view
		syncState(installSeq)

		currentView = installSeq.InstallView.NewCopy()
		log.Println("New view installed:", currentView)

		viewInstalledMsg := ViewInstalledMsg{}
		viewInstalledMsg.CurrentView = currentView

		// Send view-installed to all
		processes := installSeq.AssociatedView.GetMembersNotIn(installSeq.InstallView)
		viewOfLeavingProcesses := view.NewWithProcesses(processes...)
		go broadcastViewInstalled(viewOfLeavingProcesses, viewInstalledMsg)

		var newSeq ViewSeq
		cvIsMostUpdated := true
		for _, v := range installSeq.ViewSeq {
			if currentView.LessUpdatedThan(v) {
				newSeq = append(newSeq, v)

				cvIsMostUpdated = false
			}
		}

		if cvIsMostUpdated {
			register.mu.Unlock()
			log.Println("R/W operations enabled")

			endTime := time.Now()
			log.Printf("Reconfiguration completed, the system was unavailable for %v.\n", endTime.Sub(startTime))

			resetReconfigurationTimer <- true
		} else {
			currentViewCopy := currentView.NewCopy()

			log.Println("Generate next view sequence with:", newSeq)
			if useConsensus {
				go generateViewSequenceWithConsensus(currentViewCopy, newSeq)
			} else {
				go generateViewSequenceWithoutConsensus(currentViewCopy, newSeq)
			}
		}
	} else {
		// thisProcess is NOT on the new view
		var counter int

		log.Println("Waiting for view-installed quorum")
		for {
			viewInstalled := <-newViewInstalledChan

			if installSeq.InstallView.Equal(viewInstalled.CurrentView) {
				counter++
				if counter == installSeq.InstallView.QuorumSize() {
					break
				}
			}
		}

		log.Println("Terminating...")
		err := listener.Close()
		if err != nil {
			log.Panic(err)
		}
	}
}

// --------------------- State Update -----------------------

type State struct {
	finalValue *Value
	recv       map[view.Update]bool
}

func (thisState State) NewCopy() State {
	stateCopy := State{finalValue: &Value{}}
	stateCopy.finalValue.Value = thisState.finalValue.Value
	stateCopy.finalValue.Timestamp = thisState.finalValue.Timestamp
	stateCopy.recv = make(map[view.Update]bool, len(thisState.recv))

	for update, _ := range thisState.recv {
		stateCopy.recv[update] = true
	}
	return stateCopy
}

type stateUpdateQuorumType struct {
	associatedView *view.View

	State
	counter int

	resultChan chan State
}

type stateUpdateChanRequest struct {
	associatedView *view.View
	returnChan     chan chan State
}

func getStateUpdateQuorumCounter(stateUpdateQuorumCounterList *list.List, associatedView *view.View) (*stateUpdateQuorumType, bool) {
	for quorumCounter := stateUpdateQuorumCounterList.Front(); quorumCounter != nil; quorumCounter = quorumCounter.Next() {
		stateUpdateQuorum := quorumCounter.Value.(*stateUpdateQuorumType)
		if !stateUpdateQuorum.associatedView.Equal(associatedView) {
			continue
		}

		return stateUpdateQuorum, true
	}

	return nil, false
}

func stateUpdateProcessingLoop() {
	// ENHANCEMENT: clean up quorum counter old views
	stateUpdateQuorumCounterList := list.New()

	for {
		select {
		case stateUpdate := <-stateUpdateProcessingChan:
			if stateUpdate.AssociatedView.LessUpdatedThan(currentView) {
				log.Println("Old stateUpdate ignored")
				continue
			}

			log.Println("processing stateUpdate:", stateUpdate)

			stateUpdateQuorum, ok := getStateUpdateQuorumCounter(stateUpdateQuorumCounterList, stateUpdate.AssociatedView)
			if !ok {
				stateUpdateQuorum = &stateUpdateQuorumType{associatedView: stateUpdate.AssociatedView, State: State{&Value{Timestamp: -1}, make(map[view.Update]bool)}, counter: 0, resultChan: make(chan State, 1)}
				stateUpdateQuorumCounterList.PushBack(stateUpdateQuorum)
			}

			stateUpdateQuorum.counter++

			// merge recv
			for update, _ := range stateUpdate.Recv {
				stateUpdateQuorum.recv[update] = true
			}

			// update register value if necessary
			if stateUpdateQuorum.finalValue.Timestamp < stateUpdate.Timestamp {
				stateUpdateQuorum.finalValue.Value = stateUpdate.Value
				stateUpdateQuorum.finalValue.Timestamp = stateUpdate.Timestamp
			}

			if stateUpdateQuorum.counter == stateUpdate.AssociatedView.QuorumSize() {
				stateUpdateQuorum.resultChan <- stateUpdateQuorum.State.NewCopy()
			}
		case chanRequest := <-stateUpdateChanRequestChan:
			stateUpdateQuorum, ok := getStateUpdateQuorumCounter(stateUpdateQuorumCounterList, chanRequest.associatedView)
			if !ok {
				stateUpdateQuorum = &stateUpdateQuorumType{associatedView: chanRequest.associatedView, State: State{&Value{Timestamp: -1}, make(map[view.Update]bool)}, counter: 0, resultChan: make(chan State, 1)}
				stateUpdateQuorumCounterList.PushBack(stateUpdateQuorum)
			}

			chanRequest.returnChan <- stateUpdateQuorum.resultChan
		}
	}
}

func syncState(installSeq InstallSeq) {
	log.Println("Running syncState")

	chanRequest := stateUpdateChanRequest{associatedView: installSeq.AssociatedView, returnChan: make(chan chan State)}

	// Request the chan in which the state will be sent
	stateUpdateChanRequestChan <- chanRequest
	// Receive the chan in which the state will be sent
	stateChan := <-chanRequest.returnChan

	// get state
	state := <-stateChan

	recvMutex.Lock()
	defer recvMutex.Unlock()

	for update, _ := range state.recv {
		recv[update] = true
	}

	register.Value = state.finalValue.Value
	register.Timestamp = state.finalValue.Timestamp

	for _, update := range installSeq.InstallView.GetEntries() {
		delete(recv, update)
	}

	log.Println("State synced")
}

// ------------- Join and Leave ---------------------

func Join() {
	reconfig := ReconfigMsg{CurrentView: currentView.NewCopy(), Update: view.Update{view.Join, thisProcess}}

	// Send reconfig request to currentView
	go broadcastReconfigRequest(currentView.NewCopy(), reconfig)

}

func Leave() {
	reconfig := ReconfigMsg{CurrentView: currentView.NewCopy(), Update: view.Update{view.Leave, thisProcess}}

	// Send reconfig request to all
	go broadcastReconfigRequest(currentView.NewCopy(), reconfig)
}

// -------- REQUESTS -----------
type ReconfigurationRequest int

type ReconfigMsg struct {
	Update      view.Update
	CurrentView *view.View
}

type InstallSeq struct {
	InstallView    *view.View
	ViewSeq        ViewSeq
	AssociatedView *view.View
}

func (installSeq InstallSeq) Equal(installSeq2 InstallSeq) bool {
	if len(installSeq.ViewSeq) != len(installSeq2.ViewSeq) {
		return false
	}
	if installSeq.InstallView.Equal(installSeq2.InstallView) {
		if installSeq.AssociatedView.Equal(installSeq2.AssociatedView) {

			for i, v := range installSeq.ViewSeq {
				if !v.Equal(installSeq2.ViewSeq[i]) {
					return false
				}
			}

			return true
		}
	}

	return false
}

type InstallSeqMsg struct {
	Sender view.Process
	InstallSeq
}

func (installSeq InstallSeqMsg) String() string {
	return fmt.Sprintf("Sender: %v\nInstallView: %v\nAssociatedView: %v\nViewSeq: %v", installSeq.Sender, installSeq.InstallView, installSeq.AssociatedView, installSeq.ViewSeq)
}

func (installSeqMsg InstallSeqMsg) Equal(installSeqMsg2 InstallSeqMsg) bool {
	if installSeqMsg.Sender == installSeqMsg2.Sender {
		return installSeqMsg.InstallSeq.Equal(installSeqMsg2.InstallSeq)
	}

	return false
}

type StateUpdateMsg struct {
	Value          interface{}
	Timestamp      int
	Recv           map[view.Update]bool
	AssociatedView *view.View
}

type ViewInstalledMsg struct {
	CurrentView *view.View
}

func (r *ReconfigurationRequest) Reconfig(arg ReconfigMsg, reply *error) error {
	if !arg.CurrentView.Equal(currentView) {
		log.Printf("Reconfig request with old view: %v\n", arg.CurrentView)
		*reply = view.OldViewError{NewView: currentView.NewCopy()}
		return nil
	}

	if currentView.HasUpdate(arg.Update) {
		log.Printf("Reconfig request's Update %v already in currentView\n", arg.Update)
		return nil
	}

	recvMutex.Lock()
	defer recvMutex.Unlock()
	recv[arg.Update] = true

	log.Printf("%v added to next reconfiguration\n", arg.Update)

	return nil
}

func (r *ReconfigurationRequest) InstallSeq(arg InstallSeqMsg, reply *error) error {
	installSeqProcessingChan <- arg
	return nil
}

func (r *ReconfigurationRequest) StateUpdate(arg StateUpdateMsg, reply *error) error {
	stateUpdateProcessingChan <- arg
	return nil
}

func (r *ReconfigurationRequest) ViewInstalled(arg ViewInstalledMsg, reply *error) error {
	newViewInstalledChan <- arg
	return nil
}

func init() {
	rpc.Register(new(ReconfigurationRequest))
}

// -------- Send functions -----------

func broadcastViewInstalled(destinationView *view.View, viewInstalled ViewInstalledMsg) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult error
			errorChan <- comm.SendRPCRequest(process, "ReconfigurationRequest.ViewInstalled", viewInstalled, &discardResult)
		}(process)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan
		if err != nil {
			failedTotal++
			if failedTotal > destinationView.F() {
				log.Fatalln("Failed to send ViewInstalled to a quorum")
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}

func broadcastStateUpdate(destinationView *view.View, stateUpdateMsg StateUpdateMsg) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult error
			errorChan <- comm.SendRPCRequest(process, "ReconfigurationRequest.StateUpdate", stateUpdateMsg, &discardResult)
		}(process)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan
		if err != nil {
			failedTotal++
			if failedTotal > destinationView.F() {
				log.Fatalln("Failed to send StateUpdate to a quorum")
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}

func broadcastInstallSeq(destinationView *view.View, installSeq InstallSeqMsg) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult error
			errorChan <- comm.SendRPCRequest(process, "ReconfigurationRequest.InstallSeq", installSeq, &discardResult)
		}(process)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan
		if err != nil {
			failedTotal++
			if failedTotal > destinationView.F() {
				log.Fatalln("Failed to send InstallSeq to a quorum")
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}

func broadcastReconfigRequest(destinationView *view.View, reconfig ReconfigMsg) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult error
			errorChan <- comm.SendRPCRequest(process, "ReconfigurationRequest.Reconfig", reconfig, &discardResult)
		}(process)
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
