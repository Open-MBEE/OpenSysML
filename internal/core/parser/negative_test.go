package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestNegative verifies parser REJECTS malformed input (Phase 2, Task 2.3)
// Guards against silently accepting garbage
func TestRemovedSuccessionFormsProduceDiagnosticsAndErrorNodes(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		forbiddenNode string
	}{
		{"two_ended_then", "action def A { action a; action b; then a b; }", "SuccessionEdge"},
		{"state_member_then", "state def S { state a; state b; a then b; }", "SuccessionEdge"},
		{"named_done", "action def A { done end; }", "FinalNode"},
		{"guarded_two_ended_then", "action def A { action a; action b; then a b if x > 0; }", "ControlFlowEdge"},
		{"malformed_named_done", "action def A { done a b; }", "FinalNode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", dump)
			}
			if strings.Contains(dump, tt.forbiddenNode) {
				t.Fatalf("unexpected %s for removed spelling:\n%s", tt.forbiddenNode, dump)
			}
		})
	}
}

// A name written ahead of a kind keyword is no declaration: the stray name is
// reported and skipped without naming anything, and the members after it parse.
func TestNameBeforeKeywordIsNotADeclaration(t *testing.T) {
	tests := []struct {
		name       string
		ext        string
		src        string
		stray      string
		message    string
		wantMember string
	}{
		{"body_usage", ".sysml", "package P { attribute def A; part def B { foo attribute bar : A; attribute ok : A; } }", "foo", msgExpectedBodyMember, `name="ok"`},
		{"body_definition", ".sysml", "package P { part def B { foo part def Q; part def R; } }", "foo", msgExpectedBodyMember, `name="R"`},
		{"body_usage_with_body", ".sysml", "package P { part def B { myConstraint constraint { 1 > 0 } part q; } }", "myConstraint", msgExpectedBodyMember, `name="q"`},
		{"namespace_definition", ".sysml", "package P { x part def Q; part def R; }", "x", "expected a namespace member", `name="R"`},
		{"kerml_body_feature", ".kerml", "package P { class A; class B { foo feature bar : A; feature ok : A; } }", "foo", msgExpectedBodyMember, `name="ok"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			sf := source.New(tt.name+tt.ext, []byte(tt.src))
			p := New(sf)
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 1 {
				t.Fatalf("want one diagnostic, got %v", p.Diagnostics)
			}
			d := p.Diagnostics[0]
			if d.Message != tt.message {
				t.Errorf("message = %q, want %q", d.Message, tt.message)
			}
			if got := sf.Text(d.Span); got != tt.stray {
				t.Errorf("diagnostic points at %q, want the stray name %q", got, tt.stray)
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", dump)
			}
			if strings.Contains(dump, `name="`+tt.stray+`"`) {
				t.Errorf("stray name %q became a declaration:\n%s", tt.stray, dump)
			}
			if !strings.Contains(dump, tt.wantMember) {
				t.Errorf("member after the error was not parsed (want %s):\n%s", tt.wantMember, dump)
			}
		})
	}
}

// An expression binding end is rejected with its own message, so it stays
// distinguishable from an end that is missing altogether.
func TestBindingEndFailuresAreDistinguishable(t *testing.T) {
	const expressionMessage = "a binding end names a feature, not an expression"
	tests := []struct {
		name        string
		src         string
		wantMessage bool
	}{
		{"expression_end", "package P { part def D { attribute a; attribute b; bind a = b * 2; } }", true},
		{"expression_end_named_binding", "package P { part def D { attribute a; attribute b; binding bb bind a = b + 1; } }", true},
		{"expression_end_literal", "package P { part def D { attribute a; bind a = 2; } }", true},
		{"missing_right_end", "package P { part def D { attribute a; attribute b; binding bb bind a = ; } }", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parser panicked: %v", r)
				}
			}()
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
			if !strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("expected an ErrorNode:\n%s", ast.Dump(root))
			}
			var found bool
			for _, d := range p.Diagnostics {
				if strings.Contains(d.Message, expressionMessage) {
					found = true
				}
			}
			if found != tt.wantMessage {
				t.Fatalf("expression-end message present = %v, want %v: %v", found, tt.wantMessage, p.Diagnostics)
			}
		})
	}
}

// An expression assignment target is rejected with its own message, while a
// name or a chain ending in a feature stays accepted (SysML.xtext TargetParameter).
func TestAssignmentTargetMustNameFeature(t *testing.T) {
	const expressionMessage = "an assignment target names a feature or a feature chain, not an expression"
	tests := []struct {
		name     string
		src      string
		wantDiag bool
	}{
		{"name", "action def A { attribute x; assign x := 2; }", false},
		{"qualified", "package P { attribute x; action def A { assign P::x := 2; } }", false},
		{"chain", "action def A { part m { part n { attribute x; } } assign m.n.x := 2; }", false},
		{"indexed_chain", "action def A { part ms[0..*] { attribute x; } assign ms[1].x := 2; }", false},
		{"invocation_chain", "action def A { calc def pick { return : Leaf; } assign pick().x := 2; }", false},
		{"transition_effect", "state def S { attribute x; state a; state b; transition first a do assign x := 1 then b; }", false},
		{"literal", "action def A { assign 1 := 2; }", true},
		{"operator", "action def A { attribute x; assign x + 1 := 2; }", true},
		{"invocation", "action def A { calc def f { return : Integer = 1; } assign f() := 2; }", true},
		{"index", "action def A { attribute xs : Integer[0..*]; assign xs[1] := 2; }", true},
		{"transition_effect_literal", "state def S { state a; state b; transition first a do assign 1 := 1 then b; }", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if !tt.wantDiag {
				if len(p.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
				}
				if strings.Contains(ast.Dump(root), "ErrorNode") {
					t.Fatalf("unexpected ErrorNode:\n%s", ast.Dump(root))
				}
				return
			}
			if len(p.Diagnostics) != 1 || !strings.Contains(p.Diagnostics[0].Message, expressionMessage) {
				t.Fatalf("want exactly the expression-target diagnostic, got %v", p.Diagnostics)
			}
		})
	}
}

// The standard binding forms stay accepted: unqualified, chained, qualified
// and indexed ends (`bind [0..*] base.edges = [0..*] be;` in the Geometry library).
func TestStandardBindingFormsRemainValid(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"simple", "part def D { attribute a; attribute b; bind a = b; }"},
		{"chain", "part def D { part a { attribute x; } attribute b; bind b = a.x; }"},
		{"named_qualified", "package R { part a; } package P { part c; binding b1 bind R::a = c; }"},
		{"indexed_ends", "part def D { part base { part edges[0..*]; } part be[0..*]; bind [0..*] base.edges = [0..*] be; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("unexpected ErrorNode:\n%s", ast.Dump(root))
			}
		})
	}
}

func TestStandardSuccessionFormsRemainValid(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"implicit_target", "action def A { action a; then a; }"},
		{"implicit_done", "action def A { action a; then done; }"},
		{"explicit_succession", "action def A { action a; action b; succession first a then b; }"},
		{"guarded_succession", "action def A { attribute g = true; action a; action b; succession first a if g then b; }"},
		{"ordinary_done_name", "action def A { attribute done : Boolean; }"},
		{"ordinary_done_state_name", "state def S { state done; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
			}
			if strings.Contains(ast.Dump(root), "ErrorNode") {
				t.Fatalf("unexpected ErrorNode:\n%s", ast.Dump(root))
			}
		})
	}
}

func TestUnterminatedCommentIsReported(t *testing.T) {
	for _, src := range []string{
		"part def A;\n/* oops",
		"part def A;\n//* oops",
		"part def A;\n/*/",
		"part def A;\n/* oops\npart def B;\n",
	} {
		p := New(source.New("t.sysml", []byte(src)))
		p.ParseFile()
		found := false
		for _, d := range p.Diagnostics {
			if strings.Contains(d.Message, "unterminated comment") {
				found = true
			}
		}
		if !found {
			t.Errorf("ParseFile(%q) diagnostics = %v, want an unterminated comment reported", src, p.Diagnostics)
		}
	}
	// A closed comment is not reported.
	p := New(source.New("t.sysml", []byte("part def A;\n/* fine */\n//* also fine */\n")))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Errorf("a closed comment produced %v", p.Diagnostics)
	}
}

// A clause after the `terminate` marker is reported once, at the clause, and the
// usage keeps its marker and its body: the recovery skips to the body.
func TestTerminateMarkerClosesTheDeclaration(t *testing.T) {
	for _, clause := range []string{"terminate", ": Brake", "ordered", "= 3", "stop"} {
		src := "package P { action def A { first start; then stop; action stop terminate " + clause + " { doc /* d */ } } }"
		p := New(source.New("t.sysml", []byte(src)))
		f := p.ParseFile()
		if len(p.Diagnostics) != 1 || !strings.Contains(p.Diagnostics[0].Message, "only its body follows") {
			t.Errorf("ParseFile(%q) diagnostics = %v, want the clause after 'terminate' reported once", src, p.Diagnostics)
			continue
		}
		if at := p.Diagnostics[0].Span.Offset; at != strings.Index(src, clause+" {") {
			t.Errorf("ParseFile(%q) reports at %d, want the clause at %d", src, at, strings.Index(src, clause+" {"))
		}
		stop := findUsageNamed(f, "stop")
		if stop == nil || !stop.IsTerminate || len(stop.Relationships) != 0 || stop.Value != nil || len(stop.Members) != 1 {
			t.Errorf("ParseFile(%q) = %s, want a terminate usage with only its body", src, ast.Dump(stop))
		}
	}
}
