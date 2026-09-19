package semantics

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// classificationOf parses cond as a classification written at the document root
// of src, the way a value evaluator hands one over.
func classificationOf(t *testing.T, src, cond string) (*Model, *ast.OperatorExpr, *symbols.Scope) {
	t.Helper()
	m, root := buildModel(t, src)
	p := parser.New(source.New("<classification>", []byte(cond)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("failed to parse %q: %v", cond, p.Diagnostics)
	}
	op, ok := expr.(*ast.OperatorExpr)
	if !ok {
		t.Fatalf("%q parsed as %T, want an operator expression", cond, expr)
	}
	return m, op, root
}

// TestClassificationAgreesWithAFilterVerdict is the agreement this feature rests
// on: `@`/`@@` decided for a subject at runtime answers what the same condition
// decides as an element filter at name-resolution time.
func TestClassificationAgreesWithAFilterVerdict(t *testing.T) {
	const src = metadataModel + `
		metadata def Safety2 :> Safety;
		#Safety2 part def Harness;
		part def SeatBeltKind :> Belt;
	`
	conds := []string{"@Safety", "@CrashSafety", "@Comfort", "@@Safety", "@Safety2"}
	names := []string{"Belt", "seatBelt", "airBag", "radio", "keylessEntry", "Harness", "SeatBeltKind", "Safety"}
	for _, cond := range conds {
		for _, name := range names {
			m, op, root := classificationOf(t, src, cond)
			f := symbols.ElementFilter{Expr: op, Scope: root, Span: op.Span()}
			elem := sym(t, root, name)

			atFilter, filterErr := m.EvalElementFilter(f, elem)
			atRuntime, runtimeErr := m.EvalClassification(root, op, elem)
			if (filterErr == nil) != (runtimeErr == nil) {
				t.Fatalf("%q for %s: filter err=%v, evaluator err=%v", cond, name, filterErr, runtimeErr)
			}
			if filterErr == nil && atFilter != atRuntime {
				t.Fatalf("%q for %s: filter says %v, evaluator says %v", cond, name, atFilter, atRuntime)
			}
		}
	}
}

// `@` holds for an annotation whose metadata type specializes the classifying
// one, which is how a classification reaches an annotation through inheritance.
func TestClassificationThroughAnAnnotationSubtype(t *testing.T) {
	m, op, root := classificationOf(t, metadataModel, "@Safety")
	got, err := m.EvalClassification(root, op, sym(t, root, "airBag"))
	if err != nil || !got {
		t.Fatalf("airBag is annotated CrashSafety :> Safety, so `@Safety` should hold: %v err=%v", got, err)
	}
}

// A specializing element does not carry its supertype's metadata: an annotation
// is stated about one element (KerML 8.4.4), so specialization does not copy it.
func TestClassificationIsNotInheritedByASubtype(t *testing.T) {
	const src = metadataModel + "\npart def SeatBeltKind :> Belt;"
	m, op, root := classificationOf(t, src, "@Safety")
	got, err := m.EvalClassification(root, op, sym(t, root, "SeatBeltKind"))
	if err != nil || got {
		t.Fatalf("`@Safety` should not hold for a subtype of an annotated type: %v err=%v", got, err)
	}
}

// `@` also holds for the element's own metaclass, and `@@` holds for that alone:
// metadata stated about an element is not what the element is.
func TestClassificationMetaclassVersusAnnotation(t *testing.T) {
	const src = `
		metadata def Safety;
		metadata def PartDefinition;
		#Safety part def Belt;
	`
	for _, tc := range []struct {
		cond string
		want bool
	}{
		{"@Safety", true},
		{"@@Safety", false},
		{"@PartDefinition", true},
		{"@@PartDefinition", true},
	} {
		m, op, root := classificationOf(t, src, tc.cond)
		got, err := m.EvalClassification(root, op, sym(t, root, "Belt"))
		if err != nil {
			t.Fatalf("%q: unexpected error %v", tc.cond, err)
		}
		if got != tc.want {
			t.Errorf("%q for Belt = %v, want %v", tc.cond, got, tc.want)
		}
	}
}

// A classification the model cannot judge — a metadata type that does not
// resolve — is reported, never answered false.
func TestClassificationOutsideTheEvaluableSubset(t *testing.T) {
	m, op, root := classificationOf(t, metadataModel, "@Nonexistent")
	got, err := m.EvalClassification(root, op, sym(t, root, "Belt"))
	if !errors.Is(err, ErrFilterUnevaluable) {
		t.Fatalf("an unresolved metadata type should report ErrFilterUnevaluable, got %v err=%v", got, err)
	}
}

// A subject that denotes no element is reported the same way, so a caller cannot
// mistake "cannot judge" for "does not hold".
func TestClassificationWithoutAnElement(t *testing.T) {
	m, op, root := classificationOf(t, metadataModel, "@Safety")
	if _, err := m.EvalClassification(root, op, nil); !errors.Is(err, ErrFilterUnevaluable) {
		t.Fatalf("classifying no element should report ErrFilterUnevaluable, got %v", err)
	}
	if _, err := m.EvalClassification(root, nil, sym(t, root, "Belt")); !errors.Is(err, ErrFilterUnevaluable) {
		t.Fatalf("classifying without a condition should report ErrFilterUnevaluable, got %v", err)
	}
}

// A classification writes its subject explicitly at runtime, which a filter
// condition rejects — the same node is judged for the candidate either way.
func TestClassificationWithAnExplicitSubjectIsNotAFilterCondition(t *testing.T) {
	m, op, root := classificationOf(t, metadataModel, "seatBelt @ Safety")
	f := symbols.ElementFilter{Expr: op, Scope: root, Span: op.Span()}
	if _, err := m.EvalElementFilter(f, sym(t, root, "seatBelt")); !errors.Is(err, ErrFilterUnevaluable) {
		t.Fatalf("a filter condition takes no left operand, want ErrFilterUnevaluable, got %v", err)
	}
	got, err := m.EvalClassification(root, op, sym(t, root, "seatBelt"))
	if err != nil || !got {
		t.Fatalf("the evaluator supplies the subject, so `seatBelt @ Safety` should hold: %v err=%v", got, err)
	}
}

// A workspace that reindexes holds one element as a symbol per generation, and a
// subject reaches the evaluator from whichever generation its scope came from.
// The verdict is the element's, so it must not depend on which symbol it is.
func TestClassificationOfASubjectFromAnotherIndexGeneration(t *testing.T) {
	const src = metadataModel + "\nmetadata def Safety2 :> Safety;\n#Safety2 part def Harness;"
	_, older := buildModel(t, src)
	cases := []struct {
		name string
		cond string
		want bool
	}{
		{"Belt", "@Safety", true},
		{"seatBelt", "@Safety", true},
		{"airBag", "@Safety", true},
		{"radio", "@Comfort", true},
		{"Harness", "@Safety", true},
		{"keylessEntry", "@Safety", false},
		{"Belt", "@Comfort", false},
	}
	for _, tc := range cases {
		m, op, root := classificationOf(t, src, tc.cond)
		stale := sym(t, older, tc.name)
		if stale == sym(t, root, tc.name) {
			t.Fatalf("%s: the two generations should hold distinct symbols", tc.name)
		}
		got, err := m.EvalClassification(root, op, stale)
		if err != nil || got != tc.want {
			t.Fatalf("%q for a %s of an earlier generation = %v (err=%v), want %v", tc.cond, tc.name, got, err, tc.want)
		}
	}
}

// A body-local declaration is judged as itself: its name is unqualified, so a
// top-level element of the same name must not answer for it.
func TestClassificationOfABodyLocalNamesakeOfAnAnnotatedElement(t *testing.T) {
	const src = `
		metadata def Safety;
		#Safety part def Belt;
		calc def C {
			attribute xs : Integer[*] = (1, 2);
			attribute kept : Integer[*] = xs.?{in Belt; Belt > 1};
			return : Integer = 1;
		}
	`
	m, op, root := classificationOf(t, src, "@Safety")
	local := bodyLocalMember(t, sym(t, root, "C"), "Belt")
	got, err := m.EvalClassification(root, op, local)
	if err != nil {
		t.Fatalf("EvalClassification of a body-local Belt: unexpected error %v", err)
	}
	if got {
		t.Fatalf("a body-local Belt is @Safety, so it was judged as the annotated top-level Belt")
	}
}

// bodyLocalMember is the member named name of a body scope under owner.
func bodyLocalMember(t *testing.T, owner *symbols.Symbol, name string) *symbols.Symbol {
	t.Helper()
	if owner.Scope == nil {
		t.Fatalf("%s declares no scope", owner.Name)
	}
	for _, scope := range owner.Scope.Children() {
		if !scope.BodyLocal() {
			continue
		}
		if member, ok := scope.LookupLocal(name); ok {
			return member
		}
	}
	t.Fatalf("no body-local %q under %s", name, owner.Name)
	return nil
}
