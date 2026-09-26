package edit

import (
	"strings"
	"testing"
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
		const want = "calc def C { in x : ScalarValues::Real; in y : ScalarValues::Real; x * 2 }\n"
		if got := string(res.Content); got != want {
			t.Fatalf("content = %q, want %q", got, want)
		}
		requireClean(t, loadContent(t, "result.sysml", string(res.Content)))
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
		{
			name: "second return",
			m:    loadContent(t, "result.sysml", "calc def C { return previous : Real; }\n"),
			op: func() Operation {
				op := AddMember("C", "return", "next")
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
