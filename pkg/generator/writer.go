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

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stoewer/go-strcase"
	"github.com/yndd/ndd-yang/pkg/container"
	"github.com/yndd/ndd-yang/pkg/parser"
	"github.com/yndd/ndd-yang/pkg/resource"
)

func (g *Generator) Render() error {
	// Render the data
	for _, r := range g.Resources {
		fmt.Printf("Resource: %s, %#v\n", r.GetResourceNameWithPrefix(g.Config.Prefix), r.GetRootContainerEntry().GetKey())
		r.AssignFileName(g.Config.Prefix, "_types.go")
		if err := r.CreateFile(g.Config.OutputDir, "api", g.Config.Version); err != nil {
			return err
		}
		if err := g.WriteResourceHeader(r); err != nil {
			g.log.Debug("Write resource header error", "error", err)
			return err
		}

		for _, c := range r.ContainerList {
			if err := g.WriteResourceContainers(r, c); err != nil {
				g.log.Debug("Write resource container error", "error", err)
				return err
			}

			/*
				fmt.Printf("Nbr: %d, ResourceName: %s, Container Name: %s\n", i, *r.GetAbsoluteXPath(), c.Name)
				for n, e := range c.Entries {
					if e.Next != nil {
						fmt.Printf("  Entry: %d, Name: %s, Type: %s, Mandatory: %t WithNewContainerPointer\n", n, e.Name, c.Name+e.Name, e.Mandatory)
					} else {
						fmt.Printf("  Entry: %d, Name: %s, Type: %s, Mandatory: %t\n", n, e.Name, e.Type, e.Mandatory)
					}
				}
			*/
		}

		if err := g.WriteResourceEnd(r); err != nil {
			g.log.Debug("Write resource end error", "error", err)
			return err
		}

		// EXPERIMENTAL
		if err := g.WriteResourceLocalLeafRef(r); err != nil {
			g.log.Debug("Write resource local leafRef error", "error", err)
			return err
		}
		if err := g.WriteResourceExternalLeafRef(r); err != nil {
			g.log.Debug("Write resource external leafRef error", "error", err)
			return err
		}

		if err := r.CloseFile(); err != nil {
			return err
		}
	}
	return nil
}

// WriteResourceHeader
func (g *Generator) WriteResourceHeader(r *resource.Resource) error {
	s := struct {
		Version                string
		ApiGroup               string
		ResourceLastElement    string
		ResourceNameWithPrefix string
		ResourceTest1          *gnmi.Path
		ResourceTest2          *gnmi.Path
		ResourceTest3          *gnmi.Path
	}{
		Version:                g.Config.Version,
		ApiGroup:               g.Config.ApiGroup,
		ResourceLastElement:    strcase.LowerCamelCase(r.ResourceLastElement()),
		ResourceNameWithPrefix: r.GetResourceNameWithPrefix(g.Config.Prefix),
	}

	if err := g.Template.ExecuteTemplate(r.ResFile, "resourceHeader"+".tmpl", s); err != nil {
		return err
	}
	return nil
}

// WriteResourceContainers
func (g *Generator) WriteResourceContainers(r *resource.Resource, c *container.Container) error {
	s := struct {
		Name    string
		Entries []*container.Entry
	}{
		Name:    c.GetFullName(),
		Entries: c.Entries,
	}

	if err := g.Template.ExecuteTemplate(r.ResFile, "resourceContainer"+".tmpl", s); err != nil {
		return err
	}
	return nil
}

func (g *Generator) WriteResourceEnd(r *resource.Resource) error {

	r.GetHierarchicalElements()

	s := struct {
		Prefix                 string
		ResourceLastElement    string
		ResourceName           string
		ResourceNameWithPrefix string
		HElements              []*resource.HeInfo
	}{
		Prefix:                 g.Config.Prefix,
		ResourceLastElement:    strcase.UpperCamelCase(r.ResourceLastElement()),
		ResourceName:           r.GetResourceNameWithPrefix(""),
		ResourceNameWithPrefix: r.GetResourceNameWithPrefix(g.Config.Prefix),
		HElements:              r.GetHierarchicalElements(),
	}
	if err := g.Template.ExecuteTemplate(r.ResFile, "resourceEnd"+".tmpl", s); err != nil {
		return err
	}
	return nil
}

func (g *Generator) WriteResourceLocalLeafRef(r *resource.Resource) error {
	s := struct {
		Kind         string
		ResourceName string
		LeafRefs     []*parser.LeafRefGnmi
	}{
		Kind:         "Local",
		ResourceName: r.GetResourceNameWithPrefix(""),
		LeafRefs:     r.LocalLeafRefs,
	}
	//g.log.Debug("local leafrefs", "local leafref", r.LocalLeafRefs)
	if err := g.Template.ExecuteTemplate(r.ResFile, "resourceLeafRef"+".tmpl", s); err != nil {
		return err
	}
	return nil
}

func (g *Generator) WriteResourceExternalLeafRef(r *resource.Resource) error {
	s := struct {
		Kind         string
		ResourceName string
		LeafRefs     []*parser.LeafRefGnmi
	}{
		Kind:         "External",
		ResourceName: r.GetResourceNameWithPrefix(""),
		LeafRefs:     r.ExternalLeafRefs,
	}
	//g.log.Debug("External leafrefs", "external leafref", r.LocalLeafRefs)
	if err := g.Template.ExecuteTemplate(r.ResFile, "resourceLeafRef"+".tmpl", s); err != nil {
		return err
	}
	return nil
}
