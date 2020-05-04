package grader

import (
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type checkerResult struct {
	verdict RunVerdict
	score   float64
	err     error
}

type checkerJob struct {
	checkerPath   string
	inputPath     string
	outputPath    string
	solutionPath  string
	resultChannel chan checkerResult
}

func checkerWorker(q chan checkerJob, id int) {
	for {
		select {
		case job := <-q:
			output, err := exec.Command(job.checkerPath, job.inputPath, job.outputPath, job.solutionPath).Output()
			if err != nil {
				log.Println("Error during checking")
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
		}
	}
}

func NewCheckerJobQueue(maxWorkers int) chan checkerJob {
	ch := make(chan checkerJob)
	for i := 0; i < maxWorkers; i++ {
		go checkerWorker(ch, i)
	}
	return ch
}
