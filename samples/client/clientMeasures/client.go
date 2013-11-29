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
	sleepDuration      time.Duration
	isWrite            bool
	size               int
	numberOfOperations int
	measureLatency     bool
	totalDuration      time.Duration
)

var (
	latencies []int64
	i         int
	stopChan  <-chan time.Time
)

func init() {
	flag.DurationVar(&sleepDuration, "sleep", 400*time.Millisecond, "Duration to sleep between operations")
	flag.DurationVar(&totalDuration, "duration", 10*time.Second, "Duration to run operations")
	flag.BoolVar(&isWrite, "write", false, "Client will measure write operations")
	flag.IntVar(&size, "size", 1, "The size of the data being transfered")
	flag.IntVar(&numberOfOperations, "n", 1000, "Number of operations to perform")
	flag.BoolVar(&measureLatency, "latency", false, "Client will measure latency")

	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	flag.Parse()

	stopChan = time.After(totalDuration)

	if measureLatency {
		latency()
	} else {
		throughput()
	}
}

func latency() {
	if isWrite {
		log.Printf("Measuring latency of write operations with size %vB every %v\n", size, sleepDuration)
	} else {
		log.Printf("Measuring latency of read operations with size %vB every %v\n", size, sleepDuration)
	}

	data := createFakeData()

	if isWrite {
		for i = 0; i < numberOfOperations; i++ {
			startTime := time.Now()
			err := client.Write(data)
			endTime := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			latencies = append(latencies, endTime.Sub(startTime).Nanoseconds())
			//time.Sleep(sleepDuration)
		}
	} else {
		err := client.Write(data)
		if err != nil {
			log.Fatalln("Initial write:", err)
		}

		for i = 0; i < numberOfOperations; i++ {
			startTime := time.Now()
			_, err = client.Read()
			endTime := time.Now()
			if err != nil {
				log.Fatalln(err)
			}

			latencies = append(latencies, endTime.Sub(startTime).Nanoseconds())
			//time.Sleep(sleepDuration)
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
		if _, err := w.Write([]byte(fmt.Sprintf("%d\n", t))); err != nil {
			log.Fatalln(err)
		}
	}
}

func throughput() {
	defer saveThroughputTimes()

	if isWrite {
		log.Printf("Measuring throughput of write operations with size %vB every %v\n", size, sleepDuration)
	} else {
		log.Printf("Measuring throughput of read operations with size %vB every %v\n", size, sleepDuration)
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
				return
			default:
				//time.Sleep(sleepDuration)
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
				return
			default:
				//time.Sleep(sleepDuration)
			}
		}
	}

}

func saveThroughputTimes() {
	fmt.Printf("%v operations done in %v\n", i, totalDuration)
	//file, err := os.Create(fmt.Sprintf("/home/mateus/throughput-%v.txt", size))
	//if err != nil {
	//log.Fatalln(err)
	//}
	//defer file.Close()

	//w := bufio.NewWriter(file)
	//defer w.Flush()

	//for _, stats := range serverStats {
	//if _, err := w.Write([]byte(fmt.Sprintf("%v %v %v\n", stats.Process, stats.NumberOfOperations, stats.Duration.Seconds()))); err != nil {
	//log.Fatalln(err)
	//}
	//}
}

func createFakeData() []byte {
	data := make([]byte, size)

	n, err := io.ReadFull(rand.Reader, data)
	if n != len(data) || err != nil {
		log.Fatalln("error to generate data:", err)
	}
	return data
}
