package main

import (
	"fmt"
	"net"
)

func main() {

	udpAddress, err := net.ResolveUDPAddr("udp", ":1337")
	if err != nil {
		fmt.Println(err)
		return
	}

	conn, err := net.ListenUDP("udp", udpAddress)
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		process(conn)
	}

}

func sendResponse(conn *net.UDPConn, addr *net.UDPAddr) {
	_, err := conn.WriteToUDP([]byte("From server: Hello I got your message "), addr)
	if err != nil {
		fmt.Printf("Couldn't send response %v", err)
	}
}

func process(conn *net.UDPConn) {
	b := make([]byte, 10)
	n, remoteaddr, err := conn.ReadFromUDP(b)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Read a message from %v %s length: %v\n", remoteaddr, b, n)
	go sendResponse(conn, remoteaddr)
}
