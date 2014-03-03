/*
Freestore_client is a sample implementation of a freestore client.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mateusbraga/freestore/pkg/client"
	"github.com/mateusbraga/freestore/pkg/view"
)

var (
	nTotal         = flag.Uint64("n", math.MaxUint64, "number of times to perform a read and write operation")
	initialProcess = flag.String("initial", "", "Process to ask for the initial view")
)

func main() {
	flag.Parse()

	initialView := getInitialView()
	freestoreClient := client.New(initialView)

	var finalValue interface{}
	var err error
	for i := uint64(0); i < *nTotal; i++ {
		startRead := time.Now()
		finalValue, err = freestoreClient.Read()
		endRead := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		startWrite := time.Now()
		err = freestoreClient.Write(finalValue)
		endWrite := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		if i%1000 == 0 {
			fmt.Printf("%v: Read %v (%v)-> Write (%v)\n", i, finalValue, endRead.Sub(startRead), endWrite.Sub(startWrite))
		} else {
			fmt.Printf(".")
		}
	}
}

func getInitialView() *view.View {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	if *initialProcess == "" {
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
		process := view.Process{*initialProcess}
		initialView, err := client.GetCurrentView(process)
		if err != nil {
			log.Fatalf("Failed to get current view from process %v: %v\n", process, err)
		}
		return initialView
	}
}
