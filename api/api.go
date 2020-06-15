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

type message struct {
	SubmissionID string
	Message      interface{}
}

func sendUpdateToSyncClient(message interface{}) {
	log.Println(message)
	// TODO
}

func SendGroupResult(submissionID string, groupStatus interface{}) {
	sendUpdateToSyncClient(message{submissionID, groupStatus})
}

func SendJudgingCompleteMessage(submissionID string) {
	sendUpdateToSyncClient(message{submissionID, "Complete"})
}

func SendJudgingOnTestMessage(submissionID string, testIndex int) {
	sendUpdateToSyncClient(message{submissionID, "Judging on test #" + strconv.Itoa(testIndex)})
}

func SendCompilationErrorMessage(submissionID string) {
	sendUpdateToSyncClient(message{submissionID, "Compilation Error"})
}

func SendCompilingMessage(submissionID string) {
	sendUpdateToSyncClient(message{submissionID, "Compiling"})
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

	(*w).Write([]byte("Successfull submission: " + request.SubmissionID))
}

func InitAPI(ch chan GradingRequest, config conf.Config) {
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, ch)
	})
	http.ListenAndServe("localhost:"+strconv.Itoa(config.Glob.ListenPort), nil)
}
