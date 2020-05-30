package grader

import (
	"encoding/json"
	"io/ioutil"
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
const BASE_SRC_PATH = BASE_TMP_PATH + "/source"

/* TEST RESULT TYPES */

const (
	// ACVerdict means the program passed the test
	ACVerdict string = "Correct"
	// PartialVerdict means the program was partially correct on the test
	PartialVerdict string = "Partially Correct"
	// WAVerdict means the program got the wrong answer on the test
	WAVerdict string = "Incorrect"
	// TLEVerdict means the program timed out
	TLEVerdict string = "Time Limit Exceeded"
	// MLEVerdict means the program used too much memory
	MLEVerdict string = "Memory Limit Exceeded"
	// REVerdict means the program caused a runtime error (not including MLE)
	REVerdict string = "Memory Limit Exceeded"
	// IEVerdict means an internal error of the grader occurred
	IEVerdict string = "Judge Error"
)

// ListedSubmissionResult contains information about the result of a submission
type ListedSubmissionResult struct {
	CompileSuccessful bool     // If this is set to false, the other fields will be undefined
	Verdicts          []string // verdicts for each test case in each group
	Scores            []float64
	TimeElapsed       []float64 // time elapsed for each test case in each group
	MemoryUsage       []int     // memory usage for each test case in each group
}

// SingleTestResult denotes the metrics for one single test
type SingleTestResult struct {
	Verdict     string
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
	FullScore    float64
	Dependencies []int
	TestIndices  indexRange
}

type LangRunLimit struct {
	TimeLimit   float64
	MemoryLimit int
}

// problemManifest is a type binding for the manifest.json stored in each problem's directory.
// This is mainly needed to validate the data in manifest.json
type problemManifest struct {
	ID            string
	DefaultLimits *LangRunLimit
	Limits        map[string]LangRunLimit
	Groups        []TestGroup
	CompileFiles  map[string][]string
	Checker       string
	Grouper       string

	numTests          int
	taskBasePath      string
	inputsBasePath    string
	solutionsBasePath string
}

func getLangCompileConfig(globalConfigInstance *GlobalConfiguration, targLang string) *LangCompileConfiguration {
	// Find target language's compile configuration
	foundLang := false
	var langConfig LangCompileConfiguration
	for _, langConfig = range globalConfigInstance.CompileConfiguration {
		if langConfig.ID == targLang {
			foundLang = true
			break
		}
	}
	if !foundLang {
		return nil
	}
	return &langConfig
}

func readManifestFromFile(manifestPath string) (*problemManifest, error) {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read manifest.json file at %s", manifestPath)
	}

	var manifestInstance problemManifest
	json.Unmarshal(manifestFileBytes, &manifestInstance)

	manifestInstance.taskBasePath = path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks", manifestInstance.ID)
	manifestInstance.inputsBasePath = path.Join(manifestInstance.taskBasePath, "inputs")
	manifestInstance.solutionsBasePath = path.Join(manifestInstance.taskBasePath, "solutions")
	manifestInstance.numTests = manifestInstance.Groups[len(manifestInstance.Groups)-1].TestIndices.End

	return &manifestInstance, nil
}

