package grader

import (
	"path"
	"testing"
)

func TestReadManifest(t *testing.T) {
	pathTo := path.Join(taskBasePath, "asdf", "manifest.json")
	t.Log("Path to manifest.json: ", pathTo)
	manifestInstance, err := readManifestFromFile(pathTo)
	if err != nil {
		t.Error("Can't read manifest.json\n", err)
	}
	t.Log(manifestInstance)
}

// Tests whole grading pipeline
func TestGradeSubmission(t *testing.T) {
	jobQueue := NewIsolateJobQueue(5)
	checkerJobQueue := NewCheckerJobQueue(5)
	submissionResult, err := GradeSubmission("test_sg", "asdf", "cpp", &jobQueue, checkerJobQueue)
	if err != nil {
		t.Error("Error grading submission")
	}
	t.Log(submissionResult)
}

func TestChecker(t *testing.T) {
	// TODO: test just checker functionality
}

// TODO: Try to go for a more modular testing framework
