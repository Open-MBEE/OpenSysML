package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// runValues runs an action and returns the values it produced by name.
func runValues(t *testing.T, s *repl.Session, action string, performer ...string) map[string]string {
	t.Helper()
	v := s.RunAction(action, performer...)
	wantVerdict(t, v)
	values := map[string]string{}
	for _, nv := range v.Values {
		values[nv.Name] = nv.Value
	}
	return values
}

// wantValues checks the values a run produced; an empty want is a value the
// run must not have produced.
func wantValues(t *testing.T, got, want map[string]string) {
	t.Helper()
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %q, want %q (values: %v)", name, got[name], value, got)
		}
	}
}

// The library_calls fixture chains fUML and Alf library calls: Integer
// ToString, Concat, Including twice, Size and Boolean ToString compute their
// results through the v2 library, while IndexOf and WriteLine are refused and
// the call after IndexOf, fed by it alone, performs nothing.
func TestLibraryCallsComputeThroughTheV2Library(t *testing.T) {
	r := migrateFixtureFile(t, "library_calls")
	for _, line := range []string{
		"out result : ScalarValues::String = IntegerFunctions::ToString(x);",
		"out result : ScalarValues::String = StringFunctions::'+'(x, y);",
		"out result : ScalarValues::String[0..*] ordered nonunique = SequenceFunctions::including(seq, element);",
		"out result : ScalarValues::Integer = SequenceFunctions::size(seq);",
		"out result : ScalarValues::String = BooleanFunctions::ToString(x);",
		"/* not migrated: CallBehaviorAction 'position' — the behavior Alf SequenceFunctions::IndexOf it calls has no v2 library function: the v2 library has no function giving the position of an element in a sequence; the behavior is known by its OMG href http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-IndexOf */",
		"/* not migrated: CallBehaviorAction 'print' — the behavior fUML BasicInputOutput::WriteLine it calls has no v2 library function: writes a line to the standard output channel, which the v2 library has no function for; the behavior is known by its OMG href http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#BasicInputOutput-WriteLine */",
		"/* flow position.result to 'after'.x not written: 'position' is not migrated and produces no value */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_toString", migrate.Mapped, "calls fUML IntegerFunctions::ToString, which the v2 library computes; the behavior is known by its OMG href http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-ToString")
	wantNote(t, r, "_first", migrate.Mapped, "calls Alf SequenceFunctions::Including, which the v2 library computes; the behavior is known by its OMG href http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-Including")
	wantNote(t, r, "_position", migrate.Unmapped, "has no v2 library function: the v2 library has no function giving the position of an element in a sequence")
	wantNote(t, r, "_after", migrate.Approximated, "the pin 'x' it passes for the parameter x of fUML IntegerFunctions::plus, which must hold a value, receives none: 'position', which feeds it, produces no value; v1 never fires the call")

	s := session(t, r)
	meta(t, s, "%instantiate Recorder")
	wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{
		"toString.result": `"42"`,
		"concat.result":   `"n=42"`,
		"first.result":    `["n=42"]`,
		"second.result":   `["n=42", "n=42"]`,
		"size.result":     "2",
		"flag.result":     `"true"`,
		"position.result": "",
		"after.result":    "",
	})
}

const (
	stringType  = `<type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>`
	integerType = `<type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>`
	realType    = `<type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>`
	booleanType = `<type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>`
	naturalType = `<type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#UnlimitedNatural"/>`
)

// labeler wraps the nodes of an activity Label of a block Recorder, run from
// its initial node through the nodes in order to its final node.
func labeler(nodes string, names ...string) string {
	edges := `<edge xmi:type="uml:ControlFlow" xmi:id="_c0" source="_init" target="_` + names[0] + `"/>`
	for i := 1; i < len(names); i++ {
		edges += `
        <edge xmi:type="uml:ControlFlow" xmi:id="_c` + names[i] + `" source="_` + names[i-1] + `" target="_` + names[i] + `"/>`
	}
	return `
    <packagedElement xmi:type="uml:Class" xmi:id="_recorder" name="Recorder">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_label" name="Label">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>` + nodes + `
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        ` + edges + `
        <edge xmi:type="uml:ControlFlow" xmi:id="_cf" source="_` + names[len(names)-1] + `" target="_final"/>
      </ownedBehavior>
    </packagedElement>`
}

