package export

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The ids the reader answers for a feature's value expression and for the
// FeatureValue that owns it are the ones the conversion writes for them.
func TestElementIDReaderSpellsTheFeatureValueAsTheConversionWritesIt(t *testing.T) {
	sf, index, doc := readerIndex(t, "valued.sysml", "package P { part def R { attribute mass = 3; } }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	var expression, membership string
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			switch class.Value {
			case rdf.SysML + "LiteralInteger":
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					expression = id.Value
				}
			case rdf.SysML + "FeatureValue":
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					membership = id.Value
				}
			}
		}
	}
	if expression == "" || membership == "" {
		t.Fatalf("the conversion writes expression %q, FeatureValue %q; want both", expression, membership)
	}

	matches := index.LookupQualified("P::R::mass")
	if len(matches) != 1 {
		t.Fatalf("P::R::mass names %d declarations, want 1", len(matches))
	}
	usage, ok := matches[0].Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("P::R::mass declares %T, want a valued usage", matches[0].Decl)
	}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	if got, err := reader.Read(matches[0].DocName, usage.Value, ""); err != nil || got != expression {
		t.Errorf("Read value expression = %q, %v; want %q", got, err, expression)
	}
	if got, err := reader.ReadOwningMembership(matches[0].DocName, usage.Value, ""); err != nil || got != membership {
		t.Errorf("ReadOwningMembership value expression = %q, %v; want %q", got, err, membership)
	}
}

// The ids the reader answers for an expression's synthesized parameters (`in0`…,
// `out`), their memberships, and for its operands and their FeatureValues are
// the ones the conversion writes for them.
func TestElementIDReaderSpellsExpressionPartsAsTheConversionWritesThem(t *testing.T) {
	sf, index, doc := readerIndex(t, "operands.sysml", "package P { part def R { attribute a : ScalarValues::Real; attribute mass = a + 2; } }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	ids := func(subject rdf.Term) string {
		for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
			return id.Value
		}
		return ""
	}
	written := map[string]map[string]bool{} // class → elementIds
	direction := map[string]string{}        // parameter elementId → direction
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			name := class.Value[len(rdf.SysML):]
			switch name {
			case "ParameterMembership", "ReturnParameterMembership", "FeatureValue", "LiteralInteger", "FeatureReferenceExpression":
				if written[name] == nil {
					written[name] = map[string]bool{}
				}
				written[name][ids(subject)] = true
			case "Feature":
				for _, d := range graph.Objects(subject, rdf.SysML+"direction") {
					direction[ids(subject)] = d.Value
				}
			}
		}
	}
	// `a + 2` and the reference `a` each yield a result; the literal does not.
	if len(written["ParameterMembership"]) != 2 || len(written["ReturnParameterMembership"]) != 2 || len(written["FeatureValue"]) != 3 {
		t.Fatalf("the conversion writes %d ParameterMemberships, %d ReturnParameterMemberships, %d FeatureValues; want 2, 2, 3", len(written["ParameterMembership"]), len(written["ReturnParameterMembership"]), len(written["FeatureValue"]))
	}

	matches := index.LookupQualified("P::R::mass")
	if len(matches) != 1 {
		t.Fatalf("P::R::mass names %d declarations, want 1", len(matches))
	}
	usage := matches[0].Decl.(*ast.Usage)
	parts, typeArgument := ast.ExpressionOperands(usage.Value)
	if len(parts) != 2 || typeArgument >= 0 {
		t.Fatalf("a + 2 writes %d operands (type argument %d), want 2, none", len(parts), typeArgument)
	}
	operands := []ast.Node{parts[0].Node, parts[1].Node}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	for slot, want := range map[string]struct{ membership, direction string }{
		"in0": {"ParameterMembership", "in"},
		"in1": {"ParameterMembership", "in"},
		"out": {"ReturnParameterMembership", "out"},
	} {
		element, membership, err := reader.ReadExpressionPart(matches[0].DocName, usage.Value, slot)
		if err != nil || element == "" || membership == "" {
			t.Errorf("ReadExpressionPart %s = %q, %q, %v", slot, element, membership, err)
			continue
		}
		if direction[element] != want.direction {
			t.Errorf("%s: the conversion writes direction %q for %q, want %q", slot, direction[element], element, want.direction)
		}
		if !written[want.membership][membership] {
			t.Errorf("%s: the conversion writes no %s %q", slot, want.membership, membership)
		}
	}
	for i, class := range []string{"FeatureReferenceExpression", "LiteralInteger"} {
		element, err := reader.Read(matches[0].DocName, operands[i], "")
		if err != nil || !written[class][element] {
			t.Errorf("Read operand %d = %q, %v; the conversion writes %s %v", i, element, err, class, written[class])
		}
		membership, err := reader.ReadOwningMembership(matches[0].DocName, operands[i], "")
		if err != nil || !written["FeatureValue"][membership] {
			t.Errorf("ReadOwningMembership operand %d = %q, %v; the conversion writes FeatureValues %v", i, membership, err, written["FeatureValue"])
		}
	}
	if element, membership, err := reader.ReadExpressionPart(matches[0].DocName, operands[0], "out"); err != nil || direction[element] != "out" || !written["ReturnParameterMembership"][membership] {
		t.Errorf("ReadExpressionPart reference out = %q, %q, %v; want the written result parameter", element, membership, err)
	}
	if _, _, err := reader.ReadExpressionPart(matches[0].DocName, operands[1], "out"); err != nil {
		t.Errorf("ReadExpressionPart literal out: %v", err)
	} else if element, _, _ := reader.ReadExpressionPart(matches[0].DocName, operands[1], "out"); element != "" {
		t.Errorf("a literal has a result parameter %q", element)
	}
}

