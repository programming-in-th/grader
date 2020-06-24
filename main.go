package main

import (
	"log"
	"os"
	"sync"

	"github.com/programming-in-th/grader/api"
	"github.com/programming-in-th/grader/conf"
	"github.com/programming-in-th/grader/grader"
	"github.com/programming-in-th/grader/util"
)

func initGrader(config conf.Config) {
	// Create base tmp path for user binaries and outputs
	err := util.CreateDirIfNotExist(grader.BASE_TMP_PATH)
	if err != nil {
		log.Fatal("Error creating working tmp folder")
	}

	// Create base tmp path for source files (all submissions)
	err = util.CreateDirIfNotExist(grader.BASE_SRC_PATH)
	if err != nil {
		log.Fatalln("Error initializing API: cannot create base src path")
	}

	gradingJobDoneChannel := make(chan bool)
	gradingJobChannel := grader.NewGradingJobQueue(2, gradingJobDoneChannel, config)

	// Init handlers
	requestDoneChannel := make(chan bool)
	requestChannel := newSubmissionJobQueue(4, requestDoneChannel, gradingJobChannel, config)
	api.InitAPI(requestChannel, config)

	requestDoneChannel <- true
	gradingJobDoneChannel <- true
	close(gradingJobDoneChannel)
}

func newSubmissionJobQueue(maxWorkers int, done chan bool, gradingJobChannel chan grader.GradingJob, config conf.Config) chan api.GradingRequest {
	ch := make(chan api.GradingRequest)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(ch)
	}()

	wg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go func() {
			for {
				select {
				case request := <-ch:
					err := grader.GradeSubmission(request.SubmissionID, request.TaskID, request.TargLang, request.Code, gradingJobChannel, request.SyncUpdateChannel, config)
					if err != nil {
						// TODO: do something with the error
						log.Println(err)
					}
				case <-done:
					wg.Done()
					return
				}
			}
		}()
	}
	return ch
}

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
