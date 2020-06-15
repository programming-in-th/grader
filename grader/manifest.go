package grader

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/api"
	"github.com/programming-in-th/grader/conf"
	"github.com/programming-in-th/grader/util"
)

const BASE_TMP_PATH = "/tmp/grader"
const BASE_SRC_PATH = BASE_TMP_PATH + "/source"

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
	Verdict string
	Score   string
	Time    float64
	Memory  int
	Message string
}

// SingleGroupResults denotes the metrics for one single group (comprised of many tests)
type SingleGroupResult struct {
	Score       float64
	FullScore   float64
	TestResults []SingleTestResult
}

// GroupedSubmissionResult denotes the test results for all groups
type GroupedSubmissionResult struct {
	CompileSuccessful bool
	GroupedSuccessful bool
	Score             float64
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

// taskManifest is a type binding for the manifest.json stored in each task's directory.
// This is mainly needed to validate the data in manifest.json
type taskManifest struct {
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

func readManifestFromFile(manifestPath string, config conf.Config) (taskManifest, error) {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return taskManifest{}, errors.Wrapf(err, "Failed to read manifest.json file at %s", manifestPath)
	}

	var manifestInstance taskManifest
	err = json.Unmarshal(manifestFileBytes, &manifestInstance)
	if err != nil {
		return taskManifest{}, errors.Wrapf(err, "Failed to unmarshal manifest.json from file at %s", manifestPath)
	}

	// Decrease indices for easier handling
	for i := 0; i < len(manifestInstance.Groups); i++ {
		for j := 0; j < len(manifestInstance.Groups[i].Dependencies); j++ {
			manifestInstance.Groups[i].Dependencies[j] -= 1
		}
		manifestInstance.Groups[i].TestIndices.Start -= 1
		// Leave .End as is because we want it to be exclusive
	}

	manifestInstance.taskBasePath = path.Join(config.BasePath, "tasks", manifestInstance.ID)
	manifestInstance.inputsBasePath = path.Join(manifestInstance.taskBasePath, "inputs")
	manifestInstance.solutionsBasePath = path.Join(manifestInstance.taskBasePath, "solutions")
	manifestInstance.numTests = manifestInstance.Groups[len(manifestInstance.Groups)-1].TestIndices.End

	return manifestInstance, nil
}

// GradeSubmission is the method that is called when the web server wants to request a task to be judged
func GradeSubmission(submissionID string,
	taskID string,
	targLang string,
	code []string,
	gradingJobChannel chan GradingJob,
	config conf.Config) (*GroupedSubmissionResult, error) {

	api.SendCompilingMessage(submissionID)

	taskBasePath := path.Join(config.BasePath, "tasks")

	langConfig := conf.GetLangCompileConfig(config, targLang)
	if langConfig == nil {
		return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.New("Language not supported")
	}

	if len(code) == 0 {
		return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.New("Code passed in is empty")
	}

	// Copy source code into tmp directory
	srcFilePaths := make([]string, len(code))
	for i := 0; i < len(code); i++ {
		srcFilePaths[i] = path.Join(BASE_SRC_PATH, submissionID+"_"+strconv.Itoa(i)+"."+langConfig.Extension)
		err := ioutil.WriteFile(srcFilePaths[i], []byte(code[i]), 0644)
		if err != nil {
			return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.Wrapf(err, "Cannot copy source code into tmp directory: %s", srcFilePaths[i])
		}
	}

	// Remove source files after judging
	defer func() {
		for _, file := range srcFilePaths {
			os.Remove(file)
		}
	}()

	// Locate manifest file and read it
	manifestPath := path.Join(taskBasePath, taskID, "manifest.json")
	manifestInstance, err := readManifestFromFile(manifestPath, config)
	if err != nil {
		return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.Wrap(err, "Error reading manifest file")
	}

	// Create tmp directory for submission
	err = util.CreateDirIfNotExist(path.Join(BASE_TMP_PATH, submissionID))
	if err != nil {
		return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.Wrap(err, "Error creating working tmp folder")
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
		return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.New("Language not supported")
	}

	// Add compile files to srcFilePaths after defer statement so it doesn't delete
	if _, exists := manifestInstance.CompileFiles[targLang]; exists {
		for _, compileFile := range manifestInstance.CompileFiles[targLang] {
			srcFilePaths = append(srcFilePaths, path.Join(config.BasePath, "tasks", taskID, compileFile))
		}
	}

	// Compile program and return CE if fail
	// TODO: Handle other languages that don't need compiling
	// TODO: Compile fails without absolute paths
	var userBinPath string
	if langConfig.CompileCommands != nil && len(langConfig.CompileCommands) != 0 {
		var compileSuccessful bool
		compileSuccessful, userBinPath = compileSubmission(submissionID, taskID, srcFilePaths, langConfig.CompileCommands)
		if !compileSuccessful {
			return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, nil
		}
	} else {
		if len(srcFilePaths) > 1 {
			return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.New("Language not supported")
		}
		err := os.Rename(srcFilePaths[0], path.Join(BASE_TMP_PATH, submissionID, "bin"))
		if err != nil {
			return &GroupedSubmissionResult{CompileSuccessful: false, GroupedSuccessful: false}, errors.Wrap(err, "Failed to move source file into user_bin")
		}
		// TODO: support more than one file. For now, just move the one file into the user_bin directory
	}

	// Remove user output file to not clutter up disk
	defer func() {
		os.RemoveAll(path.Join(BASE_TMP_PATH, submissionID))
		os.Remove(userBinPath)
	}()

	log.Printf("%#v", manifestInstance.Groups)

	groupResults := GroupedSubmissionResult{CompileSuccessful: true, GroupedSuccessful: true, Score: 0}
	for i := 0; i < len(manifestInstance.Groups); i++ {

		currGroupResult := SingleGroupResult{
			Score:       -1,
			FullScore:   manifestInstance.Groups[i].FullScore,
			TestResults: make([]SingleTestResult, manifestInstance.Groups[i].TestIndices.End-manifestInstance.Groups[i].TestIndices.Start),
		}

		// If a dependency is not satisfied, skip the entire group
		foundInvalid := false
		numTests := manifestInstance.Groups[i].TestIndices.End - manifestInstance.Groups[i].TestIndices.Start
		for _, j := range manifestInstance.Groups[i].Dependencies {
			if !groupResults.GroupedSuccessful || groupResults.GroupResults[j].Score == 0 {
				foundInvalid = true
				break
			}
		}
		if foundInvalid {
			currGroupResult.Score = 0
			for j := 0; j < numTests; j++ {
				currGroupResult.TestResults = append(currGroupResult.TestResults, SingleTestResult{conf.SKVerdict, "0", 0, 0, ""})
			}
			groupResults.GroupResults = append(groupResults.GroupResults, currGroupResult)
			continue
		}

		// Otherwise, judge all tests within that group
		var wg sync.WaitGroup
		wg.Add(numTests)
		resultChannel := make(chan SingleTestResult)
		for testIndex := manifestInstance.Groups[i].TestIndices.Start; testIndex < manifestInstance.Groups[i].TestIndices.End; testIndex++ {
			go func(idx int) {
				gradingJobChannel <- GradingJob{manifestInstance, submissionID, targLang, userBinPath, idx, resultChannel}
				currResult := <-resultChannel
				currGroupResult.TestResults[idx-manifestInstance.Groups[i].TestIndices.Start] = currResult
				log.Printf("Test #%d done", idx)
				wg.Done()
			}(testIndex)
		}
		wg.Wait()

		// Run grouper
		var grouperPath string
		if manifestInstance.Grouper != "custom" {
			grouperPath = path.Join(config.BasePath, "config", "defaultGroupers", manifestInstance.Grouper)
		} else {
			grouperPath = path.Join(manifestInstance.taskBasePath, "grouper")
		}

		// TODO: use same worker model as isolate and checker?
		grouperOutput, err := exec.Command(grouperPath,
			submissionID,
			strconv.FormatFloat(manifestInstance.Groups[i].FullScore, 'f', -1, 64),
			strconv.Itoa(manifestInstance.Groups[i].TestIndices.Start+1),
			strconv.Itoa(manifestInstance.Groups[i].TestIndices.End)).Output()

		if err != nil {
			log.Print(errors.Wrapf(err, "Grouper failed for task %s on submission ID %s", manifestInstance.ID, submissionID))
			groupResults.GroupedSuccessful = false
			grouperOutput = []byte("0") // fall through
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(string(grouperOutput)), 64)
		if err != nil {
			log.Print(errors.Wrapf(err, "Grouper failed for task %s on submission ID %s", manifestInstance.ID, submissionID))
			groupResults.GroupedSuccessful = false
			score = 0
		}
		currGroupResult.Score = score
		groupResults.Score += score
		groupResults.GroupResults = append(groupResults.GroupResults, currGroupResult)
		api.SendGroupResult(submissionID, currGroupResult)
	}

	api.SendJudgingCompleteMessage(submissionID)

	return &groupResults, nil
}
