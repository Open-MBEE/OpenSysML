package resolve

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A view of fourteen sibling parts, each exposed, plus a metadata annotation:
// resolving a name in it once walked every sibling import's target, which is
// factorial in the number of exposes and never finished. It must analyze in
// milliseconds and report no diagnostics.
func TestMembershipImportPruningBoundsWork(t *testing.T) {
	var exposes strings.Builder
	for i := 0; i < 14; i++ {
		exposes.WriteString("\t\t\texpose p" + strconv.Itoa(i) + ";\n")
	}
	src := "package P {\n\tpart def D {\n" +
		"\t\tpart p0;\n\t\tpart p1;\n\t\tpart p2;\n\t\tpart p3;\n\t\tpart p4;\n" +
		"\t\tpart p5;\n\t\tpart p6;\n\t\tpart p7;\n\t\tpart p8;\n\t\tpart p9;\n" +
		"\t\tpart p10;\n\t\tpart p11;\n\t\tpart p12;\n\t\tpart p13;\n" +
		"\t\tview V {\n" + exposes.String() +
		"\t\t\t@DiagramLayout::Canvas { unit = \"px\"; width = 1; height = 1; }\n\t\t}\n\t}\n}"
	root := parsedRoot(t, "app.sysml", src)
	idx := symbols.NewIndex()
	idx.AddDocument("lib.sysml", parsedRoot(t, "lib.sysml",
		"package DiagramLayout { metadata def Canvas { attribute unit; attribute width; attribute height; } }"))
	idx.AddDocument("app.sysml", root)
	r := New(idx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.ResolveDocument("app.sysml", root)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("analysis of a view with 14 membership exposes did not finish in 10s")
	}
	for _, d := range r.Diagnostics {
		t.Fatalf("unexpected diagnostic: %v", d)
	}
}

// A membership import surfaces the target's short name as well as its declared
// name, so the prune must keep an import whose last segment or target could be
// short-named — including an import written under the short name itself.
func TestMembershipImportSurfacesShortNames(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"lib.sysml": "package P { part def X; part def <s> Y; }",
		"app.sysml": "package Q { import P::Y; part a : s; part b : Y; } " +
			"package R { import P::s; part c : Y; part d : s; }",
	})
	r := New(idx)
	q := scopeOf(t, idx.DocumentRoot("app.sysml"), "Q")
	for _, name := range []string{"s", "Y"} {
		if _, ok := r.ResolveName(q, name, ident(name)); !ok {
			t.Fatalf("%s unresolved through import P::Y; diags=%v", name, r.Diagnostics)
		}
	}
	r2 := New(idx)
	rr := scopeOf(t, idx.DocumentRoot("app.sysml"), "R")
	for _, name := range []string{"s", "Y"} {
		if _, ok := r2.ResolveName(rr, name, ident(name)); !ok {
			t.Fatalf("%s unresolved through import P::s; diags=%v", name, r2.Diagnostics)
		}
	}
}

// A lookup pruned because no symbol carried the name as a short name depends on
// that answer: registering a short-named symbol must drop the frame.
func TestFramesDependOnShortNameReads(t *testing.T) {
	w := newTrackedIndex(t, map[string]string{
		"lib.sysml":  "package Lib { part def Y; }",
		"user.sysml": "package User { import Lib::Y; part x : s; }",
	})
	w.analyzeAll()
	dropped := w.put("lib.sysml", "package Lib { part def Y; part def <s> Z; }")
	want := map[string]bool{"lib.sysml": true, "user.sysml": true}
	for _, doc := range dropped {
		delete(want, doc)
	}
	if len(want) != 0 {
		t.Fatalf("dropped %v, want lib.sysml and user.sysml", dropped)
	}
}
