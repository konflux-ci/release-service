/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// GetRelativeDependencyPath returns the relative path for a given module or an empty string on error.
func GetRelativeDependencyPath(moduleName string) string {
	var crdPath string
	currentWorkDir, _ := os.Getwd()

	goModFilepath, err := FindGoModFilepath(currentWorkDir)
	if err != nil {
		return ""
	}

	goModReadFile, err := os.Open(filepath.Clean(goModFilepath))
	if err != nil {
		return ""
	}
	defer func() {
		err := goModReadFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	scanner := bufio.NewScanner(goModReadFile)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), moduleName) {
			crdPath = strings.Trim(scanner.Text(), "\t")
			break
		}
	}

	return strings.ReplaceAll(crdPath, " ", "@")
}

// FindGoModFilepath returns the go.mod file system path for the current project or empty string and an error when file is not found.
func FindGoModFilepath(currentWorkDir string) (string, error) {
	for {
		currentWorkDir = filepath.Dir(currentWorkDir)
		if currentWorkDir != "/" {
			goModFilepath := currentWorkDir + "/go.mod"
			_, err := os.Stat(goModFilepath)
			if err == nil {
				return goModFilepath, nil
			}
		} else {
			return "", errors.New("go.mod file not found")
		}
	}
}
