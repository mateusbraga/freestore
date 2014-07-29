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

	initialView := getInitialView(*bindAddr, *initialProcess)

	go func() {
		log.Println("Running pprof:", http.ListenAndServe("localhost:6060", nil))
	}()

	freestoreServer, err := server.New(*bindAddr, initialView, *useConsensus)
	if err != nil {
		log.Fatalln(err)
	}
	freestoreServer.Run()
}

func getInitialView(bindAddr string, initialProc string) *view.View {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	if initialProc == "" {
		var updates []view.Update
		switch {
		case strings.Contains(hostname, "node-"): // emulab.net
			updates = []view.Update{
				view.Update{Type: view.Join, Process: view.Process{"10.1.1.2:5000"}},
				view.Update{Type: view.Join, Process: view.Process{"10.1.1.3:5000"}},
				view.Update{Type: view.Join, Process: view.Process{"10.1.1.4:5000"}},
			}
		default:
			updates = []view.Update{
				view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}},
				view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}},
				view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}},
			}
		}
		hardCodedView := view.NewWithUpdates(updates...)

		// don't ask for current view if the process is in the hardCodedView
		if hardCodedView.HasMember(view.Process{bindAddr}) {
			return hardCodedView
		}

		for _, u := range updates {
			v, err := client.GetCurrentView(u.Process)
			if err != nil {
				log.Println("Failed to get current view from process %v: %v\n", u.Process, err)
				continue
			}

			if v.Equal(hardCodedView) {
				return hardCodedView
			} else {
				return v
			}
		}
		log.Fatalln("Fatal: None of the hard coded initial processes are responsive.")
		return nil
	} else {
		process := view.Process{initialProc}
		initialView, err := client.GetCurrentView(process)
		if err != nil {
			log.Fatalf("Failed to get current view from process %v: %v\n", process, err)
		}
		return initialView
	}
}
