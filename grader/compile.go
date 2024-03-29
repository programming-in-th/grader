package grader

import (
	"log"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/programming-in-th/grader/conf"
)

// Compiles user source into one file according to arguments in manifest.json
func compileSubmission(submissionID string, taskID string, targLang string, srcPaths []string, compPaths []string, config conf.Config) (bool, string) {
	args := []string{path.Join(BASE_TMP_PATH, submissionID)}
	args = append(args, srcPaths...)
	args = append(args, compPaths...)
	out, err := exec.Command(
		path.Join(config.BasePath, "config", "compileScripts", targLang),
		args...,
	).Output()
	if err != nil {
		log.Println(errors.Wrap(err, "Compile error: error executing compile script"))
		log.Println("Args:", args)
		log.Println("Output:", string(out))
		return false, ""
	}
	out_lines := strings.Split(string(out), "\n")
	for i := 0; i < 2; i++ {
		out_lines[i] = strings.TrimSpace(out_lines[i])
	}
	if len(out_lines) != 2 {
		log.Println(errors.Wrap(err, "Compile error: compile script output is invalid"))
	}
	// Get return code from stdout
	returnCode, err := strconv.Atoi(string(out_lines[0]))
	if err != nil {
		log.Println(errors.Wrap(err, "Compile error: compile script output is invalid"))
	}
	return returnCode == 0, out_lines[1]
}
