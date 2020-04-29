package grader

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"path"
	"reflect"
	"sync"

	"github.com/pkg/errors"
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
	compileSuccessful bool     // If this is set to false, the other fields will be undefined
	verdicts          []string // verdicts for each test case in each group
	timeElapsed       []string // time elapsed for each test case in each group
	memoryUsage       []string // memory usage for each test case in each group
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
	// TODO: Add test groups

	compileCommands map[string]string // Compile commands for each language
	execFilePath    string
	checkCommand    string
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

func GradeSubmission(problemID string, targLang string, jq *jobQueue) (*SubmissionResult, error) {
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
	err = exec.Command(manifestInstance.compileCommands[targLang]).Run()
	if err != nil {
		return &SubmissionResult{compileSuccessful: false}, nil
	}

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
			jq.q <- isolateJob{
				execFilePath:  manifestInstance.execFilePath,
				timeLimit:     manifestInstance.timeLimit,
				memoryLimit:   manifestInstance.memoryLimit,
				testInput:     manifestInstance.testInputs[i],
				resultChannel: ch,
			}
			testResults[i] = <-ch
		}(i)
	}
	wg.Wait()

	// TODO: Get outputs from checker and determine verdict
	return nil, nil
}
