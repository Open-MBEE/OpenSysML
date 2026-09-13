package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// classificationModel annotates elements in each form the parser accepts —
// prefix metadata, an annotation body, a metadata usage, and an external
// `about` — plus a metadata type specializing the classifying one.
const classificationModel = `
	metadata def Safety {
		attribute level;
	}
	metadata def CrashSafety :> Safety;
	metadata def Comfort;

	#Safety part def Belt;
	part seatBelt { @Safety{level = 3;} }
	part airBag { metadata crash : CrashSafety { level = 5; } }
	part radio;
	metadata comfort : Comfort about radio;
	part keylessEntry;
	part def SeatBeltKind :> Belt;
`

// constraintVerdict evaluates the named constraint of src, bound to no object.
func constraintVerdict(t *testing.T, src, name string) (bool, error) {
	t.Helper()
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 10000)
	return ctx.EvaluateConstraint(resolveSymbol(t, root, name), root)
}

// A classification with an explicit subject holds for the element the subject
// names, in each form the annotation can be written in.
func TestEvalClassificationOverNamedSubjects(t *testing.T) {
	for _, tc := range []struct {
		cond string
		want bool
	}{
		{"Belt @ Safety", true},
		{"seatBelt @ Safety", true},
		{"airBag @ Safety", true},
		{"radio @ Comfort", true},
		{"radio @ Safety", false},
		{"keylessEntry @ Safety", false},
		{"SeatBeltKind @ Safety", false},
	} {
		src := classificationModel + "\nconstraint c { " + tc.cond + " }"
		got, err := constraintVerdict(t, src, "c")
		if tc.want {
			if err != nil || !got {
				t.Errorf("`%s` = %v err=%v, want true", tc.cond, got, err)
			}
			continue
		}
		if got || !errors.Is(err, ErrViolated) {
			t.Errorf("`%s` = %v err=%v, want a violated assertion", tc.cond, got, err)
		}
	}
}

// `@@` classifies an element by its own metaclass, so metadata stated about the
// element does not satisfy it — the difference from `@`.
func TestEvalClassificationMetaVersusAnnotation(t *testing.T) {
	const src = `
		metadata def Safety;
		metadata def PartDefinition;
		#Safety part def Belt;
		constraint annotated { Belt @ Safety }
		constraint metaAnnotated { Belt @@ Safety }
		constraint metaclass { Belt @@ PartDefinition }
	`
	if got, err := constraintVerdict(t, src, "annotated"); err != nil || !got {
		t.Fatalf("`Belt @ Safety` = %v err=%v, want true", got, err)
	}
	if got, err := constraintVerdict(t, src, "metaAnnotated"); got || !errors.Is(err, ErrViolated) {
		t.Fatalf("`Belt @@ Safety` = %v err=%v, want a violated assertion: metadata is not the metaclass", got, err)
	}
	if got, err := constraintVerdict(t, src, "metaclass"); err != nil || !got {
		t.Fatalf("`Belt @@ PartDefinition` = %v err=%v, want true", got, err)
	}
}

// A classification is evaluated in a calc body as any other operator is.
func TestEvalClassificationInACalcBody(t *testing.T) {
	src := classificationModel + `
		calc def classify {
			in flag : Boolean;
			return : Boolean = flag and Belt @ Safety;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 10000)
	got, err := ctx.InvokeCalc(resolveSymbol(t, root, "classify"),
		[]Value{{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}}, root)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got.Kind != ValConst || got.Const.Kind != semantics.ValBool || !got.Const.Bool {
		t.Fatalf("classify(true) = %+v, want true", got)
	}
}

// A classification that leaves its subject out classifies the object being
// evaluated, which is the form a constraint inside an annotated definition uses.
func TestEvalClassificationOfTheObjectBeingEvaluated(t *testing.T) {
	const src = `
		metadata def Safety;
		#Safety part def Vehicle {
			constraint tagged { @Safety }
			constraint namedSelf { self @ Safety }
		}
		part def Cart {
			constraint tagged { @Safety }
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(NewModel(model, resolver), 10000)

	for _, tc := range []struct {
		typ, constraint string
		want            bool
	}{
		{"Vehicle", "tagged", true},
		{"Vehicle", "namedSelf", true},
		{"Cart", "tagged", false},
	} {
		typ := resolveSymbol(t, root, tc.typ)
		inst, err := ctx.Instantiate(typ)
		if err != nil {
			t.Fatalf("Instantiate(%s): %v", tc.typ, err)
		}
		cons, ok := typ.Scope.LookupLocal(tc.constraint)
		if !ok {
			t.Fatalf("constraint %s of %s not found", tc.constraint, tc.typ)
		}
		got, err := ctx.EvaluateConstraintOn(cons, typ.Scope, inst)
		if tc.want {
			if err != nil || !got {
				t.Errorf("%s::%s = %v err=%v, want true", tc.typ, tc.constraint, got, err)
			}
			continue
		}
		if got || !errors.Is(err, ErrViolated) {
			t.Errorf("%s::%s = %v err=%v, want a violated assertion", tc.typ, tc.constraint, got, err)
		}
	}
}

