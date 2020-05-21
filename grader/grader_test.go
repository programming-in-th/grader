package grader

import (
	"os"
	"path"
	"testing"
)

func TestReadManifest(t *testing.T) {
	pathTo := path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks", "asdf", "manifest.json")
	t.Log("Path to manifest.json: ", pathTo)
	manifestInstance, err := readManifestFromFile(pathTo)
	if err != nil {
		t.Error("Can't read manifest.json\n", err)
	}
	t.Log(manifestInstance)
}

// Tests whole grading pipeline
func TestGradeSubmission(t *testing.T) {
	jobQueueDone := make(chan bool)
	jobQueue := NewIsolateJobQueue(2, jobQueueDone)
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := NewCheckerJobQueue(5, checkerJobQueueDone)
	src := make([]string, 1)
	src[0] = "/home/szawinis/go/src/github.com/programming-in-th/grader/testing/asdf/ac.cpp"
	submissionResult, err := GradeSubmission("submissionID", "asdf", "cpp", src, &jobQueue, checkerJobQueue)
	if err != nil {
		t.Error("Error grading submission")
	}
	t.Log(submissionResult)
	jobQueueDone <- true
	checkerJobQueueDone <- true

}

func TestCompile(t *testing.T) {
	src := make([]string, 1)
	src[0] = "/home/szawinis/go/src/github.com/programming-in-th/grader/testing/asdf/ac.cpp"
	manifest, _ := readManifestFromFile("/home/szawinis/go/src/github.com/programming-in-th/grader/testing/asdf/manifest.json")
	successful, binPath := compileSubmission("submissionID", "asdf", "cpp", src, manifest)
	t.Log("Compile success?", successful)
	t.Log("User binary path:", binPath)
}

// TODO: Try to go for a more modular testing framework
