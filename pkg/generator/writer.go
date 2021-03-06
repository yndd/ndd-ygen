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
	"os"
	"path/filepath"

	"github.com/yndd/ndd-yang/pkg/container"
	"github.com/yndd/ndd-yang/pkg/leafref"
	"github.com/yndd/ndd-yang/pkg/yparser"
)

func renderContainers(c *container.Container) {
	if c != nil {

		fmt.Printf("container info: %v\n", c)
		fmt.Printf("getContainerList name: %s\n", c.GetFullNameWithRoot())

		for _, e := range c.GetEntries() {
			fmt.Printf("  entry Name: %s, Type: %s\n", e.Name, e.Type)
		}

		for _, c := range c.GetChildren() {
			renderContainers(c)
		}
	}

}

func (g *Generator) Render() error {
	if g.GetConfig().GetResourceMapAll() {
		// Render the data
		//for _, r := range g.GetActualResources()[:0] {
		r := g.GetActualResources()[0]
		fmt.Printf("Resource: %s\n", r.GetResourcePath())
		fmt.Printf("Render Resource: %s\n", r.GetResourceNameWithPrefix(g.GetConfig().GetPrefix()))
		fmt.Printf("Render Resource path: %s\n", yparser.GnmiPath2XPath(r.GetActualGnmiFullPathWithKeys(), true))
		renderContainers(r.RootContainer)
		//}
	} else {
		// Render the data
		for _, r := range g.GetActualResources()[1:] {
			//fmt.Printf("Resource: %s\n", r.GetResourcePath())
			//fmt.Printf("Render Resource: %s\n", r.GetResourceNameWithPrefix(g.GetConfig().GetPrefix()))
			//fmt.Printf("Render Resource path: %s\n", yparser.GnmiPath2XPath(r.GetActualGnmiFullPathWithKeys(), true))
			renderContainers(r.RootContainer)
		}
	}
	//g.RenderSchemaMethods()

	/*
		for _, c := range r.ContainerList {
			fmt.Printf("Render Container: HasState: %t, name: %s\n", c.HasState, c.GetFullName())
			for _, e := range c.GetEntries() {
				fmt.Printf("  Render Container Entry: state: %t, name: %s\n", e.ReadOnly, e.Name)
			}
		}
	*/
	//r.AssignFileName(g.GetConfig().GetPrefix(), "_types.go")
	/*
		if err := r.CreateFile(g.GetConfig().GetOutputDir(), "api", g.GetConfig().GetVersion()); err != nil {
			return err
		}
		if err := g.WriteResourceHeader(r); err != nil {
			g.log.Debug("Write resource header error", "error", err)
			return err
		}

		/*
		for _, c := range r.ContainerList {
			if err := g.WriteResourceContainers(r, c); err != nil {
				g.log.Debug("Write resource container error", "error", err)
				return err
			}


		}
	*/

	/*
		if err := g.WriteResourceEnd(r); err != nil {
			g.log.Debug("Write resource end error", "error", err)
			return err
		}
	*/

	// EXPERIMENTAL
	/*
		if err := g.WriteResourceLocalLeafRef(r); err != nil {
			g.log.Debug("Write resource local leafRef error", "error", err)
			return err
		}
		if err := g.WriteResourceExternalLeafRef(r); err != nil {
			g.log.Debug("Write resource external leafRef error", "error", err)
			return err
		}
	*/

	/*
		if err := r.CloseFile(); err != nil {
			return err
		}
	*/

	return nil
}

/*
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
		Version:                g.GetConfig().GetVersion(),
		ApiGroup:               g.GetConfig().GetApiGroup(),
		ResourceLastElement:    strcase.LowerCamelCase(r.ResourceLastElement()),
		ResourceNameWithPrefix: r.GetResourceNameWithPrefix(g.GetConfig().GetPrefix()),
	}

	if err := g.getTemplate().ExecuteTemplate(r.ResFile, "resourceHeader"+".tmpl", s); err != nil {
		return err
	}
	return nil
}
*/

