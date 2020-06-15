package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/programming-in-th/grader/conf"
)

type GradingRequest struct {
	SubmissionID      string
	TaskID            string
	TargLang          string
	Code              []string
	SyncUpdateChannel chan SyncUpdateMessage
}

type SyncUpdateMessage struct {
	SubmissionID string
	Message      interface{}
}

func listenAndUpdateSync(ch chan SyncUpdateMessage, port int) {
	for {
		message := <-ch
		log.Println(message)
	}
}

func SendGroupResult(submissionID string, groupStatus interface{}, ch chan SyncUpdateMessage) {
	ch <- SyncUpdateMessage{submissionID, groupStatus}
}

func SendJudgingCompleteMessage(submissionID string, ch chan SyncUpdateMessage) {
	ch <- SyncUpdateMessage{submissionID, "Complete"}
}

func SendJudgedTestMessage(submissionID string, testIndex int, ch chan SyncUpdateMessage) {
	ch <- SyncUpdateMessage{submissionID, "Judged test #" + strconv.Itoa(testIndex)}
}

func SendCompilationErrorMessage(submissionID string, ch chan SyncUpdateMessage) {
	ch <- SyncUpdateMessage{submissionID, "Compilation Error"}
}

func SendCompilingMessage(submissionID string, ch chan SyncUpdateMessage) {
	ch <- SyncUpdateMessage{submissionID, "Compiling"}
}

func handleHTTPSubmitRequest(w *http.ResponseWriter, r *http.Request, ch chan GradingRequest, syncUpdateChannel chan SyncUpdateMessage) {
	var request GradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(*w, err.Error(), http.StatusBadRequest)
	}
	request.SyncUpdateChannel = syncUpdateChannel

	log.Println("New request with submission ID", request.SubmissionID)

	// Send request to submission worker
	ch <- request

	(*w).Write([]byte("Successfull submission: " + request.SubmissionID))
}

func InitAPI(ch chan GradingRequest, config conf.Config) {
	syncUpdateChannel := make(chan SyncUpdateMessage)
	go listenAndUpdateSync(syncUpdateChannel, config.Glob.UpdatePort)
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, ch, syncUpdateChannel)
	})
	http.ListenAndServe("localhost:"+strconv.Itoa(config.Glob.ListenPort), nil)
}
