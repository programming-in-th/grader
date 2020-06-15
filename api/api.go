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
	SyncClientChannel chan SyncMessage
}

type SyncMessage struct {
	SubmissionID string
	Message      interface{}
}

func listenAndUpdateSync(ch chan SyncMessage, port int) {
	for {
		message := <-ch
		log.Println(message)
	}
}

func SendGroupResult(submissionID string, groupStatus interface{}, ch chan SyncMessage) {
	ch <- SyncMessage{submissionID, groupStatus}
}

func SendJudgingCompleteMessage(submissionID string, ch chan SyncMessage) {
	ch <- SyncMessage{submissionID, "Complete"}
}

func SendJudgedTestMessage(submissionID string, testIndex int, ch chan SyncMessage) {
	ch <- SyncMessage{submissionID, "Judged test #" + strconv.Itoa(testIndex)}
}

func SendCompilationErrorMessage(submissionID string, ch chan SyncMessage) {
	ch <- SyncMessage{submissionID, "Compilation Error"}
}

func SendCompilingMessage(submissionID string, ch chan SyncMessage) {
	ch <- SyncMessage{submissionID, "Compiling"}
}

func handleHTTPSubmitRequest(w *http.ResponseWriter, r *http.Request, ch chan GradingRequest, syncClientChannel chan SyncMessage) {
	var request GradingRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(*w, err.Error(), http.StatusBadRequest)
	}
	request.SyncClientChannel = syncClientChannel

	log.Println("New request with submission ID", request.SubmissionID)

	// Send request to submission worker
	ch <- request

	(*w).Write([]byte("Successfull submission: " + request.SubmissionID))
}

func InitAPI(ch chan GradingRequest, config conf.Config) {
	syncClientChannel := make(chan SyncMessage)
	go listenAndUpdateSync(syncClientChannel, config.Glob.UpdatePort)
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, ch, syncClientChannel)
	})
	http.ListenAndServe("localhost:"+strconv.Itoa(config.Glob.ListenPort), nil)
}
