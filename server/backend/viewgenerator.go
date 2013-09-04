package backend

import (
	"log"
	"net/rpc"
	"sync"

	"mateusbraga/gotf/view"
)

var (
	viewGenerators   []ViewGeneratorInfo
	viewGeneratorsMu sync.Mutex
)

type ViewGeneratorInfo struct {
	AssociatedView view.View
	jobChan        chan ViewGeneratorJob
}

type ViewGeneratorJob interface{}

func findMostUpdatedView(seq []view.View) view.View {
	for i, v := range seq {
		isMostUpdated := true
		for _, v2 := range seq[i+1:] {
			if !v.Contains(v2) {
				isMostUpdated = false
				break
			}
		}
		if isMostUpdated {
			return v
		}
	}

	log.Fatalln("Could not find most updated view in:", seq)
	return view.New()
}

func getViewGenerator(associatedView view.View, initialSeq []view.View) ViewGeneratorInfo {
	viewGeneratorsMu.Lock()
	defer viewGeneratorsMu.Unlock()

	for _, vgi := range viewGenerators {
		if vgi.AssociatedView.Equal(associatedView) {
			return vgi
		}
	}

	vgi := ViewGeneratorInfo{}
	vgi.AssociatedView = view.New()
	vgi.AssociatedView.Set(associatedView)
	vgi.jobChan = make(chan ViewGeneratorJob, 20) //TODO
	go ViewGeneratorWorker(vgi.AssociatedView, initialSeq, vgi.jobChan)

	viewGenerators = append(viewGenerators, vgi)
	log.Println("Created vgi", vgi)
	return vgi
}

func ViewGeneratorWorker(associatedView view.View, seq []view.View, jobChan chan ViewGeneratorJob) {
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
						if v.Equal(v2) {
							found = true
							break
						}
					}
					if !found { // v is a new view
						hasChanges = true
						log.Println("New view received:", v)

						hasConflict := true
						for _, v2 := range proposedSeq {
							if v.Contains(v2) || v2.Contains(v) {
								hasConflict = false
								break
							}
						}

						if hasConflict {
							log.Println("Has conflict!")
							if findMostUpdatedView(jobPointer.LastConvergedSeq).Contains(findMostUpdatedView(lastConvergedSeq)) {
								lastConvergedSeq = jobPointer.LastConvergedSeq
							}

							oldMostUpdated := findMostUpdatedView(proposedSeq)
							receivedMostUpdated := findMostUpdatedView(jobPointer.ProposedSeq)

							auxView := view.New()
							auxView.Set(oldMostUpdated)
							auxView.Merge(receivedMostUpdated)

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
				newViewSeqChan <- newViewSeq{jobPointer.Seq, associatedView}
			}

		default:
			log.Fatalln("Something is wrong with the switch statement")
		}

	}
}

// we can change seq
func generateViewSequenceWithoutConsensus(associatedView view.View, seq []view.View) {
	log.Println("start generateViewSequenceWithoutConsensus")

	// assert only updated views on seq
	for _, view := range seq {
		if !view.Contains(associatedView) || associatedView.Contains(view) {
			log.Fatalln("Found an old view in view sequence:", seq, ". associatedView:", associatedView)
		}
	}

	_ = getViewGenerator(associatedView, seq)
}

type viewSeqQuorumCounterType struct {
	list    []*ViewSeq
	counter []int
}

func (quorumCounter *viewSeqQuorumCounterType) count(newViewSeq *ViewSeq, quorumSize int) bool {
	//TODO improve this - cleanup viewS or change algorithm
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newViewSeq) {
			quorumCounter.counter[i]++

			if quorumCounter.counter[i] == quorumSize {
				return true
			}
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
	//TODO improve this - cleanup viewS or change algorithm
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newSeqConv) {
			quorumCounter.counter[i]++

			if quorumCounter.counter[i] == quorumSize {
				return true
			}
		}
	}

	quorumCounter.list = append(quorumCounter.list, newSeqConv)
	quorumCounter.counter = append(quorumCounter.counter, 1)

	return (1 == quorumSize)
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
		if !seqConv.Seq[i].Equal(seqConv2.Seq[i]) {
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
	if len(viewSeq.LastConvergedSeq) != len(viewSeq2.LastConvergedSeq) {
		return false
	}

	for i, _ := range viewSeq.ProposedSeq {
		if !viewSeq.ProposedSeq[i].Equal(viewSeq2.ProposedSeq[i]) {
			return false
		}
	}
	for i, _ := range viewSeq.LastConvergedSeq {
		if !viewSeq.LastConvergedSeq[i].Equal(viewSeq2.LastConvergedSeq[i]) {
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
