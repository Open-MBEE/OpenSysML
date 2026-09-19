package parser

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var dumpedSuccession = regexp.MustCompile(`(?s)\(SuccessionEdge source="([^"]*)" target="([^"]*)"\)|\(Usage kind="succession".*?\(ConnectorEnd target="([^"]*)".*?\(ConnectorEnd target="([^"]*)"`)

// parseSuccessions returns the succession edges src parses to, as
// "source->target" pairs in tree order, with the parser that read it.
func parseSuccessions(t *testing.T, src string) ([]string, *Parser) {
	t.Helper()
	p := New(source.New("succession.sysml", []byte(src)))
	dump := ast.Dump(p.ParseFile())

	var edges []string
	for _, m := range dumpedSuccession.FindAllStringSubmatch(dump, -1) {
		if m[1] != "" {
			edges = append(edges, m[1]+"->"+m[2])
		} else {
			edges = append(edges, m[3]+"->"+m[4])
		}
	}
	return edges, p
}

// A member-attached `then` carries the same succession as the edge notation, so
// it is desugared into one: the member before it is the source and the member
// after it the target, whatever the members are and however they are laid out.
func TestMemberAttachedThenDesugars(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			"structural body",
			"part def P { part a; then part b; }",
			[]string{"a->b"},
		},
		{
			"newline before the member does not reverse the pair",
			"part def P { part a;\n\tthen\n\tpart b;\n}",
			[]string{"a->b"},
		},
		{
			"chained thens sequence each pair in declaration order",
			"action def A { action a; then action b; then action c; }",
			[]string{"a->b", "b->c"},
		},
		{
			"the edge notation is unchanged",
			"action def A { action a; action b; succession first a then b; }",
			[]string{"a->b"},
		},
		{
			"an edge is not the source of the next succession",
			"action def A { action a; action b; succession first a then b; then action c; }",
			[]string{"a->b", "b->c"},
		},
		{
			"a one-name edge takes the member before it as its source",
			"action def A { action a; action b; then a; }",
			[]string{"b->a"},
		},
		{
			"the short state form is named, so it is sequenced",
			"state def S { state a; then state b; }",
			[]string{"a->b"},
		},
		{
			"a one-name edge whose target is a keyword the body declares",
			"action def A { action a; then done; }",
			[]string{"a->@done"},
		},
		{
			"a two-name edge whose source is a keyword the body declares",
			"action def A { action end; action b; succession first end then b; }",
			[]string{"end->b"},
		},
		{
			"a member named after the feature it references is a succession end",
			"action def A { perform a; then action b; }",
			[]string{"a->b"},
		},
		{
			"a performed step after a named member is sequenced, not chained",
			"action A { perform v; then perform t; then perform s; }",
			[]string{"v->t", "t->s"},
		},
		{
			"a performed step after a statement stays a statement of the block",
			"action A { while i <= 2 { assign v := 1; then perform body; } }",
			nil,
		},
		{
			"a calculation body reads the edge form it is written back as",
			"calc def C { part a; part b; succession first a then b; }",
			[]string{"a->b"},
		},
		{
			"a requirement body reads the edge form it is written back as",
			"requirement def R { part a; part b; succession first a then b; }",
			[]string{"a->b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, p := parseSuccessions(t, tt.src)
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Join(edges, " ") != strings.Join(tt.want, " ") {
				t.Errorf("succession edges %v, want %v", edges, tt.want)
			}
		})
	}
}

// A positional `then` sequences from the nearest feature before it (SysML v2
// §7.17.4; the pilot's UsageUtil.getPreviousFeature): a member that is not a
// feature — documentation, a comment, an import, an alias, a nested definition
// or package — is read past, while a usage that is not an edge stays the source.
func TestThenSequencesFromTheNearestFeatureBefore(t *testing.T) {
	tests := []struct {
		name   string
		member string
		want   string
	}{
		{"doc", "doc /* a then b */", "a->b"},
		{"comment", "comment /* a then b */", "a->b"},
		{"comment about", "comment about a /* the first */", "a->b"},
		{"textual representation", "rep asText language \"text\" /* a then b */", "a->b"},
		{"import", "private import Q::*;", "a->b"},
		{"alias", "alias Bump for Step;", "a->b"},
		{"nested part def", "part def Inner;", "a->b"},
		{"nested action def", "action def Inner;", "a->b"},
		{"nested package", "package Inner;", "a->b"},
		{"multiplicity declaration", "multiplicity m [1];", "a->b"},
		{"several non-features", "doc /* a then b */ part def Inner; private import Q::*;", "a->b"},
		{"metadata about stays a source", "metadata Note about a;", "@metadata->b"},
		{"prefix metadata stays a source", "@Note;", "@*ast.PrefixMetadata->b"},
		{"attribute stays a source", "attribute k;", "k->b"},
		{"part stays a source", "part p;", "p->b"},
		{"action stays a source", "action c : Step;", "c->b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package P { action def Step; metadata def Note; package Q { part def W; }\n" +
				"action def A { action a : Step; " + tt.member + " then action b : Step; } }"
			edges, p := parseSuccessions(t, src)
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Join(edges, " ") != tt.want {
				t.Errorf("succession edges %v, want %v", edges, tt.want)
			}
		})
	}
}

