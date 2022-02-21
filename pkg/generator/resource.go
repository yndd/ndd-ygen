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
	"path/filepath"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/yndd/ndd-yang/pkg/container"
	"github.com/yndd/ndd-yang/pkg/resource"
	"github.com/yndd/ndd-yang/pkg/yparser"
)

func (g *Generator) GetModuleName(namespace string) string {
	for moduleName, m := range g.getModules() {
		if m.Namespace.Name == namespace {
			return moduleName
		}
	}
	return ""
}

func (g *Generator) IsResourceBoundary(respath string) bool {
	inputPath := yparser.Xpath2GnmiPath(respath, 0)
	for _, r := range g.GetResources()[1:] {
		//fmt.Printf("resource Path: %s\n", *r.GetAbsoluteXPath())
		// if the input path is smaller than the resource we know there is no match
		if len(r.GetAbsolutePath().GetElem()) == len(inputPath.GetElem()) {
			found := true
			for i, PathElem := range r.GetAbsolutePath().GetElem() {
				// if the name of the PathElem don't match this is not a resource that matches
				if PathElem.GetName() != inputPath.GetElem()[i].GetName() {
					found = false
					break
				}
			}
			// if found we can return, Since we found an exact match
			if found {
				//fmt.Printf("resource boundary Path: %s\n", respath)
				return true
			}
		}
	}
	return false
}