// An invocation's receiver, positional and named arguments read the ids the
// conversion writes for them (`operand`, `a%d`, `n%d`), with their FeatureValues
// and `in` parameters.
func TestElementIDReaderSpellsInvocationArgumentsAsTheConversionWritesThem(t *testing.T) {
	sf, index, doc := readerIndex(t, "invocations.sysml", `package P {
	calc def F { in a : ScalarValues::Real; in b : ScalarValues::Real; return r : ScalarValues::Real = a + b; }
	part def R {
		attribute items : ScalarValues::Real[*];
		attribute arrow = items->F(3);
		attribute named = F(a = 1, b = 2);
	}
}
`)
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	written := map[string]map[string]bool{}
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			name := class.Value[len(rdf.SysML):]
			for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
				if written[name] == nil {
					written[name] = map[string]bool{}
				}
				written[name][id.Value] = true
			}
		}
	}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	for _, c := range []struct {
		feature string
		classes []string
	}{
		{"P::R::arrow", []string{"FeatureReferenceExpression", "LiteralInteger"}},
		{"P::R::named", []string{"LiteralInteger", "LiteralInteger"}},
	} {
		matches := index.LookupQualified(c.feature)
		if len(matches) != 1 {
			t.Fatalf("%s names %d declarations", c.feature, len(matches))
		}
		usage := matches[0].Decl.(*ast.Usage)
		operands, _ := ast.ExpressionOperands(usage.Value)
		if len(operands) != len(c.classes) {
			t.Fatalf("%s: %d operands, want %d", c.feature, len(operands), len(c.classes))
		}
		for i, operand := range operands {
			element, err := reader.Read(matches[0].DocName, operand.Node, "")
			if err != nil || !written[c.classes[i]][element] {
				t.Errorf("%s operand %d (%s) = %q, %v; the conversion writes %s %v", c.feature, i, operand.Slot, element, err, c.classes[i], written[c.classes[i]])
			}
			membership, err := reader.ReadOwningMembership(matches[0].DocName, operand.Node, "")
			if err != nil || !written["FeatureValue"][membership] {
				t.Errorf("%s operand %d owning membership = %q, %v; want a written FeatureValue", c.feature, i, membership, err)
			}
			parameter, parameterMembership, err := reader.ReadExpressionPart(matches[0].DocName, usage.Value, fmt.Sprintf("in%d", operand.Index))
			if err != nil || !written["Feature"][parameter] || !written["ParameterMembership"][parameterMembership] {
				t.Errorf("%s in%d = %q, %q, %v; want the written parameter and its membership", c.feature, operand.Index, parameter, parameterMembership, err)
			}
		}
	}
}

