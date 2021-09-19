/*
Copyright 2020 Wim Henderickx.

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

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/pkg/errors"

	"github.com/yndd/ndd-runtime/pkg/logging"
	"github.com/yndd/ndd-yang/pkg/parser"
	"github.com/yndd/ndd-yang/pkg/resource"
	"github.com/yndd/ndd-ygen/pkg/templ"
	"gopkg.in/yaml.v2"
)

const (
	errResourceInputFileDoesNotExist = "resource input file does not exist, specify with -r"
	errResourceInputFileRead         = "cannot read resource input file"
	errResourceInputFileUnMarshal    = "cannot unmarshal resource input fiel"
	errCannotInitializeResources     = "cannot initialize resource from resource inout file"
	errResourceNotFound              = "cannot find resource"
	errParseTemplate                 = "cannot parse template"
)

type Generator struct {
	parser *parser.Parser
	Config *GeneratorConfig // holds the configuration for the generator
	//ResourceConfig  map[string]*ResourceDetails // holds the configuration of the resources we should generate
	Resources   []*resource.Resource // holds the resources that are being generated
	Entries     []*yang.Entry        // Yang entries parsed from the yang files
	Template    *template.Template
	log         logging.Logger
	LocalRender bool
	Debug       bool
}

type GeneratorConfig struct {
	YangImportDirs []string // the YANG files we need to import to prcess the YANG resource files
	YangModuleDirs []string // the YANG resource files

	ResourceMapInputFile string // the resource input file
	ResourceMapAll       bool   // resource map all
	OutputDir            string // the directory where the resource should be written to
	PackageName          string // the go package we want to geenrate
	Version              string // the version of the api we generate for k8s
	ApiGroup             string // the apigroup we generate for k8s
	Prefix               string // the prefix that is addded to the k8s resource api
}

// ResourceYamlInput struct
type ResourceYamlInput struct {
	Path map[string]PathDetails `yaml:"path"`
}

// PathDetails struct
type PathDetails struct {
	Excludes  []string               `yaml:"excludes"`
	Hierarchy map[string]PathDetails `yaml:"hierarchy"`
}

// Option can be used to manipulate Options.
type Option func(g *Generator)

func WithDebug(d bool) Option {
	return func(g *Generator) {
		g.Debug = d
	}
}

func WithLogging(l logging.Logger) Option {
	return func(g *Generator) {
		g.log = l
	}
}

func WithYangImportDirs(d []string) Option {
	return func(g *Generator) {
		g.Config.YangImportDirs = d
	}
}

func WithYangModuleDirs(d []string) Option {
	return func(g *Generator) {
		g.Config.YangModuleDirs = d
	}
}

func WithResourceMapInputFile(s string) Option {
	return func(g *Generator) {
		g.Config.ResourceMapInputFile = s
	}
}

func WithResourceMapAll(a bool) Option {
	return func(g *Generator) {
		g.Config.ResourceMapAll = a
	}
}

func WithOutputDir(s string) Option {
	return func(g *Generator) {
		g.Config.OutputDir = s
	}
}

func WithPackageName(s string) Option {
	return func(g *Generator) {
		g.Config.PackageName = s
	}
}

func WithVersion(s string) Option {
	return func(g *Generator) {
		g.Config.Version = s
	}
}

func WithAPIGroup(s string) Option {
	return func(g *Generator) {
		g.Config.ApiGroup = s
	}
}

func WithPrefix(s string) Option {
	return func(g *Generator) {
		g.Config.Prefix = s
	}
}

func WithLocalRender(b bool) Option {
	return func(g *Generator) {
		g.LocalRender = b
	}
}

// NewYangGoCodeGenerator function defines a new generator
func NewGenerator(opts ...Option) (*Generator, error) {
	g := &Generator{
		parser: parser.NewParser(),
		Config: new(GeneratorConfig),
		//ResourceConfig:  make(map[string]*ResourceDetails),
		Resources: make([]*resource.Resource, 0),
	}

	for _, o := range opts {
		o(g)
	}

	// process templates to render the resources
	if g.LocalRender {
		var err error
		g.Template, err = templ.ParseTemplates("./templates/")
		if err != nil {
			return nil, errors.New(errParseTemplate)
		}
	}

	// Process resource
	// Check if the resource input file exists
	fmt.Printf("resource input filename : %s\n", g.Config.ResourceMapInputFile)
	if !fileExists(g.Config.ResourceMapInputFile) {
		return nil, errors.New(errResourceInputFileDoesNotExist)
	}

	c := new(ResourceYamlInput)
	yamlFile, err := ioutil.ReadFile(g.Config.ResourceMapInputFile)
	if err != nil {
		return nil, errors.Wrap(err, errResourceInputFileRead)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, errors.Wrap(err, errResourceInputFileUnMarshal)
	}

	// initialize the resources from the YAML input file, we start at the root level using "/" path
	if err := g.InitializeResourcesNew(c.Path, "/", 1); err != nil {
		return nil, errors.Wrap(err, errCannotInitializeResources)
	}
	// show the result of the processed resources
	g.ShowResources()

	// initialize goyang, with the information supplied from the flags
	g.Entries, err = g.InitializeGoYang()
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *Generator) GetEntries() []*yang.Entry {
	return g.Entries
}

func (g *Generator) ShowConfiguration() {
	log := g.log.WithValues("API Group", g.Config.ApiGroup,
		"Version", g.Config.Version,
		"Config file", g.Config.ResourceMapInputFile,
		"Yang import directory", g.Config.YangImportDirs)
	log.Debug("Generator configuration")
}

func (g *Generator) ShowResources() {
	for i, r := range g.Resources {
		fmt.Printf("Nbr: %d, Resource Path: %s, Exclude: %v\n", i, *r.GetAbsoluteXPath(), r.GetExcludeRelativeXPath())
	}
}

func (g *Generator) FindResource(p string) (*resource.Resource, error) {
	//fmt.Printf("find resource\n")
	for _, r := range g.Resources {
		//fmt.Printf("find resource path %s %s\n", p, *parser.GnmiPathToXPath(r.Path))
		if p == *g.parser.GnmiPathToXPath(r.Path, false) {
			return r, nil
		}
	}
	return nil, errors.New(errResourceNotFound)
}

// initializes the resource based on the YAML file input
// The result is stored in the []*Resource list
// A resource contains the relative information of the resource.
// To DependsOn allows you to reference parent resources
func (g *Generator) InitializeResourcesNew(pd map[string]PathDetails, pp string, offset int) error {
	// we just need to generic resourcePath and we dont need to process the individual paths
	// the first entry is sufficient
	// we just take the first element of the path and we are done
	if g.Config.ResourceMapAll {
		for path := range pd {
			opts := []resource.Option{
				resource.WithXPath("/" + strings.Split(path, "/")[1]),
			}
			g.Resources = append(g.Resources, resource.NewResource(opts...))
			return nil
		}
	}
	for path, pathdetails := range pd {
		//g.log.Debug("Path information", "Path", path, "parent path", pp)
		opts := []resource.Option{
			resource.WithXPath(path),
		}
		if pp != "/" {
			// this is a hierarchical resource, find the hierarchical dependency
			r, err := g.FindResource(pp)
			if err != nil {
				return err
			}
			opts = append(opts, resource.WithDependsOn(r))
		}
		// exclude belongs to the previous resource and hence we have to
		// append the exclude element info to the previous path
		for _, e := range pathdetails.Excludes {
			g.log.Debug("Exludes", "Exclude", e)
			opts = append(opts, resource.WithExclude(filepath.Join(path, "/", e)))
		}

		// initialize the resource before processing the next hierarchy since the process will check
		// the dependency and if not initialized the parent resource will not be found.
		g.Resources = append(g.Resources, resource.NewResource(opts...))
		if pathdetails.Hierarchy != nil {
			// run the procedure in a hierarchical way, offset is 0 since the resource does not have
			// a duplicate element in the path
			if err := g.InitializeResourcesNew(pathdetails.Hierarchy, path, 0); err != nil {
				return err
			}
		}

	}
	return nil
}

// GOYANG processing
// Read and validate the import directory with yang module
func (g *Generator) InitializeGoYang() ([]*yang.Entry, error) {
	// GOYANG processing
	// Read and validate the import directory with yang module
	for _, path := range g.Config.YangImportDirs {
		expanded, err := yang.PathsWithModules(path)
		if err != nil {
			return nil, err
			//continue
		}
		//g.log.Debug("Expanded info", "Expanded", expanded)
		yang.AddPath(expanded...)
	}
	//g.log.Debug("Yang Path Info", "Path", yang.Path)

	// Initialize yang modules
	ms := yang.NewModules()

	// Read the yang directory
	for _, d := range g.Config.YangModuleDirs {
		fi, err := os.Stat(d)
		if err != nil {
			return nil, err
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			// Handle directory files input
			files, err := ioutil.ReadDir(d)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				//g.log.Debug("Yang File Info", "FileName", d+"/"+f.Name())
				if err := ms.Read(d + "/" + f.Name()); err != nil {
					return nil, err
				}
			}
		case mode.IsRegular():
			// Handle file input
			//g.log.Debug("Yang File Info", "FileName", fi.Name())
			if err := ms.Read(filepath.Dir(d) + fi.Name()); err != nil {
				return nil, err
				//continue
			}
		}
	}

	// Process the yang modules
	errs := ms.Process()
	if len(errs) > 0 {
		for err := range errs {
			g.log.Debug("Error", "error", err)
		}
	}
	// Keep track of the top level modules we read in.
	// Those are the only modules we want to process.
	mods := map[string]*yang.Module{}
	var names []string
	for _, m := range ms.Modules {
		if mods[m.Name] == nil {
			mods[m.Name] = m
			names = append(names, m.Name)
		}
	}
	sort.Strings(names)
	entries := make([]*yang.Entry, len(names))
	for x, n := range names {
		entries[x] = yang.ToEntry(mods[n])
	}
	return entries, nil
}

func (g *Generator) Run() error {
	// Augment the data
	for _, e := range g.Entries {
		//g.log.Debug("Yang global Entry: ", "Nbr", i, "Name", e.Name)

		// initialize an empty path
		path := gnmi.Path{
			Elem: make([]*gnmi.PathElem, 0),
		}
		if err := g.ResourceGenerator("", path, e, false, ""); err != nil {
			return err
		}
	}
	return nil
}
