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
	// SKVerdict means the test was skipped because a dependent group was not passed
	SKVerdict string = "Skipped"
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
	Score       string
	TimeElapsed float64
	MemoryUsage int
	Message     string
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

	// Decrease indices for easier handling
	for i := 0; i < len(manifestInstance.Groups); i++ {
		for j := 0; j < len(manifestInstance.Groups[i].Dependencies); j++ {
			manifestInstance.Groups[i].Dependencies[j] -= 1
		}
		manifestInstance.Groups[i].TestIndices.Start -= 1
		// Leave .End as is because we want it to be exclusive
	}

	manifestInstance.taskBasePath = path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks", manifestInstance.ID)
	manifestInstance.inputsBasePath = path.Join(manifestInstance.taskBasePath, "inputs")
	manifestInstance.solutionsBasePath = path.Join(manifestInstance.taskBasePath, "solutions")
	manifestInstance.numTests = manifestInstance.Groups[len(manifestInstance.Groups)-1].TestIndices.End

	return &manifestInstance, nil
}

func waitForTestResult(manifestInstance *problemManifest, submissionID string, targLang string, userBinPath string, testIndex int, q *IsolateJobQueue, cjq chan CheckerJob, globalConfigInstance *GlobalConfiguration) (*SingleTestResult, error) {
	// Initialize channels for parallel judging
	isolateResultChannel := make(chan isolateTestResult)
	checkerChannel := make(chan checkerResult)
	defer func() {
		close(isolateResultChannel)
		close(checkerChannel)
	}()

	// Dispatch job to an isolate worker
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
		path.Join(manifestInstance.inputsBasePath, strconv.Itoa(testIndex+1)+".in"),
		path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(testIndex+1)+".out"),
		isolateResultChannel,
	}
	log.Println("Pushing job into job queue:")
	log.Println(job)
	q.q <- job

	// Wait for isolate worker to finish
	isolateResult := <-isolateResultChannel

	// Check for fatal errors first and return corresponding results without running checker
	if isolateResult.verdict == isolate.IsolateRunXX || isolateResult.verdict == isolate.IsolateRunOther {
		writeCheckFile(submissionID, testIndex, IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict])
		return &SingleTestResult{IEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, globalConfigInstance.DefaultMessages[IEVerdict]}, isolateResult.err
	}

	if isolateResult.verdict != isolate.IsolateRunOK {
		if isolateResult.verdict == isolate.IsolateRunMLE {
			writeCheckFile(submissionID, testIndex, MLEVerdict, "0", globalConfigInstance.DefaultMessages[MLEVerdict])
			return &SingleTestResult{MLEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, globalConfigInstance.DefaultMessages[MLEVerdict]}, nil
		} else if isolateResult.verdict == isolate.IsolateRunRE {
			writeCheckFile(submissionID, testIndex, REVerdict, "0", globalConfigInstance.DefaultMessages[REVerdict])
			return &SingleTestResult{REVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, globalConfigInstance.DefaultMessages[REVerdict]}, nil
		} else if isolateResult.verdict == isolate.IsolateRunTLE {
			writeCheckFile(submissionID, testIndex, TLEVerdict, "0", globalConfigInstance.DefaultMessages[TLEVerdict])
			return &SingleTestResult{TLEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, globalConfigInstance.DefaultMessages[TLEVerdict]}, nil
		} else {
			writeCheckFile(submissionID, testIndex, IEVerdict, "0", globalConfigInstance.DefaultMessages[IEVerdict])
			return &SingleTestResult{IEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, globalConfigInstance.DefaultMessages[IEVerdict]}, nil
		}
	} else {
		// Assuming the verdict is isolate.IsolateRunOK, we run the checker
		var checkerPath string
		if manifestInstance.Checker != "custom" {
			checkerPath = path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "config", "defaultCheckers", manifestInstance.Checker)
		} else {
			checkerPath = path.Join(manifestInstance.taskBasePath, "checker")
		}
		job := CheckerJob{
			submissionID,
			testIndex,
			checkerPath,
			path.Join(manifestInstance.inputsBasePath, strconv.Itoa(testIndex+1)+".in"),
			path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(testIndex+1)+".out"),
			path.Join(manifestInstance.solutionsBasePath, strconv.Itoa(testIndex+1)+".sol"),
			checkerChannel,
		}
		cjq <- job

		// Wait for checker to finish
		result := <-checkerChannel

		return &SingleTestResult{result.verdict, result.score, isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, result.message}, nil
	}
}