/*
// WriteResourceContainers
func (g *Generator) WriteResourceContainers(r *resource.Resource, c *container.Container) error {
	s := struct {
		Name    string
		Entries []*container.Entry
	}{
		Name:    c.GetFullName(),
		Entries: c.Entries,
	}

	if err := g.getTemplate().ExecuteTemplate(r.ResFile, "resourceContainer"+".tmpl", s); err != nil {
		return err
	}
	return nil
}
*/
/*
func (g *Generator) WriteResourceEnd(r *resource.Resource) error {

	r.GetHierarchicalElements()

	s := struct {
		Prefix                 string
		ResourceLastElement    string
		ResourceName           string
		ResourceNameWithPrefix string
		HElements              []*resource.HeInfo
	}{
		Prefix:                 g.GetConfig().GetPrefix(),
		ResourceLastElement:    strcase.UpperCamelCase(r.ResourceLastElement()),
		ResourceName:           r.GetResourceNameWithPrefix(""),
		ResourceNameWithPrefix: r.GetResourceNameWithPrefix(g.GetConfig().GetPrefix()),
		HElements:              r.GetHierarchicalElements(),
	}
	if err := g.getTemplate().ExecuteTemplate(r.ResFile, "resourceEnd"+".tmpl", s); err != nil {
		return err
	}
	return nil
}
*/

/*
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
	if err := g.getTemplate().ExecuteTemplate(r.ResFile, "resourceLeafRef"+".tmpl", s); err != nil {
		return err
	}
	return nil
}
*/
/*
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
	if err := g.getTemplate().ExecuteTemplate(r.ResFile, "resourceLeafRef"+".tmpl", s); err != nil {
		return err
	}
	return nil
}
*/

func (g *Generator) RenderSchema() error {
	if err := g.renderSchema(g.GetResources()[0].RootContainer); err != nil {
		return err
	}

	return nil
}

func (g *Generator) renderSchema(c *container.Container) error {
	//fmt.Printf("Container FullName %s\n", c.GetFullNameWithRoot())

	f, err := os.Create(filepath.Join(g.GetConfig().GetOutputDir(), "yangschema", c.GetFullNameWithRoot()+".go"))
	if err != nil {
		return err
	}

	if err := g.WriteContainer(f, c); err != nil {
		g.log.Debug("Write container error", "error", err)
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	for _, c := range c.Children {
		if err := g.renderSchema(c); err != nil {
			return err
		}

	}
	return nil
}

func (g *Generator) RenderSchemaMethods() error {
	fmt.Println("%%%%%%%%%%%%%%%%%%%%%%")
	for _, r := range g.GetResources() {
		fmt.Printf("ResourceName %s\n", r.GetAbsoluteName())
		if r.GetParent() != nil {
			fmt.Printf("  Parent %s\n", r.GetParent().GetAbsoluteName())
		}
		for _, child := range r.GetChildren() {
			fmt.Printf("  Child %s\n", child.GetAbsoluteName())
		}
	}
	fmt.Println("%%%%%%%%%%%%%%%%%%%%%%")
	return nil
}

func (g *Generator) WriteContainer(f *os.File, c *container.Container) error {
	s := struct {
		Name             string
		Module           string
		Namespace        string
		Prefix           string
		FullName         string
		Keys             []string
		Children         []string
		ResourceBoundary bool
		LeafRefs         []*leafref.LeafRef
		Defaults         map[string]string
	}{
		Name:             c.GetName(),
		Module:           c.GetModuleName(),
		Namespace:        c.GetNamespace(),
		Prefix:           c.GetPrefixName(),
		FullName:         c.GetFullNameWithRoot(),
		Keys:             c.GetKeyNames(),
		Children:         c.GetChildrenNames(),
		ResourceBoundary: c.GetResourceBoundary(),
		LeafRefs:         c.GetLeafRefs(),
		Defaults:         c.GetDefaults(),
	}
	//g.log.Debug("External leafrefs", "external leafref", r.LocalLeafRefs)
	if err := g.getTemplate().ExecuteTemplate(f, "container.tmpl", s); err != nil {
		return err
	}
	return nil
}
