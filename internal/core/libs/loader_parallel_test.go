package libs

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestLoadAllMatchesSerialLoad locks the concurrent load to the index a serial
// load builds: every document, symbol, declaration, wildcard import, filter,
// re-export visibility and installed fact must be the same.
func TestLoadAllMatchesSerialLoad(t *testing.T) {
	src := DefaultSource()

	serial := symbols.NewIndex()
	sl := NewLoader(src, nil)
	for _, name := range src.List() {
		if err := sl.load(name, serial); err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
	}
	serial.ExpandWildcardImports()
	sl.installFacts(serial, true)

	parallel := symbols.NewIndex()
	pl := NewLoader(src, nil)
	if err := pl.LoadAll(parallel); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if sl.setDigest() != pl.setDigest() {
		t.Errorf("set digest differs: serial %s, parallel %s", sl.setDigest(), pl.setDigest())
	}
	want, got := indexFingerprint(serial), indexFingerprint(parallel)
	if want != got {
		t.Errorf("index differs from a serial load:\n%s", firstDifference(want, got))
	}
	if want, got := indexRecords(serial), indexRecords(parallel); !reflect.DeepEqual(want, got) {
		t.Errorf("derived records differ from a serial load")
	}
}

// indexFingerprint renders everything an index answers about its contents, one
// line per fact, so two indexes compare by string.
func indexFingerprint(idx *symbols.Index) string {
	var b strings.Builder
	docs := idx.Documents()
	sort.Strings(docs)
	for _, doc := range docs {
		fmt.Fprintf(&b, "doc %s kind=%v root=%v\n", doc, idx.DocumentKind(doc), idx.DocumentRoot(doc) != nil)
	}
	visible := sha256.New() // the visibility matrix is too large to keep as text
	for _, fqn := range idx.FQNs() {
		syms := idx.LookupQualified(fqn)
		fmt.Fprintf(&b, "%s: %d symbols, declaring=%v\n", fqn, len(syms), idx.Declaring(fqn) != nil)
		for i, sym := range syms {
			fmt.Fprintf(&b, "  [%d] %s kind=%v short=%q vis=%v decl=%v name=%v doc=%s effective=%v library=%v fqn=%s\n",
				i, sym.Name, sym.Kind, sym.ShortName, sym.Visibility, sym.DeclSpan, sym.NameSpan,
				sym.DocName, sym.EffectiveName(), idx.Library(sym), idx.GetFQN(sym))
			fmt.Fprintf(&b, "      decl=%x trivia=%v facts=%s\n",
				sha256.Sum256([]byte(ast.Dump(sym.Decl))), sym.LeadingTrivia, factsString(sym.Facts))
			for _, doc := range docs {
				if idx.ReexportVisible(doc, fqn, sym) {
					fmt.Fprintf(visible, "%s\x00%s\x00%d\n", doc, fqn, i)
				}
			}
		}
		for _, imp := range idx.WildcardImportsOf(fqn) {
			fmt.Fprintf(&b, "  import %s private=%v filter=%s\n", imp.Target, imp.Private, filterString(imp.Filter))
		}
		for _, f := range idx.NamespaceFiltersOf(fqn) {
			fmt.Fprintf(&b, "  filter %s\n", filterString(f))
		}
	}
	fmt.Fprintf(&b, "visibility %x\n", visible.Sum(nil))
	return b.String()
}

func factsString(f *symbols.LibraryFacts) string {
	if f == nil {
		return "nil"
	}
	var unit, dim string
	if f.Unit != nil {
		unit = fmt.Sprintf("%+v", *f.Unit)
	}
	if f.Dimension != nil {
		dim = fmt.Sprintf("%+v", *f.Dimension)
	}
	return fmt.Sprintf("{supers=%q unit=%s dim=%s abstract=%v}", f.Supers, unit, dim, f.Abstract)
}

func filterString(f symbols.ElementFilter) string {
	if f.IsZero() {
		return "none"
	}
	return fmt.Sprintf("%v %x", f.Span, sha256.Sum256([]byte(ast.Dump(f.Expr))))
}

// indexRecords derives the facts record of every document, the surface the
// on-disk cache persists.
func indexRecords(idx *symbols.Index) []*IndexRecord {
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	docs := idx.Documents()
	sort.Strings(docs)
	recs := make([]*IndexRecord, 0, len(docs))
	for _, doc := range docs {
		rec, _ := recordFromIndex(doc, idx, model)
		recs = append(recs, rec)
	}
	return recs
}

// firstDifference reports the first line at which two renderings differ.
func firstDifference(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n  serial:   %s\n  parallel: %s", i+1, w, g)
		}
	}
	return "identical"
}
