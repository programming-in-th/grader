package grader

import (
	"log"
	"os/exec"
	"path"
)

// Compiles user source into one file according to arguments in manifest.json
func compileSubmission(submissionID string, problemID string, targLang string, sourceFilePaths []string, manifestInstance *problemManifest) (bool, string) {
	// This should make a copy
	compileCommands := manifestInstance.CompileCommands[targLang]
	// Regexp gets contents of first [i] match including brackets
	sourceFileIndex := 0
	for i, arg := range compileCommands {
		if len(arg) != 9 {
			continue
		}
		if arg[:9] == "$USER_SRC" {
			if sourceFileIndex == len(sourceFilePaths) {
				log.Println("Compile error: too many $USER_SRC but not enough source files.")
				return false, ""
			}
			compileCommands[i] = sourceFilePaths[sourceFileIndex]
			sourceFileIndex++
		} else if arg[:9] == "$USER_BIN" {
			compileCommands[i] = path.Join(manifestInstance.userBinBasePath, submissionID)
		}
	}
	err := exec.Command(compileCommands[0], compileCommands[1:]...).Run()
	if err != nil {
		log.Println("Compile error. Make sure source files are valid paths and manifest.json is using absolute paths only\n", err)
		return false, ""
	}
	return true, path.Join(manifestInstance.userBinBasePath, submissionID)
}
