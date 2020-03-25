package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func throwLogFileCorrupted() {
	log.Fatal("Log file corrupted")
}

func getMetaFields() map[string]bool { // might not need these
	metaFields := map[string]bool{ // TODO: check these for validity by doing tests on TLE, MLE, RE samples
		"cg-mem":        true,
		"cg-oom-killed": true,
		"csw-forced":    false,
		"csw-voluntary": false,
		"exitcode":      false,
		"exitsig":       true,
		"killed":        true,
		"max-rss":       false,
		"message":       true,
		"status":        true,
		"time":          false,
		"time-wall":     false,
	}
	return metaFields
}

type IsolateInstance struct {
	execFile    string
	ioMode      int // 0 = user's program already handles file IO, 1 = script needs to redirect IO
	boxID       int
	logFile     string // any path
	timeLimit   float64
	memoryLimit int
	inputFile   string // CAN ONLY BE PATH WITHIN BOX as defined by isolate
	outputFile  string // same goes for this
}

func (instance *IsolateInstance) buildIsolateArguments() []string {
	args := make([]string, 0)
	args = append(args, []string{"-b", strconv.Itoa(instance.boxID)}...)
	args = append(args, []string{"-M", instance.logFile}...)
	args = append(args, []string{"-t", strconv.FormatFloat(instance.timeLimit, 'f', -1, 64)}...)
	args = append(args, []string{"-m", strconv.Itoa(instance.memoryLimit)}...)
	args = append(args, []string{"-w", strconv.FormatFloat(instance.timeLimit+5, 'f', -1, 64)}...) // five extra seconds for wall clock
	args = append(args, []string{"-x", "2"}...)                                                    // add two seconds to extra time
	if instance.ioMode == 1 {                                                                      // TODO: copy inputFile to box directory and touch outputFile
		args = append(args, []string{"-i", instance.inputFile}...) // PITFALL: BE CAREFUL ABOUT ABSOLUTE/RELATIVE PATHS HERE
		args = append(args, []string{"-o", instance.outputFile}...)
	}
	fmt.Println(args)
	return args
}

func (instance *IsolateInstance) isolateRun() string {
	// Run isolate --run
	args := append(instance.buildIsolateArguments()[:], []string{"--run", "--", instance.execFile}...)
	var exitCode int
	if err := exec.Command("isolate", args...).Run(); err != nil {
		exitCode = err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		exitCode = 0
	}

	if exitCode == 0 {
		fmt.Println("Output produced successfully")
		return "OK"
	}

	// Read and parse log file
	logFileBytes, err := ioutil.ReadFile(instance.logFile)
	checkError(err)
	contents := string(logFileBytes)
	props := make(map[string]string)
	for _, line := range strings.Split(contents, "\n") { // PITFALL: What if the file doesn't follow the correct format?
		if len(line) == 0 {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) != 2 {
			throwLogFileCorrupted()
		}
		props[pair[0]] = pair[1]
	}

	if instance.checkTLE(props) {
		return "TLE"
	}

	if instance.checkMLE(props) {
		return "MLE"
	}

	if instance.checkRE(props) {
		return "RE"
	}

	if !instance.checkXX(props) {
		log.Fatal("Internal error of grader in judging program")
	}

	// read logFile to find if there are any errors
	// If there are, terminate immediately with 0 points
	// Otherwise, continue to checker script (which will take output from the file and process it)
	// IMPORTANT: move output file out of box directory, otherwise it will be deleted after cleanup <- add as a doc comment
	return "XX" // RETURN ENUM INSTEAD
}

func (instance *IsolateInstance) isolateInit() {
	err := exec.Command("isolate", "-b", strconv.Itoa(instance.boxID), "--init").Run()
	checkError(err)
}

func (instance *IsolateInstance) isolateCleanup() { // needs to handle case where it can't clean up
	err := exec.Command("isolate", "-b", strconv.Itoa(instance.boxID), "--cleanup").Run()
	checkError(err)
}

// NOTE: errors need to be handled more gracefully -- all currently fatal errors should be returned as a status instead