// The plain Membership an expression owns to what it names reads the id the
// conversion writes for it (`referent`, `function`, `targetFeature`).
func TestElementIDReaderSpellsReferenceMembershipsAsTheConversionWritesThem(t *testing.T) {
	sf, index, doc := readerIndex(t, "references.sysml", `package P {
	calc def F { in a : ScalarValues::Real; return r : ScalarValues::Real = a; }
	part def Pt { attribute q : ScalarValues::Real; }
	part def R {
		attribute a : ScalarValues::Real;
		part p : Pt;
		attribute ref = a;
		attribute call = F(1);
		attribute chain = p.q;
	}
}
`)
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	memberships := map[string]bool{}
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			if class.Value != rdf.SysML+"Membership" {
				continue
			}
			for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
				memberships[id.Value] = true
			}
		}
	}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	for feature, slot := range map[string]string{"P::R::ref": "referent", "P::R::call": "function", "P::R::chain": "targetFeature"} {
		matches := index.LookupQualified(feature)
		if len(matches) != 1 {
			t.Fatalf("%s names %d declarations", feature, len(matches))
		}
		usage := matches[0].Decl.(*ast.Usage)
		element, membership, err := reader.ReadExpressionPart(matches[0].DocName, usage.Value, slot)
		if err != nil || element != "" || !memberships[membership] {
			t.Errorf("%s %s = %q, %q, %v; the conversion writes Memberships %v", feature, slot, element, membership, err, memberships)
		}
	}
}

// A change trigger's parts read as changeArgument writes them: `in0` binds the
// wrapper `a0` (a FeatureReferenceExpression), whose referent is the condition
// at `change` under a FeatureMembership, and whose result is its own `out`.
func TestElementIDReaderSpellsAChangeTriggerAsTheConversionWritesIt(t *testing.T) {
	sf, index, doc := readerIndex(t, "change.sysml", `package P {
	action def A {
		attribute a : ScalarValues::Real;
		action acc accept e when a > 1;
	}
}
`)
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	classOf := map[string]string{}
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
				classOf[id.Value] = strings.TrimPrefix(class.Value, rdf.SysML)
			}
		}
	}
	matches := index.LookupQualified("P::A::acc::e")
	if len(matches) != 1 {
		t.Fatalf("payload names %d declarations", len(matches))
	}
	trigger := matches[0].Decl.(*ast.Usage).Value
	condition := trigger.(*ast.ChangeEvent).Condition
	reader := NewElementIDReader([]ElementIDDocument{doc})
	for slot, want := range map[string][2]string{
		"in0":    {"Feature", "ParameterMembership"},
		"a0":     {"FeatureReferenceExpression", "FeatureValue"},
		"a0.out": {"Feature", "ReturnParameterMembership"},
	} {
		element, membership, err := reader.ReadExpressionPart(matches[0].DocName, trigger, slot)
		if err != nil || classOf[element] != want[0] || classOf[membership] != want[1] {
			t.Errorf("%s = %s (%s), %s (%s), %v; want %v", slot, element, classOf[element], membership, classOf[membership], err, want)
		}
	}
	element, err := reader.Read(matches[0].DocName, condition, "")
	if err != nil || classOf[element] != "OperatorExpression" || !strings.HasSuffix(element, "_pa0_pchange") {
		t.Errorf("condition = %s (%s), %v", element, classOf[element], err)
	}
	membership, err := reader.ReadOwningMembership(matches[0].DocName, condition, "")
	if err != nil || classOf[membership] != "FeatureMembership" {
		t.Errorf("condition membership = %s (%s), %v", membership, classOf[membership], err)
	}
	if operand, _, err := reader.ReadExpressionPart(matches[0].DocName, condition, "in0"); err != nil || classOf[operand] != "Feature" {
		t.Errorf("condition in0 = %s (%s), %v", operand, classOf[operand], err)
	}
}
