package main

import (
	"fmt"

	"github.com/nomasters/haystack/server"
)

func main() {

	svr, err := server.New()
	if err != nil {
		panic(err)
	}
	if err := svr.Run(); err != nil {
		fmt.Println(err)
	}
}
