package grader

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"

	"github.com/pkg/errors"
)

func convInterfaceSlicetoStringSlice(inp []interface{}) []string {
	ret := make([]string, 0)
	for _, v := range inp {
		ret = append(ret, v.(string))
	}
	return ret
}

func readManifestFromFile(manifestPath string) (*problemManifest, error) {
	manifestFileBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read manifest.json file at %s", manifestPath)
	}

	var manifestInstance problemManifest
	json.Unmarshal(manifestFileBytes, &manifestInstance)

	manifestInstance.taskBasePath = path.Join(os.Getenv("GRADER_TASK_BASE_PATH"), manifestInstance.ID)
	manifestInstance.inputsBasePath = path.Join(manifestInstance.taskBasePath, "inputs")
	manifestInstance.solutionsBasePath = path.Join(manifestInstance.taskBasePath, "solutions")

	// Check if compile command keys matches language support
	compileCommandKeys := make([]string, len(manifestInstance.CompileCommands))
	i := 0
	for k := range manifestInstance.CompileCommands {
		compileCommandKeys[i] = k
		i++
	}

	sort.Slice(compileCommandKeys, func(i, j int) bool { return compileCommandKeys[i] < compileCommandKeys[j] })
	sort.Slice(manifestInstance.LangSupport, func(i, j int) bool { return manifestInstance.LangSupport[i] < manifestInstance.LangSupport[j] })

	if !reflect.DeepEqual(compileCommandKeys, manifestInstance.LangSupport) {
		return nil, errors.New("Manifest.json invalid: every language supported must have compile commands and vice versa")
	}

	return &manifestInstance, nil
}
