package main

import (
	"github.com/nomasters/haystack/cmd"
)

// TODO: use this as a way to run haystack client and haystack server
// think about what this interface might be like, but it would be useful to
// be able to have the CLI test send/receive of messages

func main() {
	cmd.Execute()
}
