package libs_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

const vendoredExtensions = "OpenSysML Libraries/"

// extensionsMeta reads the Extensions Library's .meta.json the sync script
// vendors beside its .project.json, at the pinned upstream commit.
func extensionsMeta(t *testing.T) project.Meta {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("extensions-project", project.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := project.ParseMeta(content)
	if err != nil {
		t.Fatal(err)
	}
	return meta
}

// vendoredExtensionUnits lists the embedded extension units by their path in
// the published project, libraries/<Unit>, with their bytes.
func vendoredExtensionUnits(t *testing.T) map[string][]byte {
	t.Helper()
	src := libs.EmbeddedSource()
	units := map[string][]byte{}
	for _, name := range src.List() {
		if !strings.HasPrefix(name, vendoredExtensions) {
			continue
		}
		content, err := src.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		units["libraries/"+strings.TrimPrefix(name, vendoredExtensions)] = content
	}
	return units
}

// Each vendored unit is recorded in the upstream .meta.json with its SHA-256 and each recorded
// unit is vendored: the manifests come from the pin, so an edit on either side fails this gate.
func TestVendoredExtensionUnitsMatchTheirRecordedChecksums(t *testing.T) {
	meta := extensionsMeta(t)
	units := vendoredExtensionUnits(t)
	if len(units) == 0 {
		t.Fatal("no extension units are embedded")
	}
	for path, content := range units {
		sums, ok := meta.Checksum[path]
		if !ok {
			t.Errorf("%s is vendored but %s records no checksum for it", path, project.MetaFile)
			continue
		}
		for _, sum := range sums {
			if sum.Algorithm != "SHA256" {
				t.Errorf("%s: checksum algorithm %q, want SHA256", path, sum.Algorithm)
				continue
			}
			digest := sha256.Sum256(content)
			if got := hex.EncodeToString(digest[:]); !strings.EqualFold(got, sum.Value) {
				t.Errorf("%s: SHA256 %s, recorded as %s", path, got, sum.Value)
			}
		}
	}
	for path := range meta.Checksum {
		if _, ok := units[path]; !ok && project.IsModelFile(path) {
			t.Errorf("%s records %s, which is not vendored", project.MetaFile, path)
		}
	}
}

// Every vendored unit declares exactly the root namespaces the upstream index
// names for it, and no root is indexed to a unit that is not vendored.
func TestVendoredExtensionUnitsDeclareTheIndexedRoots(t *testing.T) {
	meta := extensionsMeta(t)
	units := vendoredExtensionUnits(t)
	indexed := map[string][]string{}
	for root, path := range meta.Index {
		if _, ok := units[path]; !ok {
			t.Errorf("index entry %q names %s, which is not vendored", root, path)
			continue
		}
		indexed[path] = append(indexed[path], root)
	}
	for path, content := range units {
		declared := rootNames(parser.New(source.New(path, content)).ParseFile())
		want := indexed[path]
		sort.Strings(declared)
		sort.Strings(want)
		if strings.Join(declared, ",") != strings.Join(want, ",") {
			t.Errorf("%s declares %q, the index names %q for it", path, declared, want)
		}
	}
}

// rootNames lists the names the members of a root namespace declare.
func rootNames(root *ast.RootNamespace) []string {
	var out []string
	for _, m := range root.Members {
		if v, ok := m.(*ast.Membership); ok {
			m = v.Member
		}
		switch d := m.(type) {
		case *ast.Package:
			out = append(out, d.Ident.Name)
		case *ast.Namespace:
			out = append(out, d.Ident.Name)
		}
	}
	return out
}
