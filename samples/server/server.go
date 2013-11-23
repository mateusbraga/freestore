package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"mateusbraga/freestore/server"
)

//var port uint
var join bool
var master string
var bindAddr string
var useConsensus bool

func init() {
	flag.BoolVar(&join, "join", true, "Set join to join current view automatically")
	flag.BoolVar(&useConsensus, "consensus", false, "Set consensus to use consensus on reconfiguration")

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalln(err)
	}

	if hostname == "MateusPc" {
		flag.StringVar(&bindAddr, "bind", "[::]:5000", "Set this process address")
		flag.StringVar(&bindAddr, "b", "[::]:5000", "Set this process address")
		flag.StringVar(&master, "master", "[::]:5000", "Set process to get first current view")
	} else {
		if strings.Contains(hostname, "node-") {
			node, err := strconv.ParseInt(hostname[5:strings.Index(hostname, ".")], 10, 0)
			if err != nil {
				log.Fatalln(err)
			}

			flag.StringVar(&bindAddr, "bind", fmt.Sprintf("10.1.1.%v:5000", node+1), "Set this process address")
			flag.StringVar(&bindAddr, "b", fmt.Sprintf("10.1.1.%v:5000", node+1), "Set this process address")
		} else {
			log.Fatalln("invalid hostname:", hostname)
		}

		flag.StringVar(&master, "master", "10.1.1.2:5000", "Set process to get first current view")
	}
}

func main() {
	flag.Parse()

	server.Run(bindAddr, join, master, useConsensus)
}