// A state's deferral declares no feature, so a `then` after it sequences from the state before.
func TestThenSequencesPastADeferral(t *testing.T) {
	edges, p := parseSuccessions(t, "state def S { state a; defer Ping; then state b; }")
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	if got := strings.Join(edges, " "); got != "a->b" {
		t.Errorf("succession edges %q, want a->b", got)
	}
}

// A `then` with only non-feature members before it has nothing to sequence
// from: it is diagnosed, and no succession is built from the member it passed.
func TestThenWithNoFeatureBeforeItIsDiagnosed(t *testing.T) {
	for name, body := range map[string]string{
		"nothing":            "",
		"only documentation": "doc /* b */",
		"only a definition":  "part def Inner;",
		"only an import":     "private import Q::*;",
		"only an alias":      "alias Bump for Step;",
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			src := "package P { action def Step; package Q { part def W; }\n" +
				"action def A { " + body + " then action b : Step; } }"
			edges, p := parseSuccessions(t, src)
			if len(edges) != 0 {
				t.Errorf("succession edges %v, want none", edges)
			}
			if len(p.Diagnostics) != 1 || !strings.Contains(p.Diagnostics[0].Message, "has no member before it to sequence from") {
				t.Fatalf("diagnostics %v, want one saying the `then` has no member before it", p.Diagnostics)
			}
		})
	}
}

// A one-name edge (`then b;`, `if x then b;`, `else b;`) leaves its source to the
// member before it, so one that no feature precedes is diagnosed the same way.
func TestOneNameEdgeWithNoFeatureBeforeItIsDiagnosed(t *testing.T) {
	for name, body := range map[string]string{
		"nothing":            "",
		"only documentation": "doc /* b */",
		"only a definition":  "part def Inner;",
		"only an import":     "private import Q::*;",
		"only an alias":      "alias Bump for Step;",
	} {
		for spelling, edge := range map[string]string{
			"then":    "then b;",
			"if then": "if true then b;",
			"else":    "else b;",
		} {
			t.Run(name+"/"+spelling, func(t *testing.T) {
				src := "package P { action def Step; package Q { part def W; }\n" +
					"action def A { " + body + " " + edge + " action b : Step; } }"
				p := New(source.New("succession.sysml", []byte(src)))
				p.ParseFile()
				if len(p.Diagnostics) != 1 || !strings.Contains(p.Diagnostics[0].Message, "has no member before it to sequence from") {
					t.Fatalf("diagnostics %v, want one saying the edge has no member before it", p.Diagnostics)
				}
			})
		}
	}

	// The initial node is a member a `then` sequences from.
	_, p := parseSuccessions(t, "package P { action def Step; action def A { first start; then b; action b : Step; } }")
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
}

