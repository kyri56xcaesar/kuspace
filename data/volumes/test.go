package main

import (
	"fmt"
	"strings"
)

const (
	subject string = "/guild/metrics/data"
	sub2    string = "/guild/metrics"
	sub3    string = "/guild/metrics/"
)

func main() {
	split := strings.SplitN(subject, "/", 4)
	split2 := strings.SplitN(sub2, "/", 4)
	split3 := strings.SplitN(sub3, "/", 4)

	fmt.Printf("Split 1, Content: %+v, length: %v\n", split, len(split))
	fmt.Printf("Split 2, Content: %+v, length: %v\n", split2, len(split2))
	fmt.Printf("Split 3, Content: %+v, length: %v\n", split3, len(split3))
}