// GradeSubmission is the method that is called when the web server wants to request a problem to be judged
func GradeSubmission(submissionID string, problemID string, targLang string, code []string, ijq *IsolateJobQueue, cjq chan CheckerJob, globalConfigInstance *GlobalConfiguration) (*GroupedSubmissionResult, error) {
	taskBasePath := path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks")

	langConfig := getLangCompileConfig(globalConfigInstance, targLang)
	if langConfig == nil {
		return &GroupedSubmissionResult{CompileSuccessful: false}, errors.New("Language not supported")
	}

	if len(code) == 0 {
		return &GroupedSubmissionResult{CompileSuccessful: false}, errors.New("Code passed in is empty")
	}

	// Copy source code into tmp directory
	srcFilePaths := make([]string, len(code))
	for i := 0; i < len(code); i++ {
		srcFilePaths[i] = path.Join(BASE_SRC_PATH, submissionID+"_"+strconv.Itoa(i)+"."+langConfig.Extension)
		err := ioutil.WriteFile(srcFilePaths[i], []byte(code[i]), 0644)
		if err != nil {
			return &GroupedSubmissionResult{CompileSuccessful: false}, errors.Wrapf(err, "Cannot copy source code into tmp directory: %s", srcFilePaths[i])
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
		return &GroupedSubmissionResult{CompileSuccessful: false}, errors.Wrap(err, "Error reading manifest file")
	}

	// Create tmp directory for submission
	err = util.CreateDirIfNotExist(path.Join(BASE_TMP_PATH, submissionID))
	if err != nil {
		return &GroupedSubmissionResult{CompileSuccessful: false}, errors.Wrap(err, "Error creating working tmp folder")
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
		return &GroupedSubmissionResult{CompileSuccessful: false}, errors.New("Language not supported")
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
			return &GroupedSubmissionResult{CompileSuccessful: false}, errors.New("Language not supported")
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

	groupResults := GroupedSubmissionResult{CompileSuccessful: true}
	for i := 0; i < len(manifestInstance.Groups); i++ {

		// If a dependency is not satisfied, skip the entire group
		foundInvalid := false
		numTests := manifestInstance.Groups[i].TestIndices.End - manifestInstance.Groups[i].TestIndices.Start
		for _, j := range manifestInstance.Groups[i].Dependencies {
			if groupResults.GroupResults[j].Score == 0 {
				foundInvalid = true
				break
			}
		}
		if foundInvalid {
			groupResults.GroupResults[i].Score = 0
			for j := 0; j < numTests; j++ {
				groupResults.GroupResults[i].TestResults[j] = SingleTestResult{SKVerdict, "0", 0, 0, ""}
			}
			continue
		}

		// Otherwise, judge all tests within that group
		var wg sync.WaitGroup
		wg.Add(numTests)
		currGroupResult := SingleGroupResult{Score: -1, TestResults: make([]SingleTestResult, 0)}
		for testIndex := manifestInstance.Groups[i].TestIndices.Start; testIndex < manifestInstance.Groups[i].TestIndices.End; testIndex++ {
			go func(idx int) {
				currResult, err := waitForTestResult(manifestInstance, submissionID, targLang, userBinPath, idx, ijq, cjq, globalConfigInstance)
				if err != nil {
					log.Println(errors.Wrapf(err, "Error while judging submission %s at test %d", submissionID, idx))
				}
				currGroupResult.TestResults = append(currGroupResult.TestResults, *currResult)
				wg.Done()
			}(testIndex)
		}

		// Run grouper
		var grouperPath string
		if manifestInstance.Grouper != "custom" {
			grouperPath = path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "config", "defaultGroupers", manifestInstance.Grouper)
		} else {
			grouperPath = path.Join(manifestInstance.taskBasePath, "grouper")
		}
		log.Println(grouperPath)
		// TODO: group results
	}

	return nil, nil
}
