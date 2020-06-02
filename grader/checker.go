package grader

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

type CheckerJob struct {
	submissionID  string
	testCaseIndex int
	checkerPath   string
	inputPath     string
	outputPath    string
	solutionPath  string
	resultChannel chan checkerResult
}

type checkerResult struct {
	verdict string
	score   string
	message string
}

var possibleCheckerVerdicts = []string{ACVerdict, PartialVerdict, WAVerdict, IEVerdict}

func writeCheckFile(submissionID string, testCaseIndex int, verdict string, score string, message string) error {
	checkerFile, err := os.Create(path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(testCaseIndex+1)+".check"))
	if err != nil {
		log.Fatal("Error during checking. Cannot create .check file")
	}
	_, err = checkerFile.WriteString(verdict + "\n" + score + "\n" + message)
	if err != nil {
		log.Fatal("Error during checking. Cannot write to .check file")
	}
	return nil
}

func checkerWorker(q chan CheckerJob, id int, done chan bool, globalConfigInstance *GlobalConfiguration) {
	for {
		select {
		case job := <-q:
			// Arguments: [path to checker binary, path to input file, path to user's produced output file, path to solution output (for checkers that diff)]
			output, err := exec.Command(job.checkerPath, job.inputPath, job.outputPath, job.solutionPath).Output()
			if err != nil {
				log.Fatal(errors.Wrapf(err, "Error during checking. Did you chmod +x the checker executable? Checker job: %#v", job))
				writeCheckFile(job.submissionID, job.testCaseIndex, IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict])
				job.resultChannel <- checkerResult{IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict]}
				continue
			}
			outputLines := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(outputLines) < 2 || len(outputLines) > 3 || !(func(arr []string, targ string) bool {
				found := false
				for _, elem := range arr {
					if elem == targ {
						found = true
						break
					}
				}
				return found
			}(possibleCheckerVerdicts, outputLines[0])) {
				writeCheckFile(job.submissionID, job.testCaseIndex, IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict])
				job.resultChannel <- checkerResult{IEVerdict, "0", ""}
				continue
			}
			if len(outputLines) == 2 {
				writeCheckFile(job.submissionID, job.testCaseIndex, outputLines[0], outputLines[1], globalConfigInstance.DefaultMessages[outputLines[0]])
				job.resultChannel <- checkerResult{outputLines[0], outputLines[1], globalConfigInstance.DefaultMessages[outputLines[0]]}
			} else {
				writeCheckFile(job.submissionID, job.testCaseIndex, outputLines[0], outputLines[1], outputLines[2])
				job.resultChannel <- checkerResult{outputLines[0], outputLines[1], outputLines[2]}
			}
		case <-done:
			break
		}
	}
}

func NewCheckerJobQueue(maxWorkers int, done chan bool, globalConfigInstance *GlobalConfiguration) chan CheckerJob {
	ch := make(chan CheckerJob)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(ch)
	}()

	wg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go func(i int) {
			checkerWorker(ch, i, done, globalConfigInstance)
			wg.Done()
		}(i)
	}
	return ch
}
