package grader

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/isolate"
)

const taskBasePath = "/home/szawinis/go/src/github.com/programming-in-th/grader/testing/" // IMPORTANT: CHANGE LATER

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

// SubmissionResult contains information about the result of a submission
type SubmissionResult struct {
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

	taskBasePath      string
	userBinBasePath   string
	inputsBasePath    string
	outputsBasePath   string
	solutionsBasePath string
	checkerPath       string
}

func convInterfaceSlicetoStringSlice(inp []interface{}) []string {
	ret := make([]string, 0)
	for _, v := range inp {
		ret = append(ret, v.(string))
	}
	return ret
}

func readManifestFromFile(manifestPath string) (*problemManifest, error) {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read manifest.json file at %s", manifestPath)
	}

	var v interface{}
	json.Unmarshal(manifestFileBytes, &v)
	data := v.(map[string]interface{})

	var manifestInstance problemManifest
	manifestInstance.id = data["id"].(string)
	manifestInstance.taskBasePath = path.Join(taskBasePath, manifestInstance.id)
	manifestInstance.timeLimit = data["timeLimit"].(float64)
	manifestInstance.memoryLimit = int(data["memoryLimit"].(float64))
	manifestInstance.langSupport = convInterfaceSlicetoStringSlice(data["langSupport"].([]interface{}))
	// TODO: simple indexing
	manifestInstance.testInputs = convInterfaceSlicetoStringSlice(data["testInputs"].([]interface{}))
	manifestInstance.testSolutions = convInterfaceSlicetoStringSlice(data["testSolutions"].([]interface{}))
	manifestInstance.compileCommands =
		func(inp map[string]interface{}) map[string][]string {
			ret := make(map[string][]string)
			for k, v := range inp {
				ret[k] = convInterfaceSlicetoStringSlice(v.([]interface{}))
			}
			return ret
		}(data["compileCommands"].(map[string]interface{}))

	manifestInstance.userBinBasePath = path.Join(manifestInstance.taskBasePath, "user_bin")
	manifestInstance.inputsBasePath = path.Join(manifestInstance.taskBasePath, "inputs")
	manifestInstance.outputsBasePath = path.Join(manifestInstance.taskBasePath, "outputs")
	manifestInstance.solutionsBasePath = path.Join(manifestInstance.taskBasePath, "solutions")
	manifestInstance.checkerPath = data["checkerPath"].(string)

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

// Compiles user source into one file according to arguments in manifest.json
func compileSubmission(submissionID string, problemID string, targLang string, sourceFilePaths []string, manifestInstance *problemManifest) (bool, string) {
	// This should make a copy
	compileCommands := manifestInstance.compileCommands[targLang]
	// Regexp gets contents of first [i] match including brackets
	reSrc := regexp.MustCompile(`\[(.*?)\]`)
	for i, arg := range compileCommands {
		// TODO: substitue this check with regex
		if len(arg) < 9 {
			continue
		}
		if arg[:9] == "$USER_SRC" {
			// Find $USER_SRC[0]
			val := reSrc.FindString(arg)
			val = strings.ReplaceAll(val, "[", "")
			val = strings.ReplaceAll(val, "]", "")
			if val == "" {
				log.Println("Compile error. Make sure user source files are of the form $USER_SRC[i], where i is the index of the desired source file specified in sourceFilePaths[]")
				return false, ""
			}
			index, err := strconv.ParseInt(val, 0, 0)
			if err != nil {
				log.Println("Compile error. Make sure i in $USER_SRC[i] is a valid integer. Actual value:", val)
				return false, ""
			}
			if int(index) >= len(sourceFilePaths) {
				log.Println("Compile error. Make sure i in $USER_SRC[i] is not out of bounds")
			}
			compileCommands[i] = sourceFilePaths[index]
		} else if arg[:9] == "$USER_BIN" {
			compileCommands[i] = path.Join(manifestInstance.userBinBasePath, submissionID)
			// TODO: check if chmod is needed
		}
	}
	err := exec.Command(compileCommands[0], compileCommands[1:]...).Run()
	if err != nil {
		log.Println("Compile error. Make sure source files are valid paths and manifest.json is using absolute paths only\n", err)
		return false, ""
	}
	return true, path.Join(manifestInstance.userBinBasePath, submissionID)
}

// GradeSubmission is the method that is called when the web server wants to request a problem to be judged
func GradeSubmission(submissionID string, problemID string, targLang string, sourceFilePaths []string, ijq *isolateJobQueue, cjq chan checkerJob) (*SubmissionResult, error) {
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
			return &SubmissionResult{CompileSuccessful: false}, nil
		}
	} else {
		// TODO: support more than one file
		// TODO: for now, just move the one file into the user_bin directory
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
				userBinPath,
				manifestInstance.timeLimit,
				manifestInstance.memoryLimit,
				path.Join(manifestInstance.inputsBasePath, manifestInstance.testInputs[i]),
				path.Join(manifestInstance.outputsBasePath, submissionID+"_output_"+strings.TrimSpace(strconv.Itoa(i))),
				ch,
			}
			log.Println("Pushing job into job queue:")
			log.Println(job)
			ijq.q <- job
			testResults[i] = <-ch
		}(i)
	}

	// Compile final submission results
	wg.Add(len(testResults))
	result := SubmissionResult{
		CompileSuccessful: true,
		TimeElapsed:       make([]float64, 0),
		MemoryUsage:       make([]int, 0),
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
				job := checkerJob{
					manifestInstance.checkerPath,
					manifestInstance.testInputs[i],
					path.Join(manifestInstance.outputsBasePath, submissionID+"_output_"+strconv.Itoa(i)),
					manifestInstance.testSolutions[i],
					ch,
				}
				cjq <- job
				checkedResults := <-ch
				if checkedResults.verdict == IEVerdict {
					log.Println(checkedResults.err)
					result.Verdicts = append(result.Verdicts, IEVerdict)
				} else {
					result.Verdicts = append(result.Verdicts, checkedResults.verdict)
				}
			}(i)
		}
	}

	// Waits for both isolate and checker job queues
	wg.Wait()

	return &result, nil
}
