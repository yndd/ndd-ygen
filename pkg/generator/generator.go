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
	"github.com/yndd/ndd-ygen/pkg/utils"
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
	config *Config // holds the configuration for the generator
	//ResourceConfig  map[string]*ResourceDetails // holds the configuration of the resources we should generate
	resources      []*resource.Resource // holds the resources that are being generated
	entries        []*yang.Entry // Yang entries parsed from the yang files
	template       *template.Template
	log            logging.Logger
	localRender    bool
	debug          bool
}

// Option can be used to manipulate Options.
type Option func(g *Generator)

func WithDebug(d bool) Option {
	return func(g *Generator) {
		g.debug = d
	}
}

func WithLogging(l logging.Logger) Option {
	return func(g *Generator) {
		g.log = l
	}
}

func WithParser(l logging.Logger) Option {
	return func(g *Generator) {
		g.parser = parser.NewParser(parser.WithLogger(l))
	}
}

func WithYangImportDirs(d []string) Option {
	return func(g *Generator) {
		g.config.yangImportDirs = d
	}
}

func WithYangModuleDirs(d []string) Option {
	return func(g *Generator) {
		g.config.yangModuleDirs = d
	}
}

func WithResourceMapInputFile(s string) Option {
	return func(g *Generator) {
		g.config.resourceMapInputFile = s
	}
}

func WithResourceMapAll(a bool) Option {
	return func(g *Generator) {
		g.config.resourceMapAll = a
	}
}

func WithOutputDir(s string) Option {
	return func(g *Generator) {
		g.config.outputDir = s
	}
}

func WithPackageName(s string) Option {
	return func(g *Generator) {
		g.config.packageName = s
	}
}

func WithVersion(s string) Option {
	return func(g *Generator) {
		g.config.version = s
	}
}

func WithAPIGroup(s string) Option {
	return func(g *Generator) {
		g.config.apiGroup = s
	}
}

func WithPrefix(s string) Option {
	return func(g *Generator) {
		g.config.prefix = s
	}
}

func WithLocalRender(b bool) Option {
	return func(g *Generator) {
		g.localRender = b
	}
}

// NewYangGoCodeGenerator function defines a new generator
func NewGenerator(opts ...Option) (*Generator, error) {
	g := &Generator{
		parser: parser.NewParser(),
		config: &Config{},
		//ResourceConfig:  make(map[string]*ResourceDetails),
		resources: make([]*resource.Resource, 0),
	}

	for _, o := range opts {
		o(g)
	}

	// process templates to render the resources
	if g.GetLocalRender() {
		if err := g.InitTemplates(); err != nil {
			return nil, errors.New(errParseTemplate)
		}
	}

	// Process resource
	// Check if the resource input file exists
	//fmt.Printf("resource input filename : %s\n", g.GetConfig().GetResourceMapInputFile())
	if !utils.FileExists(g.GetConfig().GetResourceMapInputFile()) {
		return nil, errors.New(errResourceInputFileDoesNotExist)
	}

	c := &ResourceYamlInput{}
	yamlFile, err := ioutil.ReadFile(g.GetConfig().GetResourceMapInputFile())
	if err != nil {
		return nil, errors.Wrap(err, errResourceInputFileRead)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, errors.Wrap(err, errResourceInputFileUnMarshal)
	}

	// initialize the resources from the YAML input file, we start at the root level using "/" path
	if err := g.InitializeResources(c.Path, "/", 1); err != nil {
		return nil, errors.Wrap(err, errCannotInitializeResources)
	}
	// show the result of the processed resources
	//g.ShowResources()

	// initialize goyang, with the information supplied from the flags
	g.entries, err = g.InitializeGoYang()
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *Generator) GetConfig() *Config {
	return g.config
}

func (g *Generator) GetResources() []*resource.Resource {
	return g.resources
}

func (g *Generator) GetEntries() []*yang.Entry {
	return g.entries
}

func (g *Generator) GetTemplate() *template.Template {
	return g.template
}

func (g *Generator) GetLocalRender() bool {
	return g.localRender
}

func (g *Generator) GetDebug() bool {
	return g.debug
}

func (g *Generator) InitTemplates() error {
	var err error
	g.template, err = templ.ParseTemplates("./templates/")
	if err != nil {
		return err
	}
	return nil
}

func (g *Generator) ShowConfiguration() {
	log := g.log.WithValues("API Group", g.GetConfig().GetApiGroup(),
		"Version", g.GetConfig().GetVersion(),
		"Config file", g.GetConfig().GetResourceMapInputFile(),
		"Yang import directory", g.GetConfig().GetYangImportDirs())
	log.Debug("Generator configuration")
}

func (g *Generator) ShowResources() {
	for i, r := range g.GetResources() {
		if r.GetDependsOn() != nil {
			fmt.Printf("Nbr: %d, Resource Path: %s, Exclude: %v, DependsOnPath: %v\n", i, *r.GetAbsoluteXPath(), r.GetExcludeRelativeXPath(), *g.parser.GnmiPathToXPath(r.GetDependsOnPath(), false))
		} else {
			fmt.Printf("Nbr: %d, Resource Path: %s, Exclude: %v, DependsOn: %v\n", i, *r.GetAbsoluteXPath(), r.GetExcludeRelativeXPath(), r.GetDependsOn())
		}
		fmt.Printf(" HierResourceElements: %v\n", r.GetHierResourceElements().GetHierResourceElements())
		for _, subres := range r.GetActualSubResources() {
			fmt.Printf("  Subsresource: %s\n", *g.parser.GnmiPathToXPath(subres, false))
		}
	}
}

