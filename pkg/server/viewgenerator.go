package server

import (
	"log"
	"net/rpc"
	"sync"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/consensus"
	"github.com/mateusbraga/freestore/pkg/view"
)

const LEADER_PROCESS_POSITION int = 0

var (
	viewGenerators   []viewGeneratorInstance
	viewGeneratorsMu sync.Mutex
)

type viewGeneratorInstance struct {
	AssociatedView *view.View //Id
	jobChan        chan interface{}
}

func getOrCreateViewGenerator(associatedView *view.View, initialSeq ViewSeq) viewGeneratorInstance {
	viewGeneratorsMu.Lock()
	defer viewGeneratorsMu.Unlock()

	for _, vgi := range viewGenerators {
		if vgi.AssociatedView.Equal(associatedView) {
			return vgi
		}
	}

	// view generator does not exist. Create it.
	vgi := viewGeneratorInstance{}
	vgi.AssociatedView = associatedView.NewCopy()
	vgi.jobChan = make(chan interface{}, CHANNEL_DEFAULT_SIZE)
	viewGenerators = append(viewGenerators, vgi)

	go viewGeneratorWorker(vgi, initialSeq)

	return vgi
}

func viewGeneratorWorker(vgi viewGeneratorInstance, initialSeq ViewSeq) {
	log.Printf("Starting new viewGeneratorWorker with initialSeq: %v\n", initialSeq)

	associatedView := vgi.AssociatedView
	jobChan := vgi.jobChan

	var lastProposedSeq ViewSeq
	var lastConvergedSeq ViewSeq
	var viewSeqQuorumCounter viewSeqQuorumCounterType
	var seqConvQuorumCounter seqConvQuorumCounterType

	if len(initialSeq) != 0 {
		// Send viewSeqMsg to all
		viewSeqMsg := ViewSeqMsg{}
		viewSeqMsg.ProposedSeq = initialSeq
		viewSeqMsg.LastConvergedSeq = nil
		viewSeqMsg.AssociatedView = associatedView

		go broadcastViewSequence(associatedView.NewCopy(), viewSeqMsg)

		lastProposedSeq = initialSeq
	}

	for {
		job := <-jobChan
		switch jobPointer := job.(type) {
		case ViewSeqMsg:
			receivedViewSeqMsg := jobPointer
			log.Println("new ViewSeqMsg:", receivedViewSeqMsg)

			var newProposeSeq ViewSeq

			hasChanges := false
			hasConflict := false

		OuterLoop:
			for _, v := range receivedViewSeqMsg.ProposedSeq {
				if lastProposedSeq.HasView(v) {
					continue
				}

				log.Printf("New view received: %v\n", v)
				hasChanges = true

				// check if v conflicts with any view from lastProposedSeq
				for _, v2 := range lastProposedSeq {
					if !v.LessUpdatedThan(v2) && !v2.LessUpdatedThan(v) {
						log.Printf("Has conflict between %v and %v!\n", v, v2)
						hasConflict = true
						break OuterLoop
					}
				}
			}

			if hasChanges {
				if hasConflict {
					// set lastConvergedSeq to the one with the most updated view
					receivedLastConvergedSeqMostUpdatedView := receivedViewSeqMsg.LastConvergedSeq.GetMostUpdatedView()
					thisProcessLastConvergedSeqMostUpdatedView := lastConvergedSeq.GetMostUpdatedView()
					if thisProcessLastConvergedSeqMostUpdatedView.LessUpdatedThan(receivedLastConvergedSeqMostUpdatedView) {
						lastConvergedSeq = receivedViewSeqMsg.LastConvergedSeq
					}

					oldMostUpdated := lastProposedSeq.GetMostUpdatedView()
					receivedMostUpdated := receivedViewSeqMsg.ProposedSeq.GetMostUpdatedView()

					updates := append(oldMostUpdated.GetEntries(), receivedMostUpdated.GetEntries()...)
					auxView := view.NewWithUpdates(updates...)

					newProposeSeq = append(lastConvergedSeq, auxView)
					break
				} else {
					newProposeSeq = append(lastProposedSeq, receivedViewSeqMsg.ProposedSeq...)
				}

				viewSeqMsg := ViewSeqMsg{}
				viewSeqMsg.AssociatedView = associatedView
				viewSeqMsg.ProposedSeq = newProposeSeq
				viewSeqMsg.LastConvergedSeq = lastConvergedSeq

				go broadcastViewSequence(associatedView.NewCopy(), viewSeqMsg)

				lastProposedSeq = newProposeSeq
			}

			// Quorum check
			if viewSeqQuorumCounter.count(&receivedViewSeqMsg, associatedView.QuorumSize()) {
				newConvergedSeq := receivedViewSeqMsg.ProposedSeq

				log.Printf("New Converged Seq: %v\n", newConvergedSeq)
				lastConvergedSeq = newConvergedSeq

				seqConvMsg := SeqConvMsg{}
				seqConvMsg.AssociatedView = associatedView
				seqConvMsg.Seq = newConvergedSeq

				// Send seq-conv to all
				go broadcastViewSequenceConv(associatedView.NewCopy(), seqConvMsg)
			}

		case *SeqConv:
			receivedSeqConvMsg := jobPointer
			log.Println("new SeqConvMsg:", receivedSeqConvMsg)

			// Quorum check
			if seqConvQuorumCounter.count(receivedSeqConvMsg, associatedView.QuorumSize()) {
				generatedViewSeqChan <- generatedViewSeq{ViewSeq: receivedSeqConvMsg.Seq, AssociatedView: associatedView}
			}
		default:
			log.Fatalln("Something is wrong with the switch statement")
		}

	}
}

