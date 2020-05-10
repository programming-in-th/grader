package main

import "log"
import "github.com/programming-in-th/grader/grader"

func main() {
	jobQueueDone := make(chan bool)
	jobQueue := grader.NewIsolateJobQueue(2, jobQueueDone)
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := grader.NewCheckerJobQueue(5, checkerJobQueueDone)

	src := make([]string, 1)
	src[0] = "/home/szawinis/go/src/github.com/programming-in-th/grader/testing/asdf/ac.cpp"
	submissionResult, err := grader.GradeSubmission("submissionID", "asdf", "cpp", src, &jobQueue, checkerJobQueue)
	if err != nil {
		log.Println("Error grading submission")
	}
	log.Println(submissionResult)

	jobQueueDone <- true
	checkerJobQueueDone <- true
}

// TODO: handle box ids
// NOTE: filesystem access is already restricted for the use cases of freopen
