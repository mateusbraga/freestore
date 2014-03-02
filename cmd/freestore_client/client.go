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
	nTotal = flag.Uint64("n", math.MaxUint64, "number of times to perform a read and write operation")
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

	return nil
}
