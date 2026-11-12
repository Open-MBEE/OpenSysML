// Package constructcover derives the syntactic features a parsed model exhibits
// and picks the subset of a corpus that exhibits every one of them.
package constructcover

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Parse parses one source file and returns its features: parsing never fails,
// so a file with diagnostics contributes the "diagnostics" feature instead.
func Parse(name string, content []byte) []string {
	sf := source.New(name, content)
	p := parser.New(sf)
	root := p.ParseFile()
	return Features(sf.Kind(), root, len(p.Diagnostics) > 0)
}

// Features is the sorted set of syntactic features of a parse tree: source kind,
// diagnostics, node types, parent-field→child-type edges, enum values, set flags.
func Features(kind source.Kind, root ast.Node, diagnostics bool) []string {
	set := map[string]bool{"kind " + kind.String(): true}
	if diagnostics {
		set["diagnostics"] = true
	} else {
		set["clean"] = true
	}
	w := &walker{set: set, seen: map[visitKey]bool{}}
	w.node(root)
	features := make([]string, 0, len(set))
	for f := range set {
		features = append(features, f)
	}
	sort.Strings(features)
	return features
}

type visitKey struct {
	typ reflect.Type
	ptr uintptr
}

type walker struct {
	set  map[string]bool
	seen map[visitKey]bool
}

var (
	nodeType   = reflect.TypeOf((*ast.Node)(nil)).Elem()
	astPkgPath = reflect.TypeOf(ast.NodeBase{}).PkgPath()
)

// node records n's type and fields, then the nodes reachable through them.
func (w *walker) node(n ast.Node) {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		key := visitKey{v.Type(), v.Pointer()}
		if w.seen[key] {
			return
		}
		w.seen[key] = true
		v = v.Elem()
	}
	name := typeName(v.Type())
	w.set["node "+name] = true
	if v.Kind() == reflect.Struct {
		w.fields(name, v)
	}
}

// fields records the scalar features of a struct's exported fields and the
// edges to the nodes they hold, naming nested non-node structs by field path.
func (w *walker) fields(parent string, v reflect.Value) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Type == reflect.TypeOf(ast.NodeBase{}) {
			continue
		}
		fv := v.Field(i)
		path := parent + "." + f.Name
		switch {
		case fv.Kind() == reflect.Bool:
			if fv.Bool() {
				w.set["flag "+path] = true
			}
		case isEnum(f.Type):
			w.set[fmt.Sprintf("enum %s=%v", path, fv.Interface())] = true
		case fv.Kind() == reflect.String && isKeywordField(f.Name):
			w.set[fmt.Sprintf("enum %s=%s", path, fv.String())] = true
		default:
			w.children(path, fv)
		}
	}
}

// children records an edge from the field at path to every node directly
// reachable through v (via containers and non-node structs) and walks each.
func (w *walker) children(path string, v reflect.Value) {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return
		}
		if n, ok := v.Interface().(ast.Node); ok {
			w.set["edge "+path+">"+typeName(reflect.Indirect(reflect.ValueOf(n)).Type())] = true
			w.node(n)
			return
		}
		w.children(path, v.Elem())
	case reflect.Struct:
		if v.Type().PkgPath() == astPkgPath && !reflect.PointerTo(v.Type()).Implements(nodeType) {
			w.fields(path, v)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			w.children(path, v.Index(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			w.children(path, v.MapIndex(k))
		}
	}
}

// isEnum reports whether t is a Kind-style enumeration: a named integer type
// declared by the ast package.
func isEnum(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return t.PkgPath() == astPkgPath
	}
	return false
}

// isKeywordField reports whether a string field holds a notation keyword
// rather than model-specific text such as a name.
func isKeywordField(name string) bool {
	for _, suffix := range []string{"Keyword", "Kind", "Operator", "Language"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func typeName(t reflect.Type) string {
	if t.PkgPath() == astPkgPath {
		return "ast." + t.Name()
	}
	return t.String()
}

// Cover is a deterministic greedy set cover: each step takes the file adding the
// most uncovered features, a committed file then the lesser path winning a tie.
func Cover(features map[string][]string, committed func(path string) bool) []string {
	paths := make([]string, 0, len(features))
	for p := range features {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	covered := map[string]bool{}
	var chosen []string
	for {
		best, bestGain := "", 0
		for _, p := range paths {
			gain := 0
			for _, f := range features[p] {
				if !covered[f] {
					gain++
				}
			}
			if gain > bestGain || (gain == bestGain && gain > 0 && committed(p) && !committed(best)) {
				best, bestGain = p, gain
			}
		}
		if bestGain == 0 {
			break
		}
		chosen = append(chosen, best)
		for _, f := range features[best] {
			covered[f] = true
		}
	}
	sort.Strings(chosen)
	return chosen
}

// Uncovered lists the features of the corpus that no file of subset exhibits,
// each with one file that has it, sorted by feature.
func Uncovered(features map[string][]string, subset []string) []string {
	covered := map[string]bool{}
	for _, p := range subset {
		for _, f := range features[p] {
			covered[f] = true
		}
	}
	example := map[string]string{}
	for p, fs := range features {
		for _, f := range fs {
			if covered[f] {
				continue
			}
			if prev, ok := example[f]; !ok || p < prev {
				example[f] = p
			}
		}
	}
	out := make([]string, 0, len(example))
	for f, p := range example {
		out = append(out, f+"\t"+p)
	}
	sort.Strings(out)
	return out
}
