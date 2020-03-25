package main

func main() {
	checkRootPermissions()
	instance := &IsolateInstance{boxID: 0, execFile: "program", ioMode: 1, logFile: "meta", timeLimit: 1, memoryLimit: 262644, inputFile: "input", outputFile: "output"} // TODO: handle box ids
	instance.isolateRun()
	// instance.isolateCleanup()
}

// TODO: handle paths
// NOTE: filesystem access is already restricted for the use cases of freopen
// NOTE2: isolate is required to be installed to /usr/bin
// NOTE: input, output files are scoped within the box directory, but logFile is not
