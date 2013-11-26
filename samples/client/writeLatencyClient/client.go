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
	"mateusbraga/freestore/view"
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

		client.StartMeasurements()
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
		serverStats := client.EndMeasurements()

		saveLatencyTimes(times, size)
		saveThroughputTimes(serverStats, size)
	}
}

func saveLatencyTimes(times []int64, size int) {
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

func saveThroughputTimes(serverStats map[view.Process]client.ServerStats, size int) {
	file, err := os.Create(fmt.Sprintf("/home/mateus/throughput-%v.txt", size))
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	for process, stats := range serverStats {
		if _, err := w.Write([]byte(fmt.Sprintf("%v %v %v\n", process, stats.NumberOfOperations, stats.Duration.Seconds()))); err != nil {
			log.Fatalln(err)
		}
	}
}
