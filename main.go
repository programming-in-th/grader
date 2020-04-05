package main

import (
	"fmt"

	"github.com/programming-in-th/grader/isolate"
)

func main() {
	instance := isolate.NewInstance(
		"/usr/bin/isolate",
		2,
		"/home/szawinis/program",
		1,
		"/home/szawinis/meta",
		5,
		0,
		262144,
		"input",
		"output",
		"/home/szawinis/resulting_output",
		"/home/szawinis/input",
		"/home/szawinis/output",
	)

	initSuccess := instance.Init()
	if initSuccess {
		status, metrics := instance.Run()
		fmt.Println(status, metrics)
	}
	cleanupSuccess := instance.Cleanup()
	fmt.Println(cleanupSuccess)
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
