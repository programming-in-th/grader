package main

import (
	"log"
	"os"
)

func main() {
	err := os.RemoveAll("/var/local/lib/isolate")
	if err != nil {
		log.Fatal("Failed to rm /var/local/lib/isolate")
	}
	initGrader()
}
