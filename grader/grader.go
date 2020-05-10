package grader

import (
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/isolate"
)

const taskBasePath = "/home/szawinis/go/src/github.com/programming-in-th/grader/testing/" // TODO: IMPORTANT! CHANGE LATER

// RunVerdict denotes the possible verdicts after running, including Correct, WA, TLE, RE and other errors
// This does not include CE
// Make sure to not confuse this with isolate.RunVerdict, which although is similar, is completely different
type RunVerdict string

const (
	// CorrectVerdict means the program passed the test
	ACVerdict RunVerdict = "P"
	// WAVerdict means the program got the wrong answer on the test
	WAVerdict RunVerdict = "-"
	// TLEVerdict means the program timed out
	TLEVerdict RunVerdict = "T"
	// REVerdict means the program caused a runtime error (including MLE)
	REVerdict RunVerdict = "X"
	// IEVerdict means an internal error of the grader occurred
	IEVerdict RunVerdict = "?"
)

// ListedSubmissionResult contains information about the result of a submission
type ListedSubmissionResult struct {
	CompileSuccessful bool         // If this is set to false, the other fields will be undefined
	Verdicts          []RunVerdict // verdicts for each test case in each group
	Scores            []float64
	TimeElapsed       []float64 // time elapsed for each test case in each group
	MemoryUsage       []int     // memory usage for each test case in each group
}

// problemManifest is a type binding for the manifest.json stored in each problem's directory.
// This is mainly needed to validate the data in manifest.json
type problemManifest struct {
	id            string
	timeLimit     float64
	memoryLimit   int
	langSupport   []string
	testInputs    []string // names of input files (inside of inputs/ DO NOT specify path)
	testSolutions []string // names of solution files (inside of solutions/ DO NOT specify path)
	// TODO: Add test groups

	compileCommands map[string][]string // Compile commands for each language
	checkerPath     string

	taskBasePath      string
	userBinBasePath   string
	inputsBasePath    string
	outputsBasePath   string
	solutionsBasePath string
}

func waitForIsolateTestResults(manifestInstance *problemManifest, submissionID string, userBinPath string, q *IsolateJobQueue) []isolateTestResult {
	// For each test case, run in isolate and send to checker
	testResults := make([]isolateTestResult, len(manifestInstance.testInputs))
	var wg sync.WaitGroup
	wg.Add(len(manifestInstance.testInputs))

	for i := 0; i < len(testResults); i++ {
		go func(i int) {
			ch := make(chan isolateTestResult)
			defer func() {
				close(ch)
				wg.Done()
			}()
			job := isolateJob{
				userBinPath,
				manifestInstance.timeLimit,
				manifestInstance.memoryLimit,
				path.Join(manifestInstance.inputsBasePath, manifestInstance.testInputs[i]),
				path.Join(manifestInstance.outputsBasePath, submissionID+"_output_"+strings.TrimSpace(strconv.Itoa(i))),
				ch,
			}
			log.Println("Pushing job into job queue:")
			log.Println(job)
			q.q <- job
			testResults[i] = <-ch
		}(i)
	}

	// Need to wait first for all isolate runs to complete first, or else checker can attempt to read non-existent output files
	wg.Wait()
	return testResults
}

