package server

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/nomasters/haystack/logger"
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

// all of this will move to CMD
// the idea here is that I should use inversion of control on the server
// and the logging and configs should all be configurable at the cmd level
// This means I need a design spike on ergonomics of initializing the server
// and running it as a CLI command + CLI switches, config file design, etc.
// the end goal here is to make it trivial to dockerize or throw in a process monitor.

// what logging do I want? I'm guessing it should be configurable
// new considerations to have:
// - max open request limit - this should be guided by the storage backend, most likely, I need to think about how transparent
//   this is to the end user
// - how do we want to build in IOC for logging? What log levels are supported?
// - what gest moved to the CLI

// server is a struct that contains all the settings required for a haystack server
type server struct {
	address     string
	protocol    string
	storage     storage.GetSetCloser
	workers     uint64
	ctx         context.Context
	gracePeriod time.Duration
	logger      logger.Logger
}

type request struct {
	body []byte
	addr net.Addr
}

// Option TBD
type Option func(*server) error

const (
	defaultAddress     = ":1337"
	defaultProtocol    = "udp"
	defaultGracePeriod = 2 * time.Second
	minGracePeriod     = 0 * time.Millisecond
)

// NOTE: this might actually need to move to the cmd. it seems more like a runtime implementation detail
// UPDATE: this should, in fact, be part of the CMD implementation, it should not be opinionated in the server itself
// so that others can use it as they see fit.

// WithShutdownGracePeriod allows you to set the gracePeriod
func WithShutdownGracePeriod(duration time.Duration) Option {
	if duration <= minGracePeriod {
		duration = defaultGracePeriod
	}

	return func(svr *server) error {
		svr.gracePeriod = duration
		return nil
	}

}

// WithStorage allows setting a storage.GetSetCloser in the server runtime
func WithStorage(s storage.GetSetCloser) Option {
	return func(svr *server) error {
		svr.storage = s
		return nil
	}
}

// WithContext takes context.Context and passes it to the server struct
func WithContext(ctx context.Context) Option {
	return func(svr *server) error {
		svr.ctx = ctx
		return nil
	}
}

// WithWorkerCount takes count of uint64 and passes it to the server struct
// if worker count is set to zero, workers are set to runtime.NumCPU()
func WithWorkerCount(count uint64) Option {
	if count == 0 {
		count = uint64(runtime.NumCPU())
	}
	return func(svr *server) error {
		svr.workers = count
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
		protocol:    defaultProtocol,
		workers:     uint64(runtime.NumCPU()),
		storage:     memory.New(storage.DefaultTTL, 2000000),
		ctx:         context.Background(),
		gracePeriod: defaultGracePeriod,
		logger:      logger.New(),
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

	for i := 0; i < int(s.workers); i++ {
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
			s.logger.Fatal("failed to gracefully exit")
		}
	}()

	for i := 0; i < int(s.workers); i++ {
		<-done
	}
	if err := s.storage.Close(); err != nil {
		return err
	}
	complete = true
	s.logger.Info("graceful exit")
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
	return err
}
