package main

import (
	"bufio"
	"encoding/hex"
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

	reqCount := 10000

	// p := make([]byte, 448)
	// rand.Read(p)
	// n, _ := needle.New(p)

	// hash, _ := hex.DecodeString("b4c2d91741ae9e73141e58169141ce0b45b61855e5185b9ae308779dd9720788")
	fullNeedle, _ := hex.DecodeString("b4c2d91741ae9e73141e58169141ce0b45b61855e5185b9ae308779dd9720788ac735574d26e83946d23321d793ea409907c553d55deb3ab6184f5674d94d47578fcbb4b01bb1c3f528677234b50b3d5605cf3981e39a92ac5481034067be083e076bea838793efb005e1863a1aba7339cbd392799eb90cf640449b03cb137511ca06d5188a179bf65dc04c3898dd2899e7b96c7972e814f3fa55dfc927b0caec6d0f2d1fbfcc79a39f0d8e133f201035d1e5808a27a0efb15f3d63f2495339e045cbf5516050fa05f96b495ea92024fb3ae4ab1716c1d6168c7236789f51949362d4b9c4ffbd37a07c10efaf7ffaa7d2aaddcf10fabc56ba6df10334d7c3ee0bf82229b0fee4220c5507fd05d93cb646ec3d1575c42aa51661e9315e7bc370897b1ed70468117a714912914da5a5bd4929b194e0cbba5b5944b4346cf31f21dc4a3b734085833f846d95b682d82e1794e925c692daa7423efeb5b4fc5489c32970e56d12139cd1870b562860e1e8a78af8202ae6288cb391e7112ca20b51d94ccfb08ad8698805662094e9f086f8f9c813eda6c8590da8cadceda3f35c15cada7ac1c890776835a200187780ff2cbd2f126c479df35f4acdbe6c41eca51dd996488ef7096a908e9e9e2eec9c610514a407bec93e93f87c92e06ef0f94b3432f")
	// fmt.Printf("%x\n", hash)
	// fmt.Println("---")
	// fmt.Printf("%x\n", fullNeedle)
	t1 := time.Now()

	for i := 0; i < reqCount; i++ {
		taskChan <- task{
			payload: fullNeedle,
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
