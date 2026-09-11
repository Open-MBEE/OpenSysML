package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// transitionTargetIn returns the target name of the first transition declared
// in root, the name node name resolution keyed its verdict by.
func transitionTargetIn(t *testing.T, root *ast.RootNamespace) *ast.QualifiedName {
	t.Helper()
	var walk func(members []ast.Node) *ast.QualifiedName
	walk = func(members []ast.Node) *ast.QualifiedName {
		for _, member := range members {
			if membership, ok := member.(*ast.Membership); ok {
				member = membership.Member
			}
			switch n := member.(type) {
			case *ast.TransitionMember:
				return n.Target
			case *ast.Definition:
				if qn := walk(n.Members); qn != nil {
					return qn
				}
			case *ast.Usage:
				if qn := walk(n.Members); qn != nil {
					return qn
				}
			}
		}
		return nil
	}
	qn := walk(root.Members)
	if qn == nil {
		t.Fatal("no transition member found")
	}
	return qn
}

// A transition endpoint naming a vertex of its machine resolves, whether the
// vertex is a sibling, a nested state, a state of a sibling orthogonal region,
// or a history pseudostate of a composite state.
func TestResolveEndpointsThatNameVertices(t *testing.T) {
	cases := map[string]string{
		"sibling": `
			state def M {
				entry; then idle;
				state idle;
				state busy;
				transition first idle then busy;
			}
		`,
		"nested state": `
			state def M {
				entry; then outer;
				state outer {
					entry; then inner;
					state inner;
				}
				state done;
				transition first outer::inner then done;
			}
		`,
		"unqualified nested state": `
			state def M {
				entry; then outer;
				state outer {
					entry; then inner;
					state inner;
				}
				state done;
				transition first inner then done;
			}
		`,
		"sibling orthogonal region": `
			state def M {
				state running parallel {
					state left {
						entry; then lidle;
						state lidle;
					}
					state right {
						entry; then ridle;
						state ridle;
					}
				}
				transition first running::left::lidle then running::right::ridle;
			}
		`,
		"history of a composite state": `
			state def M {
				entry; then idle;
				state idle;
				state comp {
					history resume;
					entry; then working;
					state working;
				}
				state done;
				transition first idle then comp::resume;
				transition first comp::working then done;
			}
		`,
		"sourceless accept then": `
			state def M {
				entry; then idle;
				state idle;
				accept Ping then done;
				state done;
			}
		`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			r := resolveDoc(t, "d.sysml", src)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
			}
		})
	}
}

// A misspelled endpoint is a name-resolution diagnostic, reported where the name
// is written and suggesting the vertex it was meant to name.
func TestResolveEndpointMisspelledIsReportedWithASuggestion(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then idle;
			state idle;
			state busy;
			transition first idle then busyy;
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the misspelled endpoint, got %v", r.Diagnostics)
	}
	diag := r.Diagnostics[0]
	if !strings.Contains(diag.Message, "busyy") || !strings.Contains(diag.Message, "did you mean busy") {
		t.Errorf("expected an unresolved message suggesting busy, got %q", diag.Message)
	}
	if diag.Code != "unresolved" {
		t.Errorf("expected the unresolved code, got %q", diag.Code)
	}
	if len(diag.Fixes) != 1 || !strings.Contains(diag.Fixes[0].Title, "busy") {
		t.Errorf("expected a fix replacing the endpoint with busy, got %v", diag.Fixes)
	}
}

// A qualified endpoint's fix corrects the spelling of the vertex it names and
// keeps the qualifiers saying which state the vertex lives in.
func TestResolveEndpointMisspelledQualifiedFixKeepsTheQualifier(t *testing.T) {
	src := `state def M {
	entry; then alpha;
	state alpha { entry; then work; state work; }
	state beta { entry; then work; state work; }
	transition first beta::workk then alpha;
}`
	r := resolveDoc(t, "d.sysml", src)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %v", r.Diagnostics)
	}
	fixes := r.Diagnostics[0].Fixes
	if len(fixes) != 1 {
		t.Fatalf("expected one fix, got %v", fixes)
	}
	edit := fixes[0].Edits[0]
	if got := src[edit.Span.Offset : edit.Span.Offset+edit.Span.Len]; got != "workk" {
		t.Errorf("expected the fix to replace the last segment, it replaces %q", got)
	}
	if edit.NewText != "work" {
		t.Errorf("expected the fix to write work, got %q", edit.NewText)
	}
}

// An endpoint that resolves to something which is not a vertex names no state a
// transition can start or end at, which the name-resolution tier reports.
func TestResolveEndpointNotAVertexIsReported(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			attribute count;
			entry; then idle;
			state idle;
			transition first idle then count;
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the non-vertex endpoint, got %v", r.Diagnostics)
	}
	diag := r.Diagnostics[0]
	if diag.Code != CodeNotAVertex {
		t.Errorf("expected the %s code, got %q", CodeNotAVertex, diag.Code)
	}
	if !strings.Contains(diag.Message, "count") {
		t.Errorf("expected the endpoint named in the message, got %q", diag.Message)
	}
}

