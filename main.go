package main

import (
	"fmt"
)

func main() {
	checkRootPermissions()
	instance := &IsolateInstance{
		boxID:             2,
		execFile:          "/home/szawinis/program",
		isolateExecFile:   "program",
		ioMode:            1,
		logFile:           "meta",
		timeLimit:         5,
		memoryLimit:       262144,
		inputFile:         "/home/szawinis/input",
		outputFile:        "/home/szawinis/output",
		isolateInputFile:  "input",
		isolateOutputFile: "output"}
	// TODO: TRIM EVERY STRING in constructor
	instance.isolateInit()
	status, metrics := instance.isolateRun()
	instance.isolateCleanup()
	fmt.Println(status, metrics)
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
// NOTE2: isolate is required to be installed to /usr/bin
// NOTE: input, output files are scoped within the box directory, but logFile is not
