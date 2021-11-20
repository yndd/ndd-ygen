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
	"github.com/yndd/ndd-yang/pkg/yparser"
)

func (g *Generator) IsResourceBoundary(respath string) bool {
	inputPath := yparser.Xpath2GnmiPath(respath, 0)
	for _, r := range g.GetResources()[1:] {
		fmt.Printf("InputPath: %s, resource Path: %s\n", yparser.GnmiPath2XPath(inputPath, false), yparser.GnmiPath2XPath(r.GetAbsoluteGnmiPath(), false))
		// if the input path is smaller than the resource we know there is no match
		if len(r.GetAbsoluteGnmiPath().GetElem()) == len(inputPath.GetElem()) {
			found := true
			for i, PathElem := range r.GetAbsoluteGnmiPath().GetElem() {
				// if the name of the PathElem don't match this is not a resource that matches
				if PathElem.GetName() != inputPath.GetElem()[i].GetName() {
					found = false
					break
				}
			}
			// if found we can return, Since we found an exact match
			if found {
				return true
			}
		}
	}
	return false
}

func (g *Generator) GetActualResources() []*resource.Resource {
	resources := make([]*resource.Resource, 0)
	if !g.GetConfig().GetResourceMapAll() {
		// when generating the full resource e.g. for state use cases or the runtime yang schema
		// we can just return the first resource entry
		resources = append(resources, g.GetResources()[0])
	} else {
		// when generating the individual resources we skip the first resource since
		// it is used as a full resource generator
		resources = g.GetResources()[:1]
	}
	return resources
}

// FindBestMatchfinds the resource that has the best match, otherwise the resource is not found
// it uses the pathElem names to compare between the resource path and the input path
func (g *Generator) FindBestMatch(inputPath *gnmi.Path) (*resource.Resource, bool) {
	minLength := 0
	resMatch := &resource.Resource{}
	found := false

	// loop over all resources depending on the scenario
	// option 1: for full resources it is the first element
	// option 2: for individual resources it is all except the first resource
	for _, r := range g.GetActualResources() {
		// if the input path is smaller than the resource we know there is no match
		if len(r.GetAbsoluteGnmiPath().GetElem()) <= len(inputPath.GetElem()) {
			found = true
			// given we know the input PathElem are >= the resource Elements we can compare
			// the elements using the index of the resource PathElem
			for i, PathElem := range r.GetAbsoluteGnmiPath().GetElem() {
				// if the name of the PathElem don't match this is not a resource that matches
				if PathElem.GetName() != inputPath.GetElem()[i].GetName() {
					found = false
					break
				}
			}

			// if the PathElem are bigger than the previously found this is a better match
			if found && len(r.GetAbsoluteGnmiPath().GetElem()) > minLength {
				resMatch = r
				minLength = len(r.GetAbsoluteGnmiPath().GetElem())
			}
			/*
				if strings.Contains(*g.parser.GnmiPathToXPath(&inputPath, false), "/nokia-conf/configure/router/ospf") {
					fmt.Printf("FindBestMatchNew: Resource Path: %s, xpath: %s, length: %d, found: %t\n", *g.parser.GnmiPathToXPath(r.GetAbsoluteGnmiPath(), false), *g.parser.GnmiPathToXPath(&inputPath, false), minLength, found)
				}
			*/
		}
	}

	if resMatch.Path != nil {
		return resMatch, true
	}
	return resMatch, false
}

