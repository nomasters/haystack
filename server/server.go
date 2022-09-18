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

// server is a struct that contains all the settings required for a haystack server
type server struct {
	address      string
	ttl          uint64
	protocol     string
	storage      storage.GetSetCloser
	workers      int
	ctx          context.Context
	gracePeriod  time.Duration
	presharedKey [32]byte
	privateKey   [64]byte
}

type request struct {
	body []byte
	addr net.Addr
}

// Option TBD
type Option func(*server) error

const (
	defaultAddress  = ":1337"
	defaultTTL      = 60 * 60 * 24
	defaultProtocol = "udp"
)

// WithTTL takes a uint64 and sets the server ttl
func WithTTL(ttl uint64) Option {
	return func(svr *server) error {
		svr.ttl = ttl
		return nil
	}
}

// WithPresharedKey takes a [32]byte and sets the server presharedKey
func WithPresharedKey(psk [32]byte) Option {
	return func(svr *server) error {
		svr.presharedKey = psk
		return nil
	}
}

// ListenAndServe initiates and runs the haystack server and returns an error.
func ListenAndServe(address string, opts ...Option) error {
	if address == "" {
		address = defaultAddress
	}
	s := server{
		address:     address,
		ttl:         defaultTTL,
		protocol:    defaultProtocol,
		workers:     runtime.NumCPU(),
		storage:     memory.New(10*time.Second, 2000),
		ctx:         context.Background(),
		gracePeriod: 2 * time.Second,
	}

	for _, opt := range opts {
		if err := opt(&s); err != nil {
			return err
		}
	}

	conn, err := net.ListenPacket(s.protocol, s.address)
	if err != nil {
		return err
	}
	// what value should I set here?
	reqChan := make(chan *request, s.workers*64)
	go newListener(conn, reqChan)

	ctx, cancel := context.WithCancel(s.ctx)
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt)

	defer func() {
		signal.Stop(stopSig)
	}()

	doneChan := make(chan struct{}, s.workers)

	for i := 0; i < s.workers; i++ {
		go s.newWorker(ctx, conn, reqChan, doneChan)
	}

	<-stopSig
	return s.shutdown(cancel, doneChan)
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

func (s *server) shutdown(cancel context.CancelFunc, done <-chan struct{}) error {
	cancel()
	complete := false
	go func() {
		// todo: set this to something longer?
		time.Sleep(s.gracePeriod)
		if !complete {
			log.Println("failed to gracefully exit")
			os.Exit(1)
		}
	}()

	for i := 0; i < s.workers; i++ {
		<-done
	}
	if err := s.storage.Close(); err != nil {
		return err
	}
	complete = true
	log.Println("graceful exit")
	return nil
}

func (s *server) newWorker(ctx context.Context, conn net.PacketConn, reqChan <-chan *request, done chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			done <- struct{}{}
			return
		case r := <-reqChan:
			switch len(r.body) {
			case needle.HashLength:
				if err := s.handleHash(conn, r); err != nil {
					log.Println(err)
				}
			case needle.NeedleLength:
				if err := s.handleNeedle(conn, r); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (s *server) handleHash(conn net.PacketConn, r *request) error {
	var hash [needle.HashLength]byte
	copy(hash[:], r.body)
	n, err := s.storage.Get(hash)
	if err != nil {
		return err
	}
	_, err = conn.WriteTo(n.Bytes(), r.addr)
	return err
}

func (s *server) handleNeedle(conn net.PacketConn, r *request) error {
	n, err := needle.FromBytes(r.body)
	if err != nil {
		return err
	}
	if err := s.storage.Set(n); err != nil {
		return err
	}

	t := time.Now()
	resp := NewResponse(t, n.Hash(), s.presharedKey, s.privateKey)
	_, err = conn.WriteTo(resp.Bytes(), r.addr)
	return err
}
