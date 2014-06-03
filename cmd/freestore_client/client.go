// Command freestore_client runs a sample implementation of a freestore client.
//
// Most of the work is done at github.com/mateusbraga/freestore/pkg/client.
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

func main() {
	nTotal := flag.Uint64("n", math.MaxUint64, "number of times to perform a read and write operation")
	initialProcess := flag.String("initial", "", "Process to ask for the initial view")
	retryProcess := flag.String("retry", "", "Process to ask for a newer view")
	flag.Parse()

	freestoreClient, err := client.New(getInitialViewFunc(*initialProcess), getFurtherViewsFunc(*retryProcess))
	if err != nil {
		log.Fatalln("FATAL:", err)
	}

	var finalValue interface{}
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

func getInitialViewFunc(initialProc string) func() (*view.View, error) {
	return func() (*view.View, error) {
		hostname, err := os.Hostname()
		if err != nil {
			log.Panicln(err)
		}

		if initialProc == "" {
			switch {
			case strings.Contains(hostname, "node-"): // emulab.net
				updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"10.1.1.2:5000"}},
					view.Update{Type: view.Join, Process: view.Process{"10.1.1.3:5000"}},
					view.Update{Type: view.Join, Process: view.Process{"10.1.1.4:5000"}},
				}
				return view.NewWithUpdates(updates...), nil
			default:
				updates := []view.Update{view.Update{Type: view.Join, Process: view.Process{"[::]:5000"}},
					view.Update{Type: view.Join, Process: view.Process{"[::]:5001"}},
					view.Update{Type: view.Join, Process: view.Process{"[::]:5002"}},
				}
				return view.NewWithUpdates(updates...), nil
			}
		} else {
			process := view.Process{initialProc}
			initialView, err := client.GetCurrentView(process)
			if err != nil {
				log.Fatalf("Failed to get current view from process %v: %v\n", process, err)
			}
			return initialView, nil
		}
	}
}

func getFurtherViewsFunc(retryProcess string) func() (*view.View, error) {
	return func() (*view.View, error) {
		view, err := client.GetCurrentView(view.Process{retryProcess})
		if err != nil {
			return nil, err
		}

		return view, nil
	}
}
