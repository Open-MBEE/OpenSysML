// Package enginecontract reads the vendored engine-contract.json: the
// qualified names in the extension libraries that the engine binds to.
// Tests in internal/ and tests/hygiene share it so the manifest is parsed the
// same way everywhere it gates.
package enginecontract

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// Dir is the vendored directory inside the bundled library that the upstream
// files map to.
const Dir = "OpenSysML Libraries"

// File is the manifest inside Dir.
const File = Dir + "/engine-contract.json"

// Kinds are the manifest's entry kinds.
const (
	KindPackage  = "package"
	KindFunction = "function"
	KindMetadata = "metadata"
	KindDocument = "document"
	KindElement  = "element"
)

// Source reads the bundled library the way libs.Source does.
type Source interface {
	List() []string
	Read(name string) ([]byte, error)
}

// Entry is one manifest record.
type Entry struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Parameters []string `json:"parameters"`
	Attributes []string `json:"attributes"`
}

// Manifest is the parsed engine-contract.json.
type Manifest struct {
	Contract int     `json:"contract"`
	Entries  []Entry `json:"entries"`
}

// ByName indexes the manifest's entries by their qualified name.
func (m Manifest) ByName() map[string]Entry {
	out := make(map[string]Entry, len(m.Entries))
	for _, e := range m.Entries {
		out[e.Name] = e
	}
	return out
}

// Load reads and validates the manifest from src.
func Load(src Source) (Manifest, error) {
	var m Manifest
	data, err := src.Read(File)
	if err != nil {
		return m, fmt.Errorf("reading %s: %w", File, err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parsing %s: %w", File, err)
	}
	names := make([]string, 0, len(m.Entries))
	for _, e := range m.Entries {
		names = append(names, e.Name)
		switch e.Kind {
		case KindFunction:
			if e.Parameters == nil {
				return m, fmt.Errorf("%s: function entries require parameters", e.Name)
			}
		case KindMetadata:
			if e.Attributes == nil {
				return m, fmt.Errorf("%s: metadata entries require attributes", e.Name)
			}
		case KindPackage, KindDocument, KindElement:
		default:
			return m, fmt.Errorf("%s: unknown kind %q", e.Name, e.Kind)
		}
	}
	if !sort.StringsAreSorted(names) || len(names) != len(m.Entries) {
		return m, fmt.Errorf("%s: entries are not sorted or contain duplicates", File)
	}
	return m, nil
}

// Packages lists the extension package names the vendored directory carries:
// the stems of its .sysml and .kerml files.
func Packages(src Source) ([]string, error) {
	var out []string
	for _, name := range src.List() {
		dir, base := path.Split(name)
		if dir != Dir+"/" {
			continue
		}
		ext := path.Ext(base)
		if ext == ".sysml" || ext == ".kerml" {
			out = append(out, strings.TrimSuffix(base, ext))
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("%s holds no .sysml or .kerml file", Dir)
	}
	return out, nil
}
