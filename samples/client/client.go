/*
This is a simple client
*/
package main

import (
	"fmt"
	"log"
	"time"

	"mateusbraga/gotf/freestore/client"
)

func main() {
	var finalValue int
	var err error

	for {
		startRead := time.Now()
		finalValue, err = client.Read()
		endRead := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		startWrite := time.Now()
		err = client.Write(finalValue + 1)
		endWrite := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Printf("Read %v (%v)-> Write (%v)\n", finalValue, endRead.Sub(startRead), endWrite.Sub(startWrite))
		time.Sleep(1 * time.Second)
	}
}
