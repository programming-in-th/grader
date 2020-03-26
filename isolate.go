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

func (instance *IsolateInstance) checkErrorAndCleanup(err error) {
	if err != nil {
		instance.isolateCleanup()
		log.Fatal(err)
	}
}

func (instance *IsolateInstance) throwLogFileCorruptedAndCleanup() {
	instance.isolateCleanup()
	log.Fatal("Log file corrupted")
}

// IsolateInstance defines an instance of an isolate lifecycle
// from initialization to cleanup.
type IsolateInstance struct {
	execFile          string
	isolateExecFile   string // will not be set a first, but will be right after initialization
	ioMode            int    // 0 = user's program already handles file IO, 1 = script needs to redirect IO
	boxID             int
	logFile           string // any path
	timeLimit         float64
	extraTime         float64 // TODO: interface this with grader
	memoryLimit       int
	isolateDirectory  string // will not be set at first, but will be right after initialization
	isolateInputFile  string // CAN ONLY BE PATH WITHIN BOX as defined by isolate
	isolateOutputFile string // same goes for this
	inputFile         string // Input file from test case
	outputFile        string // Output file from test case
}

// IsolateRunStatus denotes possible states after isolate run
type IsolateRunStatus string
const (
	// IsolateRunOK = No errors (but WA can be possible since checker has not been run)
	IsolateRunOK    IsolateRunStatus = "OK" 
	// IsolateRunTLE = Time limit exceeded
	IsolateRunTLE   IsolateRunStatus = "TLE"
	// IsolateRunMLE = Memory limit exceeded
	IsolateRunMLE   IsolateRunStatus = "MLE"
	// IsolateRunRE = Runtime error (any runtime error that is not MLE, including asserting false, invalid memory access, etc)
	IsolateRunRE    IsolateRunStatus = "RE"
	// IsolateRunXX = Internal error of isolate
	IsolateRunXX    IsolateRunStatus = "XX"
	// IsolateRunOther = Placeholder in case something went wrong in this script
	IsolateRunOther IsolateRunStatus = "??"
)

// IsolateRunMetrics contains info on time and memory usage after running isolate
type IsolateRunMetrics struct {
	timeElapsed float64
	memoryUsage int
	wallTimeElapsed float64
}

func (instance *IsolateInstance) buildIsolateArguments() []string {
	args := make([]string, 0)
	args = append(args, []string{"-b", strconv.Itoa(instance.boxID)}...)
	args = append(args, []string{"-M", instance.logFile}...)
	args = append(args, []string{"-t", strconv.FormatFloat(instance.timeLimit, 'f', -1, 64)}...)
	args = append(args, []string{"-m", strconv.Itoa(instance.memoryLimit)}...)
	args = append(args, []string{"-w", strconv.FormatFloat(instance.timeLimit+5, 'f', -1, 64)}...) // five extra seconds for wall clock
	args = append(args, []string{"-x", strconv.FormatFloat(instance.extraTime, 'f', -1, 64)}...)
	if instance.ioMode == 1 {
		args = append(args, []string{"-i", instance.isolateInputFile}...)
		args = append(args, []string{"-o", instance.isolateOutputFile}...)
	}
	fmt.Println(args)
	return args
}

func (instance *IsolateInstance) isolateRun() (IsolateRunStatus, *IsolateRunMetrics) {
	// Run isolate --run
	args := append(instance.buildIsolateArguments()[:], []string{"--run", "--", instance.isolateExecFile}...)
	var exitCode int
	if err := exec.Command("isolate", args...).Run(); err != nil {
		exitCode = err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		exitCode = 0
	}

	// Read and parse log file into a map
	logFileBytes, err := ioutil.ReadFile(instance.logFile)
	instance.checkErrorAndCleanup(err)
	contents := string(logFileBytes)
	props := make(map[string]string)
	for _, line := range strings.Split(contents, "\n") { // PITFALL: What if the file doesn't follow the correct format?
		if len(line) == 0 {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) != 2 {
			instance.throwLogFileCorruptedAndCleanup()
		}
		props[strings.TrimSpace(pair[0])] = strings.TrimSpace(pair[1])
	}

	// If the error is XX, then the fields required for run metrics won't be there
	if instance.checkXX(props) {
		return IsolateRunXX, nil
	}

	// Validate fields and extract run metrics from the map
	memoryUsageString, memoryUsageExists := props["max-rss"]
	timeElapsedString, timeElapsedExists := props["time"]
	wallTimeElapsedString, wallTimeElapsedExists := props["time-wall"]
	if !memoryUsageExists || !timeElapsedExists || !wallTimeElapsedExists {
		instance.throwLogFileCorruptedAndCleanup()
	}
	memoryUsage, err := strconv.Atoi(memoryUsageString)
	instance.checkErrorAndCleanup(err)
	timeElapsed, err := strconv.ParseFloat(timeElapsedString, 64)
	instance.checkErrorAndCleanup(err)
	wallTimeElapsed, err := strconv.ParseFloat(wallTimeElapsedString, 64)
	instance.checkErrorAndCleanup(err)
	metricObject := IsolateRunMetrics {timeElapsed: timeElapsed, memoryUsage: memoryUsage, wallTimeElapsed: wallTimeElapsed}

	if exitCode == 0 {
		return IsolateRunOK, &metricObject
	}


	code, _ := instance.checkRE(props) // TODO: utilize exitSig string
	if code == 1 {
		return IsolateRunMLE, &metricObject
	} else if code == 2 {
		return IsolateRunRE, &metricObject
	}
	if instance.checkTLE(props) {
		return IsolateRunTLE, &metricObject
	}

	return IsolateRunOther, &metricObject
	// If there are, terminate immediately with 0 points
	// Otherwise, continue to checker script (which will take output from the file and process it)
	// IMPORTANT: move output file out of box directory, otherwise it will be deleted after cleanup <- add as a doc comment
}

func (instance *IsolateInstance) isolateInit() {
	bytes, err := exec.Command("isolate", "-b", strconv.Itoa(instance.boxID), "--init").Output()
	outputString := strings.TrimSpace(string(bytes))
	instance.isolateDirectory = fmt.Sprintf("%s/box/", outputString)
	instance.checkErrorAndCleanup(err)
	// TODO: validate nonexistent input file
	err = exec.Command("cp", instance.inputFile, instance.isolateDirectory+instance.isolateInputFile).Run()
	instance.checkErrorAndCleanup(err)
	err = exec.Command("cp", instance.outputFile, instance.isolateDirectory+instance.isolateOutputFile).Run()
	instance.checkErrorAndCleanup(err)
	err = exec.Command("cp", instance.execFile, instance.isolateDirectory+instance.isolateExecFile).Run()
	instance.checkErrorAndCleanup(err)
}

func (instance *IsolateInstance) isolateCleanup() { // needs to handle case where it can't clean up
	err := exec.Command("isolate", "-b", strconv.Itoa(instance.boxID), "--cleanup").Run()
	instance.checkErrorAndCleanup(err)
}

// TODO: errors need to be handled more gracefully -- all currently fatal errors should be returned as a status instead
// Specs for little details and protocols should be put in a separate document
