package grader

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/pkg/errors"
)

var possibleCheckerVerdicts = []string{"Correct", "Partially Correct", "Wrong Answer", "Time Limit Exceeded", "Memory Limit exceeded", "Runtime Error"}

type LangCompileConfiguration struct {
	ID              string
	Extension       string
	CompileCommands []string
}

type GlobalConfiguration struct {
	CompileConfiguration []LangCompileConfiguration
	DefaultMessages      map[string]string
	IsolateBinPath       string
}

func ReadGlobalConfig(globalConfigPath string) (*GlobalConfiguration, error) {
	configFileBytes, err := ioutil.ReadFile(globalConfigPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read global configuration file at %s", globalConfigPath)
	}

	var globalConfigInstance GlobalConfiguration
	json.Unmarshal(configFileBytes, &globalConfigInstance)

	// Check that each verdict is present
	for _, checkerVerdict := range possibleCheckerVerdicts {
		if _, exists := globalConfigInstance.DefaultMessages[checkerVerdict]; !exists {
			log.Fatal("Global configuration format incorrect: incomplete parameters")
		}
	}

	return &globalConfigInstance, nil
}
