package grader

import (
	"os"
	"path"
	"testing"
)

func TestReadManifest(t *testing.T) {
	pathTo := path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "tasks", "rectsum", "manifest.json")
	t.Log("Path to manifest.json: ", pathTo)
	manifestInstance, err := readManifestFromFile(pathTo)
	if err != nil {
		t.Error("Can't read manifest.json\n", err)
	}
	t.Log(manifestInstance)
}

// Tests whole grading pipeline
func TestGradeSubmission(t *testing.T) {
	src := make([]string, 1)
	src[0] = "/home/szawinis/testing/rectsum_test.cpp"
	gc, err := ReadGlobalConfig(path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "globalConfig.json"))
	if err != nil {
		t.Error("Error grading submission: can't read global config")
	}
	jobQueueDone := make(chan bool)
	jobQueue := NewIsolateJobQueue(2, jobQueueDone, "/usr/bin/isolate")
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := NewCheckerJobQueue(5, checkerJobQueueDone, gc)
	submissionResult, err := GradeSubmission("submissionID", "rectsum", "cpp", src, &jobQueue, checkerJobQueue, gc)
	if err != nil {
		t.Error("Error grading submission")
	}
	t.Log(submissionResult)
	jobQueueDone <- true
	checkerJobQueueDone <- true

}

func TestCompile(t *testing.T) {
	gc, err := ReadGlobalConfig(path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), "globalConfig.json"))
	if err != nil {
		t.Error("Error grading submission: can't read global config")
	}
	src := make([]string, 1)
	src[0] = "/home/szawinis/testing/rectsum_test.cpp"
	successful, binPath := compileSubmission("submissionID", "rectsum", src, gc.CompileConfiguration[0].CompileCommands)
	t.Log("Compile success?", successful)
	t.Log("User binary path:", binPath)
}

// TODO: Try to go for a more modular testing framework