func waitForCheckerResults(testResults []isolateTestResult, manifestInstance *problemManifest, submissionID string, cjq chan CheckerJob) ListedSubmissionResult {
	// Compile final submission results
	var wg sync.WaitGroup
	wg.Add(len(testResults))
	result := ListedSubmissionResult{
		CompileSuccessful: true,
		TimeElapsed:       make([]float64, 0),
		MemoryUsage:       make([]int, 0),
		Scores:            make([]float64, 0),
	}
	for i := 0; i < len(testResults); i++ {
		if testResults[i].verdict == isolate.IsolateRunXX || testResults[i].verdict == isolate.IsolateRunOther {
			result.Verdicts = append(result.Verdicts, IEVerdict)
			result.TimeElapsed = append(result.TimeElapsed, 0)
			result.MemoryUsage = append(result.MemoryUsage, 0)
			if testResults[i].err != nil {
				log.Println(testResults[i].err)
			}
			continue
		}
		result.TimeElapsed = append(result.TimeElapsed, testResults[i].metrics.TimeElapsed)
		result.MemoryUsage = append(result.MemoryUsage, testResults[i].metrics.MemoryUsage)
		if testResults[i].verdict != isolate.IsolateRunOK {
			if testResults[i].verdict == isolate.IsolateRunMLE || testResults[i].verdict == isolate.IsolateRunRE {
				result.Verdicts = append(result.Verdicts, REVerdict)
			} else if testResults[i].verdict == isolate.IsolateRunTLE {
				result.Verdicts = append(result.Verdicts, TLEVerdict)
			}
			continue
		} else {
			// Get outputs from checker to determine verdict
			go func(i int) {
				ch := make(chan checkerResult)
				defer func() {
					close(ch)
					wg.Done()
				}()
				job := CheckerJob{
					manifestInstance.checkerPath,
					path.Join(manifestInstance.inputsBasePath, manifestInstance.testInputs[i]),
					path.Join(manifestInstance.outputsBasePath, submissionID+"_output_"+strconv.Itoa(i)),
					path.Join(manifestInstance.solutionsBasePath, manifestInstance.testSolutions[i]),
					ch,
				}
				cjq <- job
				checkedResults := <-ch
				if checkedResults.verdict == IEVerdict {
					log.Println(checkedResults.err)
					result.Verdicts = append(result.Verdicts, IEVerdict)
				} else {
					result.Verdicts = append(result.Verdicts, checkedResults.verdict)
					result.Scores = append(result.Scores, checkedResults.score)
				}
			}(i)
		}
	}

	wg.Wait()

	return result
}

// GradeSubmission is the method that is called when the web server wants to request a problem to be judged
func GradeSubmission(submissionID string, problemID string, targLang string, sourceFilePaths []string, ijq *IsolateJobQueue, cjq chan CheckerJob) (*ListedSubmissionResult, error) {
	if len(sourceFilePaths) == 0 {
		log.Fatal("No source files provided")
	}
	// Locate manifest file and read it
	manifestPath := path.Join(taskBasePath, problemID, "manifest.json")
	manifestInstance, err := readManifestFromFile(manifestPath)
	if err != nil {
		return nil, errors.Wrap(err, "Error in grading submission")
	}

	// Check if target language is supported
	langSupportContainsTargLang := false
	for _, lang := range manifestInstance.langSupport {
		if lang == targLang {
			langSupportContainsTargLang = true
		}
	}
	if !langSupportContainsTargLang {
		return nil, errors.New("Error in grading submission: language not supported")
	}

	// Compile program and return CE if fail
	// TODO: Handle other languages that don't need compiling
	// TODO: Compile fails without absolute paths
	var userBinPath string
	if len(manifestInstance.compileCommands) != 0 {
		var compileSuccessful bool
		compileSuccessful, userBinPath = compileSubmission(submissionID, problemID, targLang, sourceFilePaths, manifestInstance)
		if !compileSuccessful {
			return &ListedSubmissionResult{CompileSuccessful: false}, nil
		}
	} else {
		if len(sourceFilePaths) > 1 {
			log.Fatal("Grader does not support more than one source file for interpreted languages")
		}
		err := os.Rename(sourceFilePaths[0], path.Join(manifestInstance.userBinBasePath, submissionID))
		if err != nil {
			return &ListedSubmissionResult{CompileSuccessful: false}, errors.Wrap(err, "Failed to move source file into user_bin")
		}
		// TODO: support more than one file
		// TODO: for now, just move the one file into the user_bin directory
	}

	testResults := waitForIsolateTestResults(manifestInstance, submissionID, userBinPath, ijq)
	result := waitForCheckerResults(testResults, manifestInstance, submissionID, cjq)

	return &result, nil
}
