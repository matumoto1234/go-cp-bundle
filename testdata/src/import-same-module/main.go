package importsamemodule

import (
	f "fmt"
	"importsamemodule/add"
)

func main() {
	var a, b int
	f.Scan(&a, &b)
	f.Println(add.AddInt(a, b))
}
