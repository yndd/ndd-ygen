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
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/yndd/ndd-yang/pkg/container"
	"github.com/yndd/ndd-yang/pkg/resource"
)

// FindBestMatch finds the string which matches the most
func (g *Generator) FindBestMatch(path gnmi.Path) (*resource.Resource, bool) {
	minLength := 0
	resMatch := &resource.Resource{}
	found := false
	for _, r := range g.Resources {
		if strings.Contains(*g.parser.GnmiPathToXPath(&path, false), *r.GetAbsoluteXPath()) {
			// find the string which matches the most
			// should be the last match normally since we added them
			// to the list from root to lower hierarchy
			if len([]rune(*r.GetAbsoluteXPath())) > minLength {
				minLength = len([]rune(*r.GetAbsoluteXPath()))
				resMatch = r
				found = true
			}
		}
	}
	return resMatch, found
}

// FindBestMatchNew finds the resource that has the best match, otherwise the resource is not found
// it uses the pathElem names to compare between the resource path and the input path
func (g *Generator) FindBestMatchNew(inputPath gnmi.Path) (*resource.Resource, bool) {
	minLength := 0
	resMatch := &resource.Resource{}
	found := false
	// loop over all resources
	for _, r := range g.Resources {
		// if the input path is smaller than the resource we know there is no match
		if len(r.GetResourcePath().GetElem()) <= len(inputPath.GetElem()) {
			found = true
			// given we know the input PathElem are >= the resource Elements we can compare
			// the elements using the index of the resource PathElem
			for i, PathElem := range r.GetResourcePath().GetElem() {
				// if the name of the PathElem does not match this is not a resource that matches
				if PathElem.GetName() != inputPath.GetElem()[i].GetName() {
					found = false
					break
				}
			}
			// if the PathElem are bigger than the previously found this is a better match
			if found && len(r.GetResourcePath().GetElem()) > minLength {
				resMatch = r
				minLength = len(r.GetResourcePath().GetElem())
			}
		}
	}

	if resMatch.Path != nil {
		return resMatch, true
	}
	return resMatch, false
}

func (g *Generator) ifExcluded(path gnmi.Path, excludePaths []*gnmi.Path) bool {
	for _, exclPath := range excludePaths {
		fmt.Printf("Excluded Path : %s\n", *g.parser.GnmiPathToXPath(exclPath, true))
		// if the length of the path is less than the exclude path there is no exclusion
		if len(path.GetElem()) >= len(exclPath.GetElem()) {
			found := false
			for i, exlPathElem := range exclPath.GetElem() {
				if exlPathElem.GetName() != path.GetElem()[i].GetName() {
					found = false
					break
				}
				found = true
			}
			// when all the PathElem matches this path of the tree is excluded
			if found {
				return true
			}
		}
	}
	return false
}

// IsResourcesInit checks if the resource is part of the resource table and if no excludes exist
func (g *Generator) DoesResourceMatch(path gnmi.Path) (*resource.Resource, bool) {
	//fmt.Printf("Path: %s\n", *parser.GnmiPathToXPath(path))
	if r, ok := g.FindBestMatchNew(path); ok {
		//fmt.Printf("match path: %s \n", *r.GetAbsoluteXPath())
		// check excludes
		if g.ifExcluded(path, r.Excludes) {
			return r, false
		}
		return r, true

	}
	return nil, false
}

