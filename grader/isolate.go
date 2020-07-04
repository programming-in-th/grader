package grader

import (
	"log"
	"strconv"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/isolate"
)

type isolateTestResult struct {
	verdict isolate.RunVerdict
	metrics isolate.RunMetrics
	err     error
}

// WaitGroup should be started outside of this
func runIsolate(
	userBinPath string,
	timeLimit float64,
	memoryLimit int,
	inputPath string,
	outputPath string,
	isolateBinPath string,
	runnerScriptPath string,
	boxIDPool *safeBoxIDPool,
) isolateTestResult {
	// Find minimum excludant in box ID pool
	boxIDPool.Mux.Lock()
	boxID := 0
	for {
		used := boxIDPool.BoxIDs[boxID]
		if !used {
			boxIDPool.BoxIDs[boxID] = true
			break
		}
		boxID++
	}
	boxIDPool.Mux.Unlock()

	// Run a new isolate instance
	instance := isolate.NewInstance(
		isolateBinPath,
		boxID,
		userBinPath,
		1,
		"/tmp/tmp_isolate_grader_"+strconv.Itoa(boxID),
		timeLimit,
		timeLimit+1,
		memoryLimit,
		outputPath,
		inputPath,
		runnerScriptPath,
	)

	err := instance.Init()
	if err != nil {
		// Make sure we unlock box IDs
		boxIDPool.Mux.Lock()
		boxIDPool.BoxIDs[boxID] = false
		boxIDPool.Mux.Unlock()
		return isolateTestResult{verdict: isolate.IsolateRunOther, err: errors.Wrap(err, "Error initializing isolate instance")}
	}
	verdict, metrics := instance.Run()
	err = instance.Cleanup()
	if err != nil {
		log.Fatal("Error cleaning up isolate instance") // We make this fatal because if it keeps recurring, we can't recover from it
	}

	// Make sure box ID is unlocked
	// We don't defer this because the isolate instance MUST be cleaned up before others can use it
	boxIDPool.Mux.Lock()
	boxIDPool.BoxIDs[boxID] = false
	boxIDPool.Mux.Unlock()

	return isolateTestResult{verdict, metrics, nil}
}
