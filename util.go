package main

import (
	"log"
	"os/exec"
	"strconv"
)

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func checkRootPermissions() {
	cmd := exec.Command("id", "-u")
	output, err := cmd.Output()
	checkError(err)
	// output has a trailing \n, so we need to use a slice of one below the last index
	id, err := strconv.Atoi(string(output[:len(output)-1]))
	checkError(err)
	if id != 0 {
		log.Fatal("Grader must be run as root")
	}
}
