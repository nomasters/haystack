package main

import (
	"bufio"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"
)

type task struct {
	mu      *sync.Mutex
	wg      *sync.WaitGroup
	counter *int
	payload []byte
}

func main() {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:1337")
	if err != nil {
		fmt.Println(err)
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	counter := 0

	var wg sync.WaitGroup
	var mu sync.Mutex

	taskChan := make(chan task)

	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(taskChan, conn)
	}

	t1 := time.Now()

	reqCount := 100000

	for i := 0; i < reqCount; i++ {
		taskChan <- task{
			payload: make([]byte, 480),
			mu:      &mu,
			wg:      &wg,
			counter: &counter,
		}
	}
	wg.Wait()
	t2 := time.Now()
	d := t2.Sub(t1)
	fmt.Println("count:", counter, float64(reqCount)/d.Seconds())
}

func worker(job chan task, conn *net.UDPConn) {
	for {
		processJob(<-job, conn)
	}
}

func processJob(j task, conn *net.UDPConn) {
	j.wg.Add(1)
	defer j.wg.Done()
	p := make([]byte, 480)
	conn.Write(j.payload)
	_, err := bufio.NewReader(conn).Read(p)
	if err == nil {
		j.mu.Lock()
		*j.counter++
		j.mu.Unlock()
	} else {
		fmt.Printf("Some error %v\n", err)
	}
}
