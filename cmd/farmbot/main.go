package main

import "fmt"

// version is overridden by release builds with -ldflags.
var version = "dev"

func main() {
	fmt.Println(version)
}
