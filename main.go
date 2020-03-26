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
	instance.IsolateInit()
	status, metrics := instance.IsolateRun()
	instance.IsolateCleanup()
	fmt.Println(status, metrics)
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
// NOTE: isolate is required to be installed to /usr/bin
