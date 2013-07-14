/*
This is a Quorum client
*/
package main

import (
	"fmt"
	"log"
	"runtime"

	"mateusbraga/gotf/client/frontend"
)

func main() {
	_ = log.Ldate
	var finalValue int

	fmt.Println(" ---- Start ---- ")
	finalValue = frontend.Read()
	fmt.Println("Final Read value:", finalValue)

	fmt.Println(" ---- Start 2 ---- ")
	frontend.Write(5)

	fmt.Println(" ---- Start 3 ---- ")
	finalValue = frontend.Read()
	fmt.Println("Final Read value:", finalValue)

	runtime.Gosched()
}
