package grader

import (
	"log"
	"sync"

	"github.com/programming-in-th/grader/isolate"
)

type safeBoxIDPool struct {
	boxIDs map[int]bool
	mux    sync.Mutex
}

type isolateTestResult struct {
	verdict isolate.RunVerdict
	metrics *isolate.RunMetrics
}

type isolateJob struct {
	execFilePath  string
	timeLimit     float64
	memoryLimit   int
	testInput     string
	resultChannel chan isolateTestResult
}

type jobQueue struct {
	q         chan isolateJob
	boxIDPool *safeBoxIDPool
}

// WaitGroup should be started outside of this
func runIsolate(
	job isolateJob,
	boxID int,
) {

	// Run a new isolate instance
	instance := isolate.NewInstance(
		"/usr/bin/isolate",
		boxID,
		job.execFilePath,
		1,
		"/home/szawinis/meta", // CHANGE
		job.timeLimit,
		0, // CHANGE
		job.memoryLimit,
		"input",
		"output",
		"/home/szawinis/resulting_output",
		job.testInput,
	)

	err := instance.Init()
	if err != nil {
		log.Println("Error initializing isolate instance")
		job.resultChannel <- isolateTestResult{verdict: isolate.IsolateRunOther, metrics: nil}
		return
	}
	verdict, metrics := instance.Run()
	err = instance.Cleanup()
	if err != nil {
		log.Fatal("Error cleaning up isolate instance") // We make this fatal because if it keeps recurring, we can't recover from it
	}
	job.resultChannel <- isolateTestResult{verdict: verdict, metrics: metrics}
}

func NewJobQueue(maxWorkers int) jobQueue {
	q := make(chan isolateJob, maxWorkers)
	boxIDPool := safeBoxIDPool{boxIDs: make(map[int]bool)}
	go func() {
		defer close(q)
		for {
			select { // TODO: done channel
			case job := <-q:
				// log.Println(job)
				// Find minimum excludant in box ID pool
				boxIDPool.mux.Lock()
				mex := 0
				for {
					used, _ := boxIDPool.boxIDs[mex]
					if !used {
						break
					}
					mex++
				}
				boxIDPool.mux.Unlock()
				runIsolate(job, mex)
				boxIDPool.mux.Lock()
				boxIDPool.boxIDs[mex] = false
				boxIDPool.mux.Unlock()
			}
		}
	}()
	return jobQueue{q: q, boxIDPool: &boxIDPool}
}

// TODO: We won't actually need test output
// TODO: public/private? also for factory function?
