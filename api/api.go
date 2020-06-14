package api

import (
	"encoding/json"
	"log"
	"net/http"
)

type GradingRequest struct {
	SubmissionID string
	TaskID       string
	TargLang     string
	Code         []string
}

func HandleHTTPSubmitRequest(w *http.ResponseWriter, r *http.Request, ch chan GradingRequest) {
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
