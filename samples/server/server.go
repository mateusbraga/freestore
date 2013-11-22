package main

import (
	"flag"

	"mateusbraga/freestore/server"
)

//var port uint
var join bool
var master string
var bindAddr string
var useConsensus bool

func init() {
	//flag.UintVar(&port, "port", 0, "Set port to listen to. Default is a random port")
	//flag.UintVar(&port, "p", 0, "Set port to listen to. Default is a random port")
	flag.BoolVar(&join, "join", true, "Set join to join current view automatically")
	flag.BoolVar(&useConsensus, "consensus", false, "Set consensus to use consensus on reconfiguration")
	flag.StringVar(&master, "master", "[::]:5000", "Set process to get first current view")
	flag.StringVar(&bindAddr, "bind", "[::]:5000", "Set this process address")
	flag.StringVar(&bindAddr, "b", "[::]:5000", "Set this process address")
}

func main() {
	flag.Parse()

	server.Run(bindAddr, join, master, useConsensus)
}
