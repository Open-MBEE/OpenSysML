package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestAddMemberModifiers(t *testing.T) {
	t.Run("abstract definition", func(t *testing.T) {
		m := loadContent(t, "abstract.sysml", "package P;\n")
		op := AddMember("P", "part def", "AbstractPart")
		op.IsAbstract = true
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := string(res.Content); !strings.Contains(got, "abstract part def AbstractPart;") {
			t.Fatalf("abstract member not written:\n%s", got)
		}
	})

	t.Run("default value", func(t *testing.T) {
		m := loadContent(t, "default.sysml", "package P {\n    attribute existing : ScalarValues::Real;\n}\n")
		op := AddMember("P", "attribute", "weight")
		op.Type, op.Value, op.IsDefault = "ScalarValues::Real", "2.0", true
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := string(res.Content); !strings.Contains(got, "attribute weight : ScalarValues::Real default = 2.0;") {
			t.Fatalf("default value not written:\n%s", got)
		}
	})

	t.Run("unnamed redefinition", func(t *testing.T) {
		m := loadContent(t, "redefines.sysml",
			"part def Base { part original; }\npart def Derived specializes Base;\n")
		op := AddMember("Derived", "part", "")
		op.Redefines = []string{"original"}
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := string(res.Content); !strings.Contains(got, "part :>> original;") {
			t.Fatalf("unnamed redefinition not written:\n%s", got)
		}
	})

	t.Run("typed reference", func(t *testing.T) {
		m := loadContent(t, "ref.sysml", "part def Base;\npackage P;\n")
		op := AddMember("P", "ref", "reference")
		op.Type = "Base"
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := string(res.Content); !strings.Contains(got, "ref reference : Base;") {
			t.Fatalf("reference member not written:\n%s", got)
		}
	})

	t.Run("direction before result expression", func(t *testing.T) {
		const src = "calc def C { in x : ScalarValues::Real; x * 2 }\n"
		m := loadContent(t, "result.sysml", src)
		op := AddMember("C", "ref", "y")
		op.Type, op.Direction = "ScalarValues::Real", "in"
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		const want = "calc def C { in x : ScalarValues::Real; in ref y : ScalarValues::Real; x * 2 }\n"
		if got := string(res.Content); got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireClean(t, loadContent(t, "result.sysml", string(res.Content)))
	})

	t.Run("ref modifiers parse as reference usages", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			direction string
			abstract  bool
			want      string
		}{
			{name: "directional", direction: "in", want: "in ref x : T;"},
			{name: "undirected", want: "ref x : T;"},
			{name: "directional abstract", direction: "in", abstract: true, want: "in abstract ref x : T;"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m := loadContent(t, "ref-modifier.sysml", "part def T;\npackage P;\n")
				op := AddMember("P", "ref", "x")
				op.Type, op.Direction, op.IsAbstract = "T", tc.direction, tc.abstract
				res, err := Apply(m, []Operation{op})
				if err != nil {
					t.Fatalf("Apply: %v", err)
				}
				if got := string(res.Content); !strings.Contains(got, tc.want) {
					t.Fatalf("reference member not written as %q:\n%s", tc.want, got)
				}
				parsed := loadContent(t, "ref-modifier.sysml", string(res.Content))
				matches := parsed.Index.LookupQualified("P::x")
				if len(matches) != 1 {
					t.Fatalf("lookup P::x = %d symbols, want 1", len(matches))
				}
				usage, ok := matches[0].Decl.(*ast.Usage)
				if !ok || !usage.IsReference {
					t.Fatalf("P::x declaration = %#v, want a reference usage", matches[0].Decl)
				}
				requireClean(t, parsed)
			})
		}
	})

	t.Run("member before full-line result comments", func(t *testing.T) {
		for _, comment := range []string{"// final answer", "/* note */"} {
			t.Run(comment, func(t *testing.T) {
				const src = "calc def C {\n  in x : ScalarValues::Real;\n"
				m := loadContent(t, "result-comment.sysml", src+comment+"\n  x * 2\n}\n")
				op := AddMember("C", "ref", "y")
				op.Type, op.Direction = "ScalarValues::Real", "in"
				res, err := Apply(m, []Operation{op})
				if err != nil {
					t.Fatalf("Apply: %v", err)
				}
				want := src + "  in ref y : ScalarValues::Real;\n" + comment + "\n  x * 2\n}\n"
				if got := string(res.Content); got != want {
					t.Fatalf("content = %q, want %q", got, want)
				}
				requireClean(t, loadContent(t, "result-comment.sysml", string(res.Content)))
			})
		}
	})

	t.Run("trailing prior-member comment stays with member", func(t *testing.T) {
		const src = "calc def C {\n  in x : ScalarValues::Real; // input\n  x * 2\n}\n"
		m := loadContent(t, "result-comment.sysml", src)
		op := AddMember("C", "ref", "y")
		op.Type, op.Direction = "ScalarValues::Real", "in"
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		const want = "calc def C {\n  in x : ScalarValues::Real; // input\n" +
			"  in ref y : ScalarValues::Real;\n  x * 2\n}\n"
		if got := string(res.Content); got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireClean(t, loadContent(t, "result-comment.sysml", string(res.Content)))
	})

	t.Run("direction before constraint result expression", func(t *testing.T) {
		const src = "constraint def K { in x : ScalarValues::Real; x > 0 }\n"
		m := loadContent(t, "constraint-result.sysml", src)
		op := AddMember("K", "ref", "y")
		op.Type, op.Direction = "ScalarValues::Real", "in"
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		const want = "constraint def K { in x : ScalarValues::Real; in ref y : ScalarValues::Real; x > 0 }\n"
		if got := string(res.Content); got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireClean(t, loadContent(t, "constraint-result.sysml", string(res.Content)))
	})

	t.Run("return parameter", func(t *testing.T) {
		m := loadContent(t, "return.sysml", "calc def C { in x : ScalarValues::Real; }\n")
		op := AddMember("C", "return", "result")
		op.Type = "ScalarValues::Real"
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := string(res.Content); !strings.Contains(got, "return result : ScalarValues::Real;") {
			t.Fatalf("return parameter not written:\n%s", got)
		}
		requireClean(t, loadContent(t, "return.sysml", string(res.Content)))
	})

	t.Run("return parameter in constraint", func(t *testing.T) {
		m := loadContent(t, "constraint-return.sysml", "constraint def K { true }\n")
		op := AddMember("K", "return", "r")
		op.Type = "ScalarValues::Boolean"
		res, err := Apply(m, []Operation{op})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := string(res.Content); !strings.Contains(got, "return r : ScalarValues::Boolean;") {
			t.Fatalf("return parameter not written:\n%s", got)
		}
		requireClean(t, loadContent(t, "constraint-return.sysml", string(res.Content)))
	})
}

