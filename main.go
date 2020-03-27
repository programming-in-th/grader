package main

import (
	"fmt"

	"github.com/programming-in-th/grader/isolate"
)

func main() {
	instance := isolate.NewInstance(
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

	instance.Init()
	status, metrics := instance.Run()
	instance.Cleanup()

	fmt.Println(status, metrics)
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
// NOTE: isolate is required to be installed to /usr/bin
