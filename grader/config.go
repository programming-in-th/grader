package grader

import (
	"encoding/json"
	"io/ioutil"

	"github.com/pkg/errors"
)

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
			return nil, errors.Wrap(err, "Global configuration format incorrect: incomplete parameters")
		}
	}

	return &globalConfigInstance, nil
}
