package grader

import (
	"log"
	"path"
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
	metrics *isolate.RunMetrics
	err     error
}

type isolateJob struct {
	submissionID    string
	problemID       string
	userProgramPath string
	timeLimit       float64
	memoryLimit     int
	inputPath       string
	resultChannel   chan isolateTestResult
}

type isolateJobQueue struct {
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
		job.userProgramPath,
		1,
		"tmp", // CHANGE
		job.timeLimit,
		0, // CHANGE
		job.memoryLimit,
		"input",
		"output",
		path.Join(taskBasePath, job.problemID, job.submissionID+"_output"),
		job.inputPath,
	)

	err := instance.Init()
	if err != nil {
		job.resultChannel <- isolateTestResult{isolate.IsolateRunOther, nil, errors.Wrap(err, "Error initializing isolate instance")}
		return
	}
	verdict, metrics := instance.Run()
	err = instance.Cleanup()
	if err != nil {
		log.Fatal("Error cleaning up isolate instance") // We make this fatal because if it keeps recurring, we can't recover from it
	}
	job.resultChannel <- isolateTestResult{verdict, metrics, nil}
	return
}

// TODO: "Done" channel to close it?
func isolateWorker(q chan isolateJob, boxIDPool *safeBoxIDPool, id int) {
	defer close(q)
	for {
		select {
		case job := <-q:
			// Find minimum excludant in box ID pool
			boxIDPool.mux.Lock()
			mex := 0
			for {
				used, _ := boxIDPool.boxIDs[mex]
				if !used {
					boxIDPool.boxIDs[mex] = true
					break
				}
				mex++
			}
			boxIDPool.mux.Unlock()
			log.Printf("Running job on worker: %d", id)
			log.Println(job)
			log.Println("Box id for job:", mex)
			runIsolate(job, mex)
			boxIDPool.mux.Lock()
			boxIDPool.boxIDs[mex] = false
			boxIDPool.mux.Unlock()
		}
	}
}

func NewIsolateJobQueue(maxWorkers int) isolateJobQueue {
	q := make(chan isolateJob)
	boxIDPool := safeBoxIDPool{boxIDs: make(map[int]bool)}
	for i := 0; i < maxWorkers; i++ {
		go isolateWorker(q, &boxIDPool, i)
	}
	return isolateJobQueue{q, &boxIDPool}
}

// TODO: public/private? also for factory function?