func waitForIsolateTestResults(manifestInstance *problemManifest, submissionID string, targLang string, userBinPath string, q *IsolateJobQueue) []isolateTestResult {
	// For each test case, run in isolate and send to checker
	testResults := make([]isolateTestResult, manifestInstance.numTests)
	var wg sync.WaitGroup
	wg.Add(manifestInstance.numTests)

	for i := 0; i < len(testResults); i++ {
		go func(i int) {
			ch := make(chan isolateTestResult)
			defer func() {
				close(ch)
				wg.Done()
			}()
			var timeLimit float64
			var memoryLimit int
			if limits, exists := manifestInstance.Limits[targLang]; exists {
				timeLimit = limits.TimeLimit
				memoryLimit = limits.MemoryLimit * 1000 // Convert to KB
			} else {
				timeLimit = manifestInstance.DefaultLimits.TimeLimit
				memoryLimit = manifestInstance.DefaultLimits.MemoryLimit * 1000
			}
			job := isolateJob{
				userBinPath,
				timeLimit,
				memoryLimit,
				path.Join(manifestInstance.inputsBasePath, strconv.Itoa(i+1)+".in"),
				path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(i+1)+".out"),
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

func waitForCheckerResults(testResults []isolateTestResult, manifestInstance *problemManifest, submissionID string, cjq chan CheckerJob, globalConfigInstance *GlobalConfiguration) {
	// Compile final submission results
	var wg sync.WaitGroup
	wg.Add(len(testResults))

	for i := 0; i < len(testResults); i++ {

		// TODO: Write to check file instead
		if testResults[i].verdict == isolate.IsolateRunXX || testResults[i].verdict == isolate.IsolateRunOther {
			writeCheckFile(submissionID, i+1, IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict])
			if testResults[i].err != nil {
				log.Println(testResults[i].err)
			}
			continue
		}

		if testResults[i].verdict != isolate.IsolateRunOK {
			if testResults[i].verdict == isolate.IsolateRunMLE {
				writeCheckFile(submissionID, i+1, MLEVerdict, "0", globalConfigInstance.DefaultMessages[MLEVerdict])
			} else if testResults[i].verdict == isolate.IsolateRunRE {
				writeCheckFile(submissionID, i+1, REVerdict, "0", globalConfigInstance.DefaultMessages[REVerdict])
			} else if testResults[i].verdict == isolate.IsolateRunTLE {
				writeCheckFile(submissionID, i+1, TLEVerdict, "0", globalConfigInstance.DefaultMessages[TLEVerdict])
			} else {
				writeCheckFile(submissionID, i+1, IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict])
			}
			continue
		} else {
			// Get outputs from checker to determine verdict
			go func(i int) {
				doneChannel := make(chan bool)
				defer func() {
					close(doneChannel)
					wg.Done()
				}()
				var checkerPath string
				if manifestInstance.Checker != "custom" {
					checkerPath = path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "config", "defaultCheckers", manifestInstance.Checker)
				} else {
					checkerPath = path.Join(manifestInstance.taskBasePath, "checker")
				}
				job := CheckerJob{
					submissionID,
					i,
					checkerPath,
					path.Join(manifestInstance.inputsBasePath, strconv.Itoa(i+1)+".in"),
					path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(i+1)+".out"),
					path.Join(manifestInstance.solutionsBasePath, strconv.Itoa(i+1)+".sol"),
					doneChannel,
				}
				cjq <- job
				<-doneChannel
			}(i)
		}
	}

	wg.Wait()
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
func GradeSubmission(submissionID string, problemID string, targLang string, code []string, ijq *IsolateJobQueue, cjq chan CheckerJob, globalConfigInstance *GlobalConfiguration) (*GroupedSubmissionResult, error) {
	taskBasePath := path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks")

	langConfig := getLangCompileConfig(globalConfigInstance, targLang)
	if langConfig == nil {
		return &GroupedSubmissionResult{CompileSuccessful: false}, nil
	}

	if len(code) == 0 {
		return &GroupedSubmissionResult{CompileSuccessful: false}, nil
	}

	// Copy source code into tmp directory
	srcFilePaths := make([]string, len(code))
	for i := 0; i < len(code); i++ {
		srcFilePaths[i] = path.Join(BASE_SRC_PATH, submissionID+"_"+strconv.Itoa(i)+"."+langConfig.Extension)
		err := ioutil.WriteFile(srcFilePaths[i], []byte(code[i]), 0644)
		if err != nil {
			log.Println("Cannot copy source code into tmp directory:", srcFilePaths[i])
			return nil, errors.Wrap(err, "Cannot copy source code into tmp directory")
		}
	}

	// Remove source files after judging
	defer func() {
		for _, file := range srcFilePaths {
			os.Remove(file)
		}
	}()

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
	// TODO: change logic
	langSupportContainsTargLang := true
	if manifestInstance.DefaultLimits != nil {
		// NOTE: both limits == 0 is equivalent to it being null
		if limit, exists := manifestInstance.Limits[targLang]; exists && (limit.TimeLimit == 0 || limit.MemoryLimit == 0) {
			langSupportContainsTargLang = false
		}
	} else {
		if limit, exists := manifestInstance.Limits[targLang]; (exists && (limit.TimeLimit == 0 || limit.MemoryLimit == 0)) || !exists {
			langSupportContainsTargLang = false
		}
	}
	if !langSupportContainsTargLang {
		log.Println("no lang support")
		log.Println("targLang:", targLang)
		return nil, errors.New("Error in grading submission: language not supported")
	}

	// Add compile files to srcFilePaths after defer statement so it doesn't delete
	if _, exists := manifestInstance.CompileFiles[targLang]; exists {
		for _, compileFile := range manifestInstance.CompileFiles[targLang] {
			srcFilePaths = append(srcFilePaths, path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks", problemID, compileFile))
		}
	}

	// Compile program and return CE if fail
	// TODO: Handle other languages that don't need compiling
	// TODO: Compile fails without absolute paths
	var userBinPath string
	if langConfig.CompileCommands != nil && len(langConfig.CompileCommands) != 0 {
		var compileSuccessful bool
		compileSuccessful, userBinPath = compileSubmission(submissionID, problemID, srcFilePaths, langConfig.CompileCommands)
		if !compileSuccessful {
			return &GroupedSubmissionResult{CompileSuccessful: false}, nil
		}
	} else {
		if len(srcFilePaths) > 1 {
			log.Fatal("Grader does not support more than one source file for interpreted languages")
		}
		err := os.Rename(srcFilePaths[0], path.Join(BASE_TMP_PATH, submissionID, "bin"))
		if err != nil {
			return &GroupedSubmissionResult{CompileSuccessful: false}, errors.Wrap(err, "Failed to move source file into user_bin")
		}
		// TODO: support more than one file. For now, just move the one file into the user_bin directory
	}

	// Remove user output file to not clutter up disk
	defer func() {
		os.RemoveAll(path.Join(BASE_TMP_PATH, submissionID))
		os.Remove(userBinPath)
	}()

	isolateResults := waitForIsolateTestResults(manifestInstance, submissionID, targLang, userBinPath, ijq)
	log.Println("Isolate test case results:", isolateResults)
	waitForCheckerResults(isolateResults, manifestInstance, submissionID, cjq, globalConfigInstance)

	// TODO: group results

	return nil, nil
}
