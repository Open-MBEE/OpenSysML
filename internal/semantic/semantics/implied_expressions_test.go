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
	function O { out o : D; in i : D; return r : D; }
	struct S { feature px : D; feature py : D; }
	class E {
		feature d1 : D;
		feature d2 : D;
		feature named : D = G(b = d1, a = d2);
		feature qualified : D = G(G::b = d1, G::a = d2);
		feature positional : D = G(d1, d2);
		feature outFirst : D = O(d1);
		feature cast : D = d1 as D;
		feature tested = d1 istype D;
		feature selfTested = @D;
		feature sequence = (d1, d2);
		feature unresolved : D = Missing(d1);
		feature unresolvedNamed : D = Missing(G::b = d1);
		feature g : G;
		feature viaFeature : D = g(d1, d2);
		feature viaFeatureNamed : D = g(b = d1, a = d2);
		feature foreign : D = O(G::b = d1);
		feature built : S = new S(py = d1);
		feature selected = d1.?{in v : D; true};
		feature list : Collections::OrderedCollection;
		feature indexed = d1#(1);
		feature element = list#(1);
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
		written bool // the notation states the redefinitions: none is implied
		params  [][]string
		result  []string
	}{
		{"P::E::positional", false, [][]string{{"P::G::a"}, {"P::G::b"}}, []string{"Performances::Evaluation::result", "P::G::r"}},
		// A named argument's parameter redefines the parameter it names, not the one at its position.
		{"P::E::named", true, [][]string{{"P::G::b"}, {"P::G::a"}}, []string{"Performances::Evaluation::result", "P::G::r"}},
		// A qualified name is resolved from where the call stands.
		{"P::E::qualified", true, [][]string{{"P::G::b"}, {"P::G::a"}}, []string{"Performances::Evaluation::result", "P::G::r"}},
		// A positional argument binds the input parameters in order, past an output before them.
		{"P::E::outFirst", false, [][]string{{"P::O::i"}}, []string{"Performances::Evaluation::result", "P::O::r"}},
		{"P::E::cast", false, [][]string{{"BaseFunctions::as::seq"}}, []string{"Performances::Evaluation::result", "BaseFunctions::as::"}},
		{"P::E::sequence", false, [][]string{{"BaseFunctions::,::seq1"}, {"BaseFunctions::,::seq2"}}, []string{"Performances::Evaluation::result", "BaseFunctions::,::"}},
		// What names no function that resolves redefines the parameters of no function.
		{"P::E::unresolved", false, [][]string{nil}, []string{"Performances::Evaluation::result"}},
		// A name that is no parameter of what is invoked binds nothing, however it resolves.
		{"P::E::unresolvedNamed", true, [][]string{nil}, []string{"Performances::Evaluation::result"}},
		// A feature typed by a function is invoked through the parameters it inherits.
		{"P::E::viaFeature", false, [][]string{{"P::G::a"}, {"P::G::b"}}, []string{"P::G::r"}},
		{"P::E::viaFeatureNamed", true, [][]string{{"P::G::b"}, {"P::G::a"}}, []string{"P::G::r"}},
		{"P::E::foreign", true, [][]string{nil}, []string{"Performances::Evaluation::result", "P::O::r"}},
	}
	for _, tc := range cases {
		expr := expressionOf(t, m, root, tc.feature)
		params := m.ExpressionParameters(expr)
		if len(params) < len(tc.params) {
			t.Errorf("%s: %d parameters, want at least %d", tc.feature, len(params), len(tc.params))
			continue
		}
		for i, want := range tc.params {
			got := fqnList(m, m.DirectRedefinedFeatures(params[i]))
			if !slices.Equal(got, want) && (len(got) > 0 || len(want) > 0) {
				t.Errorf("%s parameter %d redefines %v, want %v", tc.feature, i, got, want)
			}
			// What is implied is what inheritance and resolution read, less what is written.
			implied := fqnList(m, m.ImpliedRedefinitions(params[i]))
			if tc.written && len(implied) > 0 {
				t.Errorf("%s parameter %d: the written redefinition is implied too: %v", tc.feature, i, implied)
			}
			if !tc.written && !slices.Equal(implied, got) {
				t.Errorf("%s parameter %d: DirectRedefinedFeatures %v, ImpliedRedefinitions %v", tc.feature, i, got, implied)
			}
			for _, target := range m.DirectRedefinedFeatures(params[i]) {
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
	if got := fqnList(m, m.DirectRedefinedFeatures(features[0])); !slices.Equal(got, []string{"P::S::py"}) {
		t.Errorf("the named argument feature redefines %v, want [P::S::py]", got)
	}
	if got := fqnList(m, m.ImpliedRedefinitions(features[0])); len(got) > 0 {
		t.Errorf("the redefinition a named argument writes is implied too: %v", got)
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

func TestIndexResultSpecializesTheSequenceUnlessACollection(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "a.kerml", source.KindKerML, impliedExpressionModel)
	for feature, want := range map[string]bool{"P::E::indexed": true, "P::E::element": false} {
		expr := expressionOf(t, m, root, feature)
		arguments := m.ExpressionArguments(expr)
		if len(arguments) == 0 {
			t.Fatalf("%s: the index expression has no argument", feature)
		}
		sequence := m.ExpressionResult(arguments[0])
		result := m.ExpressionResult(expr)
		if got := slices.Contains(m.ImpliedSpecializations(result), sequence); got != want {
			t.Errorf("%s: the result specializes its sequence: %t, want %t", feature, got, want)
		}
		if got := slices.Contains(m.DirectSupertypes(result), sequence); got != want {
			t.Errorf("%s: the sequence is a direct supertype of the result: %t, want %t", feature, got, want)
		}
	}
}

// TestNamedArgumentRedefinitionIsReflected covers the written redefinition of a named argument:
// reflection states it for an invocation, and for no positional or unbound argument.
func TestNamedArgumentRedefinitionIsReflected(t *testing.T) {
	m, root := buildModelWithLibraryNamedKind(t, "a.kerml", source.KindKerML, impliedExpressionModel)
	cases := []struct {
		feature string
		want    []string
	}{
		{"P::E::named", []string{"P::G::b", "P::G::a"}},
		{"P::E::qualified", []string{"P::G::b", "P::G::a"}},
		{"P::E::viaFeatureNamed", []string{"P::G::b", "P::G::a"}},
		{"P::E::positional", []string{"", ""}},
		{"P::E::foreign", []string{""}},
	}
	for _, tc := range cases {
		params := m.ExpressionParameters(expressionOf(t, m, root, tc.feature))
		if len(params) < len(tc.want) {
			t.Errorf("%s: %d parameters, want at least %d", tc.feature, len(params), len(tc.want))
			continue
		}
		for i, want := range tc.want {
			var got []string
			for _, rel := range m.ImplicitRelationships(params[i]) {
				if rel.Implicit.Form == symbols.ImplicitParameterRedefinition {
					got = append(got, m.resolver.Index().GetFQN(rel.Implicit.Member))
				}
			}
			if want == "" && len(got) > 0 || want != "" && !slices.Equal(got, []string{want}) {
				t.Errorf("%s parameter %d reflects the redefinitions %v, want %q", tc.feature, i, got, want)
			}
		}
	}
}
