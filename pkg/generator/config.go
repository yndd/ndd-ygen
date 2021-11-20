/*
Copyright 2020 Yndd.

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

package generator

// ResourceYamlInput struct
type ResourceYamlInput struct {
	Path map[string]PathDetails `yaml:"path"`
}

// PathDetails struct
type PathDetails struct {
	SubResources []string               `yaml:"sub-resources"`
	Excludes     []string               `yaml:"excludes"`
	Hierarchy    map[string]PathDetails `yaml:"hierarchy"`
}

type Config struct {
	yangImportDirs []string // the YANG files we need to import to prcess the YANG resource files
	yangModuleDirs []string // the YANG resource files

	resourceMapInputFile string // the resource input file
	resourceMapAll       bool   // resource map all
	outputDir            string // the directory where the resource should be written to
	packageName          string // the go package we want to geenrate
	version              string // the version of the api we generate for k8s
	apiGroup             string // the apigroup we generate for k8s
	prefix               string // the prefix that is addded to the k8s resource api
}

func (c *Config) GetYangImportDirs() []string {
	return c.yangImportDirs
}

func (c *Config) GetYangModuleDirs() []string {
	return c.yangModuleDirs
}

func (c *Config) GetResourceMapInputFile() string {
	return c.resourceMapInputFile
}

func (c *Config) GetResourceMapAll() bool {
	return c.resourceMapAll
}

func (c *Config) GetOutputDir() string {
	return c.outputDir
}

func (c *Config) GetPackageName() string {
	return c.packageName
}

func (c *Config) GetVersion() string {
	return c.version
}

func (c *Config) GetApiGroup() string {
	return c.apiGroup
}

func (c *Config) GetPrefix() string {
	return c.prefix
}