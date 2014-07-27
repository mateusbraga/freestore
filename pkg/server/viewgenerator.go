package server

import (
	"log"
	"net/rpc"

	"github.com/mateusbraga/freestore/pkg/comm"
	"github.com/mateusbraga/freestore/pkg/consensus"
	"github.com/mateusbraga/freestore/pkg/view"
)

const CONSENSUS_LEADER_PROCESS_POSITION int = 0

type viewGeneratorInstance struct {
	AssociatedView *view.View //Id
	jobChan        chan interface{}
}

func (s *Server) getOrCreateViewGenerator(associatedView *view.View, initialSeq ViewSeq) viewGeneratorInstance {
	s.viewGeneratorsMu.Lock()
	defer s.viewGeneratorsMu.Unlock()

	for _, vgi := range s.viewGenerators {
		if vgi.AssociatedView.Equal(associatedView) {
			return vgi
		}
	}

	// view generator does not exist. Create it.
	vgi := viewGeneratorInstance{
		AssociatedView: associatedView,
		jobChan:        make(chan interface{}, CHANNEL_DEFAULT_SIZE),
	}
	s.viewGenerators = append(s.viewGenerators, vgi)

	workerSeq := initialSeq
	if workerSeq == nil {
		workerSeq = s.getInitialViewSeq()
	}
	go s.viewGeneratorWorker(vgi, workerSeq)

	return vgi
}

