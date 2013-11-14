package backend

import (
	"container/list"
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"

	"mateusbraga/gotf/freestore/view"
)

var (
	recv      map[view.Update]bool
	recvMutex sync.RWMutex

	viewSeqProcessingChan              chan newViewSeq
	installSeqProcessingChan           chan InstallSeqMsg
	stateUpdateProcessingChan          chan StateUpdateMsg
	callbackChanStateUpdateRequestChan chan getCallbackStateUpdateRequest
	newViewInstalledChan               chan ViewInstalledMsg

	resetTimer chan bool
)

// ---------- Bootstrapping ------------
func init() {
	recv = make(map[view.Update]bool)

	viewSeqProcessingChan = make(chan newViewSeq)
	installSeqProcessingChan = make(chan InstallSeqMsg, 20)   //TODO
	stateUpdateProcessingChan = make(chan StateUpdateMsg, 20) //TODO

	newViewInstalledChan = make(chan ViewInstalledMsg, 20)                            //TODO
	callbackChanStateUpdateRequestChan = make(chan getCallbackStateUpdateRequest, 20) //TODO

	go viewSeqProcessingLoop()
	go installSeqProcessingLoop()
	go stateUpdateProcessingLoop()

	resetTimer = make(chan bool, 5)
	go resetTimerLoop()
}

func resetTimerLoop() {
	timer := time.AfterFunc(10*time.Second, reconfigurationTask)
	for {
		<-resetTimer
		timer.Reset(1 * time.Minute)
	}
}

// ---------- Others ------------
type newViewSeq struct {
	ViewSeq        []view.View
	AssociatedView view.View
}

func findLeastUpdatedView(seq []view.View) view.View {
	for i, v := range seq {
		isLeastUpdated := true
		for _, v2 := range seq[i+1:] {
			if !v.LessUpdatedThan(&v2) {
				isLeastUpdated = false
				break
			}
		}
		if isLeastUpdated {
			return v
		}
	}

	log.Panicln("BUG! Should not execute this. Failed to find least updated view on: ", seq)
	return view.New()
}

func viewSeqProcessingLoop() {
	for {
		seq := <-viewSeqProcessingChan
		log.Println("New view sequence is:", seq)

		leastUpdatedView := findLeastUpdatedView(seq.ViewSeq)

		// create installSeq to send
		installSeq := InstallSeqMsg{}
		installSeq.AssociatedView = &seq.AssociatedView
		installSeq.InstallView = &leastUpdatedView
		installSeq.ViewSeq = seq.ViewSeq
		installSeq.Sender = &thisProcess

		// Send install-seq to all from old and new view
		for _, process := range seq.AssociatedView.GetMembersAlsoIn(&leastUpdatedView) {
			go sendInstallSeq(process, installSeq)
		}
	}
}

type installSeqQuorumCounterType struct {
	list    []*InstallSeq
	counter []int
}

func (quorumCounter *installSeqQuorumCounterType) count(newInstallSeq *InstallSeq, quorumSize int) bool {
	//TODO improve this - cleanup views or change algorithm
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
	var installSeqQuorumCounter installSeqQuorumCounterType

	for {
		installSeqMsg := <-installSeqProcessingChan

		// Check for duplicate
		previousInstallSeq, ok := processToInstallSeqMsgMap[*installSeqMsg.Sender]
		processToInstallSeqMsgMap[*installSeqMsg.Sender] = &installSeqMsg
		if ok && previousInstallSeq.Equal(installSeqMsg) {
			// It's a duplicate
			continue
		}

		// Re-send install-seq to all
		for _, process := range installSeqMsg.AssociatedView.GetMembersAlsoIn(installSeqMsg.InstallView) {
			go sendInstallSeq(process, installSeqMsg)
		}

		// Quorum check
		if installSeqQuorumCounter.count(&installSeqMsg.InstallSeq, installSeqMsg.AssociatedView.QuorumSize()) {
			gotInstallSeqQuorum(installSeqMsg.InstallSeq)
		}
	}
}

