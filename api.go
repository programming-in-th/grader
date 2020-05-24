package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"

	// "cloud.google.com/go/firestore"
	// firebase "firebase.google.com/go"
	"github.com/programming-in-th/grader/grader"
	// "google.golang.org/api/option"
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

func initAPI(requestChannel chan gradingRequest, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
	globalConfig, err := grader.ReadGlobalConfig(path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "globalConfig.json"))
	if err != nil {
		log.Fatal("Error starting grader")
	}

	// Init HTTP Handlers
	go submissionWorker(requestChannel, ijq, cjq, globalConfig)

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, requestChannel)
	})
	http.ListenAndServe(":11112", nil) // TODO: set to localhost only
}
