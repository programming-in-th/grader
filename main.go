package main

import (
	"log"
	"os"

	"github.com/programming-in-th/grader/grader"
)

func main() {
	_, taskBasePathEnvSet := os.LookupEnv("GRADER_TASK_BASE_PATH")
	if !taskBasePathEnvSet {
		log.Fatal("Environment variable GRADER_TASK_BASE_PATH is not set")
	}

	requestChannel := make(chan gradingRequest)
	jobQueueDone := make(chan bool)
	jobQueue := grader.NewIsolateJobQueue(2, jobQueueDone)
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := grader.NewCheckerJobQueue(5, checkerJobQueueDone)

	initAPI(requestChannel, &jobQueue, checkerJobQueue)

	jobQueueDone <- true
	checkerJobQueueDone <- true
	close(requestChannel)
}