// A nested state body carries the members of a state body, so a `then` attached
// to one of its states is the same succession it would be one level up.
func TestMemberAttachedThenInNestedStateDesugars(t *testing.T) {
	p := New(source.New("nested.sysml", []byte("state def S { state R { state a; then state b; } }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var edges []*ast.SuccessionEdge
	for _, member := range file.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		def, ok := m.Member.(*ast.Definition)
		if !ok {
			continue
		}
		for _, stateMember := range def.Members {
			nested, ok := unwrapStateUsage(stateMember)
			if !ok {
				continue
			}
			for _, nestedMember := range nested.Members {
				if edge, ok := nestedMember.(*ast.SuccessionEdge); ok {
					edges = append(edges, edge)
				}
			}
		}
	}
	if len(edges) != 1 {
		t.Fatalf("succession edges in the nested state: %d, want 1", len(edges))
	}
	if got := qnText(edges[0].Source) + "->" + qnText(edges[0].Target); got != "a->b" {
		t.Errorf("succession %s, want a->b", got)
	}
}

// unwrapStateUsage returns the state usage a body member declares.
func unwrapStateUsage(member ast.Node) (*ast.Usage, bool) {
	if m, ok := member.(*ast.Membership); ok {
		member = m.Member
	}
	usage, ok := member.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageState {
		return nil, false
	}
	return usage, true
}

// A one-name succession takes the member before it as its source whether or not
// it carries a guard, so the two spellings reach lowering alike.
func TestOneNameGuardedEdgeTakesTheMemberBefore(t *testing.T) {
	p := New(source.New("guard.sysml", []byte("action def A { action a; action b; then a if x; }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var edge *ast.ControlFlowEdge
	for _, member := range file.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		def, ok := m.Member.(*ast.Definition)
		if !ok {
			continue
		}
		for _, defMember := range def.Members {
			if e, ok := defMember.(*ast.ControlFlowEdge); ok {
				edge = e
			}
		}
	}
	if edge == nil {
		t.Fatal("a guarded succession should be a control flow edge")
	}
	if got := qnText(edge.Source) + "->" + qnText(edge.Target); got != "b->a" {
		t.Errorf("guarded succession %s, want b->a", got)
	}
}

// qnText spells a qualified name the way a succession end reads.
func qnText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	var parts []string
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}

// A member beside a `then` need not declare a name: the notation binds such an
// end by position (SysML.xtext EmptySuccessionMember), which the edge carries as
// the member itself. The dump reads a positional end as `@<kind>`.
func TestSuccessionBindsUnnamedEndsByPosition(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"unnamed source", "action def A { action; then action b; }", []string{"@action->b"}},
		{"unnamed source after a named member", "action def A { action a; action; then action b; }", []string{"@action->b"}},
		{"a send is a node of the flow", "action def A { action b; then send msg to port; }", []string{"b->@send"}},
		{"anonymous member after the keyword", "action def A { action b; then action { } }", []string{"b->@action"}},
		{"anonymous typed member after the keyword", "part def P { part a; then part : T; }", []string{"a->@part"}},
		{"a loop node reached by a succession", "action def A { action b; then loop action { } until x; }", []string{"b->@loop"}},
		{"the final node a `then done` reaches", "action def A { action b; then done; }", []string{"b->@done"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, p := parseSuccessions(t, tt.src)
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected syntax errors: %v", p.Diagnostics)
			}
			if len(p.Warnings) != 0 {
				t.Errorf("warnings %v for a succession the notation binds by position", p.Warnings)
			}
			if strings.Join(edges, " ") != strings.Join(tt.want, " ") {
				t.Errorf("succession edges %v, want %v", edges, tt.want)
			}
		})
	}
}

// The member a positional end binds to is that member itself, not another of the
// same kind: a consumer resolves the end by identity, so the edge has to point at
// the node the author wrote it beside.
func TestPositionalSuccessionEndIsTheMemberItself(t *testing.T) {
	p := New(source.New("positional.sysml", []byte(
		"action def A { action a; then send first() to p; then send second() to p; }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var sends []ast.Node
	var edges []*ast.SuccessionEdge
	for _, member := range file.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		def, ok := m.Member.(*ast.Definition)
		if !ok {
			continue
		}
		for _, defMember := range def.Members {
			switch n := defMember.(type) {
			case *ast.SendStatement:
				sends = append(sends, n)
			case *ast.SuccessionEdge:
				edges = append(edges, n)
			}
		}
	}
	if len(sends) != 2 || len(edges) != 2 {
		t.Fatalf("parsed %d sends and %d successions, want 2 and 2", len(sends), len(edges))
	}
	if edges[0].TargetMember != sends[0] {
		t.Errorf("the first succession targets %p, want the first send %p", edges[0].TargetMember, sends[0])
	}
	if edges[1].SourceMember != sends[0] || edges[1].TargetMember != sends[1] {
		t.Errorf("the second succession runs %p->%p, want %p->%p",
			edges[1].SourceMember, edges[1].TargetMember, sends[0], sends[1])
	}
}

// An end the notation supplies — the source a one-name `then <target>;` takes
// from the member before it, and both ends of the edge a member-attached `then`
// desugars to — is marked as such, since it names a member rather than being a
// reference the author wrote and could misspell.
func TestSuppliedSuccessionEndsAreMarkedImplied(t *testing.T) {
	p := New(source.New("implied.sysml", []byte(
		"action def A { action a; then b; action b; then action c; }")))
	file := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}

	var edges []*ast.SuccessionEdge
	for _, member := range file.Members {
		m, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		def, ok := m.Member.(*ast.Definition)
		if !ok {
			continue
		}
		for _, defMember := range def.Members {
			if edge, ok := defMember.(*ast.SuccessionEdge); ok {
				edges = append(edges, edge)
			}
		}
	}
	if len(edges) != 2 {
		t.Fatalf("parsed %d successions, want 2", len(edges))
	}
	if !edges[0].SourceImplied || edges[0].TargetImplied {
		t.Errorf("`then b;` has source implied=%t target implied=%t, want the source only",
			edges[0].SourceImplied, edges[0].TargetImplied)
	}
	if !edges[1].SourceImplied || !edges[1].TargetImplied {
		t.Errorf("a member-attached `then` has source implied=%t target implied=%t, want both",
			edges[1].SourceImplied, edges[1].TargetImplied)
	}
}
