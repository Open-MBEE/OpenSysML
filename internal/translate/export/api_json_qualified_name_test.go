package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestUnnamedRequirementConstraintDoesNotSuppressSiblingQualifiedNames(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "analysis-demo", "lander.sysml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file := source.New(path, data)
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	evaluation := newAPIJSONEvaluation(graph)
	const constraintID = "Descent__FuelReserve___403"
	constraintPositions := evaluation.structure.byID[constraintID]
	if len(constraintPositions) != 1 {
		t.Fatalf("%s has %d graph subjects; want one", constraintID, len(constraintPositions))
	}
	constraint := evaluation.structure.elements[constraintPositions[0]]
	membership, present, known := evaluation.structure.OwningRelationship(constraint)
	if !known || !present {
		t.Fatal("constraint owning membership is unavailable")
	}
	membershipClass, known := evaluation.structure.Metaclass(membership)
	if !known || membershipClass != "RequirementConstraintMembership" {
		t.Fatalf("constraint membership = %q, known=%t; want RequirementConstraintMembership", membershipClass, known)
	}
	referenceSubsettings, known := evaluation.structure.RelatedElements(constraint, "ownedReferenceSubsetting")
	if !known || len(referenceSubsettings) != 0 {
		t.Fatalf("constraint ownedReferenceSubsetting = %v, known=%t; want none", referenceSubsettings, known)
	}
	name, ok := evaluation.evaluator.Property(constraint, "Element", "name")
	if !ok || name.Kind != metamodel.NullValue {
		reason, explained := evaluation.evaluator.Explain(constraint, "Element", "name")
		t.Errorf("constraint name = %#v, ok=%t; want null, reason=%#v explained=%t", name, ok, reason, explained)
	}

	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []struct {
		ID            string  `json:"@id"`
		QualifiedName *string `json:"qualifiedName"`
	}
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	qualifiedNames := make(map[string]*string, len(elements))
	for _, element := range elements {
		qualifiedNames[element.ID] = element.QualifiedName
	}
	for id, want := range map[string]string{
		"Descent__FuelReserve__lander":   "Descent::FuelReserve::lander",
		"Descent__FuelReserve__fuelLeft": "Descent::FuelReserve::fuelLeft",
	} {
		if got := qualifiedNames[id]; got == nil || *got != want {
			t.Errorf("%s qualifiedName = %v; want %q", id, got, want)
		}
	}
}

func TestDeclaredExhibitStateNamePrecedesReferenceNaming(t *testing.T) {
	sourceText := `package P {
		state def S;
		state def s1;
		part def Vehicle {
			exhibit state es1 : S;
			exhibit s1;
		}
	}`
	file := source.New("declared-exhibit-name.sysml", []byte(sourceText))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	evaluation := newAPIJSONEvaluation(graph)

	var declaredExhibit, referenceExhibit metamodel.Element
	for _, element := range evaluation.structure.elements {
		class, known := evaluation.structure.Metaclass(element)
		if !known || class != "ExhibitStateUsage" {
			continue
		}
		declaredName, hasDeclaredName := evaluation.structure.Attribute(element, "declaredName")
		if hasDeclaredName && declaredName.Kind == metamodel.StringValue && declaredName.String == "es1" {
			declaredExhibit = element
			continue
		}
		relationships, known := evaluation.structure.OwnedRelationships(element)
		if known {
			for _, relationship := range relationships {
				class, classKnown := evaluation.structure.Metaclass(relationship)
				if classKnown && class == "ReferenceSubsetting" {
					referenceExhibit = element
				}
			}
		}
	}
	if declaredExhibit.Key() == (metamodel.ElementKey{}) || referenceExhibit.Key() == (metamodel.ElementKey{}) {
		t.Fatalf("declared/reference ExhibitStateUsage not found: declared=%v referenced=%v",
			declaredExhibit, referenceExhibit)
	}
	if name, ok := evaluation.evaluator.Property(declaredExhibit, "Element", "name"); !ok ||
		name.Kind != metamodel.StringValue || name.String != "es1" {
		t.Errorf("declared ExhibitStateUsage name = %#v, %t; want %q", name, ok, "es1")
	}
	if name, ok := evaluation.evaluator.Property(referenceExhibit, "Element", "name"); !ok ||
		name.Kind != metamodel.StringValue || name.String != "s1" {
		t.Errorf("reference-form ExhibitStateUsage name = %#v, %t; want %q", name, ok, "s1")
	}
}

