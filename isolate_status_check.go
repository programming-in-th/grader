package main

import (
	"strconv"
)

func (instance *IsolateInstance) checkTLE(props map[string]string) bool {
	return true
}

func (instance *IsolateInstance) checkMLE(props map[string]string) bool {
	memoryUsageString, maxRssExists := props["max-rss"]
	exitsig, exitSigExists := props["exitsig"]
	status, statusExists := props["status"]
	memoryUsage, err := strconv.Atoi(memoryUsageString)
	if !maxRssExists || err != nil {
		throwLogFileCorrupted()
	}
	if !exitSigExists || !statusExists {
		return false
	}
	return memoryUsage > instance.memoryLimit && exitsig == "11" && status == "SG"
}

func (instance *IsolateInstance) checkRE(props map[string]string) bool {
	return true
}

func (instance *IsolateInstance) checkXX(props map[string]string) bool {
	return true
}
