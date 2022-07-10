package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/nomasters/haystack/needle"
	"github.com/nomasters/haystack/storage"
	"github.com/nomasters/haystack/storage/memory"
)

// Server is a struct that contains all the settings required for a haystack server
type Server struct {
	Address  string
	TTL      uint64
	Protocol string
	Storage  storage.Storage
	Workers  int
}

type request struct {
	body []byte
	addr *net.UDPAddr
}

// New returns a reference to a new Server struct
func New() (*Server, error) {
	memoryStore := memory.New(10*time.Second, 2000)

	s := Server{
		Address:  ":1337",
		TTL:      500,
		Protocol: "udp",
		Workers:  runtime.NumCPU(),
		Storage:  memoryStore,
	}
	return &s, nil
}

// Run initiates and runs the haystack server and returns an error.
func (s *Server) Run() error {
	addr, err := net.ResolveUDPAddr(s.Protocol, s.Address)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP(s.Protocol, addr)
	if err != nil {
		return err
	}
	reqChan := make(chan *request, 1000000)
	go newListener(conn, reqChan)

	ctx, cancel := context.WithCancel(context.Background())
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt)

	defer func() {
		signal.Stop(stopSig)
	}()

	doneChan := make(chan struct{}, s.Workers)

	for i := 0; i < s.Workers; i++ {
		go worker(ctx, s.Storage, conn, reqChan, doneChan)
	}

	<-stopSig
	gracefulShutdown(cancel, doneChan, s.Workers)
	return nil
}

func newListener(conn *net.UDPConn, reqChan chan<- *request) {
	buffer := make([]byte, needle.NeedleLength+1)

	for {
		n, radder, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("read error: %v", err)
		}
		if n == needle.NeedleLength || n == needle.HashLength {
			reqChan <- &request{body: buffer[:n], addr: radder}
		} else {
			log.Println("invalid length", n)
		}
	}
}

func gracefulShutdown(cancel context.CancelFunc, done <-chan struct{}, expected int) {
	cancel()
	complete := false
	go func() {
		time.Sleep(2 * time.Second)
		if !complete {
			log.Println("failed to gracefully exit")
			os.Exit(1)
		}
	}()

	for i := 0; i < expected; i++ {
		<-done
	}
	complete = true
	log.Println("graceful exit")
}

func worker(ctx context.Context, storage storage.Storage, conn *net.UDPConn, reqChan <-chan *request, done chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			done <- struct{}{}
			return
		case r := <-reqChan:
			switch len(r.body) {
			case needle.HashLength:
				fmt.Println("received hash")
				if err := handleHash(conn, r, storage); err != nil {
					log.Println(err)
				}
			case needle.NeedleLength:
				fmt.Println("received needle")
				if err := handleNeedle(conn, r, storage); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func handleHash(conn *net.UDPConn, r *request, s storage.Storage) error {
	var hash [needle.HashLength]byte
	copy(hash[:], r.body)
	n, err := s.Get(hash)
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(n.Bytes(), r.addr)
	return err
}

func handleNeedle(conn *net.UDPConn, r *request, s storage.Storage) error {
	n, err := needle.FromBytes(r.body)
	if err != nil {
		return err
	}
	if err := s.Set(n); err != nil {
		return err
	}

	t := time.Now()
	fmt.Printf("%v\n%x\n", t, n.Bytes())
	resp := NewResponse(t, n.Hash(), nil, nil)

	_, err = conn.WriteToUDP(resp.Bytes(), r.addr)
	return err
}
