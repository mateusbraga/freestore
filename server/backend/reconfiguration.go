package backend

import (
	"errors"
	"fmt"
	"log"
	"net/rpc"
	"runtime"
	"sync"
	"time"

	"mateusbraga/gotf/view"
)

var (
	recv      map[view.Update]bool
	recvMutex sync.RWMutex

	newViewSeqChan       chan newViewSeq
	newInstallSeqMsgChan chan InstallSeqMsg
	newStateUpdateChan   chan StateUpdateMsg
	newViewInstalledChan chan ViewInstalledMsg

	resetTimer chan bool
)

func init() {
	recv = make(map[view.Update]bool)

	newViewSeqChan = make(chan newViewSeq)
	newInstallSeqMsgChan = make(chan InstallSeqMsg, 20)    //TODO
	newStateUpdateChan = make(chan StateUpdateMsg, 20)     //TODO
	newViewInstalledChan = make(chan ViewInstalledMsg, 20) //TODO

	go newViewSeqListener()
	go newInstallSeqListener()

	resetTimer = make(chan bool, 20) //TODO
	go resetTimerListener()
}

func resetTimerListener() {
	timer := time.AfterFunc(10*time.Second, reconfigurationTask)
	for {
		<-resetTimer
		timer.Reset(1 * time.Minute)
	}
}

type newViewSeq struct {
	ViewSeq        []view.View
	AssociatedView view.View
}

func findLeastUpdatedView(seq []view.View) view.View {
	for i, v := range seq {
		isLeastUpdated := true
		for _, v2 := range seq[i+1:] {
			if v.Contains(v2) {
				isLeastUpdated = false
				break
			}
		}
		if isLeastUpdated {
			return v
		}
	}

	log.Panicln("Failed to find least updated view: ", seq)
	return view.New()
}

func newViewSeqListener() {
	for {
		seq := <-newViewSeqChan
		log.Println("New view sequence is:", seq)

		leastUpdatedView := findLeastUpdatedView(seq.ViewSeq)

		// create installSeq to send
		installSeq := InstallSeqMsg{}
		installSeq.AssociatedView = &seq.AssociatedView
		installSeq.InstallView = &leastUpdatedView
		installSeq.ViewSeq = seq.ViewSeq
		installSeq.Sender = &thisProcess

		// Send install-seq to all from old and new view
		for _, process := range seq.AssociatedView.GetMembersAlsoIn(leastUpdatedView) {
			go sendInstallSeq(process, installSeq)
		}
	}
}

type installSeqQuorumCounterType struct {
	list    []*InstallSeq
	counter []int
}

func (quorumCounter *installSeqQuorumCounterType) count(newInstallSeq *InstallSeq, quorumSize int) bool {
	//TODO improve this - cleanup viewS or change algorithm
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newInstallSeq) {
			quorumCounter.counter[i]++

			if quorumCounter.counter[i] == quorumSize {
				return true
			}
		}
	}

	quorumCounter.list = append(quorumCounter.list, newInstallSeq)
	quorumCounter.counter = append(quorumCounter.counter, 1)

	return (1 == quorumSize)
}

func newInstallSeqListener() {
	processToInstallSeqMsgMap := make(map[view.Process]*InstallSeqMsg)
	var installSeqQuorumCounter installSeqQuorumCounterType

	for {
		installSeqMsg := <-newInstallSeqMsgChan

		// Check for duplicate
		previousInstallSeq, ok := processToInstallSeqMsgMap[*installSeqMsg.Sender]
		processToInstallSeqMsgMap[*installSeqMsg.Sender] = &installSeqMsg
		if ok && previousInstallSeq.Equal(installSeqMsg) {
			// It's a duplicate
			continue
		}

		// Re-send install-seq to all
		for _, process := range installSeqMsg.AssociatedView.GetMembersAlsoIn(*installSeqMsg.InstallView) {
			go sendInstallSeq(process, installSeqMsg)
		}

		// Quorum check
		if installSeqQuorumCounter.count(&installSeqMsg.InstallSeq, installSeqMsg.AssociatedView.QuorumSize()) {
			gotInstallSeqQuorum(installSeqMsg.InstallSeq)
		}
	}
}

func syncState(installSeq InstallSeq) {
	log.Println("start syncState")
	recvMutex.Lock()
	defer recvMutex.Unlock()

	var counter int
	var finalValue Value
	finalValue.Timestamp = -1

	for {
		stateUpdate := <-newStateUpdateChan

		log.Println("stateUpdate is:", stateUpdate)

		if currentView.Equal(*stateUpdate.AssociatedView) {
			for update, _ := range stateUpdate.Recv {
				recv[update] = true
			}

			if finalValue.Timestamp < stateUpdate.Timestamp {
				finalValue.Value = stateUpdate.Value.(int)
				finalValue.Timestamp = stateUpdate.Timestamp
			}

			counter++
			if counter == currentView.QuorumSize() {
				register.Value = finalValue.Value
				register.Timestamp = finalValue.Timestamp
				break
			}
		}
	}

	for _, update := range installSeq.InstallView.GetEntries() {
		delete(recv, update)
	}
	log.Println("end syncState")
}

