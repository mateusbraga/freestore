/*
Freestore_client is a sample implementation of a freestore client.
*/
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/mateusbraga/freestore/pkg/client"
)

func main() {
	var finalValue interface{}
	var err error

	for {
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
		time.Sleep(300 * time.Millisecond)
	}
}
