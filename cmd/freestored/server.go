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
	"strconv"
	"strings"

	"github.com/mateusbraga/freestore/pkg/server"
	"github.com/mateusbraga/freestore/pkg/view"

	"net/http"
	_ "net/http/pprof"
)

// Flags
var (
	useConsensus    = flag.Bool("consensus", false, "Set consensus to use consensus on reconfiguration")
	numberOfServers = flag.Int("n", 3, "Number of servers in the initial view")

	bindAddr string
)

// Set remaining flags
func init() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	switch {
	case hostname == "MateusPc" || hostname == "bt": // dev environment
		flag.StringVar(&bindAddr, "bind", "[::]:5000", "Set this process address")

	case strings.Contains(hostname, "node-"): // emulab.net
		node, err := strconv.ParseInt(hostname[5:strings.Index(hostname, ".")], 10, 0)
		if err != nil {
			log.Fatalln(err)
		}

		flag.StringVar(&bindAddr, "bind", fmt.Sprintf("10.1.1.%v:5000", node+1), "Set this process address")

	default:
		log.Fatalln("invalid hostname:", hostname)
	}
}

func init() {
	// Allow multiprocessing
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	flag.Parse()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	initialView := view.New()
	switch {
	case hostname == "MateusPc" || hostname == "bt": // dev environment
		for i := 0; i < *numberOfServers; i++ {
			process := view.Process{fmt.Sprintf("[::]:500%d", i)}
			initialView.AddUpdate(view.Update{view.Join, process})
		}

	case strings.Contains(hostname, "node-"): // emulab.net
		for i := 0; i < *numberOfServers; i++ {
			process := view.Process{fmt.Sprintf("10.1.1.%d:5000", i+2)}
			initialView.AddUpdate(view.Update{view.Join, process})
		}

	default:
		log.Fatalln("invalid hostname:", hostname)
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	server.Run(bindAddr, initialView, *useConsensus)
}
