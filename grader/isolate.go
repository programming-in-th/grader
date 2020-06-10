package grader

import (
	"log"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/isolate"
)

type safeBoxIDPool struct {
	boxIDs map[int]bool
	mux    sync.Mutex
}

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
	boxIDPool *safeBoxIDPool,
) isolateTestResult {
	// Find minimum excludant in box ID pool
	boxIDPool.mux.Lock()
	boxID := 0
	for {
		used := boxIDPool.boxIDs[boxID]
		if !used {
			boxIDPool.boxIDs[boxID] = true
			break
		}
		boxID++
	}
	boxIDPool.mux.Unlock()

	log.Printf("Box id for job: %d", boxID)

	// Run a new isolate instance
	instance := isolate.NewInstance(
		isolateBinPath,
		boxID,
		userBinPath,
		1,
		"/tmp/tmp_isolate_grader_"+strconv.Itoa(boxID),
		timeLimit,
		0, // TODO: CHANGE
		memoryLimit,
		outputPath,
		inputPath,
	)

	err := instance.Init()
	if err != nil {
		// Make sure we unlock box IDs
		boxIDPool.mux.Lock()
		boxIDPool.boxIDs[boxID] = false
		boxIDPool.mux.Unlock()
		return isolateTestResult{verdict: isolate.IsolateRunOther, err: errors.Wrap(err, "Error initializing isolate instance")}
	}
	verdict, metrics := instance.Run()
	err = instance.Cleanup()
	if err != nil {
		log.Fatal("Error cleaning up isolate instance") // We make this fatal because if it keeps recurring, we can't recover from it
	}

	// Make sure box ID is unlocked
	// We don't defer this because the isolate instance MUST be cleaned up before others can use it
	boxIDPool.mux.Lock()
	boxIDPool.boxIDs[boxID] = false
	boxIDPool.mux.Unlock()

	return isolateTestResult{verdict, metrics, nil}
}
