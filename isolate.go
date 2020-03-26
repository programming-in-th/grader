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

/*----------------------TYPE DECLARATIONS----------------------*/

// IsolateInstance defines an instance of an isolate lifecycle from initialization to cleanup.
type IsolateInstance struct {
	boxID             int
	execFile          string
	isolateExecFile   string
	ioMode            int    // 0 = user's program already handles file IO, 1 = script needs to redirect IO
	logFile           string // can be both absolute and relative path
	timeLimit         float64
	extraTime         float64 // extra time allowed before kill
	memoryLimit       int
	isolateDirectory  string // box directory of isolate. Must only be set through IsolateInit()
	isolateInputFile  string // relative to box directory and must be within box directory as per isolate specs
	isolateOutputFile string // relative to box directory and must be within box directory as per isolate specs
	inputFile         string // Path to input file from test case
	outputFile        string // Path to output file from test case
}

// IsolateRunStatus denotes possible states after isolate run
type IsolateRunStatus string

const (
	// IsolateRunOK = No errors (but WA can be possible since checker has not been run)
	IsolateRunOK IsolateRunStatus = "OK"
	// IsolateRunTLE = Time limit exceeded
	IsolateRunTLE IsolateRunStatus = "TLE"
	// IsolateRunMLE = Memory limit exceeded
	IsolateRunMLE IsolateRunStatus = "MLE"
	// IsolateRunRE = Runtime error (any runtime error that is not MLE, including asserting false, invalid memory access, etc)
	IsolateRunRE IsolateRunStatus = "RE"
	// IsolateRunXX = Internal error of isolate
	IsolateRunXX IsolateRunStatus = "XX"
	// IsolateRunOther = Placeholder in case something went wrong in this script
	IsolateRunOther IsolateRunStatus = "??"
)

// IsolateRunMetrics contains info on time and memory usage after running isolate
type IsolateRunMetrics struct {
	timeElapsed     float64
	memoryUsage     int
	wallTimeElapsed float64
}

/*----------------------END TYPE DECLARATIONS----------------------*/

// NewIsolateInstance creates a new IsolateInstance
func NewIsolateInstance(
	boxID int,
	execFile string,
	ioMode int,
	logFile string,
	timeLimit float64,
	extraTime float64,
	memoryLimit int,
	isolateInputFile string,
	isolateOutputFile string,
	inputFile string,
	outputFile string) *IsolateInstance {

	return &IsolateInstance{
		boxID:             boxID,
		execFile:          strings.TrimSpace(execFile),
		ioMode:            ioMode,
		logFile:           strings.TrimSpace(logFile),
		timeLimit:         timeLimit,
		extraTime:         extraTime,
		memoryLimit:       memoryLimit,
		isolateInputFile:  strings.TrimSpace(isolateInputFile),
		isolateOutputFile: strings.TrimSpace(isolateOutputFile),
		inputFile:         strings.TrimSpace(inputFile),
		outputFile:        strings.TrimSpace(outputFile),
		isolateExecFile:   "program",
	}
}

// IsolateInit initializes the new box directory for the IsolateInstance
func (instance *IsolateInstance) IsolateInit() {
	// Isolate needs to be run as root
	checkRootPermissions()

	// Run init command
	bytes, err := exec.Command("isolate", "-b", strconv.Itoa(instance.boxID), "--init").Output()
	outputString := strings.TrimSpace(string(bytes))
	instance.isolateDirectory = fmt.Sprintf("%s/box/", outputString)
	instance.checkErrorAndCleanup(err)

	// Copy input, output and executable files to isolate directory
	// TODO: validate nonexistent input file
	err = exec.Command("cp", instance.inputFile, instance.isolateDirectory+instance.isolateInputFile).Run()
	instance.checkErrorAndCleanup(err)
	err = exec.Command("cp", instance.outputFile, instance.isolateDirectory+instance.isolateOutputFile).Run()
	instance.checkErrorAndCleanup(err)
	err = exec.Command("cp", instance.execFile, instance.isolateDirectory+instance.isolateExecFile).Run()
	instance.checkErrorAndCleanup(err)
}

// IsolateCleanup clears up the box directory for other instances to use
func (instance *IsolateInstance) IsolateCleanup() { // needs to handle case where it can't clean up
	err := exec.Command("isolate", "-b", strconv.Itoa(instance.boxID), "--cleanup").Run()
	instance.checkErrorAndCleanup(err)
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

func (instance *IsolateInstance) checkXX(props map[string]string) bool {
	status, statusExists := props["status"]
	return statusExists && strings.TrimSpace(status) == "XX"
}

func (instance *IsolateInstance) checkTLE(props map[string]string) bool {
	timeElapsedString, timeExists := props["time"]
	status := strings.TrimSpace(props["status"])
	killed := strings.TrimSpace(props["killed"])
	timeElapsed, err := strconv.ParseFloat(timeElapsedString, 64)
	if !timeExists || err != nil || (timeElapsed > instance.timeLimit && !(killed == "1" && status == "TO")) {
		instance.throwLogFileCorruptedAndCleanup()
	}
	return status == "TO"
}

func (instance *IsolateInstance) checkRE(props map[string]string) (int, string) {
	memoryUsageString, maxRssExists := props["max-rss"]
	exitSig, exitSigExists := props["exitsig"]
	status := props["status"]
	memoryUsage, err := strconv.Atoi(memoryUsageString)
	if !maxRssExists || err != nil ||
		((memoryUsage > instance.memoryLimit || exitSigExists || strings.TrimSpace(status) == "SG") &&
			!(exitSigExists && status == "SG")) ||
		(exitSigExists && strings.TrimSpace(exitSig) != "6" && strings.TrimSpace(exitSig) != "11") {
		instance.throwLogFileCorruptedAndCleanup()
	}
	if !exitSigExists {
		return 0, ""
	} else if memoryUsage > instance.memoryLimit {
		return 1, strings.TrimSpace(exitSig)
	} else {
		return 2, strings.TrimSpace(exitSig)
	}
}

// IsolateRun runs isolate on an IsolateInstance
func (instance *IsolateInstance) IsolateRun() (IsolateRunStatus, *IsolateRunMetrics) {
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
	metricObject := IsolateRunMetrics{timeElapsed: timeElapsed, memoryUsage: memoryUsage, wallTimeElapsed: wallTimeElapsed}

	// Check status and return
	if exitCode == 0 {
		return IsolateRunOK, &metricObject
	}
	code, _ := instance.checkRE(props)
	if code == 1 {
		return IsolateRunMLE, &metricObject
	} else if code == 2 {
		return IsolateRunRE, &metricObject
	}
	if instance.checkTLE(props) {
		return IsolateRunTLE, &metricObject
	}
	return IsolateRunOther, &metricObject
}

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

func (instance *IsolateInstance) checkErrorAndCleanup(err error) {
	if err != nil {
		instance.IsolateCleanup()
		log.Fatal(err)
	}
}

func (instance *IsolateInstance) throwLogFileCorruptedAndCleanup() {
	instance.IsolateCleanup()
	log.Fatal("Log file corrupted")
}

// TODO: Handle IO if needed (test for file IO already handled by the program)
// TODO: move output file out of box directory, otherwise it will be deleted after cleanup
// TODO: errors need to be handled more gracefully -- all currently fatal errors should be returned as a status instead
// Specs for little details and protocols should be put in a separate document
