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
		var times []int64

		log.Println("Start size: ", size)

		data := make([]byte, size)

		n, err := io.ReadFull(rand.Reader, data)
		if n != len(data) || err != nil {
			log.Fatalln("error to generate data:", err)
			return
		}

		for i := 0; i < 1000; i++ {

			startWrite := time.Now()
			err = client.Write(data)
			endWrite := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			times = append(times, endWrite.Sub(startWrite).Nanoseconds())
			time.Sleep(300 * time.Millisecond)
		}
		saveTime(times, size)
	}
}

func saveTime(times []int64, size int) {
	file, err := os.Create(fmt.Sprintf("/home/mateus/write-latency-%v.txt", size))
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	for _, t := range times {
		if _, err := w.Write([]byte(fmt.Sprintf("%d\n", t))); err != nil {
			log.Fatalln(err)
		}
	}
}
