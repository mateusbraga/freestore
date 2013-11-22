package server

import (
	"log"
	"net/rpc"
	"sync"

	"mateusbraga/freestore/view"
)

const (
	LEADER_PROCESS_POSITION int = 0
)

var (
	//IMPROV: clean this up periodically
	viewGenerators   []viewGeneratorInstance
	viewGeneratorsMu sync.Mutex
)

type viewGeneratorInstance struct {
	AssociatedView view.View //Id
	jobChan        chan ViewGeneratorJob
}

type ViewGeneratorJob interface{}

func getViewGenerator(associatedView view.View, initialSeq []view.View) viewGeneratorInstance {
	viewGeneratorsMu.Lock()
	defer viewGeneratorsMu.Unlock()

	for _, vgi := range viewGenerators {
		if vgi.AssociatedView.Equal(&associatedView) {
			return vgi
		}
	}

	vgi := viewGeneratorInstance{}
	vgi.AssociatedView = associatedView.NewCopy()
	vgi.jobChan = make(chan ViewGeneratorJob, CHANNEL_DEFAULT_SIZE)
	go ViewGeneratorWorker(vgi, initialSeq)

	viewGenerators = append(viewGenerators, vgi)
	log.Println("Created vgi", vgi)
	return vgi
}

func ViewGeneratorWorker(vgi viewGeneratorInstance, seq []view.View) {
	// Make a copy of the vgi
	associatedView := vgi.AssociatedView.NewCopy()
	jobChan := vgi.jobChan

	var proposedSeq []view.View
	var lastConvergedSeq []view.View
	var viewSeqQuorumCounter viewSeqQuorumCounterType
	var seqConvQuorumCounter seqConvQuorumCounterType

	if seq != nil {
		proposedSeq = seq
		log.Println("proposedSeq is:", seq)

		// Send viewSeq to all
		viewSeq := ViewSeqMsg{}
		viewSeq.ProposedSeq = proposedSeq
		viewSeq.LastConvergedSeq = nil
		viewSeq.AssociatedView = associatedView
		for _, process := range associatedView.GetMembers() {
			go sendViewSequence(process, viewSeq)
		}
	}

	for {
		job := <-jobChan
		switch jobPointer := job.(type) {
		case *ViewSeq:
			log.Println("new ViewSeq")
			log.Println(jobPointer)
			var addToProposedSeq []view.View

			hasChanges := false
			if proposedSeq == nil {
				proposedSeq = jobPointer.ProposedSeq
				hasChanges = true
			} else {
				for _, v := range jobPointer.ProposedSeq {
					found := false
					for _, v2 := range proposedSeq {
						if v.Equal(&v2) {
							found = true
							break
						}
					}
					if !found { // v is a new view
						hasChanges = true
						log.Println("New view received:", v)

						hasConflict := true
						for _, v2 := range proposedSeq {
							if v.LessUpdatedThan(&v2) || v2.LessUpdatedThan(&v) {
								hasConflict = false
								break
							}
						}

						if hasConflict {
							log.Println("Has conflict!")
							jobPointerMostUpdatedView := findMostUpdatedView(jobPointer.LastConvergedSeq)
							thisProcessMostUpdatedView := findMostUpdatedView(lastConvergedSeq)
							if thisProcessMostUpdatedView.LessUpdatedThan(&jobPointerMostUpdatedView) {
								lastConvergedSeq = jobPointer.LastConvergedSeq
							}

							oldMostUpdated := findMostUpdatedView(proposedSeq)
							receivedMostUpdated := findMostUpdatedView(jobPointer.ProposedSeq)

							auxView := oldMostUpdated.NewCopy()
							auxView.Merge(&receivedMostUpdated)

							proposedSeq = append(lastConvergedSeq, auxView)
							break
						} else {
							log.Println("Added to proposedSeq")
							addToProposedSeq = append(addToProposedSeq, v)
						}
					}
				}
			}

			if hasChanges {
				for _, v := range addToProposedSeq {
					proposedSeq = append(proposedSeq, v)
				}

				log.Println("done merge")

				viewSeqAux := ViewSeqMsg{}
				viewSeqAux.AssociatedView = associatedView
				viewSeqAux.ProposedSeq = proposedSeq
				viewSeqAux.LastConvergedSeq = lastConvergedSeq

				// Send seq-view to all
				for _, process := range associatedView.GetMembers() {
					go sendViewSequence(process, viewSeqAux)
				}
			}

			// Quorum check
			if viewSeqQuorumCounter.count(jobPointer, associatedView.QuorumSize()) {
				lastConvergedSeq = jobPointer.ProposedSeq

				seqConv := SeqConvMsg{}
				seqConv.AssociatedView = associatedView
				seqConv.Seq = jobPointer.ProposedSeq

				log.Println("Got seqConv quorum!")
				log.Println(seqConv)
				// Send seq-conv to all
				for _, process := range associatedView.GetMembers() {
					go sendViewSequenceConv(process, seqConv)
				}
			}

		case *SeqConv:
			log.Println("new SeqConv")
			log.Println(jobPointer)

			// Quorum check
			if seqConvQuorumCounter.count(jobPointer, associatedView.QuorumSize()) {
				viewSeqProcessingChan <- newViewSeq{jobPointer.Seq, associatedView}
			}

		//case StopWorker:
		//return
		default:
			log.Fatalln("Something is wrong with the switch statement")
		}

	}
}