func (g *Generator) ifExcluded(path *gnmi.Path, excludePaths []*gnmi.Path) bool {
	for _, exclPath := range excludePaths {
		//fmt.Printf("Excluded Path : %s\n", *g.parser.GnmiPathToXPath(exclPath, true))
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
func (g *Generator) DoesResourceMatch(path *gnmi.Path) (*resource.Resource, bool) {
	//fmt.Printf("Path: %s\n", yparser.GnmiPath2XPath(path, true))

	if r, ok := g.FindBestMatch(path); ok {
		//fmt.Printf("match path: %s \n", *r.GetAbsoluteXPath())
		// check excludes
		if g.ifExcluded(path, r.Excludes) {
			return r, false
		}
		return r, true

	}
	return nil, false

}

func (g *Generator) ResourceGenerator(resPath string, dynPath *gnmi.Path, e *yang.Entry, choice bool, containerKey string) error {
	// only add the pathElem this yang entry is not a choice entry
	// 1. e.IsChoice() represents that the current entry is a choice -> we can skip the processing
	// 2. choice means the previous yang entry was a choice so we need to skip one more round in processing
	if !e.IsChoice() {
		if !choice {
			resPath += filepath.Join("/", e.Name)
			dynPath.Elem = append(dynPath.Elem, (*gnmi.PathElem)(g.parser.CreatePathElem(e)))
			//fmt.Printf("resource path: %s \n", *g.parser.GnmiPathToXPath(dynPath, false))

			if r, ok := g.DoesResourceMatch(dynPath); ok {
				fmt.Printf("match path: %s \n", *r.GetAbsoluteXPath())
				switch {
				case e.RPC != nil:
				case e.ReadOnly():
					// when the resourcemapAll flag is true we also generate the read-only leafs
					if !g.GetConfig().GetResourceMapAll() {
						break
					}
					fallthrough
				default: // this is a RW config element in yang or both
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
						// fmt.Printf("Leaf Name: %s, ResPath: %s \n", e.Name, resPath)
						// fmt.Printf("Entry: Name: %s, NameSpace: %#v\n", e.Name, e)
						// add entry to the container, containerKey allows to see if a
						cPtr.Entries = append(cPtr.Entries, g.parser.CreateContainerEntry(e, nil, nil, containerKey))
						// leafRef processing
						localPath, remotePath, local := g.parser.ProcessLeafRefGnmi(e, resPath, r.GetAbsoluteGnmiActualResourcePath())
						if localPath != nil {
							// validate if the leafrefs is a local leafref or an external leafref
							if local {
								// local leafref
								r.AddLocalLeafRef(localPath, remotePath)
							} else {
								// external leafref
								r.AddExternalLeafRef(localPath, remotePath)
							}
						}
						localPath, remotePath, _ = yparser.ProcessLeafRef(e, resPath, r.GetAbsoluteGnmiActualResourcePath())
						if localPath != nil {
							// validate if the leafrefs is a local leafref or an external leafref
							fmt.Printf("LocalLeafRef localPath: %s, RemotePath: %s\n", yparser.GnmiPath2XPath(localPath, false), yparser.GnmiPath2XPath(remotePath, false))
							cPtr.AddLeafRef(localPath, remotePath)
							/*
							if local {
								// local leafref
								fmt.Printf("LocalLeafRef localPath: %s, RemotePath: %s\n", yparser.GnmiPath2XPath(localPath, false), yparser.GnmiPath2XPath(remotePath, false))
								cPtr.AddLocalLeafRef(localPath, remotePath)
							} else {
								// external leafref
								fmt.Printf("ExternalLeafRef localPath: %s, RemotePath: %s\n", yparser.GnmiPath2XPath(localPath, false), yparser.GnmiPath2XPath(remotePath, false))
								cPtr.AddExternalLeafRef(localPath, remotePath)
							}
							*/
						}
					} else { // List processing with or without a key

						// fmt.Printf("List Name: %s, ResPath: %s \n", e.Name, resPath)
						// newLevel = 0 is special since we have to initialize the container
						// for newLevl = 0 we do not have to rely on the cPtr, since there is no cPtr initialized yet
						// for newLevl = 0 we dont create an entry in the container but we create a root container entry
						if newLevel == 0 {
							// Allocate a new actual path in the resource
							r.ActualPath = &gnmi.Path{
								Elem: make([]*gnmi.PathElem, 0),
							}
							// append the entry to the actual path of the resource
							r.ActualPath.Elem = append(r.ActualPath.Elem, g.parser.CreatePathElem(e))
							// create a new container and apply to the root of the resource
							r.Container = container.NewContainer(e.Name, g.IsResourceBoundary(resPath), nil)
							// r.Container.Entries = append(r.Container.Entries, parser.CreateContainerEntry(e, nil, nil))
							// append the container Ptr to the back of the list, to track the used container Pointers per level
							// newLevel =0
							r.SetRootContainerEntry(g.parser.CreateContainerEntry(e, nil, nil, containerKey))
							r.ContainerLevelKeys[newLevel] = make([]*container.Container, 0)
							r.ContainerLevelKeys[newLevel] = append(r.ContainerLevelKeys[newLevel], r.Container)
							r.ContainerList = append(r.ContainerList, r.Container)

						} else {
							// append the entry to the actual path of the resource
							r.ActualPath.Elem = append(r.ActualPath.Elem, g.parser.CreatePathElem(e))
							// create a new container for the next iteration
							c := container.NewContainer(e.Name, g.IsResourceBoundary(resPath), cPtr)
							if newLevel == 1 {
								r.RootContainerEntry.Next = c
							}
							// allocate container entry to the original container Pointer and append to the container entry list
							// the next pointer of the entry points to the new container
							cPtr.Entries = append(cPtr.Entries, g.parser.CreateContainerEntry(e, c, cPtr, containerKey))
							// append the container Ptr to the back of the list, to track the used container Pointers per level
							// initialize the level
							r.ContainerLevelKeys[newLevel] = make([]*container.Container, 0)
							r.ContainerLevelKeys[newLevel] = append(r.ContainerLevelKeys[newLevel], c)
							r.ContainerList = append(r.ContainerList, c)

						}
					}
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
		// 1/ the choice is supplied to the next level in order to ignore 1 more path from the tree
		// 2. e.key is supplied to the next iteration as this identifies the key that is used at the containerlevel
		// the key is resolved with the name in the next level resolution and this is how we can identify
		// if a entry (which is the key name) is mandatory or not
		g.ResourceGenerator(resPath, dynPath, e.Dir[k], e.IsChoice(), e.Key)
	}
	return nil
}
