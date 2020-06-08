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
	"github.com/programming-in-th/grader/conf"
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

func writeCheckFile(submissionID string, testCaseIndex int, verdict string, score string, message string) {
	checkerFile, err := os.Create(path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(testCaseIndex+1)+".check"))
	if err != nil {
		log.Fatal("Error during checking. Cannot create .check file")
	}
	_, err = checkerFile.WriteString(verdict + "\n" + score + "\n" + message)
	if err != nil {
		log.Fatal("Error during checking. Cannot create .check file")
	}
}

func checkerWorker(q chan CheckerJob, id int, done chan bool, config *conf.Config) {
	for {
		select {
		case job := <-q:
			// Arguments: [path to checker binary, path to input file, path to user's produced output file, path to solution output (for checkers that diff)]
			output, err := exec.Command(job.checkerPath, job.inputPath, job.outputPath, job.solutionPath).Output()
			if err != nil {
				log.Fatal(errors.Wrapf(err, "Error during checking. Did you chmod +x the checker executable? Checker job: %#v", job))
				writeCheckFile(job.submissionID, job.testCaseIndex, conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict])
				job.resultChannel <- checkerResult{conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict]}
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
			}(conf.PossibleCheckerVerdicts, outputLines[0])) {
				writeCheckFile(job.submissionID, job.testCaseIndex, conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict])
				job.resultChannel <- checkerResult{conf.IEVerdict, "0", ""}
				continue
			}
			if len(outputLines) == 2 {
				writeCheckFile(job.submissionID, job.testCaseIndex, outputLines[0], outputLines[1], config.Glob.DefaultMessages[outputLines[0]])
				job.resultChannel <- checkerResult{outputLines[0], outputLines[1], config.Glob.DefaultMessages[outputLines[0]]}
			} else {
				writeCheckFile(job.submissionID, job.testCaseIndex, outputLines[0], outputLines[1], outputLines[2])
				job.resultChannel <- checkerResult{outputLines[0], outputLines[1], outputLines[2]}
			}
		case <-done:
			break
		}
	}
}

func NewCheckerJobQueue(maxWorkers int, done chan bool, config *conf.Config) chan CheckerJob {
	ch := make(chan CheckerJob)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(ch)
	}()

	wg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go func(i int) {
			checkerWorker(ch, i, done, config)
			wg.Done()
		}(i)
	}
	return ch
}