func (g *Generator) ResourceGenerator(resPath string, dynPath gnmi.Path, e *yang.Entry) error {
	resPath += filepath.Join("/", e.Name)
	dynPath.Elem = append(dynPath.Elem, (*gnmi.PathElem)(g.parser.CreatePathElem(e)))
	//fmt.Printf("resource path2: %s \n", *parser.GnmiPathToXPath(&path, false))

	if r, ok := g.DoesResourceMatch(dynPath); ok {
		fmt.Printf("match path: %s \n", *r.GetAbsoluteXPath())
		switch {
		case e.RPC != nil:
		case e.ReadOnly():
		default: // this is a RW config element in yang
			// find the containerPointer
			// we look at the level delta from the root of the resource -> newLevel
			// newLevel = 0 is special since it is the root of the container
			// newLevel = 0 since there is no container yet we cannot find the container Pointer, since it is not created so far
			newLevel := strings.Count(resPath, "/") - strings.Count(*r.GetAbsoluteXPathWithoutKey(), "/")
			var cPtr *container.Container
			if newLevel > 0 {
				r.ContainerLevel = newLevel

				cPtr = r.ContainerLevelKeys[newLevel-1][len(r.ContainerLevelKeys[newLevel-1])-1]
			}
			fmt.Printf("xpath: %s, resPath: %s, level: %d\n", *r.GetAbsoluteXPathWithoutKey(), resPath, r.ContainerLevel)

			// Leaf processing
			if e.Kind.String() == "Leaf" {
				fmt.Printf("Leaf Name: %s, ResPath: %s \n", e.Name, resPath)
				fmt.Printf("Entry: Name: %s, NameSpace: %#v\n", e.Name, e)
				// add entry to the container
				cPtr.Entries = append(cPtr.Entries, g.parser.CreateContainerEntry(e, nil, nil))
				localPath, remotePath, local := g.parser.ProcessLeafRefGnmi(e, resPath, r.GetAbsoluteGnmiActualResourcePath())
				if localPath != nil {
					// validate if the leafrefs is a local leafref or an externaal leafref
					if local {
						// local leafref
						r.AddLocalLeafRef(localPath, remotePath)
					} else {
						// external leafref
						r.AddExternalLeafRef(localPath, remotePath)
					}
				}
			} else { // List processing with or without a key
				fmt.Printf("List Name: %s, ResPath: %s \n", e.Name, resPath)
				// newLevel = 0 is special since we have to initialize the container
				// for newLevl = 0 we do not have to rely on the cPtr, since there is no cPtr initialized yet
				// for newLevl = 0 we dont create an entry in the container but we create a root container entry
				if newLevel == 0 {
					// Allocate a new actual path in the resource
					r.ActualPath = &gnmi.Path{
						Elem: make([]*gnmi.PathElem, 0),
					}
					// append the entry to the actual path of the reosurce
					r.ActualPath.Elem = append(r.ActualPath.Elem, g.parser.CreatePathElem(e))
					// create a new container and apply to the root of the resource
					r.Container = container.NewContainer(e.Name, nil)
					// r.Container.Entries = append(r.Container.Entries, parser.CreateContainerEntry(e, nil, nil))
					// append the container Ptr to the back of the list, to track the used container Pointers per level
					// newLevel =0
					r.SetRootContainerEntry(g.parser.CreateContainerEntry(e, nil, nil))
					r.ContainerLevelKeys[newLevel] = make([]*container.Container, 0)
					r.ContainerLevelKeys[newLevel] = append(r.ContainerLevelKeys[newLevel], r.Container)
					r.ContainerList = append(r.ContainerList, r.Container)

				} else {
					// append the entry to the actual path of the reosurce
					r.ActualPath.Elem = append(r.ActualPath.Elem, g.parser.CreatePathElem(e))
					// create a new container for the next iteration
					c := container.NewContainer(e.Name, cPtr)
					if newLevel == 1 {
						r.RootContainerEntry.Next = c
					}
					// allocate container entry to the original container Pointer and append to the container entry list
					// the next pointer of the entry points to the new container
					cPtr.Entries = append(cPtr.Entries, g.parser.CreateContainerEntry(e, c, cPtr))
					// append the container Ptr to the back of the list, to track the used container Pointers per level
					// initialize the level
					r.ContainerLevelKeys[newLevel] = make([]*container.Container, 0)
					r.ContainerLevelKeys[newLevel] = append(r.ContainerLevelKeys[newLevel], c)
					r.ContainerList = append(r.ContainerList, c)
				}
			}
		}
	}
	// handles the recursive analysis of the yang tree
	var names []string
	for k := range e.Dir {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		g.ResourceGenerator(resPath, dynPath, e.Dir[k])
	}
	return nil
}
