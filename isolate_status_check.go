package main

import (
	"strconv"
	"strings"
)

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
