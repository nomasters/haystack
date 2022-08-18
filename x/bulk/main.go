package main

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/nomasters/haystack"
	"github.com/nomasters/haystack/needle"
)

type task struct {
	mu      *sync.Mutex
	wg      *sync.WaitGroup
	counter *int
	payload []byte
}

var procs = runtime.NumCPU()

func main() {

	client, err := haystack.NewClient("127.0.0.1:1337")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer client.Close()

	counter := 0

	var wg sync.WaitGroup
	var mu sync.Mutex

	taskChan := make(chan task, procs*64)

	for i := 0; i < procs; i++ {
		go worker(taskChan, client)
	}

	reqCount := 2
	randReq := make([][]byte, reqCount)

	for i := 0; i < reqCount; i++ {
		p := make([]byte, 160)
		rand.Read(p)
		n, err := needle.New(p)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("%x\n", n.Bytes())
		randReq[i] = n.Bytes()

	}

	// hash, _ := hex.DecodeString("b4c2d91741ae9e73141e58169141ce0b45b61855e5185b9ae308779dd9720788")
	// fullNeedle, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f1410940e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	t1 := time.Now()

	for i := 0; i < reqCount; i++ {
		wg.Add(1)
		taskChan <- task{
			payload: randReq[i],
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

func worker(job chan task, client *haystack.Client) {
	for {
		processJob(<-job, client)
	}
}

func processJob(j task, client *haystack.Client) {
	defer j.wg.Done()
	n, err := needle.FromBytes(j.payload)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = client.Set(n)
	if err == nil {
		j.mu.Lock()
		*j.counter++
		j.mu.Unlock()
	} else {
		fmt.Printf("%v\n", err)
	}
}