func (s *Server) viewGeneratorWorker(vgi viewGeneratorInstance, initialSeq ViewSeq) {
	log.Printf("Starting new viewGeneratorWorker with initialSeq: %v\n", initialSeq)

	associatedView := vgi.AssociatedView
	jobChan := vgi.jobChan

	var lastProposedSeq ViewSeq
	var lastConvergedSeq ViewSeq
	var viewSeqQuorumCounter viewSeqQuorumCounterType
	var seqConvQuorumCounter seqConvQuorumCounterType

	countViewSeq := func(viewSeqMsg ViewSeqMsg) {
		if viewSeqQuorumCounter.count(&viewSeqMsg, associatedView.QuorumSize()) {
			newConvergedSeq := viewSeqMsg.ProposedSeq

			log.Printf("sending newConvergedSeq: %v\n", newConvergedSeq)
			lastConvergedSeq = newConvergedSeq

			seqConvMsg := SeqConvMsg{}
			seqConvMsg.AssociatedView = associatedView
			seqConvMsg.Seq = newConvergedSeq

			// Send seq-conv to all
			go broadcastViewSequenceConv(associatedView, seqConvMsg)
		}
	}

	if len(initialSeq) != 0 {
		// Send viewSeqMsg to all
		viewSeqMsg := ViewSeqMsg{}
		viewSeqMsg.ProposedSeq = initialSeq
		viewSeqMsg.LastConvergedSeq = nil
		viewSeqMsg.AssociatedView = associatedView
		viewSeqMsg.Sender = s.thisProcess

		go broadcastViewSequence(associatedView, viewSeqMsg)
		countViewSeq(viewSeqMsg)

		lastProposedSeq = initialSeq
	}

	for {
		job := <-jobChan
		switch jobPointer := job.(type) {
		case ViewSeqMsg:
			receivedViewSeqMsg := jobPointer
			log.Println("received ViewSeqMsg:", receivedViewSeqMsg)

			var newProposeSeq ViewSeq

			hasChanges := false
			hasConflict := false

		OuterLoop:
			for _, v := range receivedViewSeqMsg.ProposedSeq {
				if lastProposedSeq.HasView(v) {
					continue
				}

				log.Printf("New view discovered: %v\n", v)
				hasChanges = true

				// check if v conflicts with any view from lastProposedSeq
				for _, v2 := range lastProposedSeq {
					if v.LessUpdatedThan(v2) && v2.LessUpdatedThan(v) {
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

					updates := append(oldMostUpdated.GetUpdates(), receivedMostUpdated.GetUpdates()...)
					auxView := view.NewWithUpdates(updates...)

					newProposeSeq = lastConvergedSeq.Append(auxView)
				} else {
					newProposeSeq = lastProposedSeq.Append(receivedViewSeqMsg.ProposedSeq...)
				}
				log.Println("sending newProposeSeq:", newProposeSeq)

				viewSeqMsg := ViewSeqMsg{}
				viewSeqMsg.Sender = s.thisProcess
				viewSeqMsg.AssociatedView = associatedView
				viewSeqMsg.ProposedSeq = newProposeSeq
				viewSeqMsg.LastConvergedSeq = lastConvergedSeq

				go broadcastViewSequence(associatedView, viewSeqMsg)
				countViewSeq(viewSeqMsg)

				lastProposedSeq = newProposeSeq
			}

			// Quorum check
			countViewSeq(receivedViewSeqMsg)

		case *SeqConv:
			receivedSeqConvMsg := jobPointer
			log.Println("received SeqConvMsg:", receivedSeqConvMsg)

			// Quorum check
			if seqConvQuorumCounter.count(receivedSeqConvMsg, associatedView.QuorumSize()) {
				s.generatedViewSeqChan <- generatedViewSeq{ViewSeq: receivedSeqConvMsg.Seq, AssociatedView: associatedView}
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
func (s *Server) generateViewSequenceWithoutConsensus(associatedView *view.View, seq ViewSeq) {
	assertOnlyUpdatedViews(associatedView, seq)

	_ = s.getOrCreateViewGenerator(associatedView, seq)
}

func (s *Server) generateViewSequenceWithConsensus(associatedView *view.View, seq ViewSeq) {
	assertOnlyUpdatedViews(associatedView, seq)

	if associatedView.GetProcessPosition(s.thisProcess) == CONSENSUS_LEADER_PROCESS_POSITION {
		consensus.Propose(associatedView, s.thisProcess, &seq)
	}
	log.Println("Waiting for consensus resolution")
	value := <-consensus.GetConsensusResultChan(associatedView)

	// get startReconfigurationTime to compute reconfiguration duration
	//if startReconfigurationTime.IsZero() || startReconfigurationTime.Sub(time.Now()) > 20*time.Second {
	//startReconfigurationTime = consensus.GetConsensusStartTime(associatedView)
	//log.Println("starttime :", startReconfigurationTime)
	//}

	result, ok := value.(*ViewSeq)
	if !ok {
		log.Fatalf("FATAL: consensus on generateViewSequenceWithConsensus got %T %v\n", value, value)
	}
	log.Println("Consensus result received")

	s.generatedViewSeqChan <- generatedViewSeq{
		AssociatedView: associatedView,
		ViewSeq:        *result,
	}
}

type viewSeqQuorumCounterType struct {
	list    []*ViewSeqMsg
	counter []int
}

func (quorumCounter *viewSeqQuorumCounterType) count(newViewSeqMsg *ViewSeqMsg, quorumSize int) bool {
	for i, _ := range quorumCounter.list {
		if quorumCounter.list[i].SameButDifferentSender(*newViewSeqMsg) {
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
	Sender           view.Process
	AssociatedView   *view.View
	ProposedSeq      ViewSeq
	LastConvergedSeq ViewSeq
}

func (thisViewSeqMsg ViewSeqMsg) SameButDifferentSender(otherViewSeqMsg ViewSeqMsg) bool {
	if thisViewSeqMsg.Sender == otherViewSeqMsg.Sender {
		return false
	}

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

func (r *ViewGeneratorRequest) ProposeSeqView(arg ViewSeqMsg, reply *struct{}) error {
	vgi := globalServer.getOrCreateViewGenerator(arg.AssociatedView, nil)
	vgi.jobChan <- arg

	return nil
}

func (r *ViewGeneratorRequest) SeqConv(arg SeqConvMsg, reply *struct{}) error {
	vgi := globalServer.getOrCreateViewGenerator(arg.AssociatedView, nil)
	vgi.jobChan <- &arg.SeqConv

	return nil
}

func init() { rpc.Register(new(ViewGeneratorRequest)) }

// -------- Broadcast functions -----------
func broadcastViewSequence(destinationView *view.View, viewSeqMsg ViewSeqMsg) {
	destinationViewWithoutThisProcess := destinationView.NewCopyWithUpdates(view.Update{Type: view.Leave, Process: globalServer.thisProcess})
	comm.BroadcastRPCRequest(destinationViewWithoutThisProcess, "ViewGeneratorRequest.ProposeSeqView", viewSeqMsg)
}

func broadcastViewSequenceConv(destinationView *view.View, seqConvMsg SeqConvMsg) {
	comm.BroadcastRPCRequest(destinationView, "ViewGeneratorRequest.SeqConv", seqConvMsg)
}
