// Freestored runs a sample implementation of a freestore server.
//
// Most of the work is done at github.com/mateusbraga/freestore/pkg/server.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/mateusbraga/freestore/pkg/client"
	"github.com/mateusbraga/freestore/pkg/server"
	"github.com/mateusbraga/freestore/pkg/view"

	"net/http"
	_ "net/http/pprof"
)

// Flags
var (
	useConsensus    = flag.Bool("consensus", false, "Set consensus to use consensus on reconfiguration")
	numberOfServers = flag.Int("n", 3, "Number of servers in the initial view")
	bindAddr        = flag.String("bind", "[::]:5000", "Set this process address")
)

func init() {
	// Make it parallel
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	flag.Parse()

	initialView := getInitialView()

	go func() {
		log.Println("Running pprof:", http.ListenAndServe("localhost:6060", nil))
	}()

	server.Run(*bindAddr, initialView, *useConsensus)
}

func getInitialView() *view.View {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	switch {
	case strings.Contains(hostname, "node-"): // emulab.net
		for i := 0; i < 7; i++ {
			process := view.Process{fmt.Sprintf("10.1.1.%d:5000", i+2)}

			initialView, err := client.GetCurrentView(process)
			if err != nil {
				log.Printf("Failed to get current view of process %v: %v\n", process, err)
				continue
			}

			return initialView
		}
	default:
		for i := 0; i < 7; i++ {
			process := view.Process{fmt.Sprintf("[::]:500%v", i)}

			initialView, err := client.GetCurrentView(process)
			if err != nil {
				log.Printf("Failed to get current view of process %v: %v\n", process, err)
				continue
			}

			return initialView
		}
	}

	var initialView *view.View
	switch {
	case strings.Contains(hostname, "node-"): // emulab.net
		updates := []view.Update{}
		for i := 0; i < *numberOfServers; i++ {
			process := view.Process{fmt.Sprintf("10.1.1.%d:5000", i+2)}
			updates = append(updates, view.Update{view.Join, process})
		}
		initialView = view.NewWithUpdates(updates...)

	default:
		updates := []view.Update{}
		for i := 0; i < *numberOfServers; i++ {
			process := view.Process{fmt.Sprintf("[::]:500%d", i)}
			updates = append(updates, view.Update{view.Join, process})
		}
		initialView = view.NewWithUpdates(updates...)
	}
	return initialView
}
