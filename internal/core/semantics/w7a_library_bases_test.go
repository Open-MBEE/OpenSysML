package semantics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// stdlibModel resolves a model against the standard-library sources, so that a
// table of library names can be checked against what the library declares. The
// files are parsed here because internal/core/libs depends on this package.
func stdlibModel(t *testing.T) *Model {
	t.Helper()
	return NewModel(resolve.New(stdlibIndex(t)))
}

// stdlibIndex indexes the standard-library sources, ready for a caller that
// adds a document of its own before resolving.
func stdlibIndex(t *testing.T) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	root := filepath.Join("..", "libs", "stdlib")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := strings.ToLower(filepath.Ext(path)); ext != ".sysml" && ext != ".kerml" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		idx.AddDocument(path, parser.New(source.New(path, data)).ParseFile())
		return nil
	})
	if err != nil {
		t.Fatalf("walk the standard library: %v", err)
	}
	idx.ExpandWildcardImports()
	return idx
}

// TestW7AImplicitBaseTablesNameLibraryElements guards the keyword tables against
// a name the standard library does not declare (KerML §8.2, §8.4.2).
func TestW7AImplicitBaseTablesNameLibraryElements(t *testing.T) {
	m := stdlibModel(t)
	for _, table := range []map[string]string{implicitKerMLBases, implicitKerMLFeatureBases} {
		for keyword, fqn := range table {
			if m.symbolByFQN(fqn) == nil {
				t.Errorf("keyword %q maps to %s, which the standard library does not declare", keyword, fqn)
			}
		}
	}
	for kind, fqn := range implicitUsageBases {
		if m.symbolByFQN(fqn) == nil {
			t.Errorf("usage kind %v maps to %s, which the standard library does not declare", kind, fqn)
		}
	}
	for kind, fqn := range implicitDefinitionBases {
		if m.symbolByFQN(fqn) == nil {
			t.Errorf("definition kind %v maps to %s, which the standard library does not declare", kind, fqn)
		}
	}
}

// TestW7AMetaclassNamesAreDeclared guards the KerML keyword → metaclass mapping
// F93 reads a filter condition's metaclass from (KerML §8.2.4).
func TestW7AMetaclassNamesAreDeclared(t *testing.T) {
	m := stdlibModel(t)
	for keyword, name := range kermlMetaclassNames {
		found := false
		for _, prefix := range kermlMetaclassPrefixes {
			if m.symbolByFQN(prefix+name) != nil {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("keyword %q maps to metaclass %s, which no KerML metaclass package declares", keyword, name)
		}
	}
}
