package passes

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// applyFixes renders every fix edit against content and applies them back to
// front, collapsing several fixes' identical import insertions into one.
func applyFixes(t *testing.T, content []byte, diags []diag.Diagnostic) string {
	t.Helper()
	type rendered struct {
		span    source.Span
		newText string
	}
	seen := map[string]bool{}
	var edits []rendered
	for _, d := range diags {
		for _, fix := range d.Fixes {
			for _, e := range fix.Edits {
				span, text := e.Render(content)
				key := string(rune(span.Offset)) + "|" + text
				if seen[key] {
					continue
				}
				seen[key] = true
				edits = append(edits, rendered{span, text})
			}
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].span.Offset > edits[j].span.Offset })
	out := string(content)
	for _, e := range edits {
		out = out[:e.span.Offset] + e.newText + out[e.span.End():]
	}
	return out
}

// notationDiagnostics parses and runs the notation pass over one document,
// carrying its source text so fixes can indent like the member they rewrite.
func notationDiagnostics(t *testing.T, name, src string) (*ast.RootNamespace, *source.SourceFile, []diag.Diagnostic) {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("%s: parse errors %+v", src, p.Diagnostics)
	}
	idx := newTestIndexFromDoc(name, root)
	ctx := NewContext(name, idx, nil)
	ctx.InBatch(&kit.Batch{
		Documents: []string{name},
		Source:    source.TextOf(map[string]*source.SourceFile{name: sf}, nil),
	})
	return root, sf, (NonstandardNotationPass{}).Run(ctx, name, root)
}

