// Freestore_measures is a client that measures latency and throughput of freestore.
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

	"github.com/mateusbraga/freestore/pkg/client"
)

var (
	isWrite            = flag.Bool("write", false, "Client will measure write operations")
	size               = flag.Int("size", 1, "The size of the data being transfered")
	numberOfOperations = flag.Int("n", 1000, "Number of operations to perform (latency measurement)")
	measureLatency     = flag.Bool("latency", false, "Client will measure latency")
	measureThroughput  = flag.Bool("throughput", false, "Client will measure throughput")
	totalDuration      = flag.Duration("duration", 10*time.Second, "Duration to run operations (throughput measurement)")
)

var (
	latencies []int64
	ops       int
	stopChan  <-chan time.Time
)

func init() {
	// Make it parallel
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	flag.Parse()

	stopChan = time.After(*totalDuration)

	if *measureLatency {
		latency()
	} else if *measureThroughput {
		throughput()
	} else {
		latencyAndThroughput()
	}
}

func latencyAndThroughput() {
	if *isWrite {
		log.Printf("Measuring throughput and latency of write operations with size %vB\n", *size)
	} else {
		log.Printf("Measuring throughput and latency of read operations with size %vB\n", *size)
	}

	data := createFakeData()

	startTime := time.Now()

	if *isWrite {
		for ops = 0; ops < *numberOfOperations; ops++ {
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

		for ops = 0; ops < *numberOfOperations; ops++ {
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
	duration := endTime.Sub(startTime)
	fmt.Printf("Partial throughput: %v operations done in %v\n", ops, duration)
	saveLatencyTimes()
}

func latency() {
	if *isWrite {
		log.Printf("Measuring latency of write operations with size %vB\n", *size)
	} else {
		log.Printf("Measuring latency of read operations with size %vB\n", *size)
	}

	data := createFakeData()

	if *isWrite {
		for ops = 0; ops < *numberOfOperations; ops++ {
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

		for ops = 0; ops < *numberOfOperations; ops++ {
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
	if *isWrite {
		filename = fmt.Sprintf("/home/mateus/write-latency-%v-%v.txt", *size, time.Now().Format(time.RFC3339))
	} else {
		filename = fmt.Sprintf("/home/mateus/read-latency-%v-%v.txt", *size, time.Now().Format(time.RFC3339))
	}

	file, err := os.Create(filename)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	for _, t := range latencies {
		if _, err := w.Write([]byte(fmt.Sprintf("%v\n", t))); err != nil {
			log.Fatalln(err)
		}
	}
}

func throughput() {
	if *isWrite {
		log.Printf("Measuring throughput of write operations with size %vB for %v\n", *size, *totalDuration)
	} else {
		log.Printf("Measuring throughput of read operations with size %vB for %v\n", *size, *totalDuration)
	}

	data := createFakeData()

	if *isWrite {
		for ; ; ops++ {
			err := client.Write(data)
			if err != nil {
				log.Fatalln(err)
			}

			select {
			case <-stopChan:
				fmt.Printf("%v operations done in %v\n", ops, *totalDuration)
				return
			default:
			}
		}
	} else {
		err := client.Write(data)
		if err != nil {
			log.Fatalln("Initial write:", err)
		}

		for ; ; ops++ {
			_, err = client.Read()
			if err != nil {
				log.Fatalln(err)
			}

			select {
			case <-stopChan:
				fmt.Printf("%v operations done in %v\n", ops, *totalDuration)
				return
			default:
			}
		}
	}

}

func createFakeData() []byte {
	data := make([]byte, *size)

	n, err := io.ReadFull(rand.Reader, data)
	if n != len(data) || err != nil {
		log.Fatalln("error to generate data:", err)
	}
	return data
}