// A classification left implicit outside any object cannot be judged, and says
// so rather than answering false.
func TestEvalClassificationWithoutASubject(t *testing.T) {
	src := classificationModel + "\nconstraint c { @Safety }"
	got, err := constraintVerdict(t, src, "c")
	if !errors.Is(err, semantics.ErrFilterUnevaluable) {
		t.Fatalf("an implicit subject with no object = %v err=%v, want ErrFilterUnevaluable", got, err)
	}
	if !strings.Contains(err.Error(), "implicit") {
		t.Errorf("%v does not say the subject was left implicit", err)
	}
}

// A subject that is a datum annotates nothing, so classifying it is reported.
func TestEvalClassificationOfANonElement(t *testing.T) {
	src := classificationModel + "\nconstraint c { 42 @ Safety }"
	if got, err := constraintVerdict(t, src, "c"); !errors.Is(err, semantics.ErrFilterUnevaluable) {
		t.Fatalf("classifying a number = %v err=%v, want ErrFilterUnevaluable", got, err)
	}
}

// A metadata type that does not resolve is outside the evaluable subset and is
// reported, as it is at the model level.
func TestEvalClassificationOfAnUnresolvedType(t *testing.T) {
	src := classificationModel + "\nconstraint c { Belt @ Nonexistent }"
	if got, err := constraintVerdict(t, src, "c"); !errors.Is(err, semantics.ErrFilterUnevaluable) {
		t.Fatalf("an unresolved metadata type = %v err=%v, want ErrFilterUnevaluable", got, err)
	}
}

// TestEvalClassificationAgreesWithAnElementFilter pins the agreement between the
// two paths: the verdict the evaluator reaches for a subject is the verdict the
// same condition reaches as an element filter over that element.
func TestEvalClassificationAgreesWithAnElementFilter(t *testing.T) {
	names := []string{"Belt", "seatBelt", "airBag", "radio", "keylessEntry", "SeatBeltKind"}
	for _, cond := range []string{"@Safety", "@CrashSafety", "@Comfort", "@@Safety"} {
		for _, name := range names {
			src := classificationModel + "\nconstraint c { " + name + " " + runtimeCondSubject(cond) + " }"
			atRuntime, runtimeErr := constraintVerdict(t, src, "c")
			if runtimeErr != nil && !errors.Is(runtimeErr, ErrViolated) {
				t.Fatalf("%s %s: unexpected runtime error %v", name, cond, runtimeErr)
			}

			model, _, root := parseAndBuildModel(t, classificationModel)
			atFilter, filterErr := model.EvalElementFilter(filterCondition(t, cond, root), resolveSymbol(t, root, name))
			if filterErr != nil {
				t.Fatalf("%s %s: unexpected filter error %v", name, cond, filterErr)
			}
			if atRuntime != atFilter {
				t.Errorf("%s %s: evaluator says %v, element filter says %v", name, cond, atRuntime, atFilter)
			}
		}
	}
}

// runtimeCondSubject rewrites a filter condition (`@Safety`) into the operator
// form an expression states with an explicit subject (`@ Safety`).
func runtimeCondSubject(cond string) string {
	if strings.HasPrefix(cond, "@@") {
		return "@@ " + strings.TrimPrefix(cond, "@@")
	}
	return "@ " + strings.TrimPrefix(cond, "@")
}

// filterCondition parses cond as an element filter written in scope.
func filterCondition(t *testing.T, cond string, scope *symbols.Scope) symbols.ElementFilter {
	t.Helper()
	p := parser.New(source.New("<filter>", []byte(cond)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("failed to parse %q: %v", cond, p.Diagnostics)
	}
	if _, ok := expr.(*ast.OperatorExpr); !ok {
		t.Fatalf("%q parsed as %T, want an operator expression", cond, expr)
	}
	return symbols.ElementFilter{Expr: expr, Scope: scope, Span: expr.Span()}
}
