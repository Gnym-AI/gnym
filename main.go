package main

import (
	"fmt"
	"gnym/review"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gnym review")
		return
	}

	command := os.Args[1]
	switch command {
	case "review":
		review.Run()

	case "version":
		fmt.Println("Gnym v0.1.0")

	default:
		fmt.Println("Unknown command:", command)
	}
}