func (g *Generator) GetActualResources() []*resource.Resource {
	return g.GetResources()
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
	for _, r := range g.GetActualResources()[1:] {
		// if the input path is smaller than the resource we know there is no match
		//fmt.Printf("len r: %d, len ip: %d\n", len(r.GetAbsolutePath().GetElem()), len(inputPath.GetElem()))
		if len(r.GetAbsolutePath().GetElem()) <= len(inputPath.GetElem()) {
			found = true
			//fmt.Printf("FindBestMatch: resPath: %s, inputPath: %s\n", yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), yparser.GnmiPath2XPath(inputPath, false))
			// given we know the input PathElem are >= the resource Elements we can compare
			// the elements using the index of the resource PathElem
			for i, PathElem := range r.GetAbsolutePath().GetElem() {
				// if the name of the PathElem don't match this is not a resource that matches
				if PathElem.GetName() != inputPath.GetElem()[i].GetName() {
					found = false
					break
				}
			}

			// if the PathElem are bigger than the previously found this is a better match
			if found && len(r.GetAbsolutePath().GetElem()) > minLength {
				resMatch = r
				minLength = len(r.GetAbsolutePath().GetElem())
			}

			/*
				if strings.Contains(yparser.GnmiPath2XPath(inputPath, false), "/srl_nokia-network-instance/network-instance/aggregate-routes") {
					if found {
						fmt.Printf("FindBestMatchNew: inputPath: %s, resPath: %s, length: %d, found: %t\n", yparser.GnmiPath2XPath(inputPath, false), yparser.GnmiPath2XPath(resMatch.GetAbsolutePath(), false), minLength, found)
						fmt.Printf("resMatch Path: %s\n", yparser.GnmiPath2XPath(resMatch.Path, false))
					}

				}
			*/

		}
	}

	if resMatch.Path != nil {
		/*
			if strings.Contains(yparser.GnmiPath2XPath(inputPath, false), "/srl_nokia-network-instance/network-instance/aggregate-routes") {
				fmt.Printf("FindBestMatchNew: inputPath: %s, resPath: %s, length: %d, found: %t\n", yparser.GnmiPath2XPath(inputPath, false), yparser.GnmiPath2XPath(resMatch.GetAbsolutePath(), false), minLength, found)
			}
		*/
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

	if g.GetConfig().GetResourceMapAll() {
		inputPath := yparser.GnmiPath2XPath(path, false)
		if strings.HasPrefix(inputPath, g.schema) && len(strings.Split(inputPath, "/")) > 2 {
			//fmt.Printf("path: %s\n", yparser.GnmiPath2XPath(path, false))
			return g.rootResource, true
		}

		return nil, false

	} else {
		// this is the regular case
		if r, ok := g.FindBestMatch(path); ok {

			//fmt.Printf("match path: %s \n", yparser.GnmiPath2XPath(r.GetAbsolutePath(), false))
			// check excludes

			if g.ifExcluded(path, r.Excludes) {
				return r, false
			}
			/*
				if strings.Contains(yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), "/network-instance/aggregate-routes") {
					fmt.Printf("FindBestMatchNew: path: %s, resPath: %s\n", yparser.GnmiPath2XPath(path, false), yparser.GnmiPath2XPath(r.GetAbsolutePath(), false))
				}
			*/

			return r, true
		}
		return nil, false
	}

}

func (g *Generator) ResourceGenerator(resPath string, dynPath *gnmi.Path, e *yang.Entry, choice bool, containerKey, namespace string) error {
	// only add the pathElem this yang entry is not a choice entry
	// 1. e.IsChoice() represents that the current entry is a choice -> we can skip the processing
	// 2. choice means the previous yang entry was a choice so we need to skip one more round in processing
	newdynPath := yparser.DeepCopyGnmiPath(dynPath)
	newNamespace := ""
	newModuleName := ""
	if namespace != e.Namespace().Name {
		newNamespace = e.Namespace().Name
		newModuleName = g.GetModuleName(newNamespace)
	}

	if !e.IsChoice() {
		if !choice {
			resPath += filepath.Join("/", e.Name)
			newdynPath.Elem = append(newdynPath.Elem, (*gnmi.PathElem)(yparser.CreatePathElem(e)))
			//fmt.Printf("resource path: %s \n", yparser.GnmiPath2XPath(dynPath, false))

			/*
				if newNamespace != "" {
					fmt.Printf("path: %s, namespace: %s\n", yparser.GnmiPath2XPath(newdynPath, false), e.Namespace().Name)
				}
			*/

			if r, ok := g.DoesResourceMatch(newdynPath); ok {
				//fmt.Printf("match path: %s, dyn path: %s \n", yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), yparser.GnmiPath2XPath(dynPath, false))
				/*
					if strings.Contains(yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), "/network-instance/aggregate-routes") {
						fmt.Printf("match path: %s, dyn path: %s \n", yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), yparser.GnmiPath2XPath(dynPath, false))
						fmt.Printf("ReadOnly: %t\n", e.ReadOnly())
						fmt.Printf("RPC: %v\n", e.RPC)
					}
				*/

				switch {
				case e.RPC != nil:
				case e.ReadOnly():
					// when we dont need status we break
					if !g.healthStatus {
						break
					}
					fallthrough
				default: // this is a RW config element in yang or both
					// find the containerPointer
					// we look at the level delta from the root of the resource -> newLevel
					// newLevel = 0 is special since it is the root of the container
					// newLevel = 0 since there is no container yet we cannot find the container Pointer, since it is not created so far
					//newLevel := strings.Count(resPath, "/") - strings.Count(yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), "/")
					var newLevel int
					if g.GetConfig().GetResourceMapAll() {
						newLevel = strings.Count(resPath, "/") - 2
					} else {
						newLevel = strings.Count(resPath, "/") - strings.Count(yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), "/")
					}

					/*
						if strings.Contains(yparser.GnmiPath2XPath(dynPath, false), "/srl_nokia-interfaces/interface") {
							fmt.Printf("newLevel: %d, resPath: %s dynPath: %s\n", newLevel, yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), yparser.GnmiPath2XPath(dynPath, false))
							fmt.Printf("newLevel: %d, entryName: %s prefix: %v, namespace: %v\n", newLevel, e.Name, e.Prefix.Name, e.Namespace().Name)
						}
					*/

					//fmt.Printf("newLevel: %d, entryName: %s\n", newLevel, e.Name)
					var cPtr *container.Container
					if newLevel > 0 {
						r.ContainerLevel = newLevel

						cPtr = r.ContainerLevelKeys[newLevel-1][len(r.ContainerLevelKeys[newLevel-1])-1]

						/*
							if strings.Contains(yparser.GnmiPath2XPath(r.GetAbsolutePath(), false), "/network-instance/aggregate-routes") {
								fmt.Printf("cPtr Name %s \n", cPtr.Name)
							}
						*/
					}
					//fmt.Printf("xpath: %s, resPath: %s, level: %d\n", *r.GetAbsoluteXPathWithoutKey(), resPath, r.ContainerLevel)

					if e.Kind.String() != "Leaf" {

						//fmt.Printf("State Info container/list: state info: %t entry name: %s \n", e.ReadOnly(), e.Name)
						// List processing with or without a key
						// fmt.Printf("List Name: %s, ResPath: %s \n", e.Name, resPath)
						// newLevel = 0 is special since we have to initialize the container
						// for newLevl = 0 we do not have to rely on the cPtr, since there is no cPtr initialized yet
						// for newLevl = 0 we dont create an entry in the container but we create a root container entry
						if newLevel == 0 {
							// create a new container and apply to the root of the resource
							newModuleName := g.GetModuleName(e.Namespace().Name)
							c := container.NewContainer(e, e.Namespace().Name, newModuleName, e.ReadOnly(), g.IsResourceBoundary(resPath), r.RootContainer)
							if g.GetConfig().GetResourceMapAll() {
								r.RootContainer.AddContainerChild(c)
							} else {
								r.RootContainer = c
							}

							// append the container Ptr to the back of the list, to track the used container Pointers per level
							// newLevel =0
							r.SetRootContainerEntry(yparser.CreateContainerEntry(e, nil, nil, containerKey))
							// added for full schema
							if g.GetConfig().GetResourceMapAll() {
								r.RootContainer.Entries = append(r.RootContainer.Entries, yparser.CreateContainerEntry(e, c, c, containerKey))
							}
							r.ContainerLevelKeys[newLevel] = make([]*container.Container, 0)
							r.ContainerLevelKeys[newLevel] = append(r.ContainerLevelKeys[newLevel], c)
							r.ContainerList = append(r.ContainerList, r.RootContainer)

						} else {
							// create a new container for the next iteration
							c := container.NewContainer(e, newNamespace, newModuleName, e.ReadOnly(), g.IsResourceBoundary(resPath), cPtr)
							cPtr.AddContainerChild(c)
							if newLevel == 1 {
								r.RootContainerEntry.Next = c
							}
							// allocate container entry to the original container Pointer and append to the container entry list
							// the next pointer of the entry points to the new container
							cPtr.Entries = append(cPtr.Entries, yparser.CreateContainerEntry(e, c, cPtr, containerKey))
							// append the container Ptr to the back of the list, to track the used container Pointers per level
							// initialize the level
							r.ContainerLevelKeys[newLevel] = make([]*container.Container, 0)
							r.ContainerLevelKeys[newLevel] = append(r.ContainerLevelKeys[newLevel], c)
							r.ContainerList = append(r.ContainerList, c)
						}
					} else { // // Leaf processing
						//fmt.Printf("State Info leaf: state info: %t entry name: %s \n", e.ReadOnly(), e.Name)
						//fmt.Printf("Leaf Name: %s, ResPath: %s \n", e.Name, resPath)
						//fmt.Printf("Entry: Name: %s, Dir: %#v, Type: %v, Units: %s, List: %v\n", e.Name, e.Dir, g.parser.GetTypeName(e), e.Units, e.ListAttr)
						/*
							if e.Type.Enum != nil {
								fmt.Printf("Entry: Name: %s Enum: %v\n", e.Name, e.Type.Enum.Names())
							}
						*/
						// leaflist we create an additional container
						if e.ListAttr != nil {
							dummyYangEntry := &yang.Entry{
								Name:     e.Name,
								ListAttr: e.ListAttr,
								Prefix:   e.Prefix,
							}
							c := container.NewContainer(dummyYangEntry, newNamespace, newModuleName, e.ReadOnly(), g.IsResourceBoundary(resPath), cPtr)
							cPtr.AddContainerChild(c)
							r.ContainerList = append(r.ContainerList, c)
							centry := yparser.CreateContainerEntry(dummyYangEntry, c, cPtr, containerKey)
							cPtr.Entries = append(cPtr.Entries, centry)
							if centry.GetDefault() != "" {
								//fmt.Printf("container: %s, entry name: %s, default: %s\n", cPtr.GetFullName(), centry.GetName(), centry.GetDefault())
								cPtr.SetDefault(dummyYangEntry.Name, centry.GetDefault())
							}

							e.ListAttr = nil
							centry = yparser.CreateContainerEntry(e, nil, nil, containerKey)
							c.Entries = append(c.Entries, centry)
							if centry.GetDefault() != "" {
								//fmt.Printf("container: %s, entry name: %s, default: %s\n", c.GetFullName(), centry.GetName(), centry.GetDefault())
								cPtr.SetDefault(e.Name, centry.GetDefault())
							}

						} else {
							// add entry to the container, containerKey allows to see if a
							centry := yparser.CreateContainerEntry(e, nil, nil, containerKey)
							cPtr.Entries = append(cPtr.Entries, centry)
							if centry.GetDefault() != "" {
								//fmt.Printf("container: %s, entry name: %s, default: %s\n", cPtr.GetFullName(), centry.GetName(), centry.GetDefault())
								cPtr.SetDefault(e.Name, centry.GetDefault())
							}
							// leafRef processing
							localPath, remotePath, local := yparser.ProcessLeafRef(e, resPath, r.GetAbsoluteGnmiPathFromSource())
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
							localPath, remotePath, _ = yparser.ProcessLeafRef(e, resPath, r.GetAbsoluteGnmiPathFromSource())
							if localPath != nil {
								// validate if the leafrefs is a local leafref or an external leafref
								//fmt.Printf("LocalLeafRef localPath: %s, RemotePath: %s\n", yparser.GnmiPath2XPath(localPath, false), yparser.GnmiPath2XPath(remotePath, false))
								cPtr.AddLeafRef(localPath, remotePath)
							}
							// add static leafref paths if they match
							if remotePathString, ok := g.staticLeafRef[resPath]; ok {
								localPath := &gnmi.Path{Elem: []*gnmi.PathElem{{Name: e.Name}}}
								remotePath := yparser.Xpath2GnmiPath(remotePathString, 0)
								//fmt.Printf("localPath: %v \n   RemoteLeafRef: %v \n   RemoteLeafString %v\n", localPath, remotePath, remotePathString)
								//os.Exit(1)

								cPtr.AddLeafRef(localPath, remotePath)
							}
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
		var err error
		if err = g.ResourceGenerator(resPath, newdynPath, e.Dir[k], e.IsChoice(), e.Key, e.Namespace().Name); err != nil {
			return nil
		}
		//fmt.Printf("recursive: path: %s, entryName: %s\n", newdynPath, e.Dir[k].Name)
	}
	return nil
}