func TestAddMemberModifierRefusals(t *testing.T) {
	tests := []struct {
		name string
		m    Model
		op   Operation
		want Failure
	}{
		{
			name: "empty name without redefines",
			m:    loadContent(t, "empty.sysml", "package P;\n"),
			op:   AddMember("P", "part", ""),
			want: FailureInvalidName,
		},
		{
			name: "abstract package",
			m: func() Model {
				return loadContent(t, "package.sysml", "package P;\n")
			}(),
			op: func() Operation {
				op := AddMember("", "package", "Q")
				op.IsAbstract = true
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "abstract enumeration definition",
			m:    loadContent(t, "enum.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "enum def", "E")
				op.IsAbstract = true
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "abstract metadata usage",
			m:    loadContent(t, "metadata.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "metadata", "m")
				op.IsAbstract = true
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "direction on metadata usage",
			m:    loadContent(t, "metadata.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "metadata", "m")
				op.Direction = "in"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "default on metadata usage",
			m:    loadContent(t, "metadata.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "metadata", "m")
				op.IsDefault = true
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "redefines on metadata usage",
			m:    loadContent(t, "metadata.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "metadata", "m")
				op.Redefines = []string{"Base"}
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "value on metadata usage",
			m:    loadContent(t, "metadata.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "metadata", "m")
				op.Value = "true"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "abstract subject",
			m:    loadContent(t, "subject.sysml", "requirement def R;\n"),
			op: func() Operation {
				op := AddMember("R", "subject", "s")
				op.IsAbstract = true
				op.Type = "Thing"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "direction on definition",
			m:    loadContent(t, "direction.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "part def", "D")
				op.Direction = "in"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "invalid direction",
			m:    loadContent(t, "direction.sysml", "part def Base;\npackage P;\n"),
			op: func() Operation {
				op := AddMember("P", "part", "p")
				op.Direction, op.Type = "sideways", "Base"
				return op
			}(),
			want: FailureInvalidValue,
		},
		{
			name: "default without value",
			m:    loadContent(t, "default.sysml", "package P;\n"),
			op: func() Operation {
				op := AddMember("P", "attribute", "a")
				op.IsDefault = true
				return op
			}(),
			want: FailureInvalidValue,
		},
		{
			name: "redefines on definition",
			m:    loadContent(t, "redefines.sysml", "part def Base;\n"),
			op: func() Operation {
				op := AddMember("", "part def", "Child")
				op.Redefines = []string{"Base"}
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "return in an action",
			m:    loadContent(t, "action.sysml", "action def A;\n"),
			op: func() Operation {
				op := AddMember("A", "return", "result")
				op.Type = "Real"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "return in a part",
			m:    loadContent(t, "part.sysml", "part def P;\n"),
			op: func() Operation {
				op := AddMember("P", "return", "result")
				op.Type = "Real"
				return op
			}(),
			want: FailureIllegalKind,
		},
		{
			name: "return in a requirement",
			m:    loadContent(t, "requirement.sysml", "requirement def R;\n"),
			op: func() Operation {
				op := AddMember("R", "return", "result")
				op.Type = "Real"
				return op
			}(),
			want: FailureIllegalKind,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addFailure(t, tc.m, tc.op, tc.want)
		})
	}
}

func TestAddMemberRejectsDuplicateReturn(t *testing.T) {
	for _, tc := range []struct {
		name, owner, memberName, memberType, content string
	}{
		{
			name:       "calculation definition",
			owner:      "C",
			memberName: "next",
			memberType: "ScalarValues::Real",
			content:    "calc def C { return previous : ScalarValues::Real; }\n",
		},
		{
			name:       "constraint definition",
			owner:      "K",
			memberName: "second",
			memberType: "ScalarValues::Boolean",
			content:    "constraint def K { return original : ScalarValues::Boolean; }\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "duplicate-return.sysml", tc.content)
			op := AddMember(tc.owner, "return", tc.memberName)
			op.Type = tc.memberType
			editErr := addFailure(t, m, op, FailureIllegalKind)
			if editErr.Message != "a calculation, constraint or case body already has a return parameter" {
				t.Fatalf("error = %q, want duplicate return refusal", editErr.Message)
			}
		})
	}
}
