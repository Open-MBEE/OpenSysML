package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const impliedExpressionModel = `package P {
	datatype D;
	function G { in a : D; in b : D; return r : D; }
	struct S { feature px : D; feature py : D; }
	class E {
		feature d1 : D;
		feature d2 : D;
		feature named : D = G(b = d1, a = d2);
		feature positional : D = G(d1, d2);
		feature cast : D = d1 as D;
		feature tested = d1 istype D;
		feature selfTested = @D;
		feature sequence = (d1, d2);
		feature unresolved : D = Missing(d1);
		feature built : S = new S(py = d1);
		feature selected = d1.?{in v : D; true};
	}
}`

// expressionOf is the value expression of the feature named fqn.
func expressionOf(t *testing.T, m *Model, root *symbols.Scope, fqn string) *symbols.Symbol {
	t.Helper()
	owned, _ := m.ReflectiveElements(auditLibraryBases(t, root, fqn), "ownedElement")
	for _, element := range owned {
		if element.Kind == symbols.SymbolExpression {
			return element
		}
	}
	t.Fatalf("%s owns no expression", fqn)
	return nil
}

func fqnList(m *Model, syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, m.resolver.Index().GetFQN(sym))
	}
	return out
}

func TestExpressionParametersRedefineTheFunctionParameters(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "a.kerml", source.KindKerML, impliedExpressionModel)
	cases := []struct {
		feature string
		params  [][]string
		result  []string
	}{
		{"P::E::positional", [][]string{{"P::G::a"}, {"P::G::b"}}, []string{"Performances::Evaluation::result", "P::G::r"}},
		// A named argument's parameter redefines the parameter it names, not the one at its position.
		{"P::E::named", [][]string{{"P::G::b"}, {"P::G::a"}}, []string{"Performances::Evaluation::result", "P::G::r"}},
		{"P::E::cast", [][]string{{"BaseFunctions::as::seq"}}, []string{"Performances::Evaluation::result", "BaseFunctions::as::"}},
		{"P::E::sequence", [][]string{{"BaseFunctions::,::seq1"}, {"BaseFunctions::,::seq2"}}, []string{"Performances::Evaluation::result", "BaseFunctions::,::"}},
		// A result redefines the result of each general of its expression, as the pilot states both.
		// What names no function that resolves redefines the parameters of no function.
		{"P::E::unresolved", [][]string{nil}, []string{"Performances::Evaluation::result"}},
	}
	for _, tc := range cases {
		expr := expressionOf(t, m, root, tc.feature)
		params := m.ExpressionParameters(expr)
		if len(params) < len(tc.params) {
			t.Errorf("%s: %d parameters, want at least %d", tc.feature, len(params), len(tc.params))
			continue
		}
		for i, want := range tc.params {
			got := fqnList(m, m.ImpliedRedefinitions(params[i]))
			if !slices.Equal(got, want) && (len(got) > 0 || len(want) > 0) {
				t.Errorf("%s parameter %d redefines %v, want %v", tc.feature, i, got, want)
			}
			// Inheritance and resolution read the same redefinitions.
			if redefined := fqnList(m, m.DirectRedefinedFeatures(params[i])); !slices.Equal(redefined, got) {
				t.Errorf("%s parameter %d: DirectRedefinedFeatures %v, ImpliedRedefinitions %v", tc.feature, i, redefined, got)
			}
			for _, target := range m.ImpliedRedefinitions(params[i]) {
				if !slices.Contains(m.DirectSupertypes(params[i]), target) {
					t.Errorf("%s parameter %d: %s is no direct supertype", tc.feature, i, m.resolver.Index().GetFQN(target))
				}
			}
		}
		if got := fqnList(m, m.ImpliedRedefinitions(m.ExpressionResult(expr))); !slices.Equal(got, tc.result) {
			t.Errorf("%s result redefines %v, want %v", tc.feature, got, tc.result)
		}
	}
}

func TestExpressionFunctionTypesTheExpression(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "a.kerml", source.KindKerML, impliedExpressionModel)
	for feature, want := range map[string]string{
		"P::E::positional": "P::G",
		"P::E::cast":       "BaseFunctions::as",
		"P::E::sequence":   "BaseFunctions::,",
		"P::E::selected":   "ControlFunctions::select",
		"P::E::unresolved": "",
		"P::E::built":      "",
	} {
		expr := expressionOf(t, m, root, feature)
		function := m.ExpressionFunction(expr)
		if got := m.resolver.Index().GetFQN(function); (function == nil) != (want == "") || (function != nil && got != want) {
			t.Errorf("%s is typed by %q, want %q", feature, got, want)
		}
		if function != nil && !slices.Contains(m.DirectSupertypes(expr), function) {
			t.Errorf("%s: its function is no direct supertype", feature)
		}
	}
}

func TestConstructorResultFeatureRedefinesTheInstantiatedTypeFeature(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "a.kerml", source.KindKerML, impliedExpressionModel)
	result := m.ExpressionResult(expressionOf(t, m, root, "P::E::built"))
	if got := fqnList(m, m.DirectSupertypes(result)); !slices.Contains(got, "P::S") {
		t.Errorf("the constructor result specializes %v, want P::S among them", got)
	}
	features := m.constructorResultFeatures(result)
	if len(features) != 1 {
		t.Fatalf("the constructor result owns %d features, want 1", len(features))
	}
	if got := fqnList(m, m.ImpliedRedefinitions(features[0])); !slices.Equal(got, []string{"P::S::py"}) {
		t.Errorf("the named argument feature redefines %v, want [P::S::py]", got)
	}
	if got := m.DerivedFeaturingTypes(features[0]); !slices.Equal(got, []FeaturingType{{Type: result}}) {
		t.Errorf("the argument feature is featured by %v, want the result alone", got)
	}
}

// The type argument of a classification is the function's second parameter and is typed by the type
// written, whether or not the operand it tests is written.
func TestTypeArgumentRedefinesTheTypeParameter(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "a.kerml", source.KindKerML, impliedExpressionModel)
	for feature, function := range map[string]string{"P::E::tested": "istype", "P::E::selfTested": "@"} {
		params := m.ExpressionParameters(expressionOf(t, m, root, feature))
		argument := params[len(params)-1]
		want := "BaseFunctions::" + function + "::type"
		if got := fqnList(m, m.ImpliedRedefinitions(argument)); !slices.Equal(got, []string{want}) {
			t.Errorf("%s: the type argument redefines %v, want %s", feature, got, want)
		}
		if got := fqnList(m, m.DirectSupertypes(argument)); len(got) == 0 || got[0] != "P::D" {
			t.Errorf("%s: the type argument specializes %v, want P::D first", feature, got)
		}
	}
}
