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
	newInstallSeqChan    chan InstallSeqMsg
	newStateUpdateChan   chan StateUpdateMsg
	newViewInstalledChan chan ViewInstalledMsg

	//reconfigStatus ReconfigurationStatus
)

//type ReconfigurationStatus int

//const (
//WAIT_TIMEOUT ReconfigurationStatus = iota
//WAIT_STATE_UPDATE
//WAIT_VIEW_INSTALLED
//)

func init() {
	recv = make(map[view.Update]bool)

	newViewSeqChan = make(chan newViewSeq)
	newInstallSeqChan = make(chan InstallSeqMsg, 20)       //TODO
	newStateUpdateChan = make(chan StateUpdateMsg, 20)     //TODO
	newViewInstalledChan = make(chan ViewInstalledMsg, 20) //TODO

	go newViewSeqListener()
	go newInstallSeqListener()

	time.AfterFunc(10*time.Second, reconfigurationTask)
}

type newViewSeq struct {
	ViewSeq        []view.View
	AssociatedView view.View
}

func findLeastUpdatedView(seq []view.View) view.View {
	for i, v := range seq {
		isLeastUpdated := true
		for _, v2 := range seq[i+1:] {
			if !v.Contains(v2) {
				isLeastUpdated = false
				break
			}
		}
		if isLeastUpdated {
			return v
		}
	}

	log.Panicln("Failed to find least updated view")
	return view.New()
}

func newViewSeqListener() {
	for {
		seq := <-newViewSeqChan
		log.Println("seq is:", seq)

		leastUpdatedView := findLeastUpdatedView(seq.ViewSeq)

		auxView := view.New()
		auxView.Set(seq.AssociatedView)
		auxView.Merge(leastUpdatedView)

		// create installSeq to send
		installSeq := InstallSeqMsg{}
		installSeq.AssociatedView = &seq.AssociatedView
		installSeq.InstallView = &leastUpdatedView
		installSeq.ViewSeq = seq.ViewSeq
		installSeq.Sender = &thisProcess

		//log.Println("2installSeq.AssociatedView is:", installSeq.AssociatedView)
		//log.Println("2installSeq.InstallView is:", installSeq.InstallView)
		//log.Println("2installSeq.Sender is:", installSeq.Sender)
		//log.Println("2installSeq.ViewSeq is:", installSeq.ViewSeq)

		if thisProcess.Addr == "[::]:5000" {
			log.Println("sleep")
			time.Sleep(10 * time.Second)
		}
		// Send install-seq to all from old and new view
		for _, process := range auxView.GetMembers() {
			go sendInstallSeq(process, installSeq)
		}
		log.Println("done sending installseq")
	}
}

type installSeqQuorumCounter struct {
	installSeq *InstallSeqMsg
	counter    int
}

func newInstallSeqListener() {
	auxQuorumMap := make(map[view.Process]*InstallSeqMsg)
	auxQuorumCounter := make([]installSeqQuorumCounter, 0)

	for {
		installSeq := <-newInstallSeqChan

		//log.Println("installSeq.AssociatedView is:", installSeq.AssociatedView)
		//log.Println("installSeq.InstallView is:", installSeq.InstallView)
		//log.Println("installSeq.Sender is:", *installSeq.Sender)
		//log.Println("installSeq.ViewSeq is:", installSeq.ViewSeq)

		//pause := make(chan bool)
		//log.Println("pause")
		//pause <- true

		auxInstallSeq, ok := auxQuorumMap[*installSeq.Sender]
		if ok {
			if auxInstallSeq.Equal(installSeq) { // It's a duplicate
				//log.Println("duplicate")
				continue
			}
		}

		auxView := view.New()
		auxView.Set(*installSeq.AssociatedView)
		auxView.Merge(*installSeq.InstallView)
		auxView.AddUpdate(view.Update{view.Leave, thisProcess})

		// Re-send install-seq to all
		for _, process := range auxView.GetMembers() {
			go sendInstallSeq(process, installSeq)
		}

		auxQuorumMap[*installSeq.Sender] = &installSeq

		//TODO improve this - cleanup auxQuorumCounter or change algorithm
		found := false
		for i, _ := range auxQuorumCounter {
			if auxQuorumCounter[i].installSeq.Similar(installSeq) {
				auxQuorumCounter[i].counter++

				//TODO check condition
				if auxQuorumCounter[i].counter == installSeq.AssociatedView.QuorunSize() {
					gotInstallSeqQuorum(installSeq)
				}

				found = true
				break
			}
		}

		if !found {
			auxQuorumCounter = append(auxQuorumCounter, installSeqQuorumCounter{&installSeq, 1})
		}
	}
}

func syncState(installSeq InstallSeqMsg) {
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

			counter++
			if finalValue.Timestamp < stateUpdate.Timestamp {
				finalValue.Value = stateUpdate.Value.(int)
				finalValue.Timestamp = stateUpdate.Timestamp
			}

			if counter == currentView.QuorunSize() {
				register.Value = finalValue.Value
				register.Timestamp = finalValue.Timestamp
				break
			}
		}
	}

	for update, _ := range installSeq.InstallView.Entries {
		delete(recv, update)
	}
	log.Println("end syncState")
}

