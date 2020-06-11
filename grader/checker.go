package grader

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/conf"
)

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

func runChecker(submissionID string,
	testCaseIndex int,
	checkerPath string,
	inputPath string,
	outputPath string,
	solutionPath string,
	config conf.Config,
) checkerResult {
	// Arguments: [path to checker binary, path to input file, path to user's produced output file, path to solution output (for checkers that diff)]
	output, err := exec.Command(checkerPath, inputPath, outputPath, solutionPath).Output()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "Error during checking. Did you chmod +x the checker executable?"))
		writeCheckFile(submissionID, testCaseIndex, conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict])
		return checkerResult{conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict]}
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
		writeCheckFile(submissionID, testCaseIndex, conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict])
		return checkerResult{conf.IEVerdict, "0", ""}
	}
	if len(outputLines) == 2 {
		writeCheckFile(submissionID, testCaseIndex, outputLines[0], outputLines[1], config.Glob.DefaultMessages[outputLines[0]])
		return checkerResult{outputLines[0], outputLines[1], config.Glob.DefaultMessages[outputLines[0]]}
	} else {
		writeCheckFile(submissionID, testCaseIndex, outputLines[0], outputLines[1], outputLines[2])
		return checkerResult{outputLines[0], outputLines[1], outputLines[2]}
	}
}
