package server

import (
	"context"
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

// TODO:
// - create Server Options to configure a new server
// - think through udp and sockets, this could be useful
//   for other projects in the works like heliostat
// - how would sockets affect *net.UDPAdder? how do I generalize?
// - it will most likely need a UDP server implementation and
//  unix socket dgram implementation, this would be interesting
// to compare the speed between.
// one advantage to unix dgram is that it has guarantees that
// should work quite well for hooking into heliostat
// As I think about this further, this means I should abstract away
// how the protocols work from interacting with storage and other logic
// this would make it cleaner to manage. I should also think about
// how I'd want to have a configuration driven approach here, but maybe I put that
// in a cli implementation and keep this library simple
// - think about how to abstract the read/write to accommodate both packet types

// what logging do I want? I'm guessing it should be configurable

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
	addr net.Addr
}

type option func(*Server) error

const (
	defaultAddress  = ":1337"
	defaultTTL      = 60 * 60 * 24
	defaultProtocol = "udp"
)

var (
	defaultWorkers = runtime.NumCPU()
	defaultStorage = memory.New(10*time.Second, 2000)
)

// New returns a reference to a new Server struct
func New(opts ...option) (*Server, error) {

	s := Server{
		Address:  defaultAddress,
		TTL:      defaultTTL,
		Protocol: defaultProtocol,
		Workers:  defaultWorkers,
		Storage:  defaultStorage,
	}

	for _, opt := range opts {
		if err := opt(&s); err != nil {
			return &s, err
		}
	}

	return &s, nil
}

// Run initiates and runs the haystack server and returns an error.
func (s *Server) Run() error {

	conn, err := net.ListenPacket(s.Protocol, s.Address)
	if err != nil {
		return err
	}
	// what value should I set here?
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

func newListener(conn net.PacketConn, reqChan chan<- *request) {
	buffer := make([]byte, needle.NeedleLength+1)

	for {
		n, radder, err := conn.ReadFrom(buffer)
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
		// todo: set this to something longer?
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

func worker(ctx context.Context, storage storage.Storage, conn net.PacketConn, reqChan <-chan *request, done chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			done <- struct{}{}
			return
		case r := <-reqChan:
			switch len(r.body) {
			case needle.HashLength:
				if err := handleHash(conn, r, storage); err != nil {
					log.Println(err)
				}
			case needle.NeedleLength:
				if err := handleNeedle(conn, r, storage); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func handleHash(conn net.PacketConn, r *request, s storage.Storage) error {
	var hash [needle.HashLength]byte
	copy(hash[:], r.body)
	n, err := s.Get(hash)
	if err != nil {
		return err
	}
	_, err = conn.WriteTo(n.Bytes(), r.addr)
	return err
}

func handleNeedle(conn net.PacketConn, r *request, s storage.Storage) error {
	n, err := needle.FromBytes(r.body)
	if err != nil {
		return err
	}
	if err := s.Set(n); err != nil {
		return err
	}

	t := time.Now()
	resp := NewResponse(t, n.Hash(), nil, nil)

	_, err = conn.WriteTo(resp.Bytes(), r.addr)
	return err
}
