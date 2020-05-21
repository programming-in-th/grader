package main

import (
	"log"
	"os"
	"path"

	"github.com/programming-in-th/grader/grader"
	"github.com/programming-in-th/grader/util"
)

func main() {
	_, taskBasePathEnvSet := os.LookupEnv("GRADER_TASK_BASE_PATH")
	if !taskBasePathEnvSet {
		log.Fatal("Environment variable GRADER_TASK_BASE_PATH is not set")
	}

	// Create base tmp path for user binaries and outputs
	err := util.CreateDirIfNotExist(path.Join(grader.BASE_TMP_PATH))
	if err != nil {
		log.Fatal("Error creating working tmp folder")
	}

	// Create base tmp path for source files (all submissions)
	err = util.CreateDirIfNotExist(BASE_SRC_PATH)
	if err != nil {
		log.Fatalln("Error initializing API: cannot create base src path")
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
