package migrate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

const objectWriteFeatures = `
      <ownedAttribute xmi:type="uml:Property" xmi:id="_items" name="items" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_itemsLo" value="1"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_itemsHi" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_optionalItems" name="optionalItems" isOrdered="true">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_optionalItemsLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_optionalItemsHi" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_bag" name="bag" isOrdered="true" isUnique="false">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_bagLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_bagHi" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_unorderedBag" name="unorderedBag" isOrdered="false" isUnique="false">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_unorderedBagLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_unorderedBagHi" value="*"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_single" name="single">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_singleLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_singleHi" value="1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_part" name="part" type="_child" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_untyped" name="untyped">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_untypedLo" value="0"/>
      </ownedAttribute>`

func objectWriteActivity(parameters, nodes, flows string, order ...string) string {
	var control strings.Builder
	from := "_init"
	for i, id := range order {
		control.WriteString(fmt.Sprintf(`<edge xmi:type="uml:ControlFlow" xmi:id="_cf%d" source="%s" target="%s"/>`, i, from, id))
		from = id
	}
	control.WriteString(fmt.Sprintf(`<edge xmi:type="uml:ControlFlow" xmi:id="_cf%d" source="%s" target="_final"/>`, len(order), from))
	return `<ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">` + parameters +
		`<node xmi:type="uml:InitialNode" xmi:id="_init"/>` + nodes +
		`<node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>` + control.String() + flows +
		`</ownedBehavior>`
}

func objectWriteModel(t *testing.T, activity, extraMembers, extraApplications string) *migrate.Result {
	t.Helper()
	members := `<packagedElement xmi:type="uml:Class" xmi:id="_box" name="Box">` + objectWriteFeatures +
		activity + `</packagedElement>
      <packagedElement xmi:type="uml:Class" xmi:id="_child" name="Child"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_other" name="Other"/>` + extraMembers
	applications := `<sysml:Block xmi:id="_boxBlock" base_Class="_box"/>
      <sysml:Block xmi:id="_childBlock" base_Class="_child"/>
      <sysml:Block xmi:id="_otherBlock" base_Class="_other"/>` + extraApplications
	return migrateDocument(t, members, applications)
}

func objectWritePin(role, id, name, typ string) string {
	decl := `<` + role + ` xmi:type="uml:InputPin" xmi:id="` + id + `" name="` + name + `"`
	if typ != "" {
		decl += ` type="` + typ + `"`
	}
	return decl + `/>`
}

func objectWriteOutputPin(id, name, typ string) string {
	decl := `<result xmi:type="uml:OutputPin" xmi:id="` + id + `" name="` + name + `"`
	if typ != "" {
		decl += ` type="` + typ + `"`
	}
	return decl + `/>`
}

func objectWriteLiteralPin(role, id, name, typeHref, literalType, value string) string {
	return `<` + role + ` xmi:type="uml:ValuePin" xmi:id="` + id + `" name="` + name + `">` +
		`<type xmi:type="uml:PrimitiveType" href="` + typeHref + `"/>` +
		`<value xmi:type="uml:` + literalType + `" xmi:id="` + id + `Value" value="` + value + `"/>` +
		`</` + role + `>`
}

func objectWriteAdd(id, name, feature, attrs, object, value, insertAt, result string) string {
	return `<node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="` + id + `" name="` + name +
		`" structuralFeature="` + feature + `" ` + attrs + `>` +
		object + value + insertAt + result + `</node>`
}

func objectWriteRemove(id, name, feature, attrs, object, value, removeAt, result string) string {
	return `<node xmi:type="uml:RemoveStructuralFeatureValueAction" xmi:id="` + id + `" name="` + name +
		`" structuralFeature="` + feature + `" ` + attrs + `>` +
		object + value + removeAt + result + `</node>`
}

func objectWriteClear(id, name, feature, object, result string) string {
	return `<node xmi:type="uml:ClearStructuralFeatureAction" xmi:id="` + id + `" name="` + name +
		`" structuralFeature="` + feature + `">` + object + result + `</node>`
}

