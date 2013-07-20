package main

import (
	"flag"

	"mateusbraga/gotf/server/backend"
)

var port uint

func init() {
	flag.UintVar(&port, "port", 0, "Set port to listen to. Default is a random port")
	flag.UintVar(&port, "p", 0, "Set port to listen to. Default is a random port")
}

func main() {
	flag.Parse()

	backend.Run(port)
}
