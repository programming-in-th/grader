package grader

import (
	"log"
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
}

type isolateJob struct {
	userProgramPath string
	timeLimit       float64
	memoryLimit     int
	inputPath       string
	resultChannel   chan isolateTestResult
}

type jobQueue struct {
	q         chan isolateJob
	boxIDPool *safeBoxIDPool
}

// WaitGroup should be started outside of this
func runIsolate(
	job isolateJob,
	boxID int,
) error {

	// Run a new isolate instance
	instance := isolate.NewInstance(
		"/usr/bin/isolate",
		boxID,
		job.userProgramPath,
		1,
		"/home/szawinis/meta", // CHANGE
		job.timeLimit,
		0, // CHANGE
		job.memoryLimit,
		"input",
		"output",
		"/home/szawinis/resulting_output",
		job.inputPath,
	)

	err := instance.Init()
	if err != nil {
		job.resultChannel <- isolateTestResult{verdict: isolate.IsolateRunOther, metrics: nil}
		return errors.Wrap(err, "Error initializing isolate instance")
	}
	verdict, metrics := instance.Run()
	err = instance.Cleanup()
	if err != nil {
		log.Fatal("Error cleaning up isolate instance") // We make this fatal because if it keeps recurring, we can't recover from it
	}
	job.resultChannel <- isolateTestResult{verdict: verdict, metrics: metrics}
	return nil
}

// TODO: "Done" channel to close it?
func worker(q chan isolateJob, boxIDPool *safeBoxIDPool, id int) {
	defer close(q)
	for {
		select { // TODO: done channel
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
			err := runIsolate(job, mex)
			if err != nil {
				log.Fatalf("Error during judging %s", err)
			}
			boxIDPool.mux.Lock()
			boxIDPool.boxIDs[mex] = false
			boxIDPool.mux.Unlock()
		}
	}
}

func NewJobQueue(maxWorkers int) jobQueue {
	q := make(chan isolateJob, maxWorkers)
	boxIDPool := safeBoxIDPool{boxIDs: make(map[int]bool)}
	for i := 0; i < maxWorkers; i++ {
		go worker(q, &boxIDPool, i)
	}
	return jobQueue{q: q, boxIDPool: &boxIDPool}
}

// TODO: Buffered channel doesn't parallelize. Need actual workers to parallelize?
// DONE: We won't actually need test output
// TODO: public/private? also for factory function?
