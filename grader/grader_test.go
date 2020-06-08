package grader

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/programming-in-th/grader/conf"
)

func TestReadManifest(t *testing.T) {
	gc := conf.InitConfig("/home/szawinis/testing")
	pathTo := path.Join(gc.BasePath, "tasks", "rectsum", "manifest.json")
	t.Log("Path to manifest.json: ", pathTo)
	manifestInstance, err := readManifestFromFile(pathTo, gc)
	if err != nil {
		t.Error("Can't read manifest.json\n", err)
	}
	t.Log(manifestInstance)
	t.Log(manifestInstance.DefaultLimits)
}

// Tests whole grading pipeline
func TestGradeSubmission(t *testing.T) {
	src := make([]string, 1)
	data, _ := ioutil.ReadFile("/home/szawinis/testing/rectsum_test.cpp")
	src[0] = string(data)
	gc := conf.InitConfig("/home/szawinis/testing")
	jobQueueDone := make(chan bool)
	jobQueue := NewIsolateJobQueue(1, jobQueueDone, "/usr/bin/isolate")
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := NewCheckerJobQueue(5, checkerJobQueueDone, gc)
	submissionResult, err := GradeSubmission("submissionID", "rectsum", "cpp14", src, &jobQueue, checkerJobQueue, gc)
	if err != nil {
		t.Error("Error grading submission")
	}
	t.Log(submissionResult)
	jobQueueDone <- true
	checkerJobQueueDone <- true

}

func TestCompile(t *testing.T) {
	gc := conf.InitConfig("/home/szawinis/testing")
	src := make([]string, 1)
	src[0] = "/home/szawinis/testing/rectsum_test.cpp"
	successful, binPath := compileSubmission("submissionID", "rectsum", src, gc.Glob.CompileConfiguration[0].CompileCommands)
	t.Log("Compile success?", successful)
	t.Log("User binary path:", binPath)
}

// TODO: Try to go for a more modular testing framework
