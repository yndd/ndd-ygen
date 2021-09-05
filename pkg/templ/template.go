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

// makek8sTemplate generates a template.Template for a particular named source
// template; with a common set of helper functions.
//func makek8sTemplate(name, src string) *template.Template {
//	return template.Must(template.New(name).Funcs(templateHelperFunctions).Funcs(sprig.TxtFuncMap()).Parse(src))
//}

package templ

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/stoewer/go-strcase"
)

func ParseTemplates(path string) (*template.Template, error) {
	templ := template.New("ndd").Funcs(templateHelperFunctions).Funcs(sprig.TxtFuncMap())
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".tmpl") {
			_, err = templ.ParseFiles(path)
			if err != nil {
				return err
			}
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	return templ, nil
}

// templateHelperFunctions specifies a set of functions that are supplied as
// helpers to the templates that are used within this file.
var templateHelperFunctions = template.FuncMap{
	// inc provides a means to add 1 to a number, and is used within templates
	// to check whether the index of an element within a loop is the last one,
	// such that special handling can be provided for it (e.g., not following
	// it with a comma in a list of arguments).
	"inc":  func(i int) int { return i + 1 },
	"dec":  func(i int) int { return i - 1 },
	"mul":  func(p1 int, p2 int) int { return p1 * p2 },
	"mul3": func(p1, p2, p3 int) int { return p1 * p2 * p3 },
	"boolValue": func(b bool) int {
		if b {
			return 1
		} else {
			return 0
		}
	},
	"toUpperCamelCase": strcase.UpperCamelCase,
	"toLowerCamelCase": strcase.LowerCamelCase,
	"toKebabCase":      strcase.KebabCase,
	"toLower":          strings.ToLower,
	"toUpper":          strings.ToUpper,
	"mod":              func(i, j int) bool { return i%j == 0 },
	"deref":            func(s *string) string { return *s },
	"derefInt":         func(i *int) int { return *i },
	"list2string": func(s []*string) string {
		var str string
		for i, v := range s {
			if i < len(s)-1 {
				str = str + fmt.Sprintf("%s, ", *v)
			} else {
				str = str + *v
			}
		}
		return str
	},
	"rtCommExpr": func(vrfUpId, lmgs int, wlShortname string) string {
		// if we come here there should be at least 1 element
		rtCommExpr := fmt.Sprintf("rt-lmg%d-%d-%s", 1, vrfUpId+1, wlShortname)
		for i := 2; i <= lmgs; i++ {
			rtCommExpr += fmt.Sprintf(" OR rt-lmg%d-%d-%s", i, vrfUpId+i, wlShortname)
		}
		return rtCommExpr
	},
	"lastmap": func(s string, x map[string][]*string) bool {
		i := 0
		for k := range x {
			if k == s {
				if i == len(x)-1 {
					return true
				}
			}
			i++
		}
		return false
	},
}
