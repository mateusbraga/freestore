/*
This is a Quorum client
*/
package main

import (
	"fmt"
	"log"
	"time"

	"mateusbraga/gotf/freestore/frontend"
)

func main() {
	var finalValue int
	var err error

	for {
		startRead := time.Now()
		finalValue, err = frontend.Read()
		endRead := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		startWrite := time.Now()
		err = frontend.Write(finalValue + 1)
		endWrite := time.Now()
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Printf("Read %v (%v)-> Write (%v)\n", finalValue, endRead.Sub(startRead), endWrite.Sub(startWrite))
		time.Sleep(1 * time.Second)
	}
}
