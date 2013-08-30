/*
This is a Quorum client
*/
package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"mateusbraga/gotf/client/frontend"
)

func main() {
	_ = log.Ldate
	var finalValue int

	for {
		startRead := time.Now()
		finalValue = frontend.Read()
		endRead := time.Now()
		startWrite := time.Now()
		frontend.Write(finalValue + 1)
		endWrite := time.Now()
		fmt.Printf("Read %v (%v)-> Write (%v)\n", finalValue, endRead.Sub(startRead), endWrite.Sub(startWrite))
		time.Sleep(1 * time.Second)
	}
	//fmt.Println(" ---- Start ---- ")
	//finalValue = frontend.Read()
	//fmt.Println("Final Read value:", finalValue)

	//fmt.Println(" ---- Start 2 ---- ")
	//frontend.Write(5)

	//fmt.Println(" ---- Start 3 ---- ")
	//finalValue = frontend.Read()
	//fmt.Println("Final Read value:", finalValue)

	runtime.Gosched()
}