func gotInstallSeqQuorum(installSeq InstallSeq) {
	log.Println("start gotInstallSeqQuorum")

	startTime := time.Now()

	installViewMoreUpdatedThanCv := currentView.LessUpdatedThan(installSeq.InstallView)

	if installViewMoreUpdatedThanCv && installSeq.AssociatedView.HasMember(thisProcess) {
		// disable r/w
		register.mu.Lock()
		log.Println("R/W operations disabled")
	}

	if installSeq.AssociatedView.HasMember(thisProcess) { // If the thisProcess was on the old view

		state := StateUpdateMsg{}
		state.Value = register.Value
		state.Timestamp = register.Timestamp
		recvMutex.RLock()
		state.Recv = make(map[view.Update]bool)
		for update, _ := range recv {
			state.Recv[update] = true
		}
		recvMutex.RUnlock()
		state.AssociatedView = installSeq.AssociatedView

		// Send state-update request to all
		for _, process := range installSeq.InstallView.GetMembers() {
			go sendStateUpdate(process, state)
		}

		log.Println("State sent!")
	}

	if installViewMoreUpdatedThanCv {
		if installSeq.InstallView.HasMember(thisProcess) { // If thisProcess is on the new view
			syncState(installSeq.InstallView)

			currentView.Set(installSeq.InstallView)
			log.Println("View installed:", currentView)

			viewInstalled := ViewInstalledMsg{}
			viewInstalled.CurrentView = view.New()
			viewInstalled.CurrentView.Set(&currentView)

			// Send view-installed to all
			for _, process := range installSeq.AssociatedView.GetMembersNotIn(&currentView) {
				go sendViewInstalled(process, viewInstalled)
			}

			var newSeq []view.View
			cvIsMostUpdated := true
			for _, v := range installSeq.ViewSeq {
				if currentView.LessUpdatedThan(&v) {
					newSeq = append(newSeq, v)

					cvIsMostUpdated = false
				}
			}

			if cvIsMostUpdated {
				register.mu.Unlock()
				log.Println("R/W operations enabled")

				endTime := time.Now()
				log.Printf("Reconfiguration COMPLETED, took %v.\n", endTime.Sub(startTime))

				resetTimer <- true
			} else {
				//TODO talvez nao precisa criar essa cópia
				currentViewCopy := view.New()
				currentViewCopy.Set(&currentView)

				log.Println("Generate next view sequence with:", newSeq)
				if useConsensus {
					go generateViewSequenceWithConsensus(currentViewCopy, newSeq)
				} else {
					go generateViewSequenceWithoutConsensus(currentViewCopy, newSeq)
				}
			}
		} else { // If thisProcess is NOT on the new view
			var counter int

			log.Println("Wait for view-installed quorum")
			for {
				viewInstalled := <-newViewInstalledChan

				if installSeq.InstallView.Equal(&viewInstalled.CurrentView) {
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
}

// --------------------- State Update -----------------------

type stateUpdateQuorumType struct {
	associatedView *view.View
	finalValue     *Value
	recv           map[view.Update]bool

	counter int

	callbackChan chan *stateUpdateQuorumType
}

type getCallbackStateUpdateRequest struct {
	associatedView *view.View
	returnChan     chan chan *stateUpdateQuorumType
}

func stateUpdateProcessingLoop() {
	// TODO quorumCounters are expected to be used until all stateUpdate messages have arrived and the result collected.
	stateUpdateQuorumCounterList := list.New()

	for {
		select {
		case stateUpdate := <-stateUpdateProcessingChan:
			if stateUpdate.AssociatedView.LessUpdatedThan(&currentView) {
				log.Println("Old stateUpdate ignored")
				continue
			}

			log.Println("processing stateUpdate:", stateUpdate)

			quorumSize := stateUpdate.AssociatedView.QuorumSize()
			found := false
			// Quorum check
			for quorumCounter := stateUpdateQuorumCounterList.Front(); quorumCounter != nil; quorumCounter = quorumCounter.Next() {
				stateUpdateQuorum := quorumCounter.Value.(*stateUpdateQuorumType)

				if stateUpdateQuorum.associatedView.Equal(stateUpdate.AssociatedView) {
					stateUpdateQuorum.counter++

					if stateUpdateQuorum.counter > quorumSize {
						//Ignore after quorum
						found = true
						break
					}

					for update, _ := range stateUpdate.Recv {
						stateUpdateQuorum.recv[update] = true
					}

					if stateUpdateQuorum.finalValue.Timestamp < stateUpdate.Timestamp {
						stateUpdateQuorum.finalValue.Value = stateUpdate.Value.(int)
						stateUpdateQuorum.finalValue.Timestamp = stateUpdate.Timestamp
					}

					if stateUpdateQuorum.counter == quorumSize {
						// send stateUpdateQuorum through its channel
						go func(stateUpdate *stateUpdateQuorumType) {
							stateUpdate.callbackChan <- stateUpdate
							close(stateUpdate.callbackChan)
						}(stateUpdateQuorum)
					}
					found = true
					break
				}
			}
			if found {
				continue
			}

			newCallbackChan := make(chan *stateUpdateQuorumType)
			newStateUpdateQuorum := stateUpdateQuorumType{stateUpdate.AssociatedView, &Value{Value: stateUpdate.Value.(int), Timestamp: stateUpdate.Timestamp}, make(map[view.Update]bool), 1, newCallbackChan}

			if quorumSize == 1 {
				go func(stateUpdate *stateUpdateQuorumType) { stateUpdate.callbackChan <- stateUpdate }(&newStateUpdateQuorum)
			} else {
				stateUpdateQuorumCounterList.PushBack(&newStateUpdateQuorum)
			}
		case callbackRequest := <-callbackChanStateUpdateRequestChan:
			found := false
			for quorumCounter := stateUpdateQuorumCounterList.Front(); quorumCounter != nil; quorumCounter = quorumCounter.Next() {
				stateUpdateQuorum := quorumCounter.Value.(*stateUpdateQuorumType)

				if stateUpdateQuorum.associatedView.Equal(callbackRequest.associatedView) {
					callbackRequest.returnChan <- stateUpdateQuorum.callbackChan

					found = true
					break
				}
			}
			if found {
				continue
			}

			newCallbackChan := make(chan *stateUpdateQuorumType)
			newStateUpdateQuorum := stateUpdateQuorumType{callbackRequest.associatedView, &Value{Timestamp: -1}, make(map[view.Update]bool), 0, newCallbackChan}
			stateUpdateQuorumCounterList.PushBack(&newStateUpdateQuorum)
			callbackRequest.returnChan <- newStateUpdateQuorum.callbackChan
		}
	}
}

func syncState(installView *view.View) {
	log.Println("start syncState")

	var callbackRequest getCallbackStateUpdateRequest
	returnChan := make(chan chan *stateUpdateQuorumType)
	newView := view.New()
	newView.Set(&currentView)
	callbackRequest.associatedView = &newView
	callbackRequest.returnChan = returnChan

	// Request the chan that has the quorumState
	callbackChanStateUpdateRequestChan <- callbackRequest
	// Receive the chan that has the quorumState
	quorumStateChan := <-returnChan
	// Get the quorumState
	stateUpdateQuorum := <-quorumStateChan

	recvMutex.Lock()
	defer recvMutex.Unlock()

	for update, _ := range stateUpdateQuorum.recv {
		recv[update] = true
	}

	register.Value = stateUpdateQuorum.finalValue.Value
	register.Timestamp = stateUpdateQuorum.finalValue.Timestamp

	for _, update := range installView.GetEntries() {
		delete(recv, update)
	}

	log.Println("end syncState")
}

func reconfigurationTask() {
	recvMutex.Lock()
	defer recvMutex.Unlock()

	if len(recv) != 0 {
		log.Println("Reconfiguration from currentView:", currentView)
		var seq []view.View
		newView := view.New()

		newView.Set(&currentView)
		for update, _ := range recv {
			newView.AddUpdate(update)
		}

		seq = append(seq, newView)
		currentViewCopy := view.New()
		currentViewCopy.Set(&currentView)

		if useConsensus {
			go generateViewSequenceWithConsensus(currentViewCopy, seq)
		} else {
			go generateViewSequenceWithoutConsensus(currentViewCopy, seq)
		}
	} else {
		log.Println("Reconfiguration is not necessary! Restart timer.")
		resetTimer <- true
	}
}

type SimpleQuorumCounter struct {
	counter int
}

func (quorumCounter *SimpleQuorumCounter) count(quorumSize int) bool {
	quorumCounter.counter++
	return quorumCounter.counter == quorumSize
}

func Join() error {
	resultChan := make(chan error, currentView.N())
	errChan := make(chan error, currentView.N())
	stopChan := make(chan bool, currentView.N())
	defer fillStopChan(stopChan, currentView.N())

	reconfig := ReconfigMsg{CurrentView: view.New(), Update: view.Update{view.Join, thisProcess}}
	reconfig.CurrentView.Set(&currentView)

	// Send reconfig request to all
	for _, process := range currentView.GetMembers() {
		go sendReconfigRequest(process, reconfig, resultChan, errChan, stopChan)
	}

	// Get quorum
	var failed int
	var success int
	for {
		select {
		case result := <-resultChan:
			if result != nil {
				errChan <- result
				break
			}

			success++
			if success == currentView.QuorumSize() {
				return nil
			}
		case err := <-errChan:
			log.Println("+1 failure to reconfig:", err)

			failed++
			if failed > currentView.F() {
				return errors.New("Failed to get rec-confirm quorun")
			}
		}
	}
}

func Leave() error {
	resultChan := make(chan error, currentView.N())
	errChan := make(chan error, currentView.N())
	stopChan := make(chan bool, currentView.N())
	defer fillStopChan(stopChan, currentView.N())

	reconfig := ReconfigMsg{Update: view.Update{view.Leave, thisProcess}}
	reconfig.CurrentView = view.New()
	reconfig.CurrentView.Set(&currentView)

	// Send reconfig request to all
	for _, process := range currentView.GetMembers() {
		go sendReconfigRequest(process, reconfig, resultChan, errChan, stopChan)
	}

	// Get quorum
	var failed int
	var success int
	for {
		select {
		case result := <-resultChan:
			if result != nil {
				errChan <- result
				break
			}

			success++
			if success == currentView.QuorumSize() {
				return nil
			}
		case err := <-errChan:
			log.Println("+1 failure to reconfig:", err)

			failed++
			if failed > currentView.F() {
				return errors.New("Failed to get rec-confirm quorun")
			}
		}
	}
}

// -------- REQUESTS -----------
type ReconfigurationRequest int

type ReconfigMsg struct {
	Update      view.Update
	CurrentView view.View
}

type InstallSeq struct {
	InstallView    *view.View
	ViewSeq        []view.View
	AssociatedView *view.View
}

func (installSeq InstallSeq) Equal(installSeq2 InstallSeq) bool {
	if len(installSeq.ViewSeq) != len(installSeq2.ViewSeq) {
		return false
	}
	if installSeq.InstallView.Equal(installSeq2.InstallView) {
		if installSeq.AssociatedView.Equal(installSeq2.AssociatedView) {

			for i, v := range installSeq.ViewSeq {
				if !v.Equal(&installSeq2.ViewSeq[i]) {
					return false
				}
			}

			return true
		}
	}

	return false
}

type InstallSeqMsg struct {
	Sender *view.Process
	InstallSeq
}

func (installSeq InstallSeqMsg) String() string {
	return fmt.Sprintf("Sender: %v\nInstallView: %v\nAssociatedView: %v\nViewSeq: %v", *(installSeq.Sender), *(installSeq.InstallView), *(installSeq.AssociatedView), installSeq.ViewSeq)
}

func (installSeqMsg InstallSeqMsg) Equal(installSeqMsg2 InstallSeqMsg) bool {
	if *(installSeqMsg.Sender) == *(installSeqMsg2.Sender) {
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
	CurrentView view.View
}

func (r *ReconfigurationRequest) Reconfig(arg ReconfigMsg, reply *error) error {
	if arg.CurrentView.Equal(&currentView) {
		if !currentView.HasUpdate(arg.Update) {
			recvMutex.Lock()
			defer recvMutex.Unlock()

			recv[arg.Update] = true
			log.Printf("%v added to recv\n", arg.Update)
		}
	} else {
		err := view.OldViewError{}
		err.OldView.Set(&arg.CurrentView)
		err.NewView.Set(&currentView)
		*reply = err
		log.Printf("Reconfig with old view: %v.\n", arg.CurrentView)
	}
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
	reconfigurationRequest := new(ReconfigurationRequest)
	rpc.Register(reconfigurationRequest)
}

// -------- Send functions -----------
func sendViewInstalled(process view.Process, viewInstalled ViewInstalledMsg) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return
	}
	defer client.Close()

	var reply error
	err = client.Call("ReconfigurationRequest.ViewInstalled", viewInstalled, &reply)
	if err != nil {
		return
	}
}

func sendStateUpdate(process view.Process, state StateUpdateMsg) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return
	}
	defer client.Close()

	var reply error
	err = client.Call("ReconfigurationRequest.StateUpdate", state, &reply)
	if err != nil {
		return
	}
}

func sendInstallSeq(process view.Process, installSeq InstallSeqMsg) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return
	}
	defer client.Close()

	var reply error
	err = client.Call("ReconfigurationRequest.InstallSeq", installSeq, &reply)
	if err != nil {
		return
	}
}

func sendReconfigRequest(process view.Process, reconfig ReconfigMsg, resultChan chan error, errChan chan error, stopChan chan bool) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		select {
		case errChan <- err:
		case <-stopChan:
			log.Println("Error ignored:", err)
		}
		return
	}
	defer client.Close()

	var reply error

	err = client.Call("ReconfigurationRequest.Reconfig", reconfig, &reply)
	if err != nil {
		select {
		case errChan <- err:
		case <-stopChan:
			log.Println("Error ignored:", err)
		}
		return
	}

	select {
	case resultChan <- reply:
	case <-stopChan:
	}
}
