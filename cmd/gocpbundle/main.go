package main

import (
	"fmt"
	"os"

	"github.com/matumoto1234/gocpbundle"
)

func main() {
	err := gocpbundle.Bundle(os.Args[1])
	if err != nil {
		fmt.Println(err)
	}
}