// stateGraphOf lowers the state usage name in one parsed document through the
// standard library, the way the runtime does.
func stateGraphOf(t *testing.T, name, src, usage string) *lower.StateGraph {
	t.Helper()
	sf := source.New(name, []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("%s: parse errors %+v", src, p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot(name).LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	u, ok := pkg.Scope.LookupLocal(usage)
	if !ok {
		t.Fatalf("state %s not indexed", usage)
	}
	g, err := lower.ToStateGraphWithEndpoints(u.Decl, u.Scope,
		lower.NewLibraryStateTypes(resolve.New(idx)))
	if err != nil {
		t.Fatalf("lowering %s: %v", usage, err)
	}
	return g
}

func graphShape(g *lower.StateGraph) []string {
	shape := make([]string, 0, len(g.Pseudostates))
	for _, ps := range g.Pseudostates {
		shape = append(shape, ps.Kind.String()+":"+ps.Name)
	}
	for _, s := range g.States {
		shape = append(shape, "state:"+s.Name+":defer="+itoa(len(s.Defer)))
	}
	return shape
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	d := []byte{}
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// TestNotationFixRewritesOldSpellings: applying the fixes every old state
// spelling's diagnostic carries produces text that parses with no
// nonstandard-notation diagnostic and lowers to the same graph.
func TestNotationFixRewritesOldSpellings(t *testing.T) {
	oldForm := `package P {
	private import ScalarValues::*;
	item def Ping;
	action def setSpeed { attribute v : Integer; }
	state def M {
		entry; then a;
		state a {
			defer Ping, setSpeed(v);
		}
		choice pick; // the pick point
		junction j;
		state comp {
			entry; then inner;
			state inner;
		}
		shallow history sh;
		deep history dh;
	}
}`

	root, sf, diags := notationDiagnostics(t, "a.sysml", oldForm)
	_ = root
	if len(diags) != 5 {
		t.Fatalf("got %d diagnostics %+v, want 5", len(diags), diags)
	}
	for _, d := range diags {
		if len(d.Fixes) == 0 {
			t.Fatalf("diagnostic %q carries no fix", d.Message)
		}
	}
	fixed := applyFixes(t, sf.Bytes(), diags)
	if n := strings.Count(fixed, "private import StateMachines::*;"); n != 1 {
		t.Fatalf("fixed text carries the StateMachines import %d times:\n%s", n, fixed)
	}
	// The rewrite ends at the member's `;`: the next member keeps its line
	// and indentation rather than being swallowed into the replacement.
	for _, want := range []string{
		"#choice state pick; // the pick point\n\t\t#junction state j;",
		"#junction state j;\n\t\tstate comp {",
		"#deferred ref : setSpeed;\n\t\t}",
		"#shallowHistory state sh;\n\t\t#deepHistory state dh;\n\t}",
	} {
		if !strings.Contains(fixed, want) {
			t.Fatalf("fixed text lost the trivia after the rewritten member; want %q in:\n%s", want, fixed)
		}
	}

	_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
	if len(fixedDiags) != 0 {
		t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
	}

	gOld := stateGraphOf(t, "a.sysml", oldForm, "M")
	gNew := stateGraphOf(t, "b.sysml", fixed, "M")
	oldShape, newShape := graphShape(gOld), graphShape(gNew)
	if len(oldShape) != len(newShape) {
		t.Fatalf("rewritten graph shape %v != %v\n%s", newShape, oldShape, fixed)
	}
	for i := range oldShape {
		if oldShape[i] != newShape[i] {
			t.Fatalf("rewritten graph shape %v != %v\n%s", newShape, oldShape, fixed)
		}
	}
}

// TestNotationFixSkipsExistingImport: when an enclosing body already imports
// `StateMachines::*` the fix is the member replacement alone, one edit.
func TestNotationFixSkipsExistingImport(t *testing.T) {
	src := `package Probe {
	private import StateMachines::*;
	item def Ev;
	state def M {
		entry; then done;
		choice pick;
		state done;
	}
}`

	_, sf, diags := notationDiagnostics(t, "a.sysml", src)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics %+v, want 1", len(diags), diags)
	}
	fix := diags[0].Fixes
	if len(fix) != 1 || len(fix[0].Edits) != 1 {
		t.Fatalf("fix should carry the replacement edit alone: %+v", fix)
	}
	fixed := applyFixes(t, sf.Bytes(), diags)
	if n := strings.Count(fixed, "import StateMachines"); n != 1 {
		t.Fatalf("a second StateMachines import was inserted:\n%s", fixed)
	}
	if !strings.Contains(fixed, "#choice state pick;") {
		t.Fatalf("fixed text lacks the metadata spelling:\n%s", fixed)
	}

	_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
	if len(fixedDiags) != 0 {
		t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
	}
}

// TestNotationFixQualifiesCollidingNames: when the member's name is the
// annotation's own (`choice choice;`, `junction junction;`) or a deferred
// signal is literally named `deferred`, the fix spells the metadata qualified
// and adds no import — the qualified name resolves unshadowed on its own.
func TestNotationFixQualifiesCollidingNames(t *testing.T) {
	oldForm := `package P {
	item def deferred;
	state def M {
		entry; then a;
		state a {
			defer deferred;
		}
		choice choice;
		junction junction;
	}
}`

	_, sf, diags := notationDiagnostics(t, "a.sysml", oldForm)
	if len(diags) != 3 {
		t.Fatalf("got %d diagnostics %+v, want 3", len(diags), diags)
	}
	for _, d := range diags {
		if len(d.Fixes) != 1 || len(d.Fixes[0].Edits) != 1 {
			t.Fatalf("colliding fix %q should carry the replacement edit alone: %+v", d.Message, d.Fixes)
		}
	}
	fixed := applyFixes(t, sf.Bytes(), diags)
	for _, want := range []string{
		"#StateMachines::deferred ref : deferred;",
		"#StateMachines::choice state choice;",
		"#StateMachines::junction state junction;",
	} {
		if !strings.Contains(fixed, want) {
			t.Fatalf("fixed text lacks %q:\n%s", want, fixed)
		}
	}
	if strings.Contains(fixed, "import StateMachines") {
		t.Fatalf("a qualified fix needs no import:\n%s", fixed)
	}

	_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
	if len(fixedDiags) != 0 {
		t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
	}

	gOld := stateGraphOf(t, "a.sysml", oldForm, "M")
	gNew := stateGraphOf(t, "b.sysml", fixed, "M")
	oldShape, newShape := graphShape(gOld), graphShape(gNew)
	if len(oldShape) != len(newShape) {
		t.Fatalf("rewritten graph shape %v != %v\n%s", newShape, oldShape, fixed)
	}
	for i := range oldShape {
		if oldShape[i] != newShape[i] {
			t.Fatalf("rewritten graph shape %v != %v\n%s", newShape, oldShape, fixed)
		}
	}
}

// TestNotationFixQuotesUnrestrictedNames: a name that writes as a quoted
// unrestricted name stays quoted in the rewrite, both for a pseudostate's
// name and for a deferred trigger's target.
func TestNotationFixQuotesUnrestrictedNames(t *testing.T) {
	oldForm := `package P {
	item def 'Alert Event';
	state def M {
		entry; then a;
		state a {
			defer 'Alert Event';
		}
		choice 'pick point';
	}
}`

	_, sf, diags := notationDiagnostics(t, "a.sysml", oldForm)
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics %+v, want 2", len(diags), diags)
	}
	fixed := applyFixes(t, sf.Bytes(), diags)
	for _, want := range []string{
		"#deferred ref : 'Alert Event';",
		"#choice state 'pick point';",
	} {
		if !strings.Contains(fixed, want) {
			t.Fatalf("fixed text lacks %q:\n%s", want, fixed)
		}
	}

	_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
	if len(fixedDiags) != 0 {
		t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
	}
}

// TestNotationFixQualifiesShadowedAnnotations: a member named like the
// annotation in an enclosing body shadows it, so the fix spells the metadata
// qualified and adds no import, as it does for a same-named member.
func TestNotationFixQualifiesShadowedAnnotations(t *testing.T) {
	src := `package P {
	item def choice;
	attribute def deferred;
	item def Ping;
	state def M {
		entry; then a;
		state a {
			defer Ping;
		}
		choice pick;
	}
}`

	_, sf, diags := notationDiagnostics(t, "a.sysml", src)
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics %+v, want 2", len(diags), diags)
	}
	for _, d := range diags {
		if len(d.Fixes) != 1 || len(d.Fixes[0].Edits) != 1 {
			t.Fatalf("shadowed fix %q should carry the replacement edit alone: %+v", d.Message, d.Fixes)
		}
	}
	fixed := applyFixes(t, sf.Bytes(), diags)
	for _, want := range []string{
		"#StateMachines::deferred ref : Ping;",
		"#StateMachines::choice state pick;",
	} {
		if !strings.Contains(fixed, want) {
			t.Fatalf("fixed text lacks %q:\n%s", want, fixed)
		}
	}
	if strings.Contains(fixed, "import StateMachines") {
		t.Fatalf("a qualified fix needs no import:\n%s", fixed)
	}

	_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
	if len(fixedDiags) != 0 {
		t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
	}
}