const recorderBlock = `<sysml:Block xmi:id="_s1" base_Class="_recorder"/>`

const concatHref = "http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat"

// concatCall is a call to Concat whose x is the literal "n=" and whose y is the
// given pin, or nothing.
func concatCall(behavior, y string) string {
	return `
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_concat" name="concat">
          ` + behavior + `
          <argument xmi:type="uml:ValuePin" xmi:id="_concatX" name="x">` + stringType + `
            <value xmi:type="uml:LiteralString" xmi:id="_concatXV" value="n="/>
          </argument>` + y + `
          <result xmi:type="uml:OutputPin" xmi:id="_concatOut" name="result">` + stringType + `</result>
        </node>`
}

// Boolean ToString writes true and false in lowercase, Integer ToString writes
// no decimal point and a sign, and Real ToString writes the shortest text
// reading back as the value, as fUML 1.5 §9.3 specifies.
func TestLibraryConversionsFollowFUML(t *testing.T) {
	r := migrateDocument(t, labeler(`
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_flag" name="flag">
          <behavior xmi:type="uml:FunctionBehavior" href="http://www.omg.org/spec/FUML/20130801/fUML_Library.xmi#PrimitiveBehaviors-BooleanFunctions-ToString"/>
          <argument xmi:type="uml:ValuePin" xmi:id="_flagX" name="x">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
            <value xmi:type="uml:LiteralBoolean" xmi:id="_flagXV" value="false"/>
          </argument>
          <result xmi:type="uml:OutputPin" xmi:id="_flagOut" name="result">`+stringType+`</result>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_num" name="num">
          <behavior xmi:type="uml:FunctionBehavior" href="http://www.omg.org/spec/FUML/20130801/fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-ToString"/>
          <argument xmi:type="uml:ValuePin" xmi:id="_numX" name="x">`+integerType+`
            <value xmi:type="uml:LiteralInteger" xmi:id="_numXV" value="-7"/>
          </argument>
          <result xmi:type="uml:OutputPin" xmi:id="_numOut" name="result">`+stringType+`</result>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_real" name="real">
          <behavior xmi:type="uml:FunctionBehavior" href="http://www.omg.org/spec/FUML/20130801/fUML_Library.xmi#PrimitiveBehaviors-RealFunctions-ToString"/>
          <argument xmi:type="uml:ValuePin" xmi:id="_realX" name="x">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
            <value xmi:type="uml:LiteralReal" xmi:id="_realXV" value="2.5"/>
          </argument>
          <result xmi:type="uml:OutputPin" xmi:id="_realOut" name="result">`+stringType+`</result>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_parse" name="parse">
          <behavior xmi:type="uml:FunctionBehavior" href="http://www.omg.org/spec/FUML/20130801/fUML_Library.xmi#PrimitiveBehaviors-BooleanFunctions-ToBoolean"/>
          <argument xmi:type="uml:ValuePin" xmi:id="_parseX" name="x">`+stringType+`
            <value xmi:type="uml:LiteralString" xmi:id="_parseXV" value="true"/>
          </argument>
          <result xmi:type="uml:OutputPin" xmi:id="_parseOut" name="result">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_parseOutL" value="0"/>
          </result>
        </node>`, "flag", "num", "real", "parse"), recorderBlock)
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_flag", migrate.Mapped, "calls fUML BooleanFunctions::ToString, which the v2 library computes")
	wantNote(t, r, "_real", migrate.Approximated, "v1 leaves the text of a real unspecified beyond reading back as the same value; v2 writes the shortest such text")
	wantNote(t, r, "_parse", migrate.Approximated, `v2 reads "true" and "false" only, where v1 reads them in any letter case`)
	s := session(t, r)
	meta(t, s, "%instantiate Recorder")
	wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{
		"flag.result":  `"false"`,
		"num.result":   `"-7"`,
		"real.result":  `"2.5"`,
		"parse.result": "true",
	})
}