// The machine's entry action stands in for a start pseudostate, so a transition
// may leave it — while an ordinary action is still no vertex.
func TestResolveEndpointEntryActionIsAVertex(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry action start { }
			transition start then idle;
			state idle;
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for the entry action endpoint, got %v", r.Diagnostics)
	}
}

func TestResolveEndpointEntryActionUsesNearestRegionBinding(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			state left {
				entry action initial { }
				state ready;
				transition initial then ready;
			}
			state right {
				entry action initial { }
				state ready;
				transition initial then ready;
			}
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected each region's entry action to resolve locally, got %v", r.Diagnostics)
	}
}

func TestResolveEndpointActionBodyWinsOverEnclosingState(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			state outer {
				action inner {
					action first;
					action second;
					succession path first first then second;
				}
			}
		}
	`)
	for _, diagnostic := range r.Diagnostics {
		if strings.Contains(diagnostic.Message, "is not a state or pseudostate") {
			t.Errorf("nested action endpoint reported as state vertex: %s", diagnostic.Message)
		}
	}
}

func TestResolveEndpointOrdinaryActionIsNotAVertex(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			action work { }
			entry; then idle;
			state idle;
			transition work then idle;
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the action endpoint, got %v", r.Diagnostics)
	}
	if r.Diagnostics[0].Code != CodeNotAVertex {
		t.Errorf("expected the %s code, got %q", CodeNotAVertex, r.Diagnostics[0].Code)
	}
}

// Diagnostics belong to the name-resolution tier: the lookup lowering makes
// reports nothing, however it turns out.
func TestEndpointLookupForLoweringReportsNothing(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then idle;
			state idle;
			state busy;
			transition first idle then busy;
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "nowhere"}}}
	_, ok := r.Endpoint(nil, qn)
	if ok {
		t.Error("an endpoint naming nothing resolved")
	}
	if len(r.Diagnostics) != 0 {
		t.Errorf("a lookup made for lowering reported: %v", r.Diagnostics)
	}
}

// An endpoint the document's own resolution reported names no vertex for
// lowering either, which leaves the edge out rather than reporting it again.
func TestEndpointLookupForLoweringDoesNotReportTwice(t *testing.T) {
	src := `
		state def M {
			entry; then idle;
			state idle;
			state busy;
			transition first idle then nowhere;
		}
	`
	p := parser.New(source.New("d.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	r := New(symbols.NewIndexFromDoc("d.sysml", root))
	r.ResolveDocument("d.sysml", root)

	qn := transitionTargetIn(t, root)
	if _, ok := r.Endpoint(nil, qn); ok {
		t.Error("an endpoint naming nothing resolved")
	}
	before := len(r.Diagnostics)
	if _, ok := r.Endpoint(nil, qn); ok {
		t.Error("an endpoint naming nothing resolved")
	}
	if len(r.Diagnostics) != before {
		t.Errorf("lowering's lookup reported the endpoint a second time: %v", r.Diagnostics)
	}
}

// VertexInScope names an endpoint from the scope tree alone, for a machine
// lowering has the tree of but no resolution pass over: the innermost vertex of
// that spelling wins, and nothing outside the machine answers at all.
func TestVertexInScopeNamesTheInnermostVertexAndNothingOutside(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then alpha;
			state alpha {
				entry; then work;
				state work;
			}
			state beta {
				entry; then work;
				state work;
				transition first work then done;
			}
			state done;
		}
		state def Other {
			entry; then idle;
			state idle;
		}
	`)
	beta := scopeNamed(t, r, "d.sysml", "beta")
	work := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "work"}}}
	decl, ok := VertexInScope(beta, work)
	if !ok {
		t.Fatal("work names no vertex from inside beta")
	}
	want := beta.LookupLocalAll("work")
	if len(want) == 0 || want[0].Decl != decl {
		t.Errorf("work resolved to %v, want beta's own work", decl)
	}

	outside := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "Other"}, {Text: "idle"}}}
	if _, ok := VertexInScope(beta, outside); ok {
		t.Error("a vertex of another machine answered a scope-only lookup")
	}
}

// `then done;` names the end shot every state inherits, so it is no vertex of
// the machine and reports nothing.
func TestResolveEndpointDoneIsTheInheritedEndShot(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then idle;
			state idle;
			transition first idle then done;
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics for the completion endpoint, got %v", r.Diagnostics)
	}
}

// A machine declaring a state of its own named done names that state instead.
func TestResolveEndpointDeclaredDoneStateWins(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			entry; then idle;
			state idle;
			state done;
			transition first idle then done;
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected the declared state to resolve, got %v", r.Diagnostics)
	}
}

// A non-vertex member named done shadows the end shot, and is reported as the
// non-vertex endpoint it is rather than silently completing the machine.
func TestResolveEndpointDoneShadowedByANonVertexIsReported(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def M {
			attribute done;
			entry; then idle;
			state idle;
			transition first idle then done;
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic for the shadowed endpoint, got %v", r.Diagnostics)
	}
	if r.Diagnostics[0].Code != CodeNotAVertex {
		t.Errorf("expected the %s code, got %q", CodeNotAVertex, r.Diagnostics[0].Code)
	}
}
