package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/programming-in-th/grader/conf"
)

type GradingRequest struct {
	SubmissionID string
	TaskID       string
	TargLang     string
	Code         []string
}

func handleHTTPSubmitRequest(w *http.ResponseWriter, r *http.Request, ch chan GradingRequest) {
	var request GradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(*w, err.Error(), http.StatusBadRequest)
	}

	log.Println("New request with submission ID", request.SubmissionID)

	// Send request to submission worker
	ch <- request

	(*w).Write([]byte("Successfully submission: " + request.SubmissionID))
}

func InitAPI(ch chan GradingRequest, config conf.Config) {
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, ch)
	})
	http.ListenAndServe("localhost:"+strconv.Itoa(config.Glob.ListenPort), nil)
}
