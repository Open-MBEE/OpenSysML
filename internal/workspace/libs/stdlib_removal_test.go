package libs

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The index an editor session holds is reused across edits: it carries the whole
// standard library and takes documents as they are opened and closed. Removing
// one of them must leave the index a fresh session would have built, over the
// library too — a re-export the library's own facade packages surfaced must not
// be dropped with a user document, and one the document surfaced must not stay.
func TestRemoveDocumentEqualsFreshBuildOverStdlib(t *testing.T) {
	const (
		docA = "a.sysml"
		docB = "b.sysml"
		srcA = `package UsesSI { public import SI::*; part def Rig; }
package Facade { public import UsesSI::*; }`
		srcB = `package UsesISQ { private import ISQ::*; public import Facade::*; part def Jig; }`
	)

	reused := indexWithStdlib(t)
	addSource(t, reused, docA, srcA)
	addSource(t, reused, docB, srcB)
	reused.ExpandWildcardImports()
	if len(reused.LookupQualified("UsesSI::gram")) == 0 {
		t.Fatal("UsesSI::gram not re-exported: the case does not test what it means to")
	}
	if len(reused.LookupQualified("UsesISQ::Rig")) == 0 {
		t.Fatal("UsesISQ::Rig not re-exported through Facade")
	}
	reused.RemoveDocument(docA)

	fresh := indexWithStdlib(t)
	addSource(t, fresh, docB, srcB)
	fresh.ExpandWildcardImports()

	if diff := diffIndexes(fresh, reused); diff != "" {
		t.Errorf("removing %s left an index a fresh build would not produce:\n%s", docA, diff)
	}
}

// indexWithStdlib loads every standard library file, as a session does. The
// cache is the test's own: a symbol restored from a record carries its qualified
// name where a parsed one carries its local name (index.go), so a shared cache
// another test populates mid-run would make the two indexes differ over that
// alone.
func indexWithStdlib(t *testing.T) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	src := DefaultSource()
	cache := &Cache{dir: t.TempDir()}
	if err := NewLoader(src, cache).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
	return idx
}

func addSource(t *testing.T, idx *symbols.Index, name, src string) {
	t.Helper()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
}

// diffIndexes reports where two indexes disagree on what a name means: the
// symbols it denotes, whether each is visible outside the namespace holding it
// (which is what a private import's re-export is not), and the imports still
// driving expansion. It reports at most a few differences, which is enough to
// tell what went wrong.
func diffIndexes(want, got *symbols.Index) string {
	const maxReported = 10
	var out []string
	report := func(format string, args ...any) {
		if len(out) < maxReported {
			out = append(out, fmt.Sprintf(format, args...))
		}
	}
	for _, fqn := range union(want.FQNs(), got.FQNs()) {
		if w, g := describeName(want, fqn), describeName(got, fqn); w != g {
			report("%s:\n  want %s\n  got  %s", fqn, w, g)
		}
	}
	if len(out) == maxReported {
		out = append(out, "...")
	}
	return strings.Join(out, "\n")
}

// describeName renders what fqn means, without depending on symbol identity:
// the two indexes hold different symbols for the same declaration.
func describeName(idx *symbols.Index, fqn string) string {
	parent, _ := splitLeaf(fqn)
	visible := make(map[string]bool)
	for _, sym := range idx.LookupQualified(fqn) {
		visible[idx.GetFQN(sym)+" "+sym.Name] = true
	}
	descs := make([]string, 0, len(visible))
	for _, sym := range idx.LookupQualifiedFrom(fqn, parent) {
		key := idx.GetFQN(sym) + " " + sym.Name
		where := "hidden"
		if visible[key] {
			where = "visible"
		}
		descs = append(descs, key+" "+where)
	}
	sort.Strings(descs)
	return fmt.Sprintf("%v imports=%v", descs, idx.WildcardImportsOf(fqn))
}

func splitLeaf(fqn string) (parent, name string) {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[:i], fqn[i+2:]
	}
	return "", fqn
}

func union(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
