package main

import (
	"fmt"

	"github.com/nomasters/haystack/server"
)

func main() {
	opts := []server.Option{
		server.WithTTL(60 * 60 * 24),
	}

	if err := server.ListenAndServe(":1337", opts...); err != nil {
		fmt.Println(err)
	}
}
