package grader

import (
	"log"
	"os/exec"
	"path"

	"github.com/pkg/errors"
)

// Compiles user source into one file according to arguments in manifest.json
func compileSubmission(submissionID string, taskID string, sourceFilePaths []string, compileCommands []string) (bool, string) {
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
			compileCommands[i] = path.Join(BASE_TMP_PATH, submissionID, "bin")
		}
	}
	err := exec.Command(compileCommands[0], compileCommands[1:]...).Run()
	if err != nil {
		log.Println(errors.Wrap(err, "Compile error. Make sure source files are valid paths and manifest.json is using absolute paths only"))
		log.Println("Compile commands:", compileCommands)
		return false, ""
	}
	return true, path.Join(BASE_TMP_PATH, submissionID, "bin")
}