func objectWriteDestroy(id, name, attrs, target string) string {
	return `<node xmi:type="uml:DestroyObjectAction" xmi:id="` + id + `" name="` + name +
		`" ` + attrs + `>` + target + `</node>`
}

func objectWriteFlow(id, source, target string) string {
	return `<edge xmi:type="uml:ObjectFlow" xmi:id="` + id + `" source="` + source + `" target="` + target + `"/>`
}

func objectWriteActionBody(t *testing.T, notation []byte, name string) string {
	t.Helper()
	text := string(notation)
	start := strings.Index(text, "action "+name+" {")
	if start < 0 {
		t.Fatalf("missing action %s:\n%s", name, text)
	}
	start += len("action " + name + " {")
	depth := 1
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start:i]
			}
		}
	}
	t.Fatalf("action %s has no closing brace:\n%s", name, text)
	return ""
}

// A flow-fed object pin becomes the write target, and the first result feeds a
// second write through its own input pin.
func TestFlowFedObjectWritesBindAndChainResults(t *testing.T) {
	nodes := `<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>
      ` + objectWriteAdd("_first", "first", "_items", `isReplaceAll="true"`,
		objectWritePin("object", "_firstObject", "firstObject", "_box"),
		objectWriteLiteralPin("value", "_firstValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"),
		"", objectWriteOutputPin("_firstResult", "result", "_box")) +
		`<node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_second" name="second" structuralFeature="_items" isReplaceAll="true">` +
		objectWritePin("object", "_secondObject", "secondObject", "_box") +
		objectWriteLiteralPin("value", "_secondValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "2") +
		objectWriteOutputPin("_secondResult", "result", "_box") + `</node>`
	params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_box" direction="in"/>`
	activity := objectWriteActivity(params, nodes,
		objectWriteFlow("_sourceToFirst", "_sourceNode", "_firstObject")+
			objectWriteFlow("_firstToSecond", "_firstResult", "_secondObject"),
		"_first", "_second")
	r := objectWriteModel(t, activity, "", "")
	for _, line := range []string{
		"assign firstObject.items := value;",
		"out result : Box[1] = firstObject;",
		"assign secondObject.items := value;",
		"out result : Box[1] = secondObject;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_first", migrate.Mapped, "")
	wantNote(t, r, "_second", migrate.Mapped, "")
	wantClean(t, "object_write_flow.sysml", r)
}

// A fed pin with no resolved source is not mistaken for the activity context.
func TestFedObjectPinWithoutResolvedSourceIsWriteTarget(t *testing.T) {
	nodes := `<node xmi:type="uml:ForkNode" xmi:id="_fork"/>` +
		objectWriteAdd("_write", "write", "_single", `isReplaceAll="true"`,
			objectWritePin("object", "_object", "object", "_box"),
			objectWriteLiteralPin("value", "_value", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "")
	activity := objectWriteActivity("", nodes, objectWriteFlow("_forkToObject", "_fork", "_object"), "_write")
	r := objectWriteModel(t, activity, "", "")
	wantLine(t, r.Notation, "assign object.single := value;")
	wantClean(t, "object_write_fed_pin.sysml", r)
}

// An object flow owned by the enclosing activity feeds a nested action's pin.
func TestStructuredNodeBoundaryFeedsObjectPin(t *testing.T) {
	params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_box" direction="in"/>`
	nodes := `<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>` +
		`<node xmi:type="uml:StructuredActivityNode" xmi:id="_statement" name="statement">` +
		objectWriteAdd("_write", "write", "_single", `isReplaceAll="true"`,
			objectWritePin("object", "_object", "object", "_box"),
			objectWriteLiteralPin("value", "_value", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "") +
		`</node>`
	activity := objectWriteActivity(params, nodes, objectWriteFlow("_sourceToObject", "_sourceNode", "_object"), "_statement")
	r := objectWriteModel(t, activity, "", "")
	wantLine(t, r.Notation, "assign object.single := value;")
	wantClean(t, "object_write_structured_boundary.sysml", r)
}

// A structured node's ReadSelf flow writes through its typed object pin.
func TestStructuredNodeReadSelfTargetsFedObjectPin(t *testing.T) {
	nodes := `<node xmi:type="uml:StructuredActivityNode" xmi:id="_statement" name="statement">
        <node xmi:type="uml:InitialNode" xmi:id="_innerInit"/>
        <node xmi:type="uml:ReadSelfAction" xmi:id="_self" name="self">
          <result xmi:type="uml:OutputPin" xmi:id="_selfResult" name="result" type="_box"/>
        </node>
        ` + objectWriteClear("_clear", "clear", "_single", objectWritePin("object", "_object", "object", "_box"), "") + `
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_innerFinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_innerStartToSelf" source="_innerInit" target="_self"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_innerSelfToClear" source="_self" target="_clear"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_innerClearToFinal" source="_clear" target="_innerFinal"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_selfToObject" source="_selfResult" target="_object"/>
      </node>`
	activity := objectWriteActivity("", nodes, "", "_statement")
	r := objectWriteModel(t, activity, "", "")
	wantLine(t, r.Notation, "assign object.single := ();")
	wantClean(t, "object_write_structured_self_flow.sysml", r)
}

// A second pin-name settlement preserves the names fixed before expressions are built.
func TestObjectWritePinNamesStayStableAcrossDeclaration(t *testing.T) {
	params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_box" direction="in"/>`
	nodes := `<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>` +
		objectWriteAdd("_write", "write", "_single", `isReplaceAll="true"`,
			objectWritePin("object", "_object", "shared", "_box"),
			objectWriteLiteralPin("value", "_value", "shared", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "")
	activity := objectWriteActivity(params, nodes, objectWriteFlow("_sourceToObject", "_sourceNode", "_object"), "_write")
	r := objectWriteModel(t, activity, "", "")
	for _, line := range []string{
		"in shared : Box[1];",
		"in shared2 : ScalarValues::Integer[1] = 1;",
		"assign shared.single := shared2;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "object_write_pin_names.sysml", r)
}

// A flow-fed write cannot target a property without a v2 feature type.
func TestFlowFedObjectWriteRejectsUntypedFeature(t *testing.T) {
	params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_box" direction="in"/>`
	nodes := `<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>` +
		objectWriteAdd("_write", "write", "_untyped", `isReplaceAll="true"`,
			objectWritePin("object", "_object", "object", "_box"),
			objectWriteLiteralPin("value", "_value", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "")
	activity := objectWriteActivity(params, nodes, objectWriteFlow("_sourceToObject", "_sourceNode", "_object"), "_write")
	r := objectWriteModel(t, activity, "", "")
	wantNoStatement(t, r.Notation, "assign object.untyped")
	wantNote(t, r, "_write", migrate.Unmapped,
		"the feature untyped is untyped, so it has no v2 member to write through the object pin")
	wantClean(t, "object_write_untyped_feature.sysml", r)
}

// Collection writes replace, deduplicate, append or insert according to the
// feature's multiplicity, ordering and uniqueness.
func TestObjectWriteCollectionUpdatesRespectFeatureShape(t *testing.T) {
	nodes := objectWriteAdd("_replace", "replace", "_items", `isReplaceAll="true"`, "",
		objectWriteLiteralPin("value", "_replaceValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "") +
		objectWriteAdd("_unique", "unique", "_optionalItems", "", "",
			objectWriteLiteralPin("value", "_uniqueValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "2"), "", "") +
		objectWriteAdd("_bagAdd", "bagAdd", "_bag", "", "",
			objectWriteLiteralPin("value", "_bagValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "3"), "", "") +
		objectWriteAdd("_orderedInsert", "orderedInsert", "_items", "", "",
			objectWriteLiteralPin("value", "_orderedValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "4"),
			objectWriteLiteralPin("insertAt", "_orderedIndex", "insertAt", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "2"), "") +
		objectWriteAdd("_append", "append", "_bag", "", "",
			objectWriteLiteralPin("value", "_appendValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "5"),
			objectWriteLiteralPin("insertAt", "_appendIndex", "insertAt", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#UnlimitedNatural", "LiteralUnlimitedNatural", "*"), "") +
		objectWriteAdd("_unordered", "unordered", "_unorderedBag", "", "",
			objectWriteLiteralPin("value", "_unorderedValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "6"),
			objectWriteLiteralPin("insertAt", "_unorderedIndex", "insertAt", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "2"), "") +
		objectWriteAdd("_singleWrite", "singleWrite", "_single", "", "",
			objectWriteLiteralPin("value", "_singleValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "7"), "", "")
	activity := objectWriteActivity("", nodes, "", "_replace", "_unique", "_bagAdd", "_orderedInsert", "_append", "_unordered", "_singleWrite")
	r := objectWriteModel(t, activity, "", "")
	for name, want := range map[string]string{
		"replace":       "assign items := value;",
		"unique":        "assign optionalItems := SequenceFunctions::including(SequenceFunctions::excluding(optionalItems, value), value);",
		"bagAdd":        "assign bag := SequenceFunctions::including(bag, value);",
		"orderedInsert": "assign items := SequenceFunctions::includingAt(SequenceFunctions::excluding(items, value), value, insertAt);",
		"append":        "assign bag := SequenceFunctions::including(bag, value);",
		"unordered":     "assign unorderedBag := SequenceFunctions::including(unorderedBag, value);",
		"singleWrite":   "assign single := value;",
	} {
		body := objectWriteActionBody(t, r.Notation, name)
		if !strings.Contains(body, want) {
			t.Errorf("action %s body = %q, want %q", name, body, want)
		}
		if (name == "append" || name == "unordered") && strings.Contains(body, "includingAt(") {
			t.Errorf("action %s should ignore insertAt:\n%s", name, body)
		}
	}
	wantNote(t, r, "_singleWrite", migrate.Mapped, "")
	wantClean(t, "object_write_collections.sysml", r)
}

// Actions with no value pin or no v2 feature keep their existing placeholders.
func TestAddObjectWritePlaceholdersArePreserved(t *testing.T) {
	nodes := objectWriteAdd("_noValue", "noValue", "_items", "", "", "", "", "") +
		objectWriteAdd("_noFeature", "noFeature", "_missingFeature", "", "",
			objectWriteLiteralPin("value", "_unmappedValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "")
	activity := objectWriteActivity("", nodes, "", "_noValue", "_noFeature")
	r := objectWriteModel(t, activity, "", "")
	wantNote(t, r, "_noValue", migrate.Unmapped, "the action has no value pin")
	wantNote(t, r, "_noFeature", migrate.Unmapped, "the feature written has no v2 declaration")
	wantClean(t, "object_write_placeholders.sysml", r)
}

// A fallback uses the object's declared pin type, refusing an untyped or
// incompatible classifier instead of emitting a member access that cannot validate.
func TestFlowFedObjectWriteRejectsUntypedAndWrongClassifiers(t *testing.T) {
	cases := []struct {
		name, typeRef, want string
	}{
		{name: "untyped", want: "the object pin is untyped, so its feature items cannot be written"},
		{name: "wrong_classifier", typeRef: `_other`, want: "which has no feature items"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := `<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_write" name="write" structuralFeature="_items">
          <object xmi:type="uml:InputPin" xmi:id="_object" name="object"` + typeAttr(tc.typeRef) + `/>
          ` + objectWriteLiteralPin("value", "_value", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1") + `
        </node>`
			params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_box" direction="in"/>`
			activity := objectWriteActivity(params, nodes, objectWriteFlow("_sourceToObject", "_sourceNode", "_object"), "_write")
			r := objectWriteModel(t, activity, "", "")
			wantNoStatement(t, r.Notation, "assign object.items")
			wantNote(t, r, "_write", migrate.Unmapped, tc.want)
			wantClean(t, "object_write_invalid_target.sysml", r)
		})
	}
}

func typeAttr(typ string) string {
	if typ == "" {
		return ""
	}
	return ` type="` + typ + `"`
}

// Association ends are represented by links in v2 and cannot be assigned.
func TestObjectWriteAssociationEndStaysUnmapped(t *testing.T) {
	params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_links" direction="in"/>`
	nodes := `<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>` +
		objectWriteAdd("_write", "write", "_testsEnd", "", objectWritePin("object", "_object", "object", "_links"),
			objectWriteLiteralPin("value", "_value", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "", "")
	activity := objectWriteActivity(params, nodes, objectWriteFlow("_sourceToObject", "_sourceNode", "_object"), "_write")
	association := `<packagedElement xmi:type="uml:Association" xmi:id="_links" name="A_tests_testSuite">
        <memberEnd xmi:idref="_testsEnd"/><memberEnd xmi:idref="_ownerEnd"/>
        <ownedEnd xmi:type="uml:Property" xmi:id="_testsEnd" name="tests" type="_box" isUnique="false">
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_testsLower" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_testsUpper" value="*"/>
        </ownedEnd>
        <ownedEnd xmi:type="uml:Property" xmi:id="_ownerEnd" name="owner" type="_other"/>
      </packagedElement>`
	r := objectWriteModel(t, activity, association, "")
	wantNoStatement(t, r.Notation, "assign object.tests")
	wantNote(t, r, "_write", migrate.Unmapped,
		"the feature tests is an end owned by the association A_tests_testSuite, which the v2 class does not have; writing it creates or destroys a link")
	wantClean(t, "object_write_association_end.sysml", r)
}

// A self-context write keeps its existing target, while [0..1] is assigned as
// a scalar and does not receive the former collection approximation.
func TestObjectWriteKeepsSelfAndOptionalScalarWrites(t *testing.T) {
	nodes := objectWriteAdd("_write", "write", "_single", "", "",
		objectWriteLiteralPin("value", "_value", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "7"),
		"", objectWriteOutputPin("_result", "result", "_box"))
	activity := objectWriteActivity("", nodes, "", "_write")
	r := objectWriteModel(t, activity, "", "")
	wantLine(t, r.Notation, "assign single := value;")
	wantLine(t, r.Notation, "out result : Box[1] = this;")
	wantNote(t, r, "_write", migrate.Mapped, "")
	wantClean(t, "object_write_self.sysml", r)
}

// Value and position removals lower to sequence operations, with duplicate
// removal's one-versus-all semantic difference reported as an approximation.
func TestRemoveFeatureUsesValueAndPosition(t *testing.T) {
	nodes := objectWriteRemove("_byValue", "byValue", "_optionalItems", `isRemoveDuplicates="true"`, "",
		objectWriteLiteralPin("value", "_byValuePin", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "",
		objectWriteOutputPin("_byValueResult", "result", "_box")) +
		objectWriteRemove("_byPosition", "byPosition", "_bag", "", "",
			"", objectWriteLiteralPin("removeAt", "_removeIndex", "removeAt", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "2"), "") +
		objectWriteRemove("_duplicates", "duplicates", "_bag", "", "",
			objectWriteLiteralPin("value", "_duplicateValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "3"), "", "") +
		objectWriteRemove("_missing", "missing", "_bag", "", "", "", "", "")
	activity := objectWriteActivity("", nodes, "", "_byValue", "_byPosition", "_duplicates", "_missing")
	r := objectWriteModel(t, activity, "", "")
	for _, line := range []string{
		"assign optionalItems := SequenceFunctions::excluding(optionalItems, value);",
		"assign bag := SequenceFunctions::excludingAt(bag, removeAt, removeAt);",
		"assign bag := SequenceFunctions::excluding(bag, value);",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_byValue", migrate.Mapped, "")
	wantNote(t, r, "_byPosition", migrate.Mapped, "")
	wantNote(t, r, "_duplicates", migrate.Approximated, "every occurrence of the value is removed, where v1 removes one")
	wantNote(t, r, "_missing", migrate.Unmapped, "the action names no value or position to remove")
	wantLine(t, r.Notation, "out result : Box[1] = this;")
	wantClean(t, "object_write_remove.sysml", r)
}

// Optional features clear to the empty sequence; a required feature can pass
// through only when every result consumer replaces that same feature.
func TestClearOptionalAndReplaceAllPassThrough(t *testing.T) {
	nodes := objectWriteClear("_optional", "optional", "_optionalItems", "",
		objectWriteOutputPin("_optionalResult", "result", "_box")) +
		objectWriteClear("_pass", "pass", "_items", "",
			objectWriteOutputPin("_passResult", "result", "_box")) +
		`<node xmi:type="uml:CentralBufferNode" xmi:id="_buffer" name="buffer" type="_box"/>` +
		objectWriteAdd("_replace", "replace", "_items", `isReplaceAll="true"`,
			objectWritePin("object", "_replaceObject", "object", "_box"),
			objectWriteLiteralPin("value", "_replaceValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "8"), "", "") +
		objectWriteClear("_mandatory", "mandatory", "_items", "",
			objectWriteOutputPin("_mandatoryResult", "result", "_box"))
	activity := objectWriteActivity("", nodes,
		objectWriteFlow("_passToBuffer", "_passResult", "_buffer")+
			objectWriteFlow("_bufferToReplace", "_buffer", "_replaceObject"),
		"_optional", "_pass", "_replace", "_mandatory")
	r := objectWriteModel(t, activity, "", "")
	wantLine(t, r.Notation, "assign optionalItems := ();")
	wantNoStatement(t, r.Notation, "assign items := ();")
	wantNote(t, r, "_optional", migrate.Mapped, "")
	wantNote(t, r, "_pass", migrate.Mapped, "")
	wantNote(t, r, "_mandatory", migrate.Unmapped, "the feature items must hold at least one value, so it cannot be emptied in v2")
	wantLine(t, r.Notation, "out result : Box[1] = this;")
	wantClean(t, "object_write_clear.sysml", r)
}

// A single non-replace-all consumer prevents a required-feature clear from
// being treated as a pass-through.
func TestClearMandatoryMixedConsumersStayUnmapped(t *testing.T) {
	nodes := objectWriteClear("_clear", "clear", "_items", "",
		objectWriteOutputPin("_clearResult", "result", "_box")) +
		objectWriteAdd("_replace", "replace", "_items", `isReplaceAll="true"`,
			objectWritePin("object", "_replaceObject", "replaceObject", "_box"),
			objectWriteLiteralPin("value", "_replaceValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "8"), "", "") +
		objectWriteAdd("_append", "append", "_items", "",
			objectWritePin("object", "_appendObject", "appendObject", "_box"),
			objectWriteLiteralPin("value", "_appendValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "9"), "", "")
	activity := objectWriteActivity("", nodes,
		objectWriteFlow("_toReplace", "_clearResult", "_replaceObject")+
			objectWriteFlow("_toAppend", "_clearResult", "_appendObject"),
		"_clear", "_replace", "_append")
	r := objectWriteModel(t, activity, "", "")
	wantNote(t, r, "_clear", migrate.Unmapped, "the feature items must hold at least one value, so it cannot be emptied in v2")
	wantNoStatement(t, r.Notation, "assign items := ();")
	wantClean(t, "object_write_clear_mixed.sysml", r)
}

// Destroy always consumes its target pin, deduplicates the synthesized result,
// and reports links and retained composite parts that v2 handles differently.
func TestDestroyObjectUsesItsTargetPinAndReportsDifferences(t *testing.T) {
	params := `<ownedParameter xmi:type="uml:Parameter" xmi:id="_source" name="source" type="_box" direction="in"/>`
	nodes := `<node xmi:type="uml:ReadSelfAction" xmi:id="_self" name="self">
          <result xmi:type="uml:OutputPin" xmi:id="_selfResult" name="result" type="_box"/>
        </node>` +
		objectWriteDestroy("_selfDestroy", "selfDestroy", `isDestroyLinks="true" isDestroyOwnedObjects="true"`,
			objectWritePin("target", "_selfTarget", "selfTarget", "_box")) +
		`<node xmi:type="uml:ActivityParameterNode" xmi:id="_sourceNode" name="source" parameter="_source"/>` +
		objectWriteDestroy("_flowDestroy", "flowDestroy", `isDestroyOwnedObjects="false"`,
			objectWritePin("target", "_flowTarget", "result", "_box")) +
		objectWriteDestroy("_missing", "missing", "", "")
	activity := objectWriteActivity(params, nodes,
		objectWriteFlow("_selfFed", "_selfResult", "_selfTarget")+
			objectWriteFlow("_sourceToTarget", "_sourceNode", "_flowTarget"),
		"_selfDestroy", "_flowDestroy", "_missing")
	r := objectWriteModel(t, activity, "", "")
	for _, line := range []string{
		"OccurrenceFunctions::destroy(selfTarget)",
		"OccurrenceFunctions::destroy(result)",
		"out result2 : Box[0..1]",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_selfDestroy", migrate.Approximated, "the links the object takes part in are not destroyed with it")
	wantNote(t, r, "_flowDestroy", migrate.Approximated, "v2 ends the object's composite parts with it, where v1 keeps them")
	wantNote(t, r, "_missing", migrate.Unmapped, "the action has no target pin")
	body := objectWriteActionBody(t, r.Notation, "flowDestroy")
	for _, line := range []string{
		"out result2 : Box[0..1] = OccurrenceFunctions::destroy(result);",
		"metadata MigrationMetadata::SynthesizedName about result2;",
		"metadata MigrationMetadata::StandIn about result2;",
	} {
		if !strings.Contains(body, line) {
			t.Errorf("flowDestroy block lacks %q:\n%s", line, body)
		}
	}
	wantClean(t, "object_write_destroy.sysml", r)
}

// Migrated add, insertAt, remove and destroy actions execute against the performer.
func TestObjectWritesExecuteAgainstThePerformer(t *testing.T) {
	nodes := objectWriteAdd("_first", "first", "_optionalItems", "", "",
		objectWriteLiteralPin("value", "_firstValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "7"), "", "") +
		objectWriteAdd("_insert", "insert", "_optionalItems", "", "",
			objectWriteLiteralPin("value", "_insertValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "9"),
			objectWriteLiteralPin("insertAt", "_insertIndex", "insertAt", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "1"), "") +
		objectWriteRemove("_remove", "remove", "_optionalItems", "", "",
			objectWriteLiteralPin("value", "_removeValue", "value", "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer", "LiteralInteger", "7"), "", "") +
		`<node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_read" name="read" structuralFeature="_optionalItems">
          <result xmi:type="uml:OutputPin" xmi:id="_readResult" name="result" isOrdered="true">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_readLo" value="0"/>
			<upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_readHi" value="*"/>
          </result>
        </node>` +
		`<node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_partRead" name="partRead" structuralFeature="_part">
          <result xmi:type="uml:OutputPin" xmi:id="_partReadResult" name="result" type="_child"/>
        </node>` +
		objectWriteDestroy("_destroy", "destroy", "", objectWritePin("target", "_destroyTarget", "target", "_child"))
	activity := objectWriteActivity("", nodes,
		objectWriteFlow("_partToDestroy", "_partReadResult", "_destroyTarget"),
		"_first", "_insert", "_remove", "_read", "_partRead", "_destroy")
	r := objectWriteModel(t, activity, "", "")
	wantClean(t, "object_write_execution.sysml", r)
	s := session(t, r)
	meta(t, s, "%instantiate Box")
	got := runValues(t, s, "Box::run", "Box")
	wantValues(t, got, map[string]string{"read.result": "[9]"})
	if got["destroy.result"] == "" {
		t.Errorf("destroy.result = %q, want the destroyed performer", got["destroy.result"])
	}
}
