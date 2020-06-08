package grader

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

type checkerResult struct {
	verdict RunVerdict
	score   float64
	err     error
}

type CheckerJob struct {
	checkerPath   string
	inputPath     string
	outputPath    string
	solutionPath  string
	resultChannel chan checkerResult
}

func checkerWorker(q chan CheckerJob, id int, done chan bool) {
	for {
		select {
		case job := <-q:
			// Arguments: [path to checker binary, path to input file, path to user's produced output file, path to solution output (for checkers that diff)]
			output, err := exec.Command(job.checkerPath, job.inputPath, job.outputPath, job.solutionPath).Output()
			if err != nil {
				log.Println("Error during checking. Did you chmod +x the checker executable? Checker job:", job)
				log.Println(err)
				job.resultChannel <- checkerResult{score: 0, err: err}
				continue
			}
			outputLines := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(outputLines) < 2 || (outputLines[0] != "Correct" && outputLines[0] != "Incorrect") {
				job.resultChannel <- checkerResult{IEVerdict, 0, errors.New("Checker has invalid output format")}
				continue
			}
			score, err := strconv.ParseFloat(outputLines[1], 64)
			if err != nil {
				job.resultChannel <- checkerResult{IEVerdict, 0, errors.New("Checker has invalid output format")}
				continue
			}
			if outputLines[0] == "Correct" {
				job.resultChannel <- checkerResult{ACVerdict, score, nil}
			} else {
				job.resultChannel <- checkerResult{WAVerdict, score, nil}
			}
		case <-done:
			break
		}
	}
}

func NewCheckerJobQueue(maxWorkers int, done chan bool) chan CheckerJob {
	ch := make(chan CheckerJob)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(ch)
	}()

	wg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go func(i int) {
			checkerWorker(ch, i, done)
			wg.Done()
		}(i)
	}
	return ch
}
