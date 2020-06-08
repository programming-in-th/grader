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
	metrics *isolate.RunMetrics
	err     error
}

type isolateJob struct {
	userBinPath   string
	timeLimit     float64
	memoryLimit   int
	inputPath     string
	outputPath    string
	resultChannel chan isolateTestResult
}

type IsolateJobQueue struct {
	q         chan isolateJob
	boxIDPool *safeBoxIDPool
}

// WaitGroup should be started outside of this
func runIsolate(
	job isolateJob,
	boxID int,
	isolateBinPath string,
) {

	// Run a new isolate instance
	instance := isolate.NewInstance(
		isolateBinPath,
		boxID,
		job.userBinPath,
		1,
		"/tmp/tmp_isolate_grader_"+strconv.Itoa(boxID),
		job.timeLimit,
		0, // TODO: CHANGE
		job.memoryLimit,
		job.outputPath,
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
}

func isolateWorker(q chan isolateJob, boxIDPool *safeBoxIDPool, id int, done chan bool, isolateBinPath string) {
	for {
		select {
		case job := <-q:
			// Find minimum excludant in box ID pool
			boxIDPool.mux.Lock()
			mex := 0
			for {
				used := boxIDPool.boxIDs[mex]
				if !used {
					boxIDPool.boxIDs[mex] = true
					break
				}
				mex++
			}
			boxIDPool.mux.Unlock()
			log.Printf("Running job on worker: %d", id)
			log.Printf("Job: %#v", job)
			log.Printf("Box id for job: %d", mex)
			runIsolate(job, mex, isolateBinPath)
			boxIDPool.mux.Lock()
			boxIDPool.boxIDs[mex] = false
			boxIDPool.mux.Unlock()
		case <-done:
			break
		}
	}
}

func NewIsolateJobQueue(maxWorkers int, done chan bool, isolateBinPath string) IsolateJobQueue {
	q := make(chan isolateJob)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(q)
	}()

	wg.Add(maxWorkers)
	boxIDPool := safeBoxIDPool{boxIDs: make(map[int]bool)}
	for i := 0; i < maxWorkers; i++ {
		go func(i int) {
			isolateWorker(q, &boxIDPool, i, done, isolateBinPath)
			wg.Done()
		}(i)
	}
	return IsolateJobQueue{q, &boxIDPool}
}
