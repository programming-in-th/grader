package grader

import (
	"log"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/isolate"
	"github.com/programming-in-th/grader/util"
)

const BASE_TMP_PATH = "/tmp/grader"

/* TEST RESULT TYPES */

// RunVerdict denotes the possible verdicts after running, including Correct, WA, TLE, RE and other errors
// This does not include CE
// Make sure to not confuse this with isolate.RunVerdict, which although is similar, is completely different
type RunVerdict string

const (
	// ACVerdict means the program passed the test
	ACVerdict RunVerdict = "P"
	// PartialVerdict means the program was partially correct on the test
	PartialVerdict RunVerdict = "~"
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

// SingleTestResult denotes the metrics for one single test
type SingleTestResult struct {
	Verdict     RunVerdict
	Score       float64
	TimeElapsed float64
	MemoryUsage int
}

// SingleGroupResults denotes the metrics for one single group (comprised of many tests)
type SingleGroupResult struct {
	Score       float64
	TestResults []SingleTestResult
}

// GroupedSubmissionResult denotes the test results for all groups
type GroupedSubmissionResult struct {
	CompileSuccessful bool
	GroupResults      []SingleGroupResult
}

/* MANIFEST TYPES */

type indexRange struct {
	Start int
	End   int
}

type TestGroup struct {
	FullScore   float64
	TestIndices indexRange
}

// problemManifest is a type binding for the manifest.json stored in each problem's directory.
// This is mainly needed to validate the data in manifest.json
type problemManifest struct {
	ID            string
	TimeLimit     float64
	MemoryLimit   int
	LangSupport   []string
	TestInputs    []string // names of input files (inside of inputs/ DO NOT specify path)
	TestSolutions []string // names of solution files (inside of solutions/ DO NOT specify path)
	Groups        []TestGroup

	CompileCommands map[string][]string // Compile commands for each language

	taskBasePath      string
	inputsBasePath    string
	solutionsBasePath string
}

func waitForIsolateTestResults(manifestInstance *problemManifest, submissionID string, userBinPath string, q *IsolateJobQueue) []isolateTestResult {
	// For each test case, run in isolate and send to checker
	testResults := make([]isolateTestResult, len(manifestInstance.TestInputs))
	var wg sync.WaitGroup
	wg.Add(len(manifestInstance.TestInputs))

	for i := 0; i < len(testResults); i++ {
		go func(i int) {
			ch := make(chan isolateTestResult)
			defer func() {
				close(ch)
				wg.Done()
			}()
			job := isolateJob{
				userBinPath,
				manifestInstance.TimeLimit,
				manifestInstance.MemoryLimit,
				path.Join(manifestInstance.inputsBasePath, manifestInstance.TestInputs[i]),
				path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(i)+".out"),
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

func waitForCheckerResults(testResults []isolateTestResult, manifestInstance *problemManifest, submissionID string, cjq chan CheckerJob) []SingleTestResult {
	// Compile final submission results
	var wg sync.WaitGroup
	wg.Add(len(testResults))

	result := make([]SingleTestResult, 0)
	for i := 0; i < len(testResults); i++ {
		var currResult SingleTestResult

		if testResults[i].verdict == isolate.IsolateRunXX || testResults[i].verdict == isolate.IsolateRunOther {
			currResult.Verdict = IEVerdict
			currResult.TimeElapsed = 0
			currResult.MemoryUsage = 0
			currResult.Score = 0
			if testResults[i].err != nil {
				log.Println(testResults[i].err)
			}
			result = append(result, currResult)
			continue
		}

		currResult.TimeElapsed = testResults[i].metrics.TimeElapsed
		currResult.MemoryUsage = testResults[i].metrics.MemoryUsage
		if testResults[i].verdict != isolate.IsolateRunOK {
			if testResults[i].verdict == isolate.IsolateRunMLE || testResults[i].verdict == isolate.IsolateRunRE {
				currResult.Verdict = REVerdict
				currResult.Score = 0
			} else if testResults[i].verdict == isolate.IsolateRunTLE {
				currResult.Verdict = TLEVerdict
				currResult.Score = 0
			}
			result = append(result, currResult)
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
					path.Join(manifestInstance.taskBasePath, "checker"),
					path.Join(manifestInstance.inputsBasePath, manifestInstance.TestInputs[i]),
					path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(i)+".out"),
					path.Join(manifestInstance.solutionsBasePath, manifestInstance.TestSolutions[i]),
					ch,
				}
				cjq <- job
				checkedResults := <-ch
				if checkedResults.verdict == IEVerdict {
					log.Println(checkedResults.err)
					currResult.Verdict = IEVerdict
					currResult.Score = 0
				} else {
					currResult.Verdict = checkedResults.verdict
					currResult.Score = checkedResults.score
				}
				result = append(result, currResult)
			}(i)
		}
	}

	wg.Wait()

	return result
}

func groupIndividualResults(checkerResults []SingleTestResult, groups []TestGroup) *GroupedSubmissionResult {
	finalResults := GroupedSubmissionResult{CompileSuccessful: true}

	for _, testGroup := range groups {
		groupResult := SingleGroupResult{
			Score:       -1,
			TestResults: make([]SingleTestResult, testGroup.TestIndices.End-testGroup.TestIndices.Start),
		}
		i := 0
		for j := testGroup.TestIndices.Start; j < testGroup.TestIndices.End; j++ {
			groupResult.TestResults[i] = checkerResults[j]
			if groupResult.Score == -1 || checkerResults[j].Score < groupResult.Score {
				groupResult.Score = checkerResults[j].Score
			}
			i++
		}
		// Scale score from checker (out of 100) by full score of group
		groupResult.Score = 1.0 * groupResult.Score * testGroup.FullScore / 100
		finalResults.GroupResults = append(finalResults.GroupResults, groupResult)
	}
	return &finalResults
}

// GradeSubmission is the method that is called when the web server wants to request a problem to be judged
func GradeSubmission(submissionID string, problemID string, targLang string, sourceFilePaths []string, ijq *IsolateJobQueue, cjq chan CheckerJob) (*GroupedSubmissionResult, error) {
	taskBasePath := path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks")
	if len(sourceFilePaths) == 0 {
		log.Fatal("No source files provided")
	}
	// Locate manifest file and read it
	manifestPath := path.Join(taskBasePath, problemID, "manifest.json")
	manifestInstance, err := readManifestFromFile(manifestPath)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading manifest file")
	}

	// Create tmp directory for submission
	err = util.CreateDirIfNotExist(path.Join(BASE_TMP_PATH, submissionID))
	if err != nil {
		return nil, errors.Wrap(err, "Error creating working tmp folder")
	}

	// Check if target language is supported
	langSupportContainsTargLang := false
	for _, lang := range manifestInstance.LangSupport {
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
	if len(manifestInstance.CompileCommands) != 0 {
		var compileSuccessful bool
		compileSuccessful, userBinPath = compileSubmission(submissionID, problemID, targLang, sourceFilePaths, manifestInstance)
		if !compileSuccessful {
			return &GroupedSubmissionResult{CompileSuccessful: false}, nil
		}
	} else {
		if len(sourceFilePaths) > 1 {
			log.Fatal("Grader does not support more than one source file for interpreted languages")
		}
		err := os.Rename(sourceFilePaths[0], path.Join(BASE_TMP_PATH, submissionID, "bin"))
		if err != nil {
			return &GroupedSubmissionResult{CompileSuccessful: false}, errors.Wrap(err, "Failed to move source file into user_bin")
		}
		// TODO: support more than one file. For now, just move the one file into the user_bin directory
	}

	isolateResults := waitForIsolateTestResults(manifestInstance, submissionID, userBinPath, ijq)
	log.Println("Isolate test case results:", isolateResults)
	checkerResults := waitForCheckerResults(isolateResults, manifestInstance, submissionID, cjq)
	log.Println("Individual test case results:", checkerResults)
	finalResults := groupIndividualResults(checkerResults, manifestInstance.Groups)

	// Remove user output file to not clutter up disk
	for i := 0; i < len(manifestInstance.TestInputs); i++ {
		os.Remove(path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(i)+".out"))
	}
	os.Remove(userBinPath)

	return finalResults, nil
}