// Concat needs both its arguments: fUML fires a call only once each input pin
// holds its lower bound of values (fUML 1.5 §8.10.2, isReady), so a call whose
// y pin is fed by nothing never fires, as any starved action, and a call with no
// pin for y at all, or whose pin may hold none and gets none, is undefined in
// v1: it carries the token on and computes nothing, with the reason in its note.
func TestConcatWithoutAnArgument(t *testing.T) {
	behavior := `<behavior xmi:type="uml:FunctionBehavior" href="` + concatHref + `"/>`
	t.Run("y fed by nothing", func(t *testing.T) {
		r := migrateDocument(t, labeler(concatCall(behavior, `
          <argument xmi:type="uml:InputPin" xmi:id="_concatY" name="y">`+stringType+`</argument>`), "concat"), recorderBlock)
		wantClean(t, "t.sysml", r)
		wantNote(t, r, "_concat", migrate.Approximated, "the action never fires: its input pin 'y' must hold a value, but no object flow feeds it and it holds no value")
		wantLine(t, r.Notation, "/* not migrated: ControlFlow (_c0) — the edge leads to 'concat', which never fires: no value reaches its input pin 'y' */")
	})
	optionalY := `
          <argument xmi:type="uml:InputPin" xmi:id="_concatY" name="y">` + stringType + `
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_concatYL" value="0"/>
          </argument>`
	for _, c := range []struct{ name, y, note string }{
		{"no pin for y", "", "the call passes no argument for the parameter y of fUML StringFunctions::Concat, which must hold a value; v1 leaves the call undefined without it, so the action carries the token and performs nothing"},
		{"y may hold none and gets none", optionalY, "the pin 'y' it passes for the parameter y of fUML StringFunctions::Concat, which must hold a value, may hold none and nothing fills it; v1 leaves the call undefined without it, so the action carries the token and performs nothing"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := migrateDocument(t, labeler(concatCall(behavior, c.y), "concat"), recorderBlock)
			wantClean(t, "t.sysml", r)
			wantNote(t, r, "_concat", migrate.Approximated, c.note)
			wantLine(t, r.Notation, "/* not migrated: CallBehaviorAction 'concat' — "+c.note+" */")
			wantNoLine(t, r.Notation, "StringFunctions::'+'")
			s := session(t, r)
			meta(t, s, "%instantiate Recorder")
			wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"concat.result": ""})
		})
	}
	t.Run("y is a literal", func(t *testing.T) {
		r := migrateDocument(t, labeler(concatCall(behavior, `
          <argument xmi:type="uml:ValuePin" xmi:id="_concatY" name="y">`+stringType+`
            <value xmi:type="uml:LiteralString" xmi:id="_concatYV" value="1"/>
          </argument>`), "concat"), recorderBlock)
		wantClean(t, "t.sysml", r)
		wantNote(t, r, "_concat", migrate.Mapped, "calls fUML StringFunctions::Concat, which the v2 library computes")
		s := session(t, r)
		meta(t, s, "%instantiate Recorder")
		wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"concat.result": `"n=1"`})
	})
}

