package grammar

import (
	"reflect"
	"testing"
)

func parseShapes(t *testing.T, sources ...string) []Shape {
	t.Helper()
	var grammars []*Grammar
	for i, src := range sources {
		g, err := ParseGrammar(string(rune('A'+i))+".xtext", src)
		if err != nil {
			t.Fatal(err)
		}
		grammars = append(grammars, g)
	}
	return Shapes(grammars)
}

func shapeNamed(shapes []Shape, name string) Shape {
	for _, shape := range shapes {
		if shape.Name == name {
			return shape
		}
	}
	return Shape{}
}

func TestShapesReturnsDatatypeAndActions(t *testing.T) {
	shapes := parseShapes(t, `
grammar org.example.A
Name : ID;
terminal ID : ('a'..'z')+;
Flag returns Ecore::EBoolean : 'true';
Plain : {SysML::Plain};
Assigned : {SysML::Assigned.owner += current};
`)
	name := shapeNamed(shapes, "Name")
	if name.Returns != "Ecore::EString" || !name.ReturnsDefaulted || !name.Datatype {
		t.Errorf("Name = %+v, want defaulted Ecore string datatype", name)
	}
	flag := shapeNamed(shapes, "Flag")
	if flag.Returns != "Ecore::EBoolean" || !flag.Datatype || flag.ReturnsDefaulted {
		t.Errorf("Flag = %+v, want explicit Ecore datatype", flag)
	}
	plain := shapeNamed(shapes, "Plain")
	if plain.Returns != "SysML::Plain" || !plain.ReturnsDefaulted || plain.Datatype ||
		!reflect.DeepEqual(plain.Creates, []string{"SysML::Plain"}) ||
		len(plain.Actions) != 1 || plain.Actions[0].Metaclass != "SysML::Plain" {
		t.Errorf("Plain = %+v", plain)
	}
	assigned := shapeNamed(shapes, "Assigned")
	if len(assigned.Actions) != 1 || assigned.Actions[0] != (Action{
		Metaclass: "SysML::Assigned", Feature: "owner", Op: "+=", Line: assigned.Actions[0].Line,
	}) {
		t.Errorf("assigned action = %+v", assigned.Actions)
	}
	if len(assigned.Assignments) != 1 || assigned.Assignments[0].Feature != "owner" ||
		assigned.Assignments[0].Value != "current" ||
		!reflect.DeepEqual(assigned.Assignments[0].Owners, []string{"SysML::Assigned"}) {
		t.Errorf("assigned action assignment = %+v", assigned.Assignments)
	}
}

func TestShapesAssignmentOwnersAndValues(t *testing.T) {
	shapes := parseShapes(t, `
grammar org.example.A
Name : ID;
Delegate : {SysML::Delegate};
Lazy : feature = Name;
AfterAction : {SysML::Action} feature += Name;
AfterDelegate : Delegate feature ?= Name;
Cross : target = [SysML::Type | QualifiedName];
enum VisibilityKind returns SysML::VisibilityKind : public = 'public';
terminal ID : ('a'..'z')+;
`)
	tests := []struct {
		rule, owner, op, value string
	}{
		{"Lazy", "SysML::Lazy", "=", "Name"},
		{"AfterAction", "SysML::Action", "+=", "Name"},
		{"AfterDelegate", "SysML::Delegate", "?=", "Name"},
		{"Cross", "SysML::Cross", "=", "[SysML::Type | QualifiedName]"},
	}
	for _, test := range tests {
		shape := shapeNamed(shapes, test.rule)
		if len(shape.Assignments) != 1 {
			t.Fatalf("%s assignments = %+v", test.rule, shape.Assignments)
		}
		got := shape.Assignments[0]
		if got.Feature != "feature" && test.rule != "Cross" || got.Op != test.op || got.Value != test.value ||
			!reflect.DeepEqual(got.Owners, []string{test.owner}) {
			t.Errorf("%s assignment = %+v", test.rule, got)
		}
		if test.rule == "Cross" && !reflect.DeepEqual(got.CrossRef, []string{"SysML::Type"}) {
			t.Errorf("cross references = %v", got.CrossRef)
		}
	}
	enum := shapeNamed(shapes, "VisibilityKind")
	if len(enum.Assignments) != 1 || enum.Assignments[0].Value != "'public'" ||
		!reflect.DeepEqual(enum.Assignments[0].Owners, []string{"SysML::VisibilityKind"}) {
		t.Errorf("enum assignments = %+v", enum.Assignments)
	}
}

