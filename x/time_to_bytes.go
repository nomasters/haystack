package main

import (
	"encoding/binary"
	"fmt"
	"time"
)

func main() {
	buf := make([]byte, binary.MaxVarintLen64)
	t := (1421412414 * time.Nanosecond).Nanoseconds()
	_ = binary.PutVarint(buf, t)
	fmt.Println(buf)
	v, _ := binary.Varint(buf)
	fmt.Println(v)
}
