package main

import (
	"encoding/hex"
	"fmt"

	"github.com/nomasters/haystack"
	"github.com/nomasters/haystack/needle"
)

func main() {
	client, err := haystack.NewClient("127.0.0.1:1337")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer client.Close()

	b, _ := hex.DecodeString("f82e0ca0d2fb1da76da6caf36a9d0d2838655632e85891216dc8b545d8f1410940e4350b03d8b0c9e340321210b259d9a20b19632929b4a219254a4269c11f820c75168c6a91d309f4b134a7d715a5ac408991e1cf9415995053cf8a4e185dae22a06617ac51ebf7d232bc49e567f90be4db815c2b88ca0d9a4ef7a5119c0e592c88dfb96706e6510fb8a657c0f70f6695ea310d24786e6d980e9b33cf2665342b965b2391f6bb982c4c5f6058b9cba58038d32452e07cdee9420a8bd7f514e1")
	n, err := needle.FromBytes(b)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = client.Set(n)
	if err != nil {
		fmt.Println(err)
	}
}
