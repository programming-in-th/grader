package grader

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/programming-in-th/grader/api"
	"github.com/programming-in-th/grader/conf"
)

func TestReadManifest(t *testing.T) {
	gc := conf.InitConfig("/home/proggrader/testcases")
	pathTo := path.Join(gc.BasePath, "tasks", "o61_may08_estate", "manifest.json")
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
	data, _ := ioutil.ReadFile("/home/proggrader/test.cpp")
	src[0] = string(data)
	gc := conf.InitConfig("/home/proggrader/testcases")
	done := make(chan bool)
	jobQueue := NewGradingJobQueue(1, done, gc)
	ch := make(chan api.SyncUpdate)
	go func() {
		for {
			message := <-ch
			t.Log(message)
		}
	}()
	err := GradeSubmission("submissionID", "o61_may08_estate", "cpp14", src, jobQueue, ch, gc)
	if err != nil {
		t.Error("Error grading submission")
	}

}

func TestWaitForTestResult(t *testing.T) {
	gc := conf.InitConfig("/home/proggrader/testcases")
	pathToManifest := path.Join(gc.BasePath, "tasks", "o61_may08_estate", "manifest.json")
	manifestInstance, _ := readManifestFromFile(pathToManifest, gc)
	boxIDPool := safeBoxIDPool{BoxIDs: make(map[int]bool)}
	result := waitForTestResult(manifestInstance, "submissionID", "cpp14", "/home/proggrader/a.out", 18, gc, &boxIDPool)
	t.Log(result)
}

func TestRunIsolate(t *testing.T) {
	boxIDPool := safeBoxIDPool{BoxIDs: make(map[int]bool)}
	result := runIsolate("/home/proggrader/a.out",
		3,
		512000,
		"/home/proggrader/testcases/tasks/o61_may08_estate/inputs/19.in",
		"/home/proggrader/output",
		"/usr/local/bin/isolate",
		"/home/proggrader/testcases/config/runnerScripts/cpp14",
		&boxIDPool,
	)
	t.Log(result)
}

func TestCompile(t *testing.T) {
	gc := conf.InitConfig("/home/szawinis/testing")
	src := make([]string, 1)
	src[0] = "/home/szawinis/testing/rectsum_test.cpp"
	successful, binPath := compileSubmission("submissionID", "rectsum", "cpp14", src, gc)
	t.Log("Compile success?", successful)
	t.Log("User binary path:", binPath)
}

// TODO: Try to go for a more modular testing framework
