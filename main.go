package main

import (
	"log"
	"os"

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

	requestChannel := make(chan api.GradingRequest)
	gradingJobDoneChannel := make(chan bool)
	gradingJobChannel := grader.NewGradingJobQueue(2, gradingJobDoneChannel, config)

	// Init handlers
	go submissionWorker(requestChannel, gradingJobChannel, config)
	api.InitAPI(requestChannel, config)

	close(requestChannel)
	gradingJobDoneChannel <- true
	close(gradingJobDoneChannel)
}

func submissionWorker(requestChannel chan api.GradingRequest, gradingJobChannel chan grader.GradingJob, config conf.Config) {
	for {
		select {
		case request := <-requestChannel:
			result, err := grader.GradeSubmission(request.SubmissionID, request.TaskID, request.TargLang, request.Code, gradingJobChannel, config)
			if err != nil {
				// TODO: do something with the error
				log.Println(err)
			}
			if !result.CompileSuccessful {
				api.SendCompilationErrorMessage(request.SubmissionID)
			}
			log.Println(result)
		}
	}
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
