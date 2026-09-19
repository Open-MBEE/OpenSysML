package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A feature value is written `= expr`, `:= expr` or `default` followed by either
// of them (KerML `FeatureValue`), wherever a feature can carry a value.
func TestFeatureValueOperators(t *testing.T) {
	cases := []struct {
		body               string
		op                 string
		isDefault, initial bool
	}{
		{"attribute m = 10;", "=", false, false},
		{"attribute m := 10;", ":=", false, true},
		{"attribute m default 10;", "default", true, false},
		{"attribute m default = 10;", "default =", true, false},
		{"attribute m default := 10;", "default :=", true, true},
		{"attribute m default   = 10;", "default   =", true, false},
		{"attribute m : Real default = 10;", "default =", true, false},
		{"attribute m :> base default = 10;", "default =", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			src := "package P { attribute def Real; attribute base; part def D { " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			usage := findUsageNamed(root, "m")
			if usage == nil {
				t.Fatal("attribute m not found")
			}
			if usage.Value == nil {
				t.Error("the feature value was not read")
			}
			if got := src[usage.ValueOperatorSpan.Offset:usage.ValueOperatorSpan.End()]; got != tc.op {
				t.Errorf("value operator span %q, want %q", got, tc.op)
			}
			if !strings.HasPrefix(src[usage.ValueOperatorSpan.Offset:], tc.op) {
				t.Errorf("value operator span starts at %d, not at %q", usage.ValueOperatorSpan.Offset, tc.op)
			}
			if usage.ValueIsDefault != tc.isDefault || usage.ValueIsInitial != tc.initial {
				t.Errorf("default=%t initial=%t, want default=%t initial=%t",
					usage.ValueIsDefault, usage.ValueIsInitial, tc.isDefault, tc.initial)
			}
		})
	}
}

// valuePart is the value part a declaration carries, however it is parsed.
type valuePart struct {
	value              ast.Node
	op                 source.Span
	isDefault, initial bool
}

// findValuePart returns the value part of the usage, subject or owned
// constraint named name.
func findValuePart(node ast.Node, name string) (valuePart, bool) {
	if usage := findUsageNamed(node, name); usage != nil {
		return valuePart{usage.Value, usage.ValueOperatorSpan, usage.ValueIsDefault, usage.ValueIsInitial}, true
	}
	return findMemberValuePart(node, name)
}

// findMemberValuePart returns the value part of the subject or owned constraint
// named name under node.
func findMemberValuePart(node ast.Node, name string) (valuePart, bool) {
	var members []ast.Node
	switch v := node.(type) {
	case *ast.Membership:
		return findMemberValuePart(v.Member, name)
	case *ast.RootNamespace:
		members = v.Members
	case *ast.Package:
		members = v.Members
	case *ast.Definition:
		members = v.Members
	case *ast.Usage:
		members = v.Members
	case *ast.SubjectMember:
		if v.Ident.Name == name {
			return valuePart{v.BindingExpr, v.ValueOperatorSpan, v.ValueIsDefault, v.ValueIsInitial}, true
		}
		return valuePart{}, false
	case *ast.AssumeMember:
		if v.Ident.Name == name {
			return valuePart{v.Value, v.ValueOperatorSpan, v.ValueIsDefault, v.ValueIsInitial}, true
		}
		return valuePart{}, false
	case *ast.RequireMember:
		if v.Ident.Name == name {
			return valuePart{v.Value, v.ValueOperatorSpan, v.ValueIsDefault, v.ValueIsInitial}, true
		}
		return valuePart{}, false
	default:
		return valuePart{}, false
	}
	for _, member := range members {
		if found, ok := findMemberValuePart(member, name); ok {
			return found, true
		}
	}
	return valuePart{}, false
}

// The same operators are read on the members whose value parts are parsed apart
// from an ordinary usage: parameters, results, a requirement's subject and its
// named assumed and required constraints.
func TestFeatureValueOperatorsOnSpecialMembers(t *testing.T) {
	cases := []struct {
		name, src, param, op string
		isDefault, initial   bool
	}{
		{"parameter", "package P { action def A { in speed default = 10; } }", "speed", "default =", true, false},
		{"result", "package P { calc def C { return total default = 0; } }", "total", "default =", true, false},
		{"subject default", "package P { part vehicle; requirement def R { subject target default = vehicle; } }", "target", "default =", true, false},
		{"subject initial", "package P { part vehicle; requirement def R { subject target : Part := vehicle; } }", "target", ":=", false, true},
		{"subject binding", "package P { part vehicle; requirement def R { subject target = vehicle; } }", "target", "=", false, false},
		{"anonymous subject binding", "package P { part vehicle; requirement def R { subject = vehicle; } }", "", "=", false, false},
		{"anonymous subject initial", "package P { part vehicle; requirement def R { subject := vehicle; } }", "", ":=", false, true},
		{"anonymous subject default", "package P { part vehicle; requirement def R { subject default vehicle; } }", "", "default", true, false},
		{"anonymous subject default binding", "package P { part vehicle; requirement def R { subject default = vehicle; } }", "", "default =", true, false},
		{"anonymous subject default initial", "package P { part vehicle; requirement def R { subject default := vehicle; } }", "", "default :=", true, true},
		{"assume default", "package P { constraint c0; requirement def R { assume constraint c default = c0; } }", "c", "default =", true, false},
		{"assume binding", "package P { constraint c0; requirement def R { assume constraint c = c0; } }", "c", "=", false, false},
		{"require initial", "package P { constraint c0; requirement def R { require constraint c := c0; } }", "c", ":=", false, true},
		{"require default initial", "package P { constraint c0; requirement def R { require constraint c default := c0; } }", "c", "default :=", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(tc.src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			part, ok := findValuePart(root, tc.param)
			if !ok {
				t.Fatalf("%s not found", tc.param)
			}
			if part.value == nil || part.isDefault != tc.isDefault || part.initial != tc.initial {
				t.Errorf("value=%v default=%t initial=%t, want a value with default=%t initial=%t",
					part.value != nil, part.isDefault, part.initial, tc.isDefault, tc.initial)
			}
			if got := tc.src[part.op.Offset:part.op.End()]; got != tc.op {
				t.Errorf("value operator span %q, want %q", got, tc.op)
			}
		})
	}
}
