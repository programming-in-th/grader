package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/programming-in-th/grader/grader"
)

type gradingRequest struct {
	submissionID string
	problemID    string
	targLang     string
	code         []string
}

func handleSubmit(w http.ResponseWriter, r *http.Request, ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
	var request gradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Copy source code into /tmp directory
	filenames := make([]string, len(request.code))
	for i := 0; i < len(request.code); i++ {
		filenames[i] = path.Join("/tmp", request.submissionID+"_"+strconv.Itoa(i))
		err = ioutil.WriteFile(filenames[i], []byte(request.code[i]), 0644)
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

	result, err := grader.GradeSubmission(request.submissionID, request.problemID, request.targLang, filenames, ijq, cjq)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func handleRequest(ijq *grader.IsolateJobQueue, cjq chan grader.CheckerJob) {
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleSubmit(w, r, ijq, cjq)
	})
	http.ListenAndServe(":11112", nil)
}