func gotInstallSeqQuorum(installSeq InstallSeq) {
	log.Println("start gotInstallSeqQuorum")

	startTime := time.Now()

	installViewContainsCv := installSeq.InstallView.Contains(currentView)

	if installViewContainsCv && installSeq.AssociatedView.HasMember(thisProcess) {
		// disable r/w
		register.mu.Lock()
		log.Println("R/W operations disabled")
	}

	if installSeq.AssociatedView.HasMember(thisProcess) { // If the thisProcess was on the old view
		recvMutex.RLock()

		state := StateUpdateMsg{}
		state.Value = register.Value
		state.Timestamp = register.Timestamp
		state.Recv = recv
		state.AssociatedView = installSeq.AssociatedView

		// Send state-update request to all
		for _, process := range installSeq.InstallView.GetMembers() {
			go sendStateUpdate(process, state)
		}

		recvMutex.RUnlock()
		log.Println("State sent!")
	}

	if installViewContainsCv {
		if installSeq.InstallView.HasMember(thisProcess) { // If thisProcess is on the new view
			syncState(installSeq)

			currentView.Set(*installSeq.InstallView)
			log.Println("View installed:", currentView)

			viewInstalled := ViewInstalledMsg{}
			viewInstalled.CurrentView = view.New()
			viewInstalled.CurrentView.Set(currentView)

			// Send view-installed to all
			for _, process := range installSeq.AssociatedView.GetMembersNotIn(currentView) {
				go sendViewInstalled(process, viewInstalled)
			}

			var newSeq []view.View
			cvIsMostUpdated := true
			for _, v := range installSeq.ViewSeq {
				if v.Contains(currentView) && !currentView.Contains(v) {
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
				currentViewCopy := view.New()
				currentViewCopy.Set(currentView)

				log.Println("Generate next view sequence with:", newSeq)
				generateViewSequenceWithoutConsensus(currentViewCopy, newSeq)
			}
		} else { // If thisProcess is NOT on the new view
			var counter int

			log.Println("Wait for view-installed quorum")
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
			runtime.Gosched() // Just to reduce the chance of showing errors because it terminated too early
		}
	}
}

func reconfigurationTask() {
	recvMutex.Lock()
	defer recvMutex.Unlock()

	if len(recv) != 0 {
		log.Println("Reconfiguration from currentView:", currentView)
		var seq []view.View
		newView := view.New()

		newView.Set(currentView)
		for update, _ := range recv {
			newView.AddUpdate(update)
		}

		seq = append(seq, newView)
		currentViewCopy := view.New()
		currentViewCopy.Set(currentView)
		generateViewSequenceWithoutConsensus(currentViewCopy, seq)
	} else {
		log.Println("Reconfiguration is not necessary! Restart timer.")
		resetTimer <- true
	}
}

func generateViewSequenceWithConsensus(associatedView view.View, seq []view.View) {
	log.Println("start generateViewSequenceWithConsensus")
	// assert only updated views on seq
	for _, view := range seq {
		if !view.Contains(associatedView) {
			log.Fatalln("Found an old view in view sequence:", seq)
		}
	}

	callbackChan := make(chan interface{})
	consensus := GetConsensusOrCreate(currentView.NumberOfEntries(), callbackChan)
	if currentView.GetProcessPosition(thisProcess) == 0 {
		log.Println("CONSENSUS: leader")
		consensus.Propose(seq)
	}
	log.Println("CONSENSUS: wait learn message")
	value := <-consensus.CallbackLearnChan

	result, ok := value.(*[]view.View)
	if !ok {
		log.Fatalf("FATAL: consensus on generateViewSequenceWithConsensus got %T %v\n", value, value)
	}
	newViewSeqChan <- newViewSeq{*result, associatedView}
	log.Println("end generateViewSequenceWithConsensus")
}

type SimpleQuorumCounter struct {
	counter int
}

func (quorumCounter *SimpleQuorumCounter) count(quorumSize int) bool {
	quorumCounter.counter++
	if quorumCounter.counter == quorumSize {
		return true
	}
	return false
}

func Join() error {
	resultChan := make(chan error, currentView.N())
	errChan := make(chan error, currentView.N())

	reconfig := ReconfigMsg{CurrentView: view.New(), Update: view.Update{view.Join, thisProcess}}
	reconfig.CurrentView.Set(currentView)

	// Send reconfig request to all
	for _, process := range currentView.GetMembers() {
		go sendReconfigRequest(process, reconfig, resultChan, errChan)
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

	reconfig := ReconfigMsg{Update: view.Update{view.Leave, thisProcess}}
	reconfig.CurrentView = view.New()
	reconfig.CurrentView.Set(currentView)

	// Send reconfig request to all
	for _, process := range currentView.GetMembers() {
		go sendReconfigRequest(process, reconfig, resultChan, errChan)
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
	if installSeq.InstallView.Equal(*installSeq2.InstallView) {
		if installSeq.AssociatedView.Equal(*installSeq2.AssociatedView) {
			if len(installSeq.ViewSeq) != len(installSeq2.ViewSeq) {
				return false
			}

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
	recvMutex.Lock()
	defer recvMutex.Unlock()

	if arg.CurrentView.Equal(currentView) {
		if !currentView.HasUpdate(arg.Update) {
			recv[arg.Update] = true
			log.Printf("%v added to recv\n", arg.Update)
		}
	} else {
		err := view.OldViewError{}
		err.OldView.Set(arg.CurrentView)
		err.NewView.Set(currentView)
		*reply = err
		log.Printf("Reconfig with old view: %v.\n", arg.CurrentView)
	}
	return nil
}

func (r *ReconfigurationRequest) InstallSeq(arg InstallSeqMsg, reply *error) error {
	newInstallSeqMsgChan <- arg
	return nil
}

func (r *ReconfigurationRequest) StateUpdate(arg StateUpdateMsg, reply *error) error {
	newStateUpdateChan <- arg
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

func sendReconfigRequest(process view.Process, reconfig ReconfigMsg, resultChan chan error, errChan chan error) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		errChan <- err
		return
	}
	defer client.Close()

	var reply error

	err = client.Call("ReconfigurationRequest.Reconfig", reconfig, &reply)
	if err != nil {
		errChan <- err
		return
	}

	resultChan <- reply
}
