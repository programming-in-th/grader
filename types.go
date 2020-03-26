package main

// IsolateInstance defines an instance of an isolate lifecycle from initialization to cleanup.
type IsolateInstance struct {
	boxID             int
	execFile          string
	isolateExecFile   string // Overriden during initialization
	ioMode            int    // 0 = user's program already handles file IO, 1 = script needs to redirect IO
	logFile           string // any path
	timeLimit         float64
	extraTime         float64 // TODO: interface this with grader
	memoryLimit       int
	isolateDirectory  string // will not be set at first, but will be right after initialization
	isolateInputFile  string // CAN ONLY BE PATH WITHIN BOX as defined by isolate
	isolateOutputFile string // same goes for this
	inputFile         string // Input file from test case
	outputFile        string // Output file from test case
}

// IsolateRunStatus denotes possible states after isolate run
type IsolateRunStatus string

const (
	// IsolateRunOK = No errors (but WA can be possible since checker has not been run)
	IsolateRunOK IsolateRunStatus = "OK"
	// IsolateRunTLE = Time limit exceeded
	IsolateRunTLE IsolateRunStatus = "TLE"
	// IsolateRunMLE = Memory limit exceeded
	IsolateRunMLE IsolateRunStatus = "MLE"
	// IsolateRunRE = Runtime error (any runtime error that is not MLE, including asserting false, invalid memory access, etc)
	IsolateRunRE IsolateRunStatus = "RE"
	// IsolateRunXX = Internal error of isolate
	IsolateRunXX IsolateRunStatus = "XX"
	// IsolateRunOther = Placeholder in case something went wrong in this script
	IsolateRunOther IsolateRunStatus = "??"
)

// IsolateRunMetrics contains info on time and memory usage after running isolate
type IsolateRunMetrics struct {
	timeElapsed     float64
	memoryUsage     int
	wallTimeElapsed float64
}
