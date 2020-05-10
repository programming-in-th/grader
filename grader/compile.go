package grader

import (
	"log"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Compiles user source into one file according to arguments in manifest.json
func compileSubmission(submissionID string, problemID string, targLang string, sourceFilePaths []string, manifestInstance *problemManifest) (bool, string) {
	// This should make a copy
	compileCommands := manifestInstance.compileCommands[targLang]
	// Regexp gets contents of first [i] match including brackets
	reSrc := regexp.MustCompile(`\[(.*?)\]`)
	for i, arg := range compileCommands {
		// TODO: substitue this check with regex
		if len(arg) < 9 {
			continue
		}
		if arg[:9] == "$USER_SRC" {
			// Find $USER_SRC[0]
			val := reSrc.FindString(arg)
			val = strings.ReplaceAll(val, "[", "")
			val = strings.ReplaceAll(val, "]", "")
			if val == "" {
				log.Println("Compile error. Make sure user source files are of the form $USER_SRC[i], where i is the index of the desired source file specified in sourceFilePaths[]")
				return false, ""
			}
			index, err := strconv.ParseInt(val, 0, 0)
			if err != nil {
				log.Println("Compile error. Make sure i in $USER_SRC[i] is a valid integer. Actual value:", val)
				return false, ""
			}
			if int(index) >= len(sourceFilePaths) {
				log.Println("Compile error. Make sure i in $USER_SRC[i] is not out of bounds")
			}
			compileCommands[i] = sourceFilePaths[index]
		} else if arg[:9] == "$USER_BIN" {
			compileCommands[i] = path.Join(manifestInstance.userBinBasePath, submissionID)
			// TODO: check if chmod is needed
		}
	}
	err := exec.Command(compileCommands[0], compileCommands[1:]...).Run()
	if err != nil {
		log.Println("Compile error. Make sure source files are valid paths and manifest.json is using absolute paths only\n", err)
		return false, ""
	}
	return true, path.Join(manifestInstance.userBinBasePath, submissionID)
}
