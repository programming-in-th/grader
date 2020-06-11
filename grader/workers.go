package grader

import (
	"log"
	"path"
	"strconv"
	"sync"

	"github.com/programming-in-th/grader/conf"
	"github.com/programming-in-th/grader/isolate"
)

type GradingJob struct {
	manifestInstance taskManifest
	submissionID     string
	targLang         string
	userBinPath      string
	testIndex        int
	resultChannel    chan SingleTestResult
}

type safeBoxIDPool struct {
	BoxIDs map[int]bool
	Mux    sync.Mutex
}

func waitForTestResult(manifestInstance taskManifest,
	submissionID string,
	targLang string,
	userBinPath string,
	testIndex int,
	config conf.Config,
	boxIDPool *safeBoxIDPool,
) SingleTestResult {
	// Convert time and memory limits
	var timeLimit float64
	var memoryLimit int
	if limits, exists := manifestInstance.Limits[targLang]; exists {
		timeLimit = limits.TimeLimit
		memoryLimit = limits.MemoryLimit * 1000 // Convert to KB
	} else {
		timeLimit = manifestInstance.DefaultLimits.TimeLimit
		memoryLimit = manifestInstance.DefaultLimits.MemoryLimit * 1000
	}

	// Run isolate job
	isolateResult := runIsolate(
		userBinPath,
		timeLimit,
		memoryLimit,
		path.Join(manifestInstance.inputsBasePath, strconv.Itoa(testIndex+1)+".in"),
		path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(testIndex+1)+".out"),
		config.Glob.IsolateBinPath,
		boxIDPool,
	)

	// Check for fatal errors first and return corresponding results without running checker
	if isolateResult.verdict == isolate.IsolateRunXX || isolateResult.verdict == isolate.IsolateRunOther {
		writeCheckFile(submissionID, testIndex, conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict])
		log.Println(isolateResult.err)
		return SingleTestResult{conf.IEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, config.Glob.DefaultMessages[conf.IEVerdict]}
	}

	if isolateResult.verdict != isolate.IsolateRunOK {
		if isolateResult.verdict == isolate.IsolateRunMLE {
			writeCheckFile(submissionID, testIndex, conf.MLEVerdict, "0", config.Glob.DefaultMessages[conf.MLEVerdict])
			return SingleTestResult{conf.MLEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, config.Glob.DefaultMessages[conf.MLEVerdict]}
		} else if isolateResult.verdict == isolate.IsolateRunRE {
			writeCheckFile(submissionID, testIndex, conf.REVerdict, "0", config.Glob.DefaultMessages[conf.REVerdict])
			return SingleTestResult{conf.REVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, config.Glob.DefaultMessages[conf.REVerdict]}
		} else if isolateResult.verdict == isolate.IsolateRunTLE {
			writeCheckFile(submissionID, testIndex, conf.TLEVerdict, "0", config.Glob.DefaultMessages[conf.TLEVerdict])
			return SingleTestResult{conf.TLEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, config.Glob.DefaultMessages[conf.TLEVerdict]}
		} else {
			writeCheckFile(submissionID, testIndex, conf.IEVerdict, "0", config.Glob.DefaultMessages[conf.IEVerdict])
			return SingleTestResult{conf.IEVerdict, "0", isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, config.Glob.DefaultMessages[conf.IEVerdict]}
		}
	} else {
		// Assuming the verdict is isolate.IsolateRunOK, we run the checker
		var checkerPath string
		if manifestInstance.Checker != "custom" {
			checkerPath = path.Join(config.BasePath, "config", "defaultCheckers", manifestInstance.Checker)
		} else {
			checkerPath = path.Join(manifestInstance.taskBasePath, "checker")
		}
		checkerResult := runChecker(
			submissionID,
			testIndex,
			checkerPath,
			path.Join(manifestInstance.inputsBasePath, strconv.Itoa(testIndex+1)+".in"),
			path.Join(BASE_TMP_PATH, submissionID, strconv.Itoa(testIndex+1)+".out"),
			path.Join(manifestInstance.solutionsBasePath, strconv.Itoa(testIndex+1)+".sol"),
			config,
		)

		return SingleTestResult{checkerResult.verdict, checkerResult.score, isolateResult.metrics.TimeElapsed, isolateResult.metrics.MemoryUsage, checkerResult.message}
	}
}

func NewGradingJobQueue(maxWorkers int, done chan bool, config conf.Config) chan GradingJob {
	ch := make(chan GradingJob)
	var wg sync.WaitGroup

	go func() {
		wg.Wait()
		close(ch)
	}()

	boxIDPool := safeBoxIDPool{BoxIDs: make(map[int]bool)}
	wg.Add(maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go func(i int) {
			for {
				select {
				case job := <-ch:
					result := waitForTestResult(job.manifestInstance,
						job.submissionID,
						job.targLang,
						job.userBinPath,
						job.testIndex,
						config,
						&boxIDPool)
					job.resultChannel <- result
				case <-done:
					wg.Done()
					return
				}
			}
		}(i)
	}
	return ch
}
