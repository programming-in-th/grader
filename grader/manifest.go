package grader

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/api"
	"github.com/programming-in-th/grader/conf"
	"github.com/programming-in-th/grader/util"
)

const BASE_TMP_PATH = "/tmp/grader"
const BASE_SRC_PATH = BASE_TMP_PATH + "/source"

// SingleTestResult denotes the metrics for one single test
type SingleTestResult struct {
	Verdict string
	Score   string
	Time    int
	Memory  int
	Message string
}

// SingleGroupResult denotes the metrics for one single group (comprised of many tests)
type SingleGroupResult struct {
	Score       float64
	FullScore   float64
	TestResults []SingleTestResult
}

type PrefixGroupResult struct {
	Score           float64
	Time            int
	Memory          int
	CurrGroupResult SingleGroupResult
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

	// Decrease indices for easier handling and round full score
	for i := 0; i < len(manifestInstance.Groups); i++ {
		for j := 0; j < len(manifestInstance.Groups[i].Dependencies); j++ {
			manifestInstance.Groups[i].Dependencies[j] -= 1
		}
		manifestInstance.Groups[i].FullScore = math.Round(manifestInstance.Groups[i].FullScore*100) / 100
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
	syncUpdateChannel chan api.SyncUpdate,
	config conf.Config) error {

	api.SendCompilingMessage(submissionID, syncUpdateChannel)

	taskBasePath := path.Join(config.BasePath, "tasks")

	langConfig := conf.GetLangCompileConfig(config, targLang)
	if langConfig == nil {
		api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
		return errors.New("Language not supported")
	}

	if len(code) == 0 {
		api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
		return errors.New("Code passed in is empty")
	}

	// Copy source code into tmp directory
	srcFilePaths := make([]string, len(code))
	for i := 0; i < len(code); i++ {
		srcFilePaths[i] = path.Join(BASE_SRC_PATH, submissionID+"_"+strconv.Itoa(i)+"."+langConfig.Extension)
		err := ioutil.WriteFile(srcFilePaths[i], []byte(code[i]), 0644)
		if err != nil {
			api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
			return errors.Wrapf(err, "Cannot copy source code into tmp directory: %s", srcFilePaths[i])
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
		api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
		return errors.Wrap(err, "Error reading manifest file")
	}

	// Create tmp directory for submission
	err = util.CreateDirIfNotExist(path.Join(BASE_TMP_PATH, submissionID))
	if err != nil {
		api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
		return errors.Wrap(err, "Error creating working tmp folder")
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
		api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
		return errors.New("Language not supported")
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
			api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
			return nil
		}
	} else {
		if len(srcFilePaths) > 1 {
			api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
			return errors.New("Language not supported")
		}
		err := os.Rename(srcFilePaths[0], path.Join(BASE_TMP_PATH, submissionID, "bin"))
		if err != nil {
			api.SendCompilationErrorMessage(submissionID, syncUpdateChannel)
			return errors.Wrap(err, "Failed to move source file into user_bin")
		}
		// TODO: support more than one file. For now, just move the one file into the user_bin directory
	}

	// Remove user output file to not clutter up disk
	defer func() {
		os.RemoveAll(path.Join(BASE_TMP_PATH, submissionID))
		os.Remove(userBinPath)
	}()

	log.Printf("%#v", manifestInstance.Groups)

	groupResults := make([]SingleGroupResult, 0)
	runningScore := 0.0
	runningTime := 0
	runningMemory := 0
	allGroupsGroupedSucessfully := true
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
			if !allGroupsGroupedSucessfully || groupResults[j].Score == 0 {
				foundInvalid = true
				break
			}
		}
		if foundInvalid {
			currGroupResult.Score = 0
			for j := 0; j < numTests; j++ {
				currGroupResult.TestResults = append(currGroupResult.TestResults, SingleTestResult{conf.SKVerdict, "0", 0, 0, ""})
			}
			groupResults = append(groupResults, currGroupResult)
			continue
		}

		// Otherwise, judge all tests within that group
		resultChannel := make(chan SingleTestResult)
		willSkip := false
		for testIndex := manifestInstance.Groups[i].TestIndices.Start; testIndex < manifestInstance.Groups[i].TestIndices.End; testIndex++ {
			if !willSkip {
				gradingJobChannel <- GradingJob{manifestInstance, submissionID, targLang, userBinPath, testIndex, resultChannel}
				currResult := <-resultChannel
				currGroupResult.TestResults[testIndex-manifestInstance.Groups[i].TestIndices.Start] = currResult
				api.SendJudgedTestMessage(submissionID, testIndex, syncUpdateChannel)
				if currResult.Verdict != conf.ACVerdict && currResult.Verdict != conf.PartialVerdict {
					willSkip = true
				}
			} else {
				currGroupResult.TestResults[testIndex-manifestInstance.Groups[i].TestIndices.Start] = SingleTestResult{conf.SKVerdict, "0", 0, 0, ""}
				api.SendJudgedTestMessage(submissionID, testIndex, syncUpdateChannel)
			}
		}

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
			allGroupsGroupedSucessfully = false
			grouperOutput = []byte("0") // fall through
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(string(grouperOutput)), 64)
		if err != nil {
			log.Print(errors.Wrapf(err, "Grouper failed for task %s on submission ID %s", manifestInstance.ID, submissionID))
			allGroupsGroupedSucessfully = false
			score = 0
		}

		// Compute max group time and memory
		maxCurrGroupTime := 0
		maxCurrGroupMemory := 0
		for _, currTestResult := range currGroupResult.TestResults {
			if currTestResult.Time > maxCurrGroupTime {
				maxCurrGroupTime = currTestResult.Time
				maxCurrGroupMemory = currTestResult.Memory
			}
		}

		// Update metrics for prefix of groups
		runningScore += currGroupResult.Score
		if runningTime > maxCurrGroupTime {
			runningTime = maxCurrGroupTime
		}
		if runningMemory > maxCurrGroupMemory {
			runningMemory = maxCurrGroupMemory
		}

		currGroupResult.Score = math.Round(score*100) / 100 // CAREFUL: round of AFTER adding to running score
		groupResults = append(groupResults, currGroupResult)

		currPrefixGroupResult := PrefixGroupResult{runningScore, runningTime, runningMemory, currGroupResult}
		api.SendPrefixGroupResult(submissionID, currPrefixGroupResult, syncUpdateChannel)
	}

	api.SendJudgingCompleteMessage(submissionID, syncUpdateChannel)

	return nil
}
