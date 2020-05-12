package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/programming-in-th/grader/grader"
)

type gradingRequest struct {
	SubmissionID string
	ProblemID    string
	TargLang     string
	Code         []string
}

func handleSubmit(w http.ResponseWriter, r *http.Request, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
	var request gradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Println("New request with submission ID", request.SubmissionID)

	// Copy source code into /tmp directory
	filenames := make([]string, len(request.Code))
	for i := 0; i < len(request.Code); i++ {
		filenames[i] = path.Join("/tmp", request.SubmissionID+"_"+strconv.Itoa(i)+"."+request.TargLang)
		err = ioutil.WriteFile(filenames[i], []byte(request.Code[i]), 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Remove source files after judging
	defer func() {
		for _, file := range filenames {
			os.Remove(file)
		}
	}()

	result, err := grader.GradeSubmission(request.SubmissionID, request.ProblemID, request.TargLang, filenames, ijq, cjq)
	log.Println(result)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleRequest(ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleSubmit(w, r, ijq, cjq)
	})
	http.ListenAndServe(":11112", nil)
}