// TestNotationFixMovesDeferOutOfAChain: a `defer` rewritten in place inside a
// positional chain would leave `then` without a member it can sequence from,
// so the fix deletes the defer member's line and writes the ref ahead of the
// member the chain leaves.
func TestNotationFixMovesDeferOutOfAChain(t *testing.T) {
	src := `package P {
	item def Ping;
	state def M {
		entry;
		defer Ping;
		then a;
		state a;
	}
}`

	_, sf, diags := notationDiagnostics(t, "a.sysml", src)
	// The `then` after `defer` is itself an extension finding; the defer
	// member's fix carries the delete and insert edits.
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics %+v, want 2", len(diags), diags)
	}
	var fixes int
	for _, d := range diags {
		fixes += len(d.Fixes)
	}
	if fixes != 1 {
		t.Fatalf("got %d fixes, want the defer member's one: %+v", fixes, diags)
	}
	fixed := applyFixes(t, sf.Bytes(), diags)
	for _, want := range []string{
		"#deferred ref : Ping;\n\t\tentry;",
		"entry;\n\t\tthen a;",
		"private import StateMachines::*;",
	} {
		if !strings.Contains(fixed, want) {
			t.Fatalf("fixed text lacks %q:\n%s", want, fixed)
		}
	}
	if strings.Contains(fixed, "defer Ping;") {
		t.Fatalf("the defer member was not deleted:\n%s", fixed)
	}

	_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
	if len(fixedDiags) != 0 {
		t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
	}
}

// TestNotationFixSpellsRootedWhenStateMachinesIsTaken: a `StateMachines`
// member hides the library package the annotation names, so the fix spells the
// metadata `$::`-rooted and adds no import, whether the member sits beside the
// machine's package or inside the machine's own.
func TestNotationFixSpellsRootedWhenStateMachinesIsTaken(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"sibling package", `package StateMachines {}
package P {
	state def M {
		entry; then a;
		state a;
		choice pick;
	}
}`},
		{"enclosing package", `package P {
	package StateMachines {}
	state def M {
		entry; then a;
		state a;
		choice pick;
	}
}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, sf, diags := notationDiagnostics(t, "a.sysml", tc.src)
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics %+v, want 1", len(diags), diags)
			}
			fixed := applyFixes(t, sf.Bytes(), diags)
			if !strings.Contains(fixed, "#$::StateMachines::choice state pick;") {
				t.Fatalf("fixed text lacks the rooted spelling:\n%s", fixed)
			}
			if strings.Contains(fixed, "import ") {
				t.Fatalf("a rooted fix needs no import:\n%s", fixed)
			}
			_, _, fixedDiags := notationDiagnostics(t, "b.sysml", fixed)
			if len(fixedDiags) != 0 {
				t.Fatalf("fixed text still reports %+v:\n%s", fixedDiags, fixed)
			}
			if got := graphShape(stateGraphOf(t, "b.sysml", fixed, "M")); !reflect.DeepEqual(got, []string{"choice:pick", "state:a:defer=0"}) {
				t.Fatalf("fixed text lowers to %v, want [choice:pick state:a:defer=0]", got)
			}
		})
	}
}
