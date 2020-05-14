package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/programming-in-th/grader/grader"
	"google.golang.org/api/option"
)

type gradingRequest struct {
	SubmissionID string
	ProblemID    string
	TargLang     string
	Code         []string
}

func grade(request *gradingRequest, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) (*grader.GroupedSubmissionResult, error) {
	// Copy source code into /tmp directory
	filenames := make([]string, len(request.Code))
	for i := 0; i < len(request.Code); i++ {
		filenames[i] = path.Join("/tmp", request.SubmissionID+"_"+strconv.Itoa(i)+"."+request.TargLang)
		err := ioutil.WriteFile(filenames[i], []byte(request.Code[i]), 0644)
		if err != nil {
			return nil, err
		}
	}

	// Remove source files after judging
	defer func() {
		for _, file := range filenames {
			os.Remove(file)
		}
	}()

	result, err := grader.GradeSubmission(request.SubmissionID, request.ProblemID, request.TargLang, filenames, ijq, cjq)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func submissionWorker(ch chan gradingRequest, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
	for {
		select {
		case request := <-ch:
			result, err := grade(&request, ijq, cjq)
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
	// Init Firebase
	opt := option.WithCredentialsFile("./serviceAccountKey.json")
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v", err)
	}
	client, err := app.Firestore(context.Background())
	if err != nil {
		log.Fatalln(err)
	}

	defer client.Close()

	// Init HTTP Handlers
	go submissionWorker(requestChannel, ijq, cjq)

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, requestChannel)
	})
	http.ListenAndServe(":11112", nil)
}

func postResultsToFirestore(client *firestore.Client, result *grader.GroupedSubmissionResult) {
	// TODO: post to firestore
}
