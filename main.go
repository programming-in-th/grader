package main

import "github.com/programming-in-th/grader/grader"

func main() {
	jq := grader.NewJobQueue(5)
	grader.GradeSubmission("parade", "cpp", &jq)
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
