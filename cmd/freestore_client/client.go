/*
Freestore_client is a sample implementation of a freestore client.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/mateusbraga/freestore/pkg/client"

	"net/http"
	_ "net/http/pprof"
)

var (
	nTotal = flag.Uint64("n", math.MaxUint64, "number of times to perform a read and write operation")
)

func main() {
	var finalValue interface{}
	var err error

	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	for i := uint64(0); i < *nTotal; i++ {
		startRead := time.Now()
		finalValue, err = client.Read()
		endRead := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		startWrite := time.Now()
		err = client.Write(finalValue)
		endWrite := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Printf("Read %v (%v)-> Write (%v)\n", finalValue, endRead.Sub(startRead), endWrite.Sub(startWrite))
	}
}
