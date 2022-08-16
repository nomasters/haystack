package main

import (
	"fmt"

	"github.com/nomasters/haystack/server"
)

func main() {
	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err)
	}
}
