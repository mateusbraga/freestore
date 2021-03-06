/*
Package server implements a Freestore server.

*/
package server

import (
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/mateusbraga/freestore/pkg/view"
	//TODO count faults, faults masked, errors, failures and put in expvar
)

const (
	CHANNEL_DEFAULT_SIZE = 20
)

var (
	globalServer *Server
)

type Server struct {
	// initialization parameters
	thisProcess  view.Process
	listener     net.Listener
	useConsensus bool

	// register is protected by the mutex register.mu
	register Value

	// currentView of the server
	currentView   *view.View
	currentViewMu sync.RWMutex

	// recv keeps the updates that will be applied to the currentView in the next reconfiguration
	recv      map[view.Update]bool
	recvMutex sync.RWMutex

	// viewGenerators keep the data required to run a view generator
	viewGenerators   []viewGeneratorInstance
	viewGeneratorsMu sync.Mutex

	// registerLockOnce is used lock the register mutex only once when installing a sequence with more than one view (the register is locked when installing a view).
	registerLockOnce sync.Once

	// the channels below is how the "loop" goroutines comunicate with each other.

	generatedViewSeqChan          chan generatedViewSeq
	installSeqProcessingChan      chan InstallSeqMsg
	syncStateMsgChan              chan SyncStateMsg
	stateUpdateChanRequestChan    chan stateUpdateChanRequest
	newViewInstalledChan          chan ViewInstalledMsg
	resetReconfigurationTimerChan chan bool

	// the times below is used to measure the duration of a reconfiguration

	startReconfigurationTime time.Time
	registerLockTime         time.Time
}

// New creates a new server that will listen to bindAddr, use the initialView and use or not consensus when a reconfiguration is required.
func New(bindAddr string, initialView *view.View, useConsensusArg bool) (*Server, error) {
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		listener:                      listener,
		thisProcess:                   view.Process{listener.Addr().String()},
		currentView:                   initialView,
		recv:                          make(map[view.Update]bool),
		generatedViewSeqChan:          make(chan generatedViewSeq),
		installSeqProcessingChan:      make(chan InstallSeqMsg, CHANNEL_DEFAULT_SIZE),
		syncStateMsgChan:              make(chan SyncStateMsg, CHANNEL_DEFAULT_SIZE),
		stateUpdateChanRequestChan:    make(chan stateUpdateChanRequest, CHANNEL_DEFAULT_SIZE),
		newViewInstalledChan:          make(chan ViewInstalledMsg, CHANNEL_DEFAULT_SIZE),
		resetReconfigurationTimerChan: make(chan bool, CHANNEL_DEFAULT_SIZE),
	}
	go s.generatedViewSeqProcessingLoop()
	go s.installSeqProcessingLoop()
	go s.stateUpdateProcessingLoop()
	go s.resetReconfigurationTimerLoop()

	if globalServer != nil {
		log.Panicln("Tried to create a second Server")
	}
	globalServer = s

	s.currentViewMu.Lock()
	defer s.currentViewMu.Unlock()

	// register starts locked if it is not in the current view
	if !s.currentView.HasMember(s.thisProcess) {
		s.register.mu.Lock()
		// ask to join the view
		s.joinLocked()
	}

	return s, nil
}

func (s *Server) Run() {
	// Accept connections forever
	log.Println("Listening on address:", s.listener.Addr())
	rpc.Accept(s.listener)
}
