// Command freestored runs a sample implementation of a freestore server.
//
// Most of the work is done at github.com/mateusbraga/freestore/pkg/server.
package main

import (
	"flag"
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

func init() {
	// Make it parallel
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	useConsensus := flag.Bool("consensus", false, "Set consensus to use consensus on reconfiguration")
	bindAddr := flag.String("bind", "[::]:5000", "Set this process address")
	initialProcess := flag.String("initial", "", "Process to ask for the initial view")
	flag.Parse()

	initialView := getInitialView(*initialProcess)

	go func() {
		log.Println("Running pprof:", http.ListenAndServe("localhost:6060", nil))
	}()

	server.Run(*bindAddr, initialView, *useConsensus)
}

func getInitialView(initialProc string) *view.View {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	if initialProc == "" {
		switch {
		case strings.Contains(hostname, "node-"): // emulab.net
			updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"10.1.1.2:5000"}},
				view.Update{Type: view.Join, Process: view.Process{"10.1.1.3:5000"}},
				view.Update{Type: view.Join, Process: view.Process{"10.1.1.4:5000"}},
			}
			return view.NewWithUpdates(updates...)
		default:
			updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}},
				view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}},
				view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}},
			}
			return view.NewWithUpdates(updates...)
		}
	} else {
		process := view.Process{initialProc}
		initialView, err := client.GetCurrentView(process)
		if err != nil {
			log.Fatalf("Failed to get current view from process %v: %v\n", process, err)
		}
		return initialView
	}
}
