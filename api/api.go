package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/conf"
)

type GradingRequest struct {
	SubmissionID      string
	TaskID            string
	TargLang          string
	Code              []string
	SyncUpdateChannel chan SyncUpdate
}

type syncUpdatePayloadType string

const msgUpdateType syncUpdatePayloadType = "msg"
const groupUpdateType syncUpdatePayloadType = "group"

type SyncUpdate struct {
	payloadType  syncUpdatePayloadType
	submissionID string
	payload      interface{}
}

type SyncUpdateMessage struct {
	SubmissionID string
	Message      string
}

type SyncUpdateGroup struct {
	SubmissionID string
	Results      interface{}
}

// This is endpoint where messages finally get send to the sync client
func listenAndUpdateSync(ch chan SyncUpdate, port int) {
	for {
		message := <-ch

		var requestBody []byte
		var err error

		baseURL := "http://localhost:" + strconv.Itoa(port)
		if message.payloadType == msgUpdateType {
			baseURL += "/message"
			requestBody, err = json.Marshal(SyncUpdateMessage{message.submissionID, message.payload.(string)})
			if err != nil {
				log.Fatal(errors.Wrap(err, "Sync update not serializable"))
			}
		} else if message.payloadType == groupUpdateType {
			baseURL += "/group"
			requestBody, err = json.Marshal(SyncUpdateGroup{message.submissionID, message.payload})
			if err != nil {
				log.Fatal(errors.Wrap(err, "Sync update not serializable"))
			}
		} else {
			log.Fatal("Unsupported payload type")
		}

		log.Println(string(requestBody))
		resp, err := http.Post(baseURL, "application/json", bytes.NewBuffer(requestBody))
		if err != nil {
			log.Println(errors.Wrap(err, "Unable to send sync update"))
		}
		if resp.StatusCode != 200 {
			log.Printf("Non-OK response code from sync client: %d", resp.StatusCode)
		}
	}
}

func SendGroupResult(submissionID string, groupStatus interface{}, ch chan SyncUpdate) {
	ch <- SyncUpdate{groupUpdateType, submissionID, groupStatus}
}

func SendJudgingCompleteMessage(submissionID string, ch chan SyncUpdate) {
	ch <- SyncUpdate{msgUpdateType, submissionID, "Complete"}
}

func SendJudgedTestMessage(submissionID string, testIndex int, ch chan SyncUpdate) {
	ch <- SyncUpdate{msgUpdateType, submissionID, "Judged test #" + strconv.Itoa(testIndex+1)}
}

func SendCompilationErrorMessage(submissionID string, ch chan SyncUpdate) {
	ch <- SyncUpdate{msgUpdateType, submissionID, "Compilation Error"}
}

func SendCompilingMessage(submissionID string, ch chan SyncUpdate) {
	ch <- SyncUpdate{msgUpdateType, submissionID, "Compiling"}
}

func handleHTTPSubmitRequest(w *http.ResponseWriter, r *http.Request, ch chan GradingRequest, syncUpdateChannel chan SyncUpdate) {
	defer r.Body.Close()

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
	syncUpdateChannel := make(chan SyncUpdate)
	go listenAndUpdateSync(syncUpdateChannel, config.Glob.SyncUpdatePort)
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPSubmitRequest(&w, r, ch, syncUpdateChannel)
	})
	http.ListenAndServe("localhost:"+strconv.Itoa(config.Glob.SyncListenPort), nil)
}