func (g *Generator) ShowActualPathPerResource() {
	for _, r := range g.GetActualResources() {
		fmt.Printf("Resource Path: %s\n", *r.GetAbsoluteXPath())
		/*
		for _, pe := range r.GetActualGnmiFullPathWithKeys().GetElem() {
			fmt.Printf("Path Element: PathElem Name: %s PathElem Key: %v\n", pe.GetName(), pe.GetKey())
		}
		*/
	}
}

func (g *Generator) FindResource(p string) (*resource.Resource, error) {
	//fmt.Printf("find resource\n")
	for _, r := range g.GetResources() {
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
func (g *Generator) InitializeResources(pd map[string]PathDetails, pp string, offset int) error {
	// when we want to generate the full resource e.g. for state use cases we just need
	// the generic resourcePath and we dont need to process the individual paths in the resource file
	// the first entry is sufficient
	// we just take the first element of the path and we are done
	/*
	if g.GetConfig().GetResourceMapAll() {
		for path := range pd {
			opts := []resource.Option{
				resource.WithXPath("/" + strings.Split(path, "/")[1]),
			}
			g.resources = append(g.GetResources(), resource.NewResource(opts...))
			return nil
		}
	}
	*/
	for path, pathdetails := range pd {
		//g.log.Debug("Path information", "Path", path, "parent path", pp)
		opts := []resource.Option{}
		if pp != "/" {
			// this is a hierarchical resource, find the hierarchical dependency
			r, err := g.FindResource(pp)
			if err != nil {
				return err
			}
			//
			split := strings.Split(path, "/")
			// if the hierarchical path consists of multiple path only the last element of the
			// hierarchical path is relevaant in the hierarchical context
			// the other path elements reside in the parent resource and hence will be part of the
			// dependency path
			dp := g.parser.DeepCopyGnmiPath(r.Path)
			if len(split) > 2 {
				for i := 1; i < len(split)-1; i++ {
					dp = g.parser.AppendElemInGnmiPath(dp, split[i], []string{})
				}
			}
			// the resource path is only consisting of the last element of the hierarchical path
			opts = append(opts, resource.WithXPath("/"+split[len(split)-1]))
			// add resource dependency with dependency path
			opts = append(opts, resource.WithDependsOnPath(dp))
			opts = append(opts, resource.WithDependsOn(r))
			// add subresources
			subResPaths := make([]*gnmi.Path, 0)
			if len(pathdetails.SubResources) == 0 {
				// no subresources exists -> initialize with the resource path
				subResPaths = append(subResPaths, g.parser.XpathToGnmiPath("/"+split[len(split)-1], 0))
			}
			for _, subres := range pathdetails.SubResources {
				subResPaths = append(subResPaths, g.parser.XpathToGnmiPath(filepath.Join("/"+split[len(split)-1], subres), 0))
			}
			opts = append(opts, resource.WithSubResources(subResPaths))
			// add module
			opts = append(opts, resource.WithModule(strings.Split(path, "/")[1]))
			// add module
			opts = append(opts, resource.WithModule(r.GetModule()))
		} else {
			// this is a root resource

			// initialize the module if this is a parent resource
			// add resourcepath
			opts = append(opts, resource.WithXPath(path))
			// add subresources
			subResPaths := make([]*gnmi.Path, 0)
			if len(pathdetails.SubResources) == 0 {
				// no subresources exists -> initialize with the resource path
				subResPaths = append(subResPaths, g.parser.XpathToGnmiPath(path, 0))
			}
			for _, subres := range pathdetails.SubResources {
				subResPaths = append(subResPaths, g.parser.XpathToGnmiPath(filepath.Join(path, subres), 0))
			}
			opts = append(opts, resource.WithSubResources(subResPaths))
			// add module
			opts = append(opts, resource.WithModule(strings.Split(path, "/")[1]))
		}
		// exclude belongs to the previous resource and hence we have to
		// append the exclude element info to the previous path
		for _, e := range pathdetails.Excludes {
			g.log.Debug("Exludes", "Exclude", e)
			opts = append(opts, resource.WithExclude(filepath.Join(path, "/", e)))
		}

		// initialize the resource before processing the next hierarchy since the process will check
		// the dependency and if not initialized the parent resource will not be found.
		g.resources = append(g.GetResources(), resource.NewResource(opts...))
		if pathdetails.Hierarchy != nil {
			// run the procedure in a hierarchical way, offset is 0 since the resource does not have
			// a duplicate element in the path
			for hpath := range pathdetails.Hierarchy {
				//fmt.Printf("hpath: %s\n", hpath)
				g.GetResources()[len(g.GetResources())-1].GetHierResourceElement().AddHierResourceElement(hpath)
			}
			if err := g.InitializeResources(pathdetails.Hierarchy, path, 0); err != nil {
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
	for _, path := range g.GetConfig().GetYangImportDirs() {
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
	for _, d := range g.GetConfig().GetYangModuleDirs() {
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
	for _, e := range g.GetEntries() {
		//g.log.Debug("Yang global Entry: ", "Nbr", i, "Name", e.Name)

		// initialize an empty path
		path := &gnmi.Path{
			Elem: make([]*gnmi.PathElem, 0),
		}
		if err := g.ResourceGenerator("", path, e, false, ""); err != nil {
			return err
		}
	}
	return nil
}
