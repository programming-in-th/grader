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

func handleSubmit(w http.ResponseWriter, r *http.Request, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) *grader.GroupedSubmissionResult {
	var request gradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	log.Println("New request with submission ID", request.SubmissionID)
	log.Println(request)

	// Copy source code into /tmp directory
	filenames := make([]string, len(request.Code))
	for i := 0; i < len(request.Code); i++ {
		filenames[i] = path.Join("/tmp", request.SubmissionID+"_"+strconv.Itoa(i)+"."+request.TargLang)
		err = ioutil.WriteFile(filenames[i], []byte(request.Code[i]), 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return nil
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
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	// TODO: send "submitted" status ok as soon as the use submits
	w.WriteHeader(http.StatusOK)

	return result
}

func handleRequest(ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
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

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		result := handleSubmit(w, r, ijq, cjq)
		log.Println(result)
		postResultsToFirestore(client, result)
	})
	http.ListenAndServe(":11112", nil)
}

func postResultsToFirestore(client *firestore.Client, result *grader.GroupedSubmissionResult) {
	// TODO: post to firestore
}

// TODO: fix nested function http misdirect
// TODO: delete user's files on server