func TestUnresolvedRequirementConstraintReferenceHasTypedReason(t *testing.T) {
	path := "../../../examples/sysml-v2-training/32. Requirements/Requirement Groups.sysml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file := source.New(path, data)
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	evaluation := newAPIJSONEvaluation(graph)
	for _, id := range []string{
		"Requirement_20Groups__vehicleSpecification___402",
		"Requirement_20Groups__vehicleSpecification___403",
	} {
		positions := evaluation.structure.byID[id]
		if len(positions) != 1 {
			t.Fatalf("%s has %d graph subjects; want one", id, len(positions))
		}
		element := evaluation.structure.elements[positions[0]]
		members, known := evaluation.structure.OwnedRelationships(element)
		if !known {
			t.Fatalf("%s owned relationships are unavailable", id)
		}
		foundReference := false
		for _, member := range members {
			class, classKnown := evaluation.structure.Metaclass(member)
			if !classKnown || class != "ReferenceSubsetting" {
				continue
			}
			foundReference = true
			if !evaluation.structure.HasStatedRelatedElement(member, "referencedFeature") {
				t.Errorf("%s ReferenceSubsetting has no stated referencedFeature", id)
			}
		}
		if !foundReference {
			t.Fatalf("%s has no ReferenceSubsetting", id)
		}
		if _, ok := evaluation.evaluator.Property(element, "Element", "name"); ok {
			t.Errorf("%s name unexpectedly resolved", id)
		}
		reason, explained := evaluation.evaluator.Explain(element, "Element", "name")
		if !explained || reason.Category != metamodel.ReasonUnresolvedReference {
			t.Errorf("%s name reason = %#v, %t; want unresolved-reference", id, reason, explained)
		}
	}
}

func TestFlowEndNamesUseTheSemanticDefaultType(t *testing.T) {
	sourceText := `package P {
		item x { feature i; }
		item y { feature j; }
		flow f from x.i to y.j {
			attribute w;
		}
	}`
	file := source.New("flow-end-qualified-names.sysml", []byte(sourceText))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	evaluation := newAPIJSONEvaluation(graph)

	var flow, member metamodel.Element
	for _, element := range evaluation.structure.elements {
		class, known := evaluation.structure.Metaclass(element)
		if !known {
			continue
		}
		name, hasName := evaluation.structure.Attribute(element, "declaredName")
		if !hasName || name.Kind != metamodel.StringValue {
			continue
		}
		if class == "FlowUsage" && name.String == "f" {
			flow = element
		}
		if class == "AttributeUsage" && name.String == "w" {
			member = element
		}
	}
	if flow.Key() == (metamodel.ElementKey{}) || member.Key() == (metamodel.ElementKey{}) {
		t.Fatalf("flow or body attribute not found: flow=%v member=%v", flow, member)
	}
	ends, ok := evaluation.evaluator.Property(flow, "Flow", "flowEnd")
	if !ok || ends.Kind != metamodel.SequenceValue || len(ends.Values) != 2 {
		t.Fatalf("Flow::flowEnd = %#v, %t; want two ends", ends, ok)
	}
	for index, want := range []string{"source", "target"} {
		end := ends.Values[index]
		if end.Kind != metamodel.ElementValue {
			t.Fatalf("FlowEnd[%d] = %#v; want element", index, end)
		}
		name, ok := evaluation.evaluator.Property(end.Element, "Element", "name")
		if !ok || name.Kind != metamodel.StringValue || name.String != want {
			reason, explained := evaluation.evaluator.Explain(end.Element, "Element", "name")
			t.Errorf("FlowEnd[%d] name = %#v, %t; want library end %q, reason=%#v explained=%t",
				index, name, ok, want, reason, explained)
		}
	}
	qualifiedName, ok := evaluation.evaluator.Property(member, "Element", "qualifiedName")
	if !ok || qualifiedName.Kind != metamodel.StringValue || qualifiedName.String != "f::w" {
		t.Errorf("body attribute qualifiedName = %#v, %t; want %q", qualifiedName, ok, "f::w")
	}
}