func assertOnlyUpdatedViews(view view.View, seq []view.View) {
	for _, view := range seq {
		if view.LessUpdatedThan(&view) {
			log.Fatalf("BUG! Found an old view in view sequence: %v. view: %v\n", seq, view)
		}
	}
}

// we can change seq
func generateViewSequenceWithoutConsensus(associatedView view.View, seq []view.View) {
	log.Println("start generateViewSequenceWithoutConsensus")
	assertOnlyUpdatedViews(associatedView, seq)

	_ = getViewGenerator(associatedView, seq)
}

func generateViewSequenceWithConsensus(associatedView view.View, seq []view.View) {
	log.Println("start generateViewSequenceWithConsensus")
	assertOnlyUpdatedViews(associatedView, seq)

	consensusInstance := getConsensus(associatedView.NumberOfEntries())
	if associatedView.GetProcessPosition(thisProcess) == LEADER_PROCESS_POSITION {
		log.Println("CONSENSUS: leader")
		Propose(consensusInstance, seq)
	}
	log.Println("CONSENSUS: wait learn message")
	value := <-consensusInstance.callbackLearnChan

	result, ok := value.(*[]view.View)
	if !ok {
		log.Fatalf("FATAL: consensus on generateViewSequenceWithConsensus got %T %v\n", value, value)
	}
	viewSeqProcessingChan <- newViewSeq{*result, associatedView}
	log.Println("end generateViewSequenceWithConsensus")
}

type viewSeqQuorumCounterType struct {
	list    []*ViewSeq
	counter []int
}

func (quorumCounter *viewSeqQuorumCounterType) count(newViewSeq *ViewSeq, quorumSize int) bool {
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newViewSeq) {
			quorumCounter.counter[i]++

			return quorumCounter.counter[i] == quorumSize
		}
	}

	quorumCounter.list = append(quorumCounter.list, newViewSeq)
	quorumCounter.counter = append(quorumCounter.counter, 1)

	return (1 == quorumSize)
}

type seqConvQuorumCounterType struct {
	list    []*SeqConv
	counter []int
}

func (quorumCounter *seqConvQuorumCounterType) count(newSeqConv *SeqConv, quorumSize int) bool {
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newSeqConv) {
			quorumCounter.counter[i]++

			return quorumCounter.counter[i] == quorumSize
		}
	}

	quorumCounter.list = append(quorumCounter.list, newSeqConv)
	quorumCounter.counter = append(quorumCounter.counter, 1)

	return (1 == quorumSize)
}

func findMostUpdatedView(seq []view.View) view.View {
	if len(seq) == 0 {
		log.Panicln("ERROR: Got empty seq on findLeastUpdatedView")
	}

	mostUpdatedView := seq[0]
	for _, v := range seq[1:] {
		if mostUpdatedView.LessUpdatedThan(&v) {
			mostUpdatedView.Set(&v)
		}
	}

	return mostUpdatedView
}

// -------- REQUESTS -----------
type SeqConv struct {
	Seq []view.View
}

type SeqConvMsg struct {
	AssociatedView view.View
	SeqConv
}

func (seqConv SeqConv) Equal(seqConv2 SeqConv) bool {
	if len(seqConv.Seq) != len(seqConv2.Seq) {
		return false
	}
	for i, _ := range seqConv.Seq {
		if !seqConv.Seq[i].Equal(&seqConv2.Seq[i]) {
			return false
		}
	}
	return true
}

type ViewSeq struct {
	ProposedSeq      []view.View
	LastConvergedSeq []view.View
}

type ViewSeqMsg struct {
	AssociatedView view.View
	ViewSeq
}

func (viewSeq ViewSeq) Equal(viewSeq2 ViewSeq) bool {
	if len(viewSeq.ProposedSeq) != len(viewSeq2.ProposedSeq) {
		return false
	}

	for i, _ := range viewSeq.ProposedSeq {
		if !viewSeq.ProposedSeq[i].Equal(&viewSeq2.ProposedSeq[i]) {
			return false
		}
	}
	return true
}

type ViewGeneratorRequest int

func (r *ViewGeneratorRequest) ViewSeq(arg ViewSeqMsg, reply *error) error {
	vgi := getViewGenerator(arg.AssociatedView, nil)
	vgi.jobChan <- &arg.ViewSeq

	return nil
}

func (r *ViewGeneratorRequest) SeqConv(arg SeqConvMsg, reply *error) error {
	vgi := getViewGenerator(arg.AssociatedView, nil)
	vgi.jobChan <- &arg.SeqConv

	return nil
}

func init() {
	viewGeneratorRequest := new(ViewGeneratorRequest)
	rpc.Register(viewGeneratorRequest)
}

// -------- Send functions -----------
func sendViewSequence(process view.Process, viewSeq ViewSeqMsg) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return
	}
	defer client.Close()

	var reply error
	err = client.Call("ViewGeneratorRequest.ViewSeq", viewSeq, &reply)
	if err != nil {
		return
	}
}

func sendViewSequenceConv(process view.Process, seqConv SeqConvMsg) {
	client, err := rpc.Dial("tcp", process.Addr)
	if err != nil {
		return
	}
	defer client.Close()

	var reply error
	err = client.Call("ViewGeneratorRequest.SeqConv", seqConv, &reply)
	if err != nil {
		return
	}
}
