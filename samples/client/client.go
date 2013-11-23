/*
This is a simple client
*/
package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"mateusbraga/freestore/client"
)

func main() {
	sizes := []int{1, 256, 512, 1024}

	for _, size := range sizes {
		times := make([]time.Duration, 1000)

		data := make([]byte, size)

		n, err := io.ReadFull(rand.Reader, data)
		if n != len(data) || err != nil {
			log.Fatalln("error to generate data:", err)
			return
		}

		err = client.Write(data)
		if err != nil {
			log.Fatalln(err)
		}

		for {

			startRead := time.Now()
			_, err = client.Read()
			endRead := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			//fmt.Printf("Read %v (%v)-> Write (%v)\n", finalValue, endRead.Sub(startRead), endWrite.Sub(startWrite))
			times = append(times, endRead.Sub(startRead))
			time.Sleep(300 * time.Millisecond)
		}
		saveTime(times, size)
	}
}

func saveTime(times []time.Duration, size int) {
	file, err := os.Create(fmt.Sprintf("/home/mateus/read-latency-%v"))
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	for _, t := range times {
		if _, err := w.Write([]byte(fmt.Sprintf("%v\n", t))); err != nil {
			log.Fatalln(err)
		}
	}
}
