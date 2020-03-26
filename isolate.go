package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

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
		execFile:          execFile,
		ioMode:            ioMode,
		logFile:           logFile,
		timeLimit:         timeLimit,
		extraTime:         extraTime,
		memoryLimit:       memoryLimit,
		isolateInputFile:  isolateInputFile,
		isolateOutputFile: isolateOutputFile,
		inputFile:         inputFile,
		outputFile:        outputFile,
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
	// If there are, terminate immediately with 0 points
	// Otherwise, continue to checker script (which will take output from the file and process it)
	// IMPORTANT: move output file out of box directory, otherwise it will be deleted after cleanup <- add as a doc comment
}

// TODO: errors need to be handled more gracefully -- all currently fatal errors should be returned as a status instead
// Specs for little details and protocols should be put in a separate document
