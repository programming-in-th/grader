package main

import "github.com/programming-in-th/grader/grader"

func main() {
	jobQueueDone := make(chan bool)
	jobQueue := grader.NewIsolateJobQueue(2, jobQueueDone)
	checkerJobQueueDone := make(chan bool)
	checkerJobQueue := grader.NewCheckerJobQueue(5, checkerJobQueueDone)

	handleRequest(&jobQueue, checkerJobQueue)

	jobQueueDone <- true
	checkerJobQueueDone <- true
}
