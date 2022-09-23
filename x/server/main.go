package main

import (
	"fmt"

	"github.com/nomasters/haystack/server"
)

func main() {
	opts := []server.Option{}

	if err := server.ListenAndServe(":1337", opts...); err != nil {
		fmt.Println(err)
	}
}
