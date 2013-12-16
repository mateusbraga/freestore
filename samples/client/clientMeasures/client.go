/*
This is a simple client
*/
package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	"mateusbraga/freestore/client"
)

var (
	isWrite            bool
	size               int
	numberOfOperations int
	measureLatency     bool
	measureThroughput  bool
	totalDuration      time.Duration
)

var (
	latencies []int64
	i         int
	stopChan  <-chan time.Time
)

func init() {
	flag.DurationVar(&totalDuration, "duration", 10*time.Second, "Duration to run operations (throughput measurement)")
	flag.IntVar(&numberOfOperations, "n", 1000, "Number of operations to perform (latency measurement)")
	flag.BoolVar(&isWrite, "write", false, "Client will measure write operations")
	flag.IntVar(&size, "size", 1, "The size of the data being transfered")
	flag.BoolVar(&measureLatency, "latency", false, "Client will measure latency")
	flag.BoolVar(&measureThroughput, "throughput", false, "Client will measure throughput")

	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	flag.Parse()

	stopChan = time.After(totalDuration)

	if measureLatency {
		latency()
	} else if measureThroughput {
		throughput()
	} else {
		latencyAndThroughput()
	}
}

func latencyAndThroughput() {
	if isWrite {
		log.Printf("Measuring throughput and latency of write operations with size %vB\n", size)
	} else {
		log.Printf("Measuring throughput and latency of read operations with size %vB\n", size)
	}

	data := createFakeData()

	startTime := time.Now()

	if isWrite {
		for i = 0; i < numberOfOperations; i++ {
			timeBefore := time.Now()
			err := client.Write(data)
			timeAfter := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			latencies = append(latencies, timeAfter.Sub(timeBefore).Nanoseconds())
		}
	} else {
		err := client.Write(data)
		if err != nil {
			log.Fatalln("Initial write:", err)
		}

		for i = 0; i < numberOfOperations; i++ {
			timeBefore := time.Now()
			_, err = client.Read()
			timeAfter := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			latencies = append(latencies, timeAfter.Sub(timeBefore).Nanoseconds())
		}
	}

	endTime := time.Now()
	totalDuration = endTime.Sub(startTime)
	fmt.Printf("%v operations done in %v\n", i, totalDuration)
	saveLatencyTimes()
}

func latency() {
	if isWrite {
		log.Printf("Measuring latency of write operations with size %vB\n", size)
	} else {
		log.Printf("Measuring latency of read operations with size %vB\n", size)
	}

	data := createFakeData()

	if isWrite {
		for i = 0; i < numberOfOperations; i++ {
			timeBefore := time.Now()
			err := client.Write(data)
			timeAfter := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			latencies = append(latencies, timeAfter.Sub(timeBefore).Nanoseconds())
		}
	} else {
		err := client.Write(data)
		if err != nil {
			log.Fatalln("Initial write:", err)
		}

		for i = 0; i < numberOfOperations; i++ {
			timeBefore := time.Now()
			_, err = client.Read()
			timeAfter := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			latencies = append(latencies, timeAfter.Sub(timeBefore).Nanoseconds())
		}
	}

	saveLatencyTimes()
}

func saveLatencyTimes() {
	var filename string
	if isWrite {
		filename = fmt.Sprintf("/home/mateus/write-latency-%v.txt", size)
	} else {
		filename = fmt.Sprintf("/home/mateus/read-latency-%v.txt", size)
	}

	file, err := os.Create(filename)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	for _, t := range latencies {
		if _, err := w.Write([]byte(fmt.Sprintf("%d %v\n", t))); err != nil {
			log.Fatalln(err)
		}
	}
}

func throughput() {
	if isWrite {
		log.Printf("Measuring throughput of write operations with size %vB for %v\n", size, totalDuration)
	} else {
		log.Printf("Measuring throughput of read operations with size %vB for %v\n", size, totalDuration)
	}

	data := createFakeData()

	if isWrite {
		for ; ; i++ {
			err := client.Write(data)
			if err != nil {
				log.Fatalln(err)
			}

			select {
			case <-stopChan:
				fmt.Printf("%v operations done in %v\n", i, totalDuration)
				return
			default:
			}
		}
	} else {
		err := client.Write(data)
		if err != nil {
			log.Fatalln("Initial write:", err)
		}

		for ; ; i++ {
			_, err = client.Read()
			if err != nil {
				log.Fatalln(err)
			}

			select {
			case <-stopChan:
				fmt.Printf("%v operations done in %v\n", i, totalDuration)
				return
			default:
			}
		}
	}

}

func createFakeData() []byte {
	data := make([]byte, size)

	n, err := io.ReadFull(rand.Reader, data)
	if n != len(data) || err != nil {
		log.Fatalln("error to generate data:", err)
	}
	return data
}