func gotInstallSeqQuorum(installSeq InstallSeqMsg) {
	log.Println("start gotInstallSeqQuorum")

	startTime := time.Now()

	installViewContainsCv := installSeq.InstallView.Contains(currentView)

	if installViewContainsCv && installSeq.AssociatedView.Members[thisProcess] {
		// disable r/w
		register.mu.Lock()
	}
	log.Println("start gotInstallSeqQuorum 2")

	if installSeq.AssociatedView.Members[thisProcess] { // If the thisProcess was on the old view
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
	}

	log.Println("start gotInstallSeqQuorum 3")
	if installViewContainsCv {
		log.Println("start gotInstallSeqQuorum 4")
		if installSeq.InstallView.Members[thisProcess] { // If thisProcess is on the new view
			log.Println("start gotInstallSeqQuorum 5")
			syncState(installSeq)
			log.Println("start gotInstallSeqQuorum 5.1")

			currentView.Set(*installSeq.InstallView)

			//if !installSeq.AssociatedView.Members[thisProcess] { // If thisProcess was not on the old view
			//register.mu.Unlock()
			//}

			viewInstalled := ViewInstalledMsg{}
			viewInstalled.CurrentView = view.New()
			viewInstalled.CurrentView.Set(currentView)

			// Send view-installed to all
			for _, process := range installSeq.AssociatedView.GetMembersNotIn(currentView) {
				go sendViewInstalled(process, viewInstalled)
			}

			log.Println("start gotInstallSeqQuorum 6")
			var newSeq []view.View
			cvIsMostUpdated := true
			for _, v := range installSeq.ViewSeq {
				//TODO improve this if
				if v.Contains(currentView) && !v.Equal(currentView) {
					newSeq = append(newSeq, v)

					cvIsMostUpdated = false
				}
			}

			log.Println("start gotInstallSeqQuorum 7, cvIsMostUpdated=", cvIsMostUpdated)
			if cvIsMostUpdated {
				register.mu.Unlock()
				endTime := time.Now()
				log.Printf("Reconfiguration COMPLETED, took %v, new view: %v", endTime.Sub(startTime), currentView)
				time.AfterFunc(60*time.Second, reconfigurationTask)
			} else {
				currentViewCopy := view.New()
				currentViewCopy.Set(currentView)

				generateViewSequenceWithConsensus(currentViewCopy, newSeq, newViewSeqChan)
			}
		} else { // If thisProcess is NOT on the new view
			log.Println("start gotInstallSeqQuorum 8")
			var counter int
			for {
				viewInstalled := <-newViewInstalledChan

				if installSeq.InstallView.Equal(viewInstalled.CurrentView) {
					counter++

					if counter >= installSeq.InstallView.QuorunSize() {
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
	log.Println("end gotInstallSeqQuorum")
}

func reconfigurationTask() {
	log.Println("start reconfigurationTask")
	log.Println("currentView is:", currentView)
	recvMutex.Lock()
	defer recvMutex.Unlock()

	if len(recv) != 0 {
		var seq []view.View
		newView := view.New()

		newView.Set(currentView)
		for update, _ := range recv {
			newView.AddUpdate(update)
		}

		seq = append(seq, newView)
		currentViewCopy := view.New()
		currentViewCopy.Set(currentView)
		generateViewSequenceWithConsensus(currentViewCopy, seq, newViewSeqChan)
	} else {
		time.AfterFunc(60*time.Second, reconfigurationTask)
		log.Println("Reconfiguration is not necessary! Restart timer.")
	}
	log.Println("end reconfigurationTask")
}

func generateViewSequenceWithConsensus(associatedView view.View, seq []view.View, resultChan chan newViewSeq) {
	log.Println("start generateViewSequenceWithConsensus")
	// assert only updated views on seq
	for _, view := range seq {
		if !view.Contains(associatedView) {
			log.Fatalln("Found an old view in view sequence!")
		}
	}

	//TODO rethink channel
	callbackChan := make(chan interface{}, 1)
	consensus := GetConsensusOrCreate(currentView.NumberOfEntries(), callbackChan)
	if currentView.GetProcessPosition(thisProcess) == 0 {
		log.Println("generateViewSequenceWithConsensus leader")
		consensus.Propose(seq)
	}
	log.Println("wait callback")
	value := <-consensus.CallbackLearnChan
	log.Printf("consensus on generateViewSequenceWithConsensus got %T %v\n", value, value)

	result, ok := value.(*[]view.View)
	if !ok {
		log.Fatalf("FATAL: consensus on generateViewSequenceWithConsensus got %T %v\n", value, value)
	}
	resultChan <- newViewSeq{*result, associatedView}
	log.Println("end generateViewSequenceWithConsensus")
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

			if success >= currentView.QuorunSize() {
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

			if success >= currentView.QuorunSize() {
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

type InstallSeqMsg struct {
	InstallView    *view.View
	ViewSeq        []view.View
	AssociatedView *view.View
	Sender         *view.Process
}

func (installSeq InstallSeqMsg) String() string {
	return fmt.Sprintf("Sender: %v\nInstallView: %v\nAssociatedView: %v\nViewSeq: %v", *(installSeq.Sender), *(installSeq.InstallView), *(installSeq.AssociatedView), installSeq.ViewSeq)
}

func (installSeq InstallSeqMsg) Equal(installSeq2 InstallSeqMsg) bool {
	//TODO improve this
	if *(installSeq.Sender) == *(installSeq2.Sender) {
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
	}

	return false
}

func (installSeq InstallSeqMsg) Similar(installSeq2 InstallSeqMsg) bool {
	//TODO improve this
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
	log.Println("start reconfig")
	recvMutex.Lock()
	defer recvMutex.Unlock()

	if arg.CurrentView.Equal(currentView) {
		recv[arg.Update] = true
	} else {
		err := view.OldViewError{}
		err.OldView.Set(arg.CurrentView)
		err.NewView.Set(currentView)
		*reply = err
	}
	log.Println("end reconfig")
	return nil
}

func (r *ReconfigurationRequest) InstallSeq(arg InstallSeqMsg, reply *error) error {
	newInstallSeqChan <- arg
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
