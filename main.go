package main

import (
	"log"
	"os"

	"github.com/programming-in-th/grader/conf"
)

func main() {
	err := os.RemoveAll("/var/local/lib/isolate")
	if err != nil {
		log.Fatal("Failed to rm /var/local/lib/isolate")
	}

	if len(os.Args) < 2 {
		log.Fatal("Base path not provided")
	}
	basePath := os.Args[1]

	config := conf.InitConfig(basePath)
	initGrader(config)
}
