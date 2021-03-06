package server

import (
	"container/list"
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"os"
	"sync"
	"time"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/view"
)

func init() { rand.Seed(time.Now().UnixNano()) }

const (
	reconfigurationPeriod             = 1 * time.Minute
	firstReconfigurationTimerDuration = 10 * time.Second
)

func (s *Server) resetReconfigurationTimerLoop() {
	timer := time.AfterFunc(firstReconfigurationTimerDuration, s.startReconfiguration)
	for {
		<-s.resetReconfigurationTimerChan
		timer.Reset(reconfigurationPeriod)
	}
}

func (s *Server) startReconfiguration() {
	if !s.hasUpdatesToCurrentView() {
		// restart reconfiguration timer
		s.resetReconfigurationTimerChan <- true
		return
	}

	s.currentViewMu.RLock()
	defer s.currentViewMu.RUnlock()

	log.Println("Starting reconfiguration of currentView:", s.currentView)

	initialViewSeq := s.getInitialViewSeqLocked()

	if s.useConsensus {
		go s.generateViewSequenceWithConsensus(s.currentView, initialViewSeq)
	} else {
		go s.generateViewSequenceWithoutConsensus(s.currentView, initialViewSeq)
	}
}

func (s *Server) hasUpdatesToCurrentView() bool {
	s.recvMutex.RLock()
	defer s.recvMutex.RUnlock()

	return len(s.recv) != 0
}

func (s *Server) getInitialViewSeq() ViewSeq {
	s.currentViewMu.RLock()
	defer s.currentViewMu.RUnlock()
	return s.getInitialViewSeqLocked()
}

func (s *Server) getInitialViewSeqLocked() ViewSeq {
	s.recvMutex.RLock()
	defer s.recvMutex.RUnlock()

	var updates []view.Update
	for update, _ := range s.recv {
		updates = append(updates, update)
	}
	newView := s.currentView.NewCopyWithUpdates(updates...)
	return ViewSeq{newView}
}

type generatedViewSeq struct {
	AssociatedView *view.View
	ViewSeq        ViewSeq
}

