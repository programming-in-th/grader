package grader

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"reflect"
	"sort"
	"sync"

	"github.com/pkg/errors"
)

const taskBasePath = "/home/szawinis/go/src/github.com/programming-in-th/grader/testing/" // IMPORTANT: CHANGE LATER

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
	compileSuccessful bool         // If this is set to false, the other fields will be undefined
	verdicts          []RunVerdict // verdicts for each test case in each group
	timeElapsed       []float64    // time elapsed for each test case in each group
	memoryUsage       []int        // memory usage for each test case in each group
}

// ProblemManifest is a type binding for the manifest.json stored in each problem's directory.
// This is mainly needed to validate the data in manifest.json
type ProblemManifest struct {
	id          string
	timeLimit   float64
	memoryLimit int
	langSupport []string
	testInputs  []string // names of input files (DO NOT specify path)
	testOutputs []string // names of output files (DO NOT specify path)
	// TODO: Add test groups

	compileCommands map[string][]string // Compile commands for each language
	execFilePath    string
	checkCommand    string
}

func convInterfaceSlicetoStringSlice(inp []interface{}) []string {
	ret := make([]string, 0)
	for _, v := range inp {
		ret = append(ret, v.(string))
	}
	return ret
}

func readManifestFromFile(manifestPath string) (*ProblemManifest, error) {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read manifest.json file at %s", manifestPath)
	}

	var v interface{}
	json.Unmarshal(manifestFileBytes, &v)
	data := v.(map[string]interface{})

	var manifestInstance ProblemManifest
	manifestInstance.id = data["id"].(string)
	manifestInstance.timeLimit = data["timeLimit"].(float64)
	manifestInstance.memoryLimit = int(data["memoryLimit"].(float64))
	manifestInstance.langSupport = convInterfaceSlicetoStringSlice(data["langSupport"].([]interface{}))
	manifestInstance.testInputs = convInterfaceSlicetoStringSlice(data["testInputs"].([]interface{}))
	manifestInstance.testOutputs = convInterfaceSlicetoStringSlice(data["testOutputs"].([]interface{}))
	manifestInstance.compileCommands =
		func(inp map[string]interface{}) map[string][]string {
			ret := make(map[string][]string)
			for k, v := range inp {
				ret[k] = convInterfaceSlicetoStringSlice(v.([]interface{}))
			}
			return ret
		}(data["compileCommands"].(map[string]interface{}))
	manifestInstance.execFilePath = data["execFilePath"].(string)
	manifestInstance.checkCommand = data["checkCommand"].(string)

	// Check if compile command keys matches language support
	compileCommandKeys := make([]string, len(manifestInstance.compileCommands))
	i := 0
	for k := range manifestInstance.compileCommands {
		compileCommandKeys[i] = k
		i++
	}

	sort.Slice(compileCommandKeys, func(i, j int) bool { return compileCommandKeys[i] < compileCommandKeys[j] })
	sort.Slice(manifestInstance.langSupport, func(i, j int) bool { return manifestInstance.langSupport[i] < manifestInstance.langSupport[j] })

	if !reflect.DeepEqual(compileCommandKeys, manifestInstance.langSupport) {
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
	// TODO: Compile fails without absolute paths
	if len(manifestInstance.compileCommands) != 0 {
		err = exec.Command(manifestInstance.compileCommands[targLang][0], manifestInstance.compileCommands[targLang][1:]...).Run()
		if err != nil {
			log.Println("Compile error:", err)
			return &SubmissionResult{compileSuccessful: false}, nil
		}
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
			job := isolateJob{
				execFilePath:  manifestInstance.execFilePath,
				timeLimit:     manifestInstance.timeLimit,
				memoryLimit:   manifestInstance.memoryLimit,
				testInput:     path.Join(taskBasePath, problemID, "inputs", manifestInstance.testInputs[i]),
				resultChannel: ch,
			}
			log.Println("Pushing job into job queue:")
			log.Println(job)
			jq.q <- job
			testResults[i] = <-ch
		}(i)
	}
	wg.Wait()

	// TODO: Get outputs from checker and determine verdict
	result := SubmissionResult{
		compileSuccessful: true,
		timeElapsed:       make([]float64, 0),
		memoryUsage:       make([]int, 0),
	}
	for i := 0; i < len(testResults); i++ {
		result.timeElapsed = append(result.timeElapsed, testResults[i].metrics.TimeElapsed)
		result.memoryUsage = append(result.memoryUsage, testResults[i].metrics.MemoryUsage)
	}

	return &result, nil
}
