package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Every KerML feature declaration subsets a Feature: a `bool`, an `expr` or a
// `step` naming a data type or a class is the error the pilot reports for
// `feature f :> D` (Subsetting::subsettedFeature is typed Feature), while a
// `predicate` naming one is a classifier specialization judged by the family
// rules. Typing a feature by a classifier is fine.
func TestKerMLSubsettingMetaclassCoversEveryFeatureKeyword(t *testing.T) {
	src := `package P {
	datatype D;
	class C;
	struct S;
	behavior B;
	predicate Pr;
	bool b1 :> D;
	bool b2 :> C;
	bool b3 :> S;
	bool b4 :> B;
	bool b5 : D;
	bool b6 : C;
	bool b7 : Pr;
	predicate p1 :> D;
	expr e1 :> D;
	expr e2 : D;
	step s1 :> D;
	step s2 : D;
	feature f1 :> D;
	feature f2 : D;
	feature g;
	bool b8 :> g;
}`
	var got []string
	for _, d := range diagsIn(t, "a.kerml", src, "type") {
		pos := source.New("a.kerml", []byte(src)).Lines().PosAt(d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, d.Message))
	}
	want := []string{
		"7:13: subsets target must be a feature, found attributeDef",
		"8:13: subsets target must be a feature, found kermlType",
		"9:13: subsets target must be a feature, found kermlType",
		"10:13: subsets target must be a feature, found kermlType",
		"14:18: Cannot specialize data type or association",
		"15:13: subsets target must be a feature, found attributeDef",
		"17:13: subsets target must be a feature, found attributeDef",
		"19:16: subsets target must be a feature, found attributeDef",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func subsettingMetaclassPositions(t *testing.T, name, src string) []string {
	t.Helper()
	var got []string
	for _, d := range diagsIn(t, name, src, "type") {
		pos := source.New(name, []byte(src)).Lines().PosAt(d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, d.Message))
	}
	return got
}

// A SysML usage inherits the rule: `part eng : Engine :> Mass;` over an
// attribute definition is an error in default mode, at the target's span.
func TestSubsettingMetaclassIsAnErrorInSysMLSources(t *testing.T) {
	src := `package P6 {
    private import ScalarValues::*;
    part def Engine;
    attribute def Mass;
    part eng : Engine :> Mass;
}`
	got := subsettingMetaclassPositions(t, "p6.sysml", src)
	want := []string{"5:26: subsets target must be a feature, found attributeDef"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A definition target is rejected however it is written and whatever the usage
// kind; the pinned pilot reports each of these lines at the same column.
func TestSubsettingMetaclassRejectsDefinitionTargetsInSysML(t *testing.T) {
	src := `package Neg {
    private import ScalarValues::*;
    part def Engine;
    attribute def Mass;
    item def Box;
    occurrence def Occ;
    alias EngineAlias for Engine;
    part eng : Engine :> Mass;
    part eng2 : Engine :>> Engine;
    item box :> Box;
    attribute m :> Real;
    part def Car { part e :>> Engine; }
    part def Outer { part def Inner; part x :> Inner; }
    part eng3 :> EngineAlias;
    part eng4 :> Neg::Engine;
    occurrence o :>> Occ;
    ref part r :> Engine;
    part def Fam { variation part v : Engine { variant part vv :>> Engine; } }
}`
	got := subsettingMetaclassPositions(t, "neg.sysml", src)
	want := []string{
		"8:26: subsets target must be a feature, found attributeDef",
		"9:28: redefines target must be a feature, found partDef",
		"10:17: subsets target must be a feature, found itemDef",
		"11:20: subsets target must be a feature, found attributeDef",
		"12:31: redefines target must be a feature, found partDef",
		"13:48: subsets target must be a feature, found partDef",
		"14:18: subsets target must be a feature, found partDef",
		"15:18: subsets target must be a feature, found partDef",
		"16:22: redefines target must be a feature, found occurrenceDef",
		"17:19: subsets target must be a feature, found partDef",
		"18:68: redefines target must be a feature, found partDef",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Feature targets of every spelling keep passing, as does an unnamed redefinition
// whose borrowed name must not shadow its target; the pilot accepts this model.
func TestSubsettingMetaclassAcceptsFeatureTargetsInSysML(t *testing.T) {
	src := `package Pos {
    private import ScalarValues::*;
    private import Parts::*;
    private import Items::*;
    part def Vehicle { part wheels : Wheel[4]; attribute mass : Real; }
    part def Wheel;
    part def Car :> Vehicle {
        part frontWheels :> wheels[2];
        part spare :> Vehicle::wheels;
        attribute m :>> mass;
    }
    part car : Car;
    part fw :> car.frontWheels;
    alias W for car;
    part aliased :> W;
    part p :> parts;
    item i :> items;
    connection def Link { end a : Wheel; end b : Wheel; }
    connection def TightLink :> Link { end a2 :>> a; end b2 :>> b; }
    part def Engine;
    part def Family { variation part engine : Engine { variant part e :>> engine; } }
    occurrence def Occ { occurrence slot : Occ; }
    occurrence occ : Occ { occurrence s :>> slot; }
    part def Holder { ref part held : Wheel; }
    part h : Holder { ref part held2 :>> held; }
    part def Base { part b : Wheel; }
    part def Sub :> Base { part bb :> b; }
    part def Sub2 :> Base { part bbb :>> b; }
    part v : Vehicle { part :>> wheels { attribute :>> mass; } }
}`
	if got := subsettingMetaclassPositions(t, "pos.sysml", src); len(got) != 0 {
		t.Fatalf("expected no type diagnostics, got\n%s", strings.Join(got, "\n"))
	}
}

// The reference form (`verify RD;`) is a ReferenceSubsetting the referent rule
// judges, so a definition there is reported once.
func TestSubsettingMetaclassLeavesTheReferenceFormToTheReferentRule(t *testing.T) {
	src := `package V {
    requirement def RD;
    verification def Ver { objective o { verify RD; } }
}`
	got := subsettingMetaclassPositions(t, "verify.sysml", src)
	want := []string{"3:49: satisfy target must be a requirement usage, found requirementDef"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
