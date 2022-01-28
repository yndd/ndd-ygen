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
	"github.com/yndd/ndd-yang/pkg/resource"
	"github.com/yndd/ndd-yang/pkg/yparser"
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
	//parser *parser.Parser
	config *Config // holds the configuration for the generator
	//ResourceConfig  map[string]*ResourceDetails // holds the configuration of the resources we should generate
	schema       string
	resources    []*resource.Resource // holds the resources that are being generated
	rootResource *resource.Resource
	entries      []*yang.Entry // Yang entries parsed from the yang files
	template     *template.Template
	log          logging.Logger
	healthStatus bool
	localRender  bool
	debug        bool
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

func WithHealthStatus(b bool) Option {
	return func(g *Generator) {
		g.healthStatus = b
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
		config:    &Config{},
		resources: make([]*resource.Resource, 0),
	}

	for _, o := range opts {
		o(g)
	}

	// process templates to render the resources
	if g.GetLocalRender() {
		if err := g.initTemplates(); err != nil {
			return nil, errors.New(errParseTemplate)
		}
	}

	// Process resource
	// Check if the resource input file exists
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

	g.schema = c.Schema

	// initialize the resources from the YAML input file, we start at the root level using "/" path
	g.rootResource = resource.NewResource(nil)
	g.resources = append(g.GetResources(), g.rootResource)
	//if !g.GetConfig().GetResourceMapAll() {
	if err := g.InitializeResources(c.Path, "/", g.rootResource); err != nil {
		return nil, errors.Wrap(err, errCannotInitializeResources)
	}
	//}

	// show the result of the processed resources
	//g.ShowResources()

	// initialize goyang, with the information supplied from the flags
	g.entries, err = g.initializeGoYang()
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

func (g *Generator) GetRootResource() *resource.Resource {
	return g.rootResource
}

func (g *Generator) GetLocalRender() bool {
	return g.localRender
}

func (g *Generator) GetDebug() bool {
	return g.debug
}

func (g *Generator) getEntries() []*yang.Entry {
	return g.entries
}

func (g *Generator) getTemplate() *template.Template {
	return g.template
}

func (g *Generator) initTemplates() error {
	var err error
	g.template, err = templ.ParseTemplates("./templates/")
	if err != nil {
		return err
	}
	return nil
}

// GOYANG processing
// Read and validate the import directory with yang module
func (g *Generator) initializeGoYang() ([]*yang.Entry, error) {
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
	for _, e := range g.getEntries() {
		//g.log.Debug("Yang global Entry: ", "Nbr", i, "Name", e.Name)

		// initialize an empty path
		path := &gnmi.Path{
			Elem: make([]*gnmi.PathElem, 0),
		}
		if err := g.ResourceGenerator("", path, e, false, ""); err != nil {
			return err
		}
	}
	// updates the container has state
	g.updateContainerStateChildStatus()
	return nil
}

// updateContainerStateChildStatus updates the container HAs state info.
// we first look at the entries and if one has state, we update the state from bottom to top
func (g *Generator) updateContainerStateChildStatus() {
	for _, r := range g.GetActualResources()[1:] {
		for _, c := range r.ContainerList {
			for _, e := range c.GetEntries() {
				if e.ReadOnly {
					c.SetHasState()
				}
			}
			if c.HasState {
				c.UpdateHasState2ParentContainers()
			}
		}
	}
}

// initializes the resource based on the YAML file input or generate all individual container entries
// The result is stored in the []*Resource list
// A resource contains the relative information of the resource.
// we generate both a resource list as well as a linked list with parent and child
func (g *Generator) InitializeResources(pd map[string]PathDetails, pp string, parent *resource.Resource) error {
	for path, pathdetails := range pd {
		//g.log.Debug("Path information", "Path", path, "parent path", pp)
		opts := []resource.Option{}
		if pp == "/" {
			// this is attached to the root resource

			// initialize options that will be used in the resource
			// add resourcepath
			opts = append(opts, resource.WithXPath(path))
			// add module
			opts = append(opts, resource.WithModule(strings.Split(path, "/")[1]))
		} else {
			// this is a hierarchical resource

			// add resourcepath
			opts = append(opts, resource.WithXPath(path))
			// add module
			opts = append(opts, resource.WithModule(parent.GetModule()))
		}

		// exclude belongs to the previous resource and hence we have to
		// append the exclude element info to the previous path
		for _, e := range pathdetails.Excludes {
			g.log.Debug("Exludes", "Exclude", e)
			opts = append(opts, resource.WithExclude(filepath.Join(path, "/", e)))
		}

		// initialize the resource
		newResource := resource.NewResource(parent, opts...)
		//fmt.Printf("new resource path: %s\n", yparser.GnmiPath2XPath(newResource.GetAbsolutePath(), false))
		parent.AddChild(newResource)
		g.resources = append(g.GetResources(), newResource)
		if pathdetails.Hierarchy != nil {
			// run the procedure in a hierarchical way, offset is 0 since the resource does not have
			// a duplicate element in the path
			/*
				for hpath := range pathdetails.Hierarchy {
					g.GetResources()[len(g.GetResources())-1].GetHierResourceElement().AddHierResourceElement(hpath)
				}
			*/

			// run the resource mapping in a hierarchical way
			if err := g.InitializeResources(pathdetails.Hierarchy, path, newResource); err != nil {
				return err
			}
		}

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
		//if r.GetParent() != nil {
		fmt.Printf("Nbr: %d, Resource Path: %s, Exclude: %v, ParentPath: %v\n", i, yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), r.GetExcludeRelativeXPath(), yparser.GnmiPath2XPath(r.GetParentPath(), false))
		//} else {
		//	fmt.Printf("Nbr: %d, Resource Path: %s, Exclude: %v, DependsOn: %v\n", i, *r.GetAbsoluteXPath(), r.GetExcludeRelativeXPath(), r.GetParent())
		//}
		//fmt.Printf(" HierResourceElements: %v\n", r.GetHierResourceElements().GetHierResourceElements())
		//for _, subres := range r.GetActualSubResources() {
		//	fmt.Printf("  Subsresource: %s\n", yparser.GnmiPath2XPath(subres, false))
		//}
	}
}

func (g *Generator) ShowActualPathPerResource() {
	for _, r := range g.GetActualResources() {
		fmt.Printf("Resource Path: %s\n", yparser.GnmiPath2XPath(r.GetAbsolutePath(), false))
	}
}
