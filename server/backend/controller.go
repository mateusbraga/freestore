package backend

import (
	"log"
	"net/rpc"
	"runtime"

	"mateusbraga/gotf/view"
)

type ControllerRequest int

func (r *ControllerRequest) Terminate(anything bool, reply *bool) error {
	log.Println("Terminating...")
	err := listener.Close()
	if err != nil {
		return err
	}
	runtime.Gosched() // Just to reduce the chance of showing errors because it terminated too early
	return nil
}

func (r *ControllerRequest) Ping(anything bool, reply *bool) error {
	return nil
}

func (r *ControllerRequest) Join(view view.View, reply *bool) error {
	log.Println("ControllerRequest to join view", view)
	currentView.Set(&view)
	Join()
	return nil
}

func (r *ControllerRequest) Leave(anything bool, reply *bool) error {
	log.Println("ControllerRequest to leave view")
	Leave()
	return nil
}

func (r *ControllerRequest) Consensus(arg int, reply *int) error {
	log.Fatalln("fix new consensus!")
	//callbackChan := make(chan interface{})
	//consensus := NewConsensus(callbackChan)

	//consensus.Propose(arg)
	//value := <-callbackChan
	//log.Printf("consensus got %T %v\n", value, value)

	//result, ok := value.(int)
	//if ok {
	//*reply = result
	//}

	return nil
}

func init() {
	controllerRequest := new(ControllerRequest)
	rpc.Register(controllerRequest)
}