func (s *Server) generatedViewSeqProcessingLoop() {
	for {
		newGeneratedViewSeq := <-s.generatedViewSeqChan
		log.Println("New generated view sequence:", newGeneratedViewSeq)

		leastUpdatedView := newGeneratedViewSeq.ViewSeq.GetLeastUpdatedView()

		installSeqMsg := InstallSeqMsg{
			Sender: s.thisProcess,
			InstallSeq: InstallSeq{
				AssociatedView: newGeneratedViewSeq.AssociatedView,
				InstallView:    leastUpdatedView,
				ViewSeq:        newGeneratedViewSeq.ViewSeq,
			},
		}

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

func (s *Server) installSeqProcessingLoop() {
	processToInstallSeqMsgMap := make(map[view.Process]*InstallSeqMsg)
	// ENHANCEMENT: clean up quorum counter old views
	var installSeqQuorumCounter installSeqQuorumCounterType

	for {
		installSeqMsg := <-s.installSeqProcessingChan

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
			s.gotInstallSeqQuorum(installSeqMsg.InstallSeq)
		}
	}
}

func (s *Server) gotInstallSeqQuorum(installSeq InstallSeq) {
	s.currentViewMu.Lock()
	defer s.currentViewMu.Unlock()

	log.Println("Running gotInstallSeqQuorum", installSeq)

	installViewIsMoreUpdatedThanCv := installSeq.InstallView.MoreUpdatedThan(s.currentView)

	// don't matter if installView is old, send state if server was a member of the associated view
	if installSeq.AssociatedView.HasMember(s.thisProcess) {
		// if installView is not old, disable r/w
		if installViewIsMoreUpdatedThanCv {
			// disable R/W operations if not already disabled
			s.registerLockOnce.Do(func() {
				s.register.mu.Lock()
				s.registerLockTime = time.Now()
				log.Println("R/W operations disabled for reconfiguration")
			})
		}

		syncStateMsg := SyncStateMsg{}
		syncStateMsg.Value = s.register.Value
		syncStateMsg.Timestamp = s.register.Timestamp
		s.recvMutex.RLock()
		syncStateMsg.Recv = make(map[view.Update]bool, len(s.recv))
		for update, _ := range s.recv {
			syncStateMsg.Recv[update] = true
		}
		s.recvMutex.RUnlock()
		syncStateMsg.AssociatedView = installSeq.AssociatedView

		// Send state to all
		go broadcastStateUpdate(installSeq.InstallView, syncStateMsg)

		log.Println("State sent!")
	}

	// stop here if installView is old
	if !installViewIsMoreUpdatedThanCv {
		if installSeq.ViewSeq.HasViewMoreUpdatedThan(s.currentView) {
			s.installOthersViewsFromViewSeqLocked(installSeq)
			return
		} else {
			log.Println("installSeq does not lead to a more updated view than current view. Skipping...")
			return
		}
	}

	if installSeq.InstallView.HasMember(s.thisProcess) {
		// Process is on the new view
		s.syncState(installSeq)

		s.updateCurrentViewLocked(installSeq.InstallView)

		viewInstalledMsg := ViewInstalledMsg{}
		viewInstalledMsg.InstalledView = s.currentView

		// Send view-installed to all
		processes := installSeq.AssociatedView.GetMembersNotIn(installSeq.InstallView)
		viewOfLeavingProcesses := view.NewWithProcesses(processes...)
		go broadcastViewInstalled(viewOfLeavingProcesses, viewInstalledMsg)

		if installSeq.ViewSeq.HasViewMoreUpdatedThan(s.currentView) {
			s.installOthersViewsFromViewSeqLocked(installSeq)
		} else {
			s.register.mu.Unlock()
			log.Println("R/W operations enabled")

			endTime := time.Now()
			if installSeq.AssociatedView.HasMember(s.thisProcess) {
				log.Printf("Reconfiguration completed in %v, the system was unavailable for %v.\n", endTime.Sub(s.startReconfigurationTime), endTime.Sub(s.registerLockTime))
				s.registerLockOnce = sync.Once{}
			} else {
				log.Println("Reconfiguration completed, this process is now part of the system.")
			}

			s.resetReconfigurationTimerChan <- true
		}
	} else {
		// thisProcess is NOT on the new view
		var counter int

		log.Println("Waiting for view-installed quorum to leave")
		for {
			viewInstalled := <-s.newViewInstalledChan

			if installSeq.InstallView.Equal(viewInstalled.InstalledView) {
				counter++
				if counter == installSeq.InstallView.QuorumSize() {
					break
				}
			}
		}

		log.Println("Leaving...")
		//shutdownChan <- true
		os.Exit(0)
	}
}

func (s *Server) updateCurrentViewLocked(newView *view.View) {
	if !newView.MoreUpdatedThan(s.currentView) {
		// comment these log messages; they are just for debugging
		if newView.LessUpdatedThan(s.currentView) {
			log.Println("WARNING: Tried to Update current view with a less updated view")
		} else {
			log.Println("WARNING: Tried to Update current view with the same view")
		}
		return
	}

	s.currentView = newView
	log.Printf("CurrentView updated to: %v, ref: %v\n", s.currentView, s.currentView.ViewRef)
}

func (s *Server) installOthersViewsFromViewSeqLocked(installSeq InstallSeq) {
	var newSeq ViewSeq
	for _, v := range installSeq.ViewSeq {
		if v.MoreUpdatedThan(s.currentView) {
			newSeq = append(newSeq, v)
		}
	}

	log.Println("Generate next view sequence with:", newSeq)
	if s.useConsensus {
		go s.generateViewSequenceWithConsensus(s.currentView, newSeq)
	} else {
		go s.generateViewSequenceWithoutConsensus(s.currentView, newSeq)
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

func (s *Server) stateUpdateProcessingLoop() {
	// ENHANCEMENT: clean up quorum counter old views
	stateUpdateQuorumCounterList := list.New()

	for {
		select {
		case stateUpdate := <-s.syncStateMsgChan:
			//log.Println("processing stateUpdate:", stateUpdate)

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
		case chanRequest := <-s.stateUpdateChanRequestChan:
			stateUpdateQuorum, ok := getStateUpdateQuorumCounter(stateUpdateQuorumCounterList, chanRequest.associatedView)
			if !ok {
				stateUpdateQuorum = &stateUpdateQuorumType{associatedView: chanRequest.associatedView, State: State{&Value{Timestamp: -1}, make(map[view.Update]bool)}, counter: 0, resultChan: make(chan State, 1)}
				stateUpdateQuorumCounterList.PushBack(stateUpdateQuorum)
			}

			chanRequest.returnChan <- stateUpdateQuorum.resultChan
		}
	}
}

func (s *Server) syncState(installSeq InstallSeq) {
	log.Println("Running syncState")

	chanRequest := stateUpdateChanRequest{associatedView: installSeq.AssociatedView, returnChan: make(chan chan State)}

	// Request the chan in which the state will be sent
	s.stateUpdateChanRequestChan <- chanRequest
	// Receive the chan in which the state will be sent
	stateChan := <-chanRequest.returnChan

	// get state
	state := <-stateChan
	defer func() { stateChan <- state }()

	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()

	for update, _ := range state.recv {
		s.recv[update] = true
	}

	s.register.Value = state.finalValue.Value
	s.register.Timestamp = state.finalValue.Timestamp

	for _, update := range installSeq.InstallView.GetUpdates() {
		delete(s.recv, update)
	}

	log.Println("State synced")
}

// ------------- Join and Leave ---------------------

func (s *Server) joinLocked() {
	log.Println("Asked to Join current view:", s.currentView)
	reconfig := ReconfigMsg{AssociatedView: s.currentView, Update: view.Update{view.Join, s.thisProcess}}

	// Send reconfig request to currentView
	go broadcastReconfigRequest(s.currentView, reconfig)
}

func (s *Server) leave() {
	s.currentViewMu.RLock()
	defer s.currentViewMu.RUnlock()

	log.Println("Asked to Leave current view:", s.currentView)
	reconfig := ReconfigMsg{AssociatedView: s.currentView, Update: view.Update{view.Leave, s.thisProcess}}

	// Send reconfig request to all
	go broadcastReconfigRequest(s.currentView, reconfig)
}

// -------- REQUESTS -----------

type ReconfigMsg struct {
	Update         view.Update
	AssociatedView *view.View
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

type SyncStateMsg struct {
	Value          interface{}
	Timestamp      int
	Recv           map[view.Update]bool
	AssociatedView *view.View
}

type ViewInstalledMsg struct {
	InstalledView *view.View
}

type ReconfigurationRequest int

func init() { rpc.Register(new(ReconfigurationRequest)) }

func (r *ReconfigurationRequest) Reconfig(arg ReconfigMsg, reply *struct{}) error {
	globalServer.currentViewMu.RLock()
	defer globalServer.currentViewMu.RUnlock()

	if !arg.AssociatedView.Equal(globalServer.currentView) {
		return fmt.Errorf("Reconfig request with old view")
	}

	if globalServer.currentView.HasUpdate(arg.Update) {
		log.Printf("Reconfig request's Update %v already in currentView\n", arg.Update)
		return nil
	}

	globalServer.recvMutex.Lock()
	defer globalServer.recvMutex.Unlock()
	globalServer.recv[arg.Update] = true

	log.Printf("%v added to next reconfiguration\n", arg.Update)

	return nil
}

func (r *ReconfigurationRequest) InstallSeq(arg InstallSeqMsg, reply *struct{}) error {
	globalServer.installSeqProcessingChan <- arg
	return nil
}

func (r *ReconfigurationRequest) StateUpdate(arg SyncStateMsg, reply *struct{}) error {
	globalServer.syncStateMsgChan <- arg
	return nil
}

func (r *ReconfigurationRequest) ViewInstalled(arg ViewInstalledMsg, reply *struct{}) error {
	globalServer.newViewInstalledChan <- arg
	return nil
}

// -------- Send functions -----------

func broadcastViewInstalled(destinationView *view.View, viewInstalledMsg ViewInstalledMsg) {
	comm.BroadcastRPCRequest(destinationView, "ReconfigurationRequest.ViewInstalled", viewInstalledMsg)
}

func broadcastStateUpdate(destinationView *view.View, syncStateMsg SyncStateMsg) {
	comm.BroadcastRPCRequest(destinationView, "ReconfigurationRequest.StateUpdate", syncStateMsg)
}

func broadcastInstallSeq(destinationView *view.View, installSeqMsg InstallSeqMsg) {
	comm.BroadcastRPCRequest(destinationView, "ReconfigurationRequest.InstallSeq", installSeqMsg)
}

func broadcastReconfigRequest(destinationView *view.View, reconfigMsg ReconfigMsg) {
	comm.BroadcastRPCRequest(destinationView, "ReconfigurationRequest.Reconfig", reconfigMsg)
}