// assertOnlyUpdatedViews exits the program if any view from seq is less updated than view.
func assertOnlyUpdatedViews(baseView *view.View, seq ViewSeq) {
	for _, loopView := range seq {
		if loopView.LessUpdatedThan(baseView) {
			log.Fatalf("BUG! Found an old view in view sequence %v: %v\n", seq, loopView)
		}
	}
}

// we can change seq
func generateViewSequenceWithoutConsensus(associatedView *view.View, seq ViewSeq) {
	log.Println("Running generateViewSequenceWithoutConsensus")
	assertOnlyUpdatedViews(associatedView, seq)

	_ = getOrCreateViewGenerator(associatedView, seq)
}

func generateViewSequenceWithConsensus(associatedView *view.View, seq ViewSeq) {
	log.Println("start generateViewSequenceWithConsensus")
	assertOnlyUpdatedViews(associatedView, seq)

	if associatedView.GetProcessPosition(thisProcess) == LEADER_PROCESS_POSITION {
		consensus.Propose(associatedView, thisProcess, &seq)
	}
	log.Println("Waiting for consensus resolution")
	value := <-consensus.GetConsensusResultChan(associatedView)

	result, ok := value.(*ViewSeq)
	if !ok {
		log.Fatalf("FATAL: consensus on generateViewSequenceWithConsensus got %T %v\n", value, value)
	}
	log.Println("Consensus result received")

	generatedViewSeqChan <- generatedViewSeq{*result, associatedView}
}

type viewSeqQuorumCounterType struct {
	list    []*ViewSeqMsg
	counter []int
}

func (quorumCounter *viewSeqQuorumCounterType) count(newViewSeqMsg *ViewSeqMsg, quorumSize int) bool {
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].Equal(*newViewSeqMsg) {
			quorumCounter.counter[i]++

			return quorumCounter.counter[i] == quorumSize
		}
	}

	quorumCounter.list = append(quorumCounter.list, newViewSeqMsg)
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

// -------- REQUESTS -----------
type SeqConv struct {
	Seq ViewSeq
}

type SeqConvMsg struct {
	AssociatedView *view.View
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

type ViewSeqMsg struct {
	AssociatedView   *view.View
	ProposedSeq      ViewSeq
	LastConvergedSeq ViewSeq
}

func (thisViewSeqMsg ViewSeqMsg) Equal(otherViewSeqMsg ViewSeqMsg) bool {
	if !thisViewSeqMsg.AssociatedView.Equal(otherViewSeqMsg.AssociatedView) {
		return false
	}

	if !thisViewSeqMsg.ProposedSeq.Equal(otherViewSeqMsg.ProposedSeq) {
		return false
	}

	if !thisViewSeqMsg.LastConvergedSeq.Equal(otherViewSeqMsg.LastConvergedSeq) {
		return false
	}

	return true
}

type ViewGeneratorRequest int

func (r *ViewGeneratorRequest) ProposeSeqView(arg ViewSeqMsg, reply *error) error {
	vgi := getOrCreateViewGenerator(arg.AssociatedView, nil)
	vgi.jobChan <- arg

	return nil
}

func (r *ViewGeneratorRequest) SeqConv(arg SeqConvMsg, reply *error) error {
	vgi := getOrCreateViewGenerator(arg.AssociatedView, nil)
	vgi.jobChan <- &arg.SeqConv

	return nil
}

func init() {
	rpc.Register(new(ViewGeneratorRequest))
}

// -------- Broadcast functions -----------
func broadcastViewSequence(destinationView *view.View, viewSeq ViewSeqMsg) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult error
			errorChan <- comm.SendRPCRequest(process, "ViewGeneratorRequest.ProposeSeqView", viewSeq, &discardResult)
		}(process)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan
		if err != nil {
			failedTotal++
			if failedTotal > destinationView.F() {
				log.Fatalln("Failed to send ProposeSeqView to a quorum")
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}

func broadcastViewSequenceConv(destinationView *view.View, seqConv SeqConvMsg) {
	errorChan := make(chan error, destinationView.N())

	for _, process := range destinationView.GetMembers() {
		go func(process view.Process) {
			var discardResult error
			errorChan <- comm.SendRPCRequest(process, "ViewGeneratorRequest.SeqConv", seqConv, &discardResult)
		}(process)
	}

	failedTotal := 0
	successTotal := 0
	for {
		err := <-errorChan
		if err != nil {
			failedTotal++
			if failedTotal > destinationView.F() {
				log.Fatalln("Failed to SeqConv to a quorum")
			}
		}
		successTotal++
		if successTotal == destinationView.QuorumSize() {
			return
		}
	}
}
