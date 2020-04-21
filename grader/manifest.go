package grader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"reflect"
	"sort"
	"sync"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/isolate"
)

const taskBasePath = "/home/szawinis/grader/tasks" // IMPORTANT: CHANGE LATER

// RunVerdict denotes the possible verdicts after running, including Correct, WA, TLE, RE and other errors
// This does not include CE
// Make sure to not confuse this with isolate.RunVerdict, which although is similar, is completely different
type RunVerdict string

const (
	// CorrectVerdict means the program passed the test
	CorrectVerdict RunVerdict = "P"
	// PartialVerdict means the program got the answer partially correct
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

// SubmissionResult contains information about the result of a submission
type SubmissionResult struct {
	compileSuccessful bool       // If this is set to false, the other fields will be undefined
	verdicts          [][]string // verdicts for each test case in each group
	timeElapsed       [][]string // time elapsed for each test case in each group
	memoryUsage       [][]string // memory usage for each test case in each group
}

// ProblemManifest is a type binding for the manifest.json stored in each problem's directory.
// This is mainly needed to validate the data in manifest.json
// IMPORTANT: json.Unmarshal will make sure all attributes in manifest.json match the following names (case-insensitive)
type ProblemManifest struct {
	id          string
	timeLimit   float64
	memoryLimit int
	fullScore   float64
	langSupport []string
	testInputs  []string // absolute/relative paths to inputs
	testOutputs []string // absolute/relative paths to solution outputs
	testGroups  [][]int  // indices of input/output files for tests in each group

	compileCommands map[string]string // Compile commands for each language
	execFilePath    string
	checkCommand    string
}

type safeBoxIDPool struct {
	boxIDs map[int]bool
	mux    sync.Mutex
}

type isolateTestResult struct {
	testIndex int
	verdict   isolate.RunVerdict
	metrics   *isolate.RunMetrics
}

func readManifestFromFile(manifestPath string) (*ProblemManifest, error) {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read manifest.json file at %s", manifestPath)
	}
	var manifestInstance ProblemManifest
	json.Unmarshal(manifestFileBytes, &manifestInstance)

	// Check if compile command keys matches language support
	compileCommandKeys := make([]string, len(manifestInstance.compileCommands))
	i := 0
	for k := range manifestInstance.compileCommands {
		compileCommandKeys[i] = k
		i++
	}
	if reflect.DeepEqual(compileCommandKeys, manifestInstance.langSupport) {
		return nil, errors.New("Manifest.json invalid: every language supported must have compile commands and vice versa")
	}

	return &manifestInstance, nil
}

func runIsolate(testIndex int,
	execFilePath string,
	timeLimit float64,
	memoryLimit int,
	inputPath string,
	outputPath string,
	boxIDPool *safeBoxIDPool,
	wg *sync.WaitGroup,
	ch chan isolateTestResult,
) {
	// Find minimum excludant in box ID pool
	boxIDPool.mux.Lock()
	mex := 0
	for {
		used, _ := boxIDPool.boxIDs[mex]
		if !used {
			break
		}
		mex++
	}
	boxIDPool.mux.Unlock()

	// Run a new isolate instance
	instance := isolate.NewInstance(
		"/usr/bin/isolate",
		mex,
		execFilePath,
		1,
		"/home/szawinis/meta", // CHANGE
		timeLimit,
		0, // CHANGE
		memoryLimit,
		"input",
		"output",
		"/home/szawinis/resulting_output",
		inputPath,
		outputPath,
	)

	// In case anything fails and we return early, we want to free up the current box ID
	defer func() {
		wg.Done()
		boxIDPool.mux.Unlock()
		boxIDPool.boxIDs[mex] = false
		boxIDPool.mux.Lock()
	}()

	err := instance.Init()
	if err != nil {
		fmt.Println("Error initializing isolate instance")
		ch <- isolateTestResult{testIndex: testIndex, verdict: isolate.IsolateRunOther, metrics: nil}
		return
	}
	verdict, metrics := instance.Run()
	err = instance.Cleanup()
	if err != nil {
		log.Fatal("Error cleaning up isolate instance") // We make this fatal because if it keeps recurring, we can't recover from it
	}
	ch <- isolateTestResult{testIndex: testIndex, verdict: verdict, metrics: metrics}
}

func gradeSubmission(problemID string, targLang string, submissionID string) (*SubmissionResult, error) {
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
	err = exec.Command(manifestInstance.compileCommands[targLang]).Run()
	if err != nil {
		return &SubmissionResult{compileSuccessful: false}, nil
	}

	// For each test case, run in isolate and send to checker
	boxIDPool := safeBoxIDPool{boxIDs: make(map[int]bool)}
	var wg sync.WaitGroup
	wg.Add(len(manifestInstance.testInputs))
	ch := make(chan isolateTestResult)
	var isolateResults []isolateTestResult
	for i := 0; i < len(manifestInstance.testInputs); i++ {
		go runIsolate(i,
			manifestInstance.execFilePath,
			manifestInstance.timeLimit,
			manifestInstance.memoryLimit,
			manifestInstance.testInputs[i],
			manifestInstance.testOutputs[i],
			&boxIDPool,
			&wg,
			ch)
	}
	for result := range ch {
		isolateResults = append(isolateResults, result)
	}
	wg.Wait()
	close(ch)
	sort.SliceStable(isolateResults, func(i, j int) bool { return isolateResults[i].testIndex < isolateResults[j].testIndex })

	// TODO: introduce worker cap

	// TODO: Get outputs from checker and determine verdict
	return nil, nil
}