func TestShapesFragmentsDelegatesAndCreationAlternatives(t *testing.T) {
	shapes := parseShapes(t, `
grammar org.example.A
Name : ID;
fragment Shared returns SysML::Shared : feature = Name;
Caller : Shared;
Left : {SysML::Left} 'left';
Right : {SysML::Right} 'right';
Either : Left | Right;
`)
	fragment := shapeNamed(shapes, "Shared")
	if len(fragment.Assignments) != 1 ||
		!reflect.DeepEqual(fragment.Assignments[0].Owners, []string{"SysML::Shared"}) ||
		len(fragment.Creates) != 0 {
		t.Errorf("fragment = %+v", fragment)
	}
	caller := shapeNamed(shapes, "Caller")
	if !reflect.DeepEqual(caller.Creates, []string{"SysML::Caller"}) ||
		!reflect.DeepEqual(caller.Fragments, []string{"Shared"}) {
		t.Errorf("caller = %+v", caller)
	}
	either := shapeNamed(shapes, "Either")
	if !reflect.DeepEqual(either.Creates, []string{"SysML::Left", "SysML::Right"}) ||
		!reflect.DeepEqual(either.Delegates, []string{"Left", "Right"}) {
		t.Errorf("either = %+v", either)
	}
}

func TestShapesInheritedFragmentExpressionIDsAreGrammarScoped(t *testing.T) {
	const base = `
grammar org.example.Base
fragment Shared returns SysML::Shared : {SysML::Other.other = current};
`
	const derived = `
grammar org.example.Derived with org.example.Base
Caller returns SysML::Caller : field = 'value' Shared;
`
	shapes := parseShapes(t, base, derived)

	caller := shapeNamed(shapes, "Caller")
	if len(caller.Assignments) != 1 ||
		!reflect.DeepEqual(caller.Assignments[0].Owners, []string{"SysML::Caller"}) {
		t.Errorf("caller assignment owners = %+v, want only SysML::Caller", caller.Assignments)
	}
	fragment := shapeNamed(shapes, "Shared")
	if len(fragment.Assignments) != 1 ||
		fragment.Assignments[0].Feature != "other" ||
		!reflect.DeepEqual(fragment.Assignments[0].Owners, []string{"SysML::Other"}) {
		t.Errorf("fragment assignment = %+v, want standalone SysML::Other owner", fragment.Assignments)
	}
}

func TestShapesLeftRecursiveCreates(t *testing.T) {
	shapes := parseShapes(t, `
grammar org.example.A
B : {SysML::B};
A : B ( {SysML::X.operand += current} '+' B )*;
`)
	if got := shapeNamed(shapes, "A").Creates; !reflect.DeepEqual(got, []string{"SysML::B", "SysML::X"}) {
		t.Errorf("A creates = %v, want B and X", got)
	}
}

func TestShapesAnchorsAndInheritanceOverride(t *testing.T) {
	const base = `
grammar org.example.Base
Name : ID;
PartKeyword : 'part';
fragment Prefix : PartKeyword;
BaseElement : {SysML::BaseElement} 'base';
Caller : Prefix? PartKeyword BaseElement;
CallBase : BaseElement;
`
	const derived = `
grammar org.example.Derived with org.example.Base
@Override
BaseElement : {SysML::DerivedElement} 'derived';
DerivedCall : BaseElement;
`
	shapes := parseShapes(t, base, derived)
	caller := shapeNamed(shapes, "Caller")
	if len(caller.Anchors) < 1 || caller.Anchors[0] != "part" {
		t.Errorf("Caller anchors = %v, want datatype keyword first", caller.Anchors)
	}
	if got := shapeNamed(shapes, "DerivedCall").Creates; !reflect.DeepEqual(got, []string{"SysML::DerivedElement"}) {
		t.Errorf("inherited call creates = %v", got)
	}
}

func TestShapesAssignmentsIncludeAlternationValues(t *testing.T) {
	shapes := parseShapes(t, `
grammar org.example.A
Operator : operator = ('+' | '-');
`)
	got := shapeNamed(shapes, "Operator").Assignments
	if len(got) != 1 || got[0].Value != "'+' | '-'" {
		t.Fatalf("operator assignments = %+v", got)
	}
}
