package grader

import (
	"encoding/json"
	"io/ioutil"
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

	var v interface{}
	json.Unmarshal(manifestFileBytes, &v)
	data := v.(map[string]interface{})

	var manifestInstance problemManifest
	manifestInstance.id = data["id"].(string)
	manifestInstance.taskBasePath = path.Join(taskBasePath, manifestInstance.id)
	manifestInstance.timeLimit = data["timeLimit"].(float64)
	manifestInstance.memoryLimit = int(data["memoryLimit"].(float64))
	manifestInstance.langSupport = convInterfaceSlicetoStringSlice(data["langSupport"].([]interface{}))
	manifestInstance.testInputs = convInterfaceSlicetoStringSlice(data["testInputs"].([]interface{}))
	manifestInstance.testSolutions = convInterfaceSlicetoStringSlice(data["testSolutions"].([]interface{}))
	manifestInstance.compileCommands =
		func(inp map[string]interface{}) map[string][]string {
			ret := make(map[string][]string)
			for k, v := range inp {
				ret[k] = convInterfaceSlicetoStringSlice(v.([]interface{}))
			}
			return ret
		}(data["compileCommands"].(map[string]interface{}))

	manifestInstance.userBinBasePath = path.Join(manifestInstance.taskBasePath, "user_bin")
	manifestInstance.inputsBasePath = path.Join(manifestInstance.taskBasePath, "inputs")
	manifestInstance.outputsBasePath = path.Join(manifestInstance.taskBasePath, "outputs")
	manifestInstance.solutionsBasePath = path.Join(manifestInstance.taskBasePath, "solutions")
	manifestInstance.checkerPath = data["checkerPath"].(string)

	// Check if compile command keys matches language support
	compileCommandKeys := make([]string, len(manifestInstance.compileCommands))
	i := 0
	for k := range manifestInstance.compileCommands {
		compileCommandKeys[i] = k
		i++
	}

	sort.Slice(compileCommandKeys, func(i, j int) bool { return compileCommandKeys[i] < compileCommandKeys[j] })
	sort.Slice(manifestInstance.langSupport, func(i, j int) bool { return manifestInstance.langSupport[i] < manifestInstance.langSupport[j] })

	if !reflect.DeepEqual(compileCommandKeys, manifestInstance.langSupport) {
		return nil, errors.New("Manifest.json invalid: every language supported must have compile commands and vice versa")
	}

	return &manifestInstance, nil
}
