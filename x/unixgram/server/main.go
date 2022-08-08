package main

import (
	"fmt"
	"net"
)

const socket = "/tmp/haystack.socket"

func run() error {
	addr, err := net.ResolveUnixAddr("unixgram", socket)
	if err != nil {
		return err
	}
	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		return err
	}

	conn, err := l.AcceptUnix()
	if err != nil {
		return err
	}

	fmt.Println(conn)

	return nil
}

func main() {

}
