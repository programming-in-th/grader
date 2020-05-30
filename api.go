package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/programming-in-th/grader/grader"
	"github.com/programming-in-th/grader/util"
)

type gradingRequest struct {
	SubmissionID string
	ProblemID    string
	TargLang     string
	Code         []string
}

func submissionWorker(ch chan gradingRequest, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob, globalConfig *grader.GlobalConfiguration) {
	for {
		select {
		case request := <-ch:
			result, err := grader.GradeSubmission(request.SubmissionID, request.ProblemID, request.TargLang, request.Code, ijq, cjq, globalConfig)
			if err != nil {
				// TODO: do something with the error
				log.Println(err)
			}
			log.Println(result)
			// TODO: do something with result (post to firestore)
		}
	}
}

func handleHTTPSubmitRequest(w *http.ResponseWriter, r *http.Request, ch chan gradingRequest) {
	var request gradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(*w, err.Error(), http.StatusBadRequest)
	}

	log.Println("New request with submission ID", request.SubmissionID)

	// Send request to submission worker
	ch <- request

	(*w).Write([]byte("Successfully submission: " + request.SubmissionID))
}

func initGrader() {
	_, taskBasePathEnvSet := os.LookupEnv("GRADER_TASK_BASE_PATH")
	if !taskBasePathEnvSet {
		log.Fatal("Environment variable GRADER_TASK_BASE_PATH is not set")
	}

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

	requestChannel := make(chan gradingRequest)
	globalConfig, err := grader.ReadGlobalConfig(path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "config", "globalConfig.json"))
	if err != nil {
		log.Fatal("Error starting grader")
	}
	jobQueueDone := make(chan bool)
	jobQueue := grader.NewIsolateJobQueue(1, jobQueueDone, globalConfig.IsolateBinPath)
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := grader.NewCheckerJobQueue(5, checkerJobQueueDone, globalConfig)

	// Init HTTP Handlers
	go submissionWorker(requestChannel, &jobQueue, checkerJobQueue, globalConfig)

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, requestChannel)
	})
	http.ListenAndServe(":11112", nil) // TODO: set to localhost only

	jobQueueDone <- true
	checkerJobQueueDone <- true
	close(requestChannel)
}
