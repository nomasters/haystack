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

	"github.com/nomasters/haystack/storage"
)

// TODO investigate multicast

// this will be the primary processor for haystack server
// the idea here is to handle the two submit types
// but also sort out storage engines
// I think the primary ones should be:
// in memory, redis, dynamodb, and ssddb
// the interface should be simple, just a read and write aspect.

// Server is a struct that contains all the settings required for a haystack server
type Server struct {
	Address  string
	TTL      uint64
	Protocol string
	Storage  storage.Storage
	Workers  int
}

// New returns a reference to a new Server struct
func New() (*Server, error) {
	s := Server{
		Address:  ":1337",
		TTL:      500,
		Protocol: "udp",
		Workers:  runtime.NumCPU(),
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
	ctx, cancel := context.WithCancel(context.Background())
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt)

	defer func() {
		signal.Stop(stopSig)
	}()

	doneChan := make(chan struct{}, s.Workers)

	for i := 0; i < s.Workers; i++ {
		go worker(ctx, i, conn, doneChan)
	}

	<-stopSig
	gracefulShutdown(cancel, doneChan, s.Workers)
	return nil
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

func worker(ctx context.Context, id int, conn *net.UDPConn, done chan<- struct{}) {
	// 481 byte buffer allows us to see if the message
	// is larger than 480 bytes
	buffer := make([]byte, 481)
	var n int
	var addr *net.UDPAddr
	var err error

	for {
		select {
		case <-ctx.Done():
			done <- struct{}{}
			return
		default:
			n, addr, err = conn.ReadFromUDP(buffer)
			if err != nil {
				// todo create structured error
				log.Printf("work %v error: %v", id, err)
				return
			}
			msg := fmt.Sprintf("worker: %v, message of length: %v received:\n%x", id, n, buffer)
			_, err := conn.WriteToUDP([]byte(msg), addr)
			if err != nil {
				log.Printf("Couldn't send response %v", err)
			}
		}
	}
}
