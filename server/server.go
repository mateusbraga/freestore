package main

import (
	"flag"

	"mateusbraga/gotf/server/backend"
)

var port uint
var join bool
var master string

func init() {
	flag.UintVar(&port, "port", 0, "Set port to listen to. Default is a random port")
	flag.UintVar(&port, "p", 0, "Set port to listen to. Default is a random port")
	flag.BoolVar(&join, "join", true, "Set join to join current view automatically")
	flag.StringVar(&master, "master", "[::]:5000", "Set process to get first current view")
}

func main() {
	flag.Parse()

	backend.Run(port, join, master)
}
