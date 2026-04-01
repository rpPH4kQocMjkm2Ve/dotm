package main

import "fmt"

// Set at build time via: -ldflags "-X main.version=..."
var version = "dev"

func cmdVersion() {
	fmt.Println("dotm " + version)
}