// A library behavior is known by the href of its library document and its
// fragment, whatever date the document's URI carries — not by name: a model's
// own behavior named Concat is not the library's, while the library's Concat is
// still mapped when the model bundles a copy of it under another name.
func TestLibraryCallsAreKeyedOnTheHref(t *testing.T) {
	literalY := `
          <argument xmi:type="uml:ValuePin" xmi:id="_concatY" name="y">` + stringType + `
            <value xmi:type="uml:LiteralString" xmi:id="_concatYV" value="1"/>
          </argument>`
	t.Run("another date of the library", func(t *testing.T) {
		r := migrateDocument(t, labeler(concatCall(`<behavior xmi:type="uml:FunctionBehavior" href="https://www.omg.org/spec/FUML/1.5/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat"/>`, literalY), "concat"), recorderBlock)
		wantClean(t, "t.sysml", r)
		wantNote(t, r, "_concat", migrate.Mapped, "calls fUML StringFunctions::Concat, which the v2 library computes")
		wantLine(t, r.Notation, "out result : ScalarValues::String = StringFunctions::'+'(x, y);")
	})
	t.Run("a bundled copy of the library", func(t *testing.T) {
		r := migrateDocument(t, labeler(concatCall(`<behavior xmi:type="uml:FunctionBehavior" href="`+concatHref+`"/>`, literalY), "concat")+`
    <packagedElement xmi:type="uml:Package" xmi:id="_lib" name="fUML_Library">
      <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="PrimitiveBehaviors-StringFunctions-Concat" name="Join">
        <ownedParameter xmi:id="_px" name="x" direction="in">`+stringType+`</ownedParameter>
        <ownedParameter xmi:id="_py" name="y" direction="in">`+stringType+`</ownedParameter>
        <ownedParameter xmi:id="_pr" name="result" direction="return">`+stringType+`</ownedParameter>
      </packagedElement>
    </packagedElement>`, recorderBlock)
		wantClean(t, "t.sysml", r)
		wantNote(t, r, "_concat", migrate.Mapped, "calls fUML StringFunctions::Concat, which the v2 library computes")
		wantLine(t, r.Notation, "out result : ScalarValues::String = StringFunctions::'+'(x, y);")
		s := session(t, r)
		meta(t, s, "%instantiate Recorder")
		wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"concat.result": `"n=1"`})
	})
	t.Run("the model's own Concat", func(t *testing.T) {
		r := migrateDocument(t, labeler(concatCall(`<behavior xmi:type="uml:FunctionBehavior" xmi:idref="_own"/>`, literalY), "concat")+`
    <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="_own" name="Concat">
      <ownedParameter xmi:id="_px" name="x" direction="in">`+stringType+`</ownedParameter>
      <ownedParameter xmi:id="_py" name="y" direction="in">`+stringType+`</ownedParameter>
      <ownedParameter xmi:id="_pr" name="result" direction="return">`+stringType+`</ownedParameter>
    </packagedElement>`, recorderBlock)
		wantNoLine(t, r.Notation, "StringFunctions::'+'")
		for _, e := range entriesFor(r, "_concat") {
			if strings.Contains(e.Note, "v2 library computes") {
				t.Errorf("the model's own Concat is noted as the library's: %+v", e)
			}
		}
	})
	t.Run("another document's Concat", func(t *testing.T) {
		r := migrateDocument(t, labeler(concatCall(`<behavior xmi:type="uml:FunctionBehavior" href="http://www.example.com/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat"/>`, literalY), "concat"), recorderBlock)
		wantNoLine(t, r.Notation, "StringFunctions::'+'")
		wantNote(t, r, "_concat", migrate.Unmapped, "has no v2 declaration")
	})
}

