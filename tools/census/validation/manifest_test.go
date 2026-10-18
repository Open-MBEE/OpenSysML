package validation

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// loadCommittedManifest reads the committed manifest or fails the test.
func loadCommittedManifest(t *testing.T) (string, *manifest.Manifest) {
	t.Helper()
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRecord(m); err != nil {
		t.Fatal(err)
	}
	return root, m
}

// TestManifestCodesAreTheProbesDiagnostics: the diagnostic codes and passes a
// row records are exactly what its probes observe, so every code the manifest
// names is one the tree emits and no emitted code goes unrecorded.
func TestManifestCodesAreTheProbesDiagnostics(t *testing.T) {
	root, m := loadCommittedManifest(t)
	codes, passes := probeCodes(t, root)
	for _, c := range m.Constraints {
		if got, want := sorted(c.Codes), keys(codes[c.Name]); got != want {
			t.Errorf("%s records diagnostic codes %s, its probes observe %s", c.Name, got, want)
		}
		if got, want := sorted(c.Passes), keys(passes[c.Name]); got != want {
			t.Errorf("%s records passes %s, its probes observe %s", c.Name, got, want)
		}
	}
}

func sorted(list []string) string {
	copied := append([]string(nil), list...)
	sort.Strings(copied)
	return strings.Join(copied, ",")
}

func keys(set map[string]bool) string {
	var list []string
	for k := range set {
		list = append(list, k)
	}
	return sorted(list)
}

// TestManifestRoundTripsByteIdentical: decoding and re-encoding the committed
// manifest reproduces it byte for byte, so no field the tree records is one
// this program would drop on -update.
func TestManifestRoundTripsByteIdentical(t *testing.T) {
	root, m := loadCommittedManifest(t)
	committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if encoded := manifest.Encode(m); !bytes.Equal(encoded, committed) {
		t.Fatalf("%s is not what this program encodes; a field it does not know, or a non-canonical encoding:\n%s", manifest.Path, diffHint(committed, encoded))
	}
}

// diffHint names the first differing line of two encodings.
func diffHint(a, b []byte) string {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return fmt.Sprintf("line %d:\n  committed: %s\n  encoded:   %s", i+1, al[i], bl[i])
		}
	}
	return fmt.Sprintf("committed has %d lines, encoded %d", len(al), len(bl))
}

// constraintTierPasses lists the production files under internal/check/passes
// declaring a pass whose Level is LevelConstraint, keyed by repository path.
func constraintTierPasses(t *testing.T, root string) map[string][]string {
	t.Helper()
	dir := filepath.Join(root, "internal", "check", "passes")
	var entries []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			entries = append(entries, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]string{}
	for _, path := range entries {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Level" || fn.Recv == nil || fn.Body == nil {
				continue
			}
			constraint := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if ident, ok := n.(*ast.Ident); ok && ident.Name == "LevelConstraint" {
					constraint = true
				}
				return !constraint
			})
			if constraint {
				files[rel] = append(files[rel], receiverType(fn))
			}
		}
	}
	if len(files) == 0 {
		t.Fatal("no constraint-tier pass found under internal/check/passes")
	}
	return files
}

// TestConstraintTierPassesAreAccountedFor: every constraint-tier pass is cited
// by a row's implementation or listed among the manifest's extensions with its
// basis, and no extension names a file that is not such a pass. A file
// declaring one pass is credited by any citation into it; a file declaring
// several needs each pass type cited (`file.go:Type.method`) or listed.
func TestConstraintTierPassesAreAccountedFor(t *testing.T) {
	root, m := loadCommittedManifest(t)
	passes := constraintTierPasses(t, root)
	citedFiles, citedTypes := map[string]bool{}, map[string]bool{}
	for _, c := range m.Constraints {
		for _, ref := range implementationRef.FindAllStringSubmatch(c.Implementation, -1) {
			citedFiles[ref[1]] = true
			if typeName, _, isMethod := strings.Cut(ref[2], "."); isMethod {
				citedTypes[ref[1]+":"+typeName] = true
			}
		}
	}
	listed := map[string]bool{}
	for _, e := range m.Extensions {
		types, ok := passes[e.File]
		if !ok {
			t.Errorf("extension %s is not a constraint-tier pass file under internal/check/passes", e.File)
			continue
		}
		if !slicesContains(types, e.Pass) {
			t.Errorf("extension %s names pass %s; the file declares %s", e.File, e.Pass, strings.Join(types, ", "))
		}
		if citedTypes[e.File+":"+e.Pass] || (len(types) == 1 && citedFiles[e.File]) {
			t.Errorf("extension %s is also cited by a row; a pass is one or the other", e.Pass)
		}
		listed[e.File+":"+e.Pass] = true
		switch e.Basis {
		case "spec", "extension":
		default:
			t.Errorf("extension %s has basis %q, want spec or extension", e.File, e.Basis)
		}
	}
	var unaccounted []string
	for file, types := range passes {
		for _, typeName := range types {
			key := file + ":" + typeName
			if listed[key] || citedTypes[key] || (len(types) == 1 && citedFiles[file]) {
				continue
			}
			unaccounted = append(unaccounted, key)
		}
	}
	sort.Strings(unaccounted)
	if len(unaccounted) > 0 {
		t.Errorf("constraint-tier passes neither cited by a row nor listed as extensions in %s:\n  %s", manifest.Path, strings.Join(unaccounted, "\n  "))
	}
}

func slicesContains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// TestManifestErrataMatchTheRegistry: the errata the manifest lists are exactly
// the declared overlay, entry for entry, so neither can gain or lose one alone.
func TestManifestErrataMatchTheRegistry(t *testing.T) {
	_, m := loadCommittedManifest(t)
	registry := errata.Registry()
	if len(m.Errata) != len(registry) {
		t.Fatalf("%s lists %d errata, the registry declares %d", manifest.Path, len(m.Errata), len(registry))
	}
	for i, want := range registry {
		got := m.Errata[i]
		if got.ID != want.ID || got.Heading != want.Heading || got.Path != want.Path || got.Line != want.Line {
			t.Errorf("erratum %d: manifest records %+v, the registry declares %s %q at %s:%d", i, got, want.ID, want.Heading, want.Path, want.Line)
		}
	}
}
