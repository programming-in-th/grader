package conf

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sync"

	"github.com/pkg/errors"
)

/* TEST RESULT TYPES */

const (
	// ACVerdict means the program passed the test
	ACVerdict string = "Correct"
	// PartialVerdict means the program was partially correct on the test
	PartialVerdict string = "Partially Correct"
	// WAVerdict means the program got the wrong answer on the test
	WAVerdict string = "Incorrect"
	// TLEVerdict means the program timed out
	TLEVerdict string = "Time Limit Exceeded"
	// MLEVerdict means the program used too much memory
	MLEVerdict string = "Memory Limit Exceeded"
	// REVerdict means the program caused a runtime error (not including MLE)
	REVerdict string = "Memory Limit Exceeded"
	// IEVerdict means an internal error of the grader occurred
	IEVerdict string = "Judge Error"
	// SKVerdict means the test was skipped because a dependent group was not passed
	SKVerdict string = "Skipped"
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

type SafeBoxIDPool struct {
	BoxIDs map[int]bool
	Mux    sync.Mutex
}

type Config struct {
	BasePath  string
	BoxIDPool *SafeBoxIDPool
	Glob      GlobalConfiguration
}

var PossibleCheckerVerdicts = []string{ACVerdict, PartialVerdict, WAVerdict, IEVerdict}

func GetLangCompileConfig(config Config, targLang string) *LangCompileConfiguration {
	// Find target language's compile configuration
	foundLang := false
	var langConfig LangCompileConfiguration
	for _, langConfig = range config.Glob.CompileConfiguration {
		if langConfig.ID == targLang {
			foundLang = true
			break
		}
	}
	if !foundLang {
		return nil
	}
	return &langConfig
}

func readGlobalConfig(globalConfigPath string) (GlobalConfiguration, error) {
	configFileBytes, err := ioutil.ReadFile(globalConfigPath)
	if err != nil {
		return GlobalConfiguration{}, errors.Wrapf(err, "Failed to read global configuration file at %s", globalConfigPath)
	}

	var globalConfigInstance GlobalConfiguration
	json.Unmarshal(configFileBytes, &globalConfigInstance)

	// Check that each verdict is present
	for _, checkerVerdict := range PossibleCheckerVerdicts {
		if _, exists := globalConfigInstance.DefaultMessages[checkerVerdict]; !exists {
			return GlobalConfiguration{}, errors.Wrap(err, "Global configuration format incorrect: incomplete parameters")
		}
	}

	// Fill blanks for verdicts not specified
	if _, exists := globalConfigInstance.DefaultMessages[TLEVerdict]; !exists {
		globalConfigInstance.DefaultMessages[TLEVerdict] = ""
	}
	if _, exists := globalConfigInstance.DefaultMessages[MLEVerdict]; !exists {
		globalConfigInstance.DefaultMessages[MLEVerdict] = ""
	}
	if _, exists := globalConfigInstance.DefaultMessages[REVerdict]; !exists {
		globalConfigInstance.DefaultMessages[REVerdict] = ""
	}

	return globalConfigInstance, nil
}

func InitConfig(basePath string) Config {
	// Check if base path exists
	_, err := os.Stat(basePath)
	if err != nil && os.IsNotExist(err) {
		log.Fatal("Base path doesn't exist")
	}

	// Read global config
	globalConfig, err := readGlobalConfig(path.Join(basePath, "config", "globalConfig.json"))
	if err != nil {
		log.Fatal("Error reading global configuration file")
	}

	boxIDPool := SafeBoxIDPool{BoxIDs: make(map[int]bool)}

	confInstance := Config{basePath, &boxIDPool, globalConfig}
	return confInstance
}
