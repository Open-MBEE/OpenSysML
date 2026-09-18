package semantics_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestFilterProblemKindFollowsPilotReferentRule(t *testing.T) {
	tests := []struct {
		name string
		src  string
		cond string
		want semantics.FilterProblemKind
	}{
		{
			name: "metaclass-owned Boolean chain is evaluator unsupported",
			src: `package R {
				private import ScalarValues::*;
				metaclass M { var feature a : Boolean[1]; }
				metaclass N { var feature m : M[1]; }
			}`,
			cond: "R::N::m.a",
			want: semantics.FilterProblemUnsupported,
		},
		{
			name: "metaclass-owned integer chain is not Boolean",
			src: `package R {
				private import ScalarValues::*;
				metaclass M { var feature a : Integer[1]; }
				metaclass N { var feature m : M[1]; }
			}`,
			cond: "R::N::m.a",
			want: semantics.FilterProblemNotBoolean,
		},
		{
			name: "comparison keeps Boolean result",
			src: `package R {
				private import ScalarValues::*;
				metaclass M { var feature a : Integer[1]; }
				metaclass N { var feature m : M[1]; }
			}`,
			cond: "R::N::m.a > 2",
			want: semantics.FilterProblemUnsupported,
		},
		{
			name: "user struct feature is not model-level evaluable",
			src: `package R {
				private import ScalarValues::*;
				struct S { feature x : Boolean[1]; }
				struct T { feature s : S[1]; }
			}`,
			cond: "R::T::s.x",
			want: semantics.FilterProblemNotEvaluable,
		},
		{
			name: "package feature follows its value rule",
			src: `package R {
				private import ScalarValues::*;
				metaclass M { var feature a : Boolean[1]; }
				feature p : M[1];
			}`,
			cond: "R::p.a",
			want: semantics.FilterProblemUnsupported,
		},
		{
			name: "unresolved bare reference is not Boolean",
			src:  `package R {}`,
			cond: "Undefined",
			want: semantics.FilterProblemNotBoolean,
		},
		{
			name: "unresolved feature chain is not Boolean",
			src:  `package R {}`,
			cond: "a.b.c",
			want: semantics.FilterProblemNotBoolean,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, f := pilotFilter(t, tc.src, tc.cond)
			problems := m.CheckElementFilter(f)
			if len(problems) != 1 || problems[0].Kind != tc.want {
				t.Fatalf("problems = %#v, want one kind %v", problems, tc.want)
			}
		})
	}
}

func pilotFilter(t *testing.T, src, cond string) (*semantics.Model, symbols.ElementFilter) {
	t.Helper()
	const name = "t.kerml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocumentWithKind(name, root, source.KindKerML)
	r := resolve.New(idx)
	m := semantics.NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	exprParser := parser.New(source.New("<filter>", []byte(cond)))
	expr := exprParser.ParseExpression()
	if expr == nil || len(exprParser.Diagnostics) != 0 {
		t.Fatalf("failed to parse filter condition %q: %v", cond, exprParser.Diagnostics)
	}
	return m, symbols.ElementFilter{Expr: expr, Scope: idx.DocumentRoot(name), Span: expr.Span()}
}
