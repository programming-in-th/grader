package isolate

import (
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

/*----------------------TYPE DECLARATIONS----------------------*/

// Instance defines an instance of an isolate lifecycle from initialization to cleanup.
type Instance struct {
	isolateExecPath        string
	boxID                  int
	userProgramPath        string
	ioMode                 int    // 0 = user's program already handles file IO, 1 = script needs to redirect IO
	logFile                string // Can be both absolute and relative path
	timeLimit              float64
	extraTime              float64 // Extra time allowed before kill
	memoryLimit            int
	isolateDirectory       string // Box directory of isolate. Must only be set through IsolateInit()
	isolateInputName       string // Relative to box directory and must be within box directory as per isolate specs
	isolateOutputName      string // Relative to box directory and must be within box directory as per isolate specs
	resultOutputTargetPath string // Target path of output file after copying out of box directory
	inputPath              string // Path to input file from test case
	runnerScriptPath       string // Path to runner script
}

// RunVerdict denotes possible states after isolate run
type RunVerdict string

const (
	// IsolateRunOK = No errors (but WA can be possible since checker has not been run)
	IsolateRunOK RunVerdict = "OK"
	// IsolateRunTLE = Time limit exceeded
	IsolateRunTLE RunVerdict = "TLE"
	// IsolateRunMLE = Memory limit exceeded
	IsolateRunMLE RunVerdict = "MLE"
	// IsolateRunRE = Runtime error (any runtime error that is not MLE, including asserting false, invalid memory access, etc)
	IsolateRunRE RunVerdict = "RE"
	// IsolateRunXX = Internal error of isolate
	IsolateRunXX RunVerdict = "XX"
	// IsolateRunOther = Placeholder in case something went wrong in this script
	IsolateRunOther RunVerdict = "??"
)

// RunMetrics contains info on time and memory usage after running isolate
type RunMetrics struct {
	TimeElapsed int
	MemoryUsage int
}

/*----------------------END TYPE DECLARATIONS----------------------*/

// NewInstance creates a new Instance
func NewInstance(
	isolateExecPath string,
	boxID int,
	execFile string,
	ioMode int,
	logFile string,
	timeLimit float64,
	extraTime float64,
	memoryLimit int,
	resultOutputFile string,
	inputFile string,
	runnerScriptPath string) *Instance {

	timeLimit = math.Round(timeLimit*1000) / 1000
	extraTime = math.Round(extraTime*1000) / 1000

	return &Instance{
		isolateExecPath:        isolateExecPath,
		boxID:                  boxID,
		userProgramPath:        strings.TrimSpace(execFile),
		ioMode:                 ioMode,
		logFile:                strings.TrimSpace(logFile),
		timeLimit:              timeLimit,
		extraTime:              extraTime,
		memoryLimit:            memoryLimit,
		isolateInputName:       "input",
		isolateOutputName:      "output",
		resultOutputTargetPath: strings.TrimSpace(resultOutputFile),
		inputPath:              strings.TrimSpace(inputFile),
		runnerScriptPath:       strings.TrimSpace(runnerScriptPath),
	}
}

// Init initializes the new box directory for the Instance
func (instance *Instance) Init() error { // returns true if finished OK, otherwise returns false
	// Isolate needs to be run as root
	isRoot, err := checkRootPermissions()
	if err != nil {
		return errors.Wrap(err, "Unable to check root permissions")
	}
	if !isRoot {
		return errors.New("Init failed: isolate must be run as root")
	}

	// Run init command
	bytes, err := exec.Command(instance.isolateExecPath, "--cg", "-b", strconv.Itoa(instance.boxID), "--init").Output()
	outputString := strings.TrimSpace(string(bytes))
	instance.isolateDirectory = path.Join(outputString, "box")
	if err != nil {
		return errors.Wrapf(err, "Unable to run isolate --init command. Does a box already exist? If so, you must clean up first.")
	}

	// Copy input, output and executable files to isolate directory
	// TODO: validate nonexistent input file
	err = exec.Command("cp", instance.inputPath, path.Join(instance.isolateDirectory, instance.isolateInputName)).Run()
	if err != nil {
		return errors.Wrap(err, "Unable to copy input file into box directory")
	}
	err = exec.Command("cp", instance.userProgramPath, path.Join(instance.isolateDirectory)).Run()
	if err != nil {
		return errors.Wrap(err, "Unable to copy user exec file into box directory")
	}
	err = exec.Command("cp", instance.runnerScriptPath, path.Join(instance.isolateDirectory)).Run()
	if err != nil {
		return errors.Wrap(err, "Unable to copy runner script into box directory")
	}
	return nil
}

// Cleanup clears up the box directory for other instances to use
func (instance *Instance) Cleanup() error { // returns true if finished OK, otherwise returns false
	os.Remove(instance.logFile) // No need to catch errors on this because duplicate tmp files does nothing
	err := exec.Command(instance.isolateExecPath, "--cg", "-b", strconv.Itoa(instance.boxID), "--cleanup").Run()
	return err
}

func (instance *Instance) buildIsolateArguments() []string {
	args := make([]string, 0)
	args = append(args, "--cg")
	args = append(args, "--cg-timing")
	args = append(args, "--processes=128") // set to high number (like 128) for Java (issue #57 in ioi/isolate)
	args = append(args, []string{"-b", strconv.Itoa(instance.boxID)}...)
	args = append(args, []string{"-M", instance.logFile}...)
	args = append(args, []string{"-t", strconv.FormatFloat(instance.timeLimit, 'f', -1, 64)}...)
	args = append(args, "--cg-mem="+strconv.Itoa(instance.memoryLimit))
	args = append(args, []string{"-w", strconv.FormatFloat(instance.timeLimit+5, 'f', -1, 64)}...) // five extra seconds for wall clock
	args = append(args, []string{"-x", strconv.FormatFloat(instance.extraTime, 'f', -1, 64)}...)
	_, err := os.Stat("/etc/alternatives")
	if !os.IsNotExist(err) {
		args = append(args, "--dir=etc/alternatives") // for Java, PHP, etc.
	}
	if instance.ioMode == 1 {
		args = append(args, []string{"-i", instance.isolateInputName}...)
		args = append(args, []string{"-o", instance.isolateOutputName}...)
	}
	return args
}

func (instance *Instance) checkXX(props map[string]string) bool {
	status, statusExists := props["status"]
	return statusExists && strings.TrimSpace(status) == "XX"
}

func (instance *Instance) checkTLE(props map[string]string) (bool, bool) {
	timeElapsedString, timeExists := props["time"]
	status := strings.TrimSpace(props["status"])
	_, err := strconv.ParseFloat(timeElapsedString, 64)
	if !timeExists || err != nil {
		return false, true // second parameter denotes whether or not log file is corrupted
	}
	return status == "TO", false
}

func (instance *Instance) checkRE(props map[string]string) (int, string) {
	memoryUsageString, cgMemExists := props["cg-mem"]
	exitSig, exitSigExists := props["exitsig"]
	status := props["status"]
	memoryUsage, err := strconv.Atoi(memoryUsageString)
	if !cgMemExists || err != nil ||
		((memoryUsage > instance.memoryLimit || exitSigExists || strings.TrimSpace(status) == "SG") &&
			!(exitSigExists && status == "SG")) {
		return -1, "" // -1 status means log file was corrupted
	}
	if strings.TrimSpace(status) != "RE" && strings.TrimSpace(status) != "SG" {
		return 0, ""
	} else if memoryUsage > instance.memoryLimit {
		return 1, strings.TrimSpace(exitSig) // MLE
	} else {
		return 2, strings.TrimSpace(exitSig) // RE (assert or segmentation fault)
	}
}

// Run runs isolate on an Instance
func (instance *Instance) Run() (RunVerdict, RunMetrics) {
	// Run isolate --run
	_, runnerScriptName := filepath.Split(instance.runnerScriptPath)
	args := append(instance.buildIsolateArguments()[:], []string{"--run", "--", runnerScriptName}...)
	var exitCode int
	output, err := exec.Command(instance.isolateExecPath, args...).CombinedOutput()
	log.Println(string(output))
	if err != nil {
		exitCode = err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus()
	} else {
		exitCode = 0
	}

	// Read and parse log file into a map
	logFileBytes, err := ioutil.ReadFile(instance.logFile)
	if err != nil {
		log.Println("Cannot read log file")
		return IsolateRunOther, RunMetrics{}
	}
	contents := string(logFileBytes)
	props := make(map[string]string)
	for _, line := range strings.Split(contents, "\n") { // PITFALL: What if the file doesn't follow the correct format?
		if len(line) == 0 {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) != 2 {
			log.Println("Log file has incorrect format")
			return IsolateRunOther, RunMetrics{}
		}
		props[strings.TrimSpace(pair[0])] = strings.TrimSpace(pair[1])
	}

	// If the error is XX, then the fields required for run metrics won't be there
	if instance.checkXX(props) {
		return IsolateRunXX, RunMetrics{}
	}

	// Validate fields and extract run metrics from the map
	memoryUsageString, memoryUsageExists := props["cg-mem"]
	timeElapsedString, timeElapsedExists := props["time"]
	_, wallTimeElapsedExists := props["time-wall"]
	if !memoryUsageExists || !timeElapsedExists || !wallTimeElapsedExists {
		log.Println("Log file has incorrect format")
		return IsolateRunOther, RunMetrics{}
	}
	memoryUsage, err := strconv.Atoi(memoryUsageString)
	if err != nil {
		log.Println("Log file has incorrect format")
		return IsolateRunOther, RunMetrics{}
	}
	timeElapsedUnrounded, err := strconv.ParseFloat(timeElapsedString, 64)
	timeElapsed := int(math.Round(timeElapsedUnrounded * 1000))
	if err != nil {
		log.Println("Log file has incorrect format")
		return IsolateRunOther, RunMetrics{}
	}
	metricObject := RunMetrics{TimeElapsed: timeElapsed, MemoryUsage: memoryUsage}

	// Check status and return
	if exitCode == 0 {
		// IMPORTANT: copy output out of isolate directory
		err = exec.Command("cp", path.Join(instance.isolateDirectory, instance.isolateOutputName), instance.resultOutputTargetPath).Run()
		if err != nil {
			log.Println("Cannot copy output out of isolate directory")
			return IsolateRunOther, RunMetrics{}
		}
		return IsolateRunOK, metricObject
	}
	code, _ := instance.checkRE(props)
	if code == 1 {
		return IsolateRunMLE, metricObject
	} else if code == 2 {
		return IsolateRunRE, metricObject
	} else if code == -1 {
		log.Println("Log file has incorrect format")
		return IsolateRunOther, RunMetrics{}
	}
	if tle, logFileCorrupted := instance.checkTLE(props); tle {
		return IsolateRunTLE, metricObject
	} else if logFileCorrupted {
		log.Println("Log file corrupted")
		return IsolateRunOther, RunMetrics{}
	}
	log.Println("Verdict unknown")
	return IsolateRunOther, RunMetrics{}
}

func checkRootPermissions() (bool, error) {
	cmd := exec.Command("id", "-u")
	output, err := cmd.Output()
	if err != nil {
		return false, errors.Wrap(err, "checkRootPermissions failed: unable to get current user id")
	}
	// output has a trailing \n, so we need to use a slice of one below the last index
	id, err := strconv.Atoi(string(output[:len(output)-1]))
	if err != nil {
		return false, errors.Wrap(err, "checkRootPermissions failed: unable to parse current user id")
	}
	return id == 0, nil
}

// TODO: Handle IO if needed (test for file IO already handled by the program)
// Specs for little details and protocols should be put in a separate document
