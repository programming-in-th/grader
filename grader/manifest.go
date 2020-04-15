package grader

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// ProblemManifest is a type binding for the manifest.json stored in each problem's directory.
// This is mainly needed to validate the data in manifest.json
// IMPORTANT: json.Unmarshal will make sure all attributes in manifest.json match the following names (case-insensitive)
type ProblemManifest struct {
	id                   string
	timeLimit            float64
	memoryLimit          int
	fullScore            float64
	langSupport          []string
	testCaseDir          string
	groupFullScores      []float64
	groupTestCaseIndices []int
	isolateInputName     string
	isolateOutputName    string
}

func readManifestFromFile(manifestPath string) *ProblemManifest {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		log.Fatalf("Failed to read manifest.json file at %s", manifestPath)
	}
	var manifestInstance ProblemManifest
	json.Unmarshal(manifestFileBytes, &manifestInstance)
	return &manifestInstance
}

func gradeSubmission(problemID string, lang string, submissionID string) {
	
}
