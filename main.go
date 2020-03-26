package main

import (
	"fmt"
)

func main() {
	instance := NewIsolateInstance(
		2,
		"/home/szawinis/program",
		1,
		"meta",
		5,
		0,
		262144,
		"input",
		"output",
		"/home/szawinis/input",
		"/home/szawinis/output",
	)
	// TODO: TRIM EVERY STRING in constructor
	instance.IsolateInit()
	status, metrics := instance.IsolateRun()
	instance.IsolateCleanup()
	fmt.Println(status, metrics)
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
// NOTE2: isolate is required to be installed to /usr/bin
// NOTE: input, output files are scoped within the box directory, but logFile is not
