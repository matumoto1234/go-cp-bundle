package main

import (
	"add"
	"fmt"
)

func main() {
	var a, b int
	fmt.Scan(&a, &b)
	fmt.Println(add.Add(a, b))
}