// The bundled_library fixture calls the fUML library as MagicDraw exports it: by
// href into the used project fUML-Library.mdzip with the referentPath recorded
// beside it, ListSize and ListGet resolving to the bundled copy of the library,
// ToString and Concat to the referentPath alone. Each is known by that
// provenance, noted, and computes through the v2 library; WriteLine is refused
// and the bundled copy is skipped as library content.
func TestBundledLibraryCallsAreKnownByIdentity(t *testing.T) {
	r := migrateFixtureFile(t, "bundled_library")
	for _, line := range []string{
		"out result : ScalarValues::String = IntegerFunctions::ToString(x);",
		"out result : ScalarValues::String = StringFunctions::'+'(x, y);",
		"out result : ScalarValues::Integer = SequenceFunctions::size(list);",
		"out result : ScalarValues::String = SequenceFunctions::'#'(list, index);",
		"/* not migrated: CallBehaviorAction 'print' — the behavior fUML BasicInputOutput::WriteLine it calls has no v2 library function: writes a line to the standard output channel, which the v2 library has no function for; the behavior is known by the referentPath fUML_Library::BasicInputOutput::WriteLine recorded beside its href into the library module fUML-Library.mdzip */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "package fUML_Library")
	wantNote(t, r, "_size", migrate.Mapped, "calls fUML ListFunctions::ListSize, which the v2 library computes; the behavior is known by the copy of the library the model bundles as fUML-Library.mdzip, which its href resolves to")
	wantNote(t, r, "_get", migrate.Approximated, "calls fUML ListFunctions::ListGet, which the v2 library computes; the behavior is known by the copy of the library the model bundles as fUML-Library.mdzip, which its href resolves to; v2 fails on an index outside 1..ListSize(list)")
	wantNote(t, r, "_toString", migrate.Mapped, "calls fUML IntegerFunctions::ToString, which the v2 library computes; the behavior is known by the referentPath fUML_Library::PrimitiveBehaviors::IntegerFunctions::ToString recorded beside its href into the library module fUML-Library.mdzip")
	wantNote(t, r, "_label", migrate.Approximated, "calls fUML StringFunctions::Concat, which the v2 library computes; the behavior is known by the referentPath fUML_Library::PrimitiveBehaviors::StringFunctions::Concat recorded beside its href into the library module fUML-Library.mdzip")
	wantNote(t, r, "_fumlLibrary", migrate.Skipped, "profile or library content")

	s := session(t, r)
	meta(t, s, "%instantiate Recorder")
	wantValues(t, runValues(t, s, "Recorder::Tally", "Recorder"), map[string]string{
		"toString.result": `"3"`,
		"label.result":    `"n=3"`,
		"size.result":     "1",
		"get.result":      `"n=3"`,
	})
}

// The user_library fixture has a package of the model's own named fUML_Library
// holding a ListSize under PrimitiveBehaviors::ListFunctions, and an href into
// another used project whose referentPath reads like the library's ListGet.
// Neither is the library's: the call to ListSize calls the model's action def,
// and ListGet is refused as any behavior outside the document is.
func TestUserPackageNamedLikeTheLibraryIsNotIt(t *testing.T) {
	r := migrateFixtureFile(t, "user_library")
	wantLine(t, r.Notation, "action size : fUML_Library::PrimitiveBehaviors::ListFunctions::ListSize;")
	wantLine(t, r.Notation, "/* not migrated: CallBehaviorAction 'get' — the behavior fUML_Library::PrimitiveBehaviors::ListFunctions::ListGet it calls has no v2 declaration */")
	wantNoLine(t, r.Notation, "SequenceFunctions::")
	for _, id := range []string{"_size", "_get"} {
		for _, e := range entriesFor(r, id) {
			if strings.Contains(e.Note, "v2 library computes") || strings.Contains(e.Note, "known by") {
				t.Errorf("%s is noted as the library's: %+v", id, e)
			}
		}
	}
	wantNote(t, r, "_get", migrate.Unmapped, "has no v2 declaration")
}

// A call passing more pins than the library behavior has parameters, or taking
// more results than it gives, keeps the extra pins as placeholders with the
// arity in their notes.
func TestLibraryCallWithExtraPins(t *testing.T) {
	r := migrateDocument(t, labeler(`
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_size" name="size">
          <behavior xmi:type="uml:FunctionBehavior" href="http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-Size"/>
          <argument xmi:type="uml:ValuePin" xmi:id="_sizeSeq" name="seq">`+stringType+`
            <value xmi:type="uml:LiteralString" xmi:id="_sizeSeqV" value="a"/>
          </argument>
          <argument xmi:type="uml:ValuePin" xmi:id="_sizeExtra" name="extra">`+stringType+`
            <value xmi:type="uml:LiteralString" xmi:id="_sizeExtraV" value="b"/>
          </argument>
          <result xmi:type="uml:OutputPin" xmi:id="_sizeOut" name="result">`+integerType+`</result>
          <result xmi:type="uml:OutputPin" xmi:id="_sizeMore" name="more">`+integerType+`</result>
        </node>`, "size"), recorderBlock)
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_size", migrate.Mapped, "calls Alf SequenceFunctions::Size, which the v2 library computes")
	wantNote(t, r, "_sizeExtra", migrate.Unmapped, "Alf SequenceFunctions::Size takes 1 argument(s); the pin passes nothing")
	wantNote(t, r, "_sizeMore", migrate.Unmapped, "Alf SequenceFunctions::Size gives 1 result(s); the pin takes nothing")
	wantLine(t, r.Notation, "out result : ScalarValues::Integer = SequenceFunctions::size(seq);")
	s := session(t, r)
	meta(t, s, "%instantiate Recorder")
	wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"size.result": "1", "size.more": ""})
}
