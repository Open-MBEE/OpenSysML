package migrate_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

const alfSeq = "http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-"
const alfColl = "http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-CollectionFunctions-"

// seqPin is an input pin of a sequence call: a string sequence built by a chain
// of Including calls when elements are given, else a literal, or nothing when
// the value is empty.
type seqPin struct {
	name     string
	elements []string
	typ      string
	value    string
}

// libraryCall builds an activity calling the behavior at href once, with the
// pins in order and one result pin, of many values or of one or none.
// literals is the literal of each primitive type's ValuePin.
var literals = map[string]string{
	stringType:  "LiteralString",
	integerType: "LiteralInteger",
	realType:    "LiteralReal",
	booleanType: "LiteralBoolean",
	naturalType: "LiteralUnlimitedNatural",
}

func libraryCall(href string, pins []seqPin, resultType string, many, optional bool) string {
	var nodes, flows string
	var names []string
	for i, p := range pins {
		if p.elements == nil {
			continue
		}
		prev := ""
		for j, el := range p.elements {
			id := "_s" + strconv.Itoa(i) + "_" + strconv.Itoa(j)
			nodes += `
        <node xmi:type="uml:CallBehaviorAction" xmi:id="` + id + `" name="` + id[1:] + `">
          <behavior xmi:type="uml:FunctionBehavior" href="` + alfSeq + `Including"/>
          <argument xmi:type="uml:InputPin" xmi:id="` + id + `seq" name="seq" isOrdered="true" isUnique="false">` + stringType + `
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="` + id + `seqL" value="0"/>
            <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="` + id + `seqU" value="*"/>
          </argument>
          <argument xmi:type="uml:ValuePin" xmi:id="` + id + `el" name="element">` + stringType + `
            <value xmi:type="uml:LiteralString" xmi:id="` + id + `elV" value="` + el + `"/>
          </argument>
          <result xmi:type="uml:OutputPin" xmi:id="` + id + `out" name="result" isOrdered="true" isUnique="false">` + stringType + `
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="` + id + `outL" value="0"/>
            <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="` + id + `outU" value="*"/>
          </result>
        </node>`
			if prev != "" {
				flows += `
        <edge xmi:type="uml:ObjectFlow" xmi:id="_f` + id + `" source="` + prev + `out" target="` + id + `seq"/>`
			}
			prev = id
			names = append(names, id[1:])
		}
		flows += `
        <edge xmi:type="uml:ObjectFlow" xmi:id="_f` + p.name + `" source="` + prev + `out" target="_call` + p.name + `"/>`
	}
	call := `
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_call" name="call">
          <behavior xmi:type="uml:FunctionBehavior" href="` + href + `"/>`
	for _, p := range pins {
		typ := p.typ
		if typ == "" {
			typ = stringType
		}
		switch {
		case p.elements != nil, p.value == "":
			call += `
          <argument xmi:type="uml:InputPin" xmi:id="_call` + p.name + `" name="` + p.name + `" isOrdered="true" isUnique="false">` + typ + `
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_call` + p.name + `L" value="0"/>
            <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_call` + p.name + `U" value="*"/>
          </argument>`
		default:
			call += `
          <argument xmi:type="uml:ValuePin" xmi:id="_call` + p.name + `" name="` + p.name + `">` + typ + `
            <value xmi:type="uml:` + literals[typ] + `" xmi:id="_call` + p.name + `V" value="` + p.value + `"/>
          </argument>`
		}
	}
	mult, order := "", ""
	switch {
	case many:
		order = ` isOrdered="true" isUnique="false"`
		mult = `
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_callOutL" value="0"/>
            <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_callOutU" value="*"/>`
	case optional:
		mult = `
            <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_callOutL" value="0"/>`
	}
	call += `
          <result xmi:type="uml:OutputPin" xmi:id="_callOut" name="result"` + order + `>` + resultType + mult + `
          </result>
        </node>`
	names = append(names, "call")
	return labeler(nodes+call+flows, names...)
}

var (
	aba = []string{"a", "b", "a"}
	bc  = []string{"b", "c"}
)

// joined is the verdict of a call fed by the chains building its pins: two
// chains reach it through a join, which the migration notes as approximated.
func joined(pins []seqPin, v migrate.Verdict) migrate.Verdict {
	chains := 0
	for _, p := range pins {
		if p.elements != nil {
			chains++
		}
	}
	if chains > 1 && v == migrate.Mapped {
		return migrate.Approximated
	}
	return v
}

func seq(name string, elements ...string) seqPin { return seqPin{name: name, elements: elements} }
func str(name, value string) seqPin              { return seqPin{name: name, value: value} }
func num(name, value string) seqPin              { return seqPin{name: name, typ: integerType, value: value} }

// The Alf sequence and collection functions compute their results through the
// v2 sequence functions with the argument order and the one-based positions
// Alf 1.1 Annex B gives them; a sequence parameter fed nothing is the empty
// sequence, and a result of none is no value.
func TestAlfSequenceFunctionsCompute(t *testing.T) {
	for _, c := range []struct {
		fragment string
		pins     []seqPin
		result   string
		many     bool
		optional bool
		want     string
		verdict  migrate.Verdict
		note     string
	}{
		{"Size", []seqPin{seq("seq", aba...)}, integerType, false, false, "3", migrate.Mapped, ""},
		{"Size", []seqPin{seq("seq")}, integerType, false, false, "0", migrate.Mapped, ""},
		{"Includes", []seqPin{seq("seq", aba...), str("element", "b")}, booleanType, false, false, "true", migrate.Mapped, ""},
		{"Excludes", []seqPin{seq("seq", aba...), str("element", "b")}, booleanType, false, false, "false", migrate.Mapped, ""},
		{"Count", []seqPin{seq("seq", aba...), str("element", "a")}, integerType, false, false, "2", migrate.Mapped, ""},
		{"Count", []seqPin{seq("seq", aba...), str("element", "z")}, integerType, false, false, "0", migrate.Mapped, ""},
		{"IsEmpty", []seqPin{seq("seq")}, booleanType, false, false, "true", migrate.Mapped, ""},
		{"NotEmpty", []seqPin{seq("seq", aba...)}, booleanType, false, false, "true", migrate.Mapped, ""},
		{"IncludesAll", []seqPin{seq("seq1", aba...), seq("seq2", "b", "a")}, booleanType, false, false, "true", migrate.Mapped, ""},
		{"IncludesAll", []seqPin{seq("seq1", aba...), seq("seq2", bc...)}, booleanType, false, false, "false", migrate.Mapped, ""},
		{"ExcludesAll", []seqPin{seq("seq1", aba...), seq("seq2", "c", "d")}, booleanType, false, false, "true", migrate.Mapped, ""},
		{"Equals", []seqPin{seq("seq1", aba...), seq("seq2", aba...)}, booleanType, false, false, "true", migrate.Mapped, ""},
		{"Equals", []seqPin{seq("seq1", aba...), seq("seq2", "a", "a", "b")}, booleanType, false, false, "false", migrate.Mapped, ""},
		{"At", []seqPin{seq("seq", aba...), num("index", "2")}, stringType, false, false, `"b"`, migrate.Approximated, "v2 fails on an index outside 1..Size(seq)"},
		{"First", []seqPin{seq("seq", aba...)}, stringType, false, false, `"a"`, migrate.Mapped, ""},
		{"First", []seqPin{seq("seq")}, stringType, false, true, "null", migrate.Mapped, ""},
		{"Last", []seqPin{seq("seq", "a", "b")}, stringType, false, false, `"b"`, migrate.Mapped, ""},
		{"Union", []seqPin{seq("seq1", aba...), seq("seq2", bc...)}, stringType, true, false, `["a", "b", "a", "b", "c"]`, migrate.Mapped, ""},
		{"Intersection", []seqPin{seq("seq1", aba...), seq("seq2", bc...)}, stringType, true, false, `["b"]`, migrate.Mapped, ""},
		{"Difference", []seqPin{seq("seq1", aba...), seq("seq2", bc...)}, stringType, true, false, `["a", "a"]`, migrate.Mapped, ""},
		{"Including", []seqPin{seq("seq", aba...), str("element", "c")}, stringType, true, false, `["a", "b", "a", "c"]`, migrate.Mapped, ""},
		{"IncludeAt", []seqPin{seq("seq", aba...), str("element", "z"), num("index", "2")}, stringType, true, false, `["a", "z", "b", "a"]`, migrate.Approximated, "v2 fails on an index outside 1..Size(seq)+1, where v1 gives seq unchanged"},
		{"IncludeAt", []seqPin{seq("seq", aba...), str("element", "z"), num("index", "4")}, stringType, true, false, `["a", "b", "a", "z"]`, migrate.Approximated, ""},
		{"IncludeAllAt", []seqPin{seq("seq1", aba...), seq("seq2", bc...), num("index", "1")}, stringType, true, false, `["b", "c", "a", "b", "a"]`, migrate.Approximated, ""},
		{"Excluding", []seqPin{seq("seq", aba...), str("element", "a")}, stringType, true, false, `["b"]`, migrate.Mapped, ""},
		{"ExcludeAt", []seqPin{seq("seq", aba...), num("index", "3")}, stringType, true, false, `["a", "b"]`, migrate.Approximated, "v2 fails on an index outside 1..Size(seq), where v1 gives seq unchanged"},
		{"ReplacingAt", []seqPin{seq("seq", aba...), num("index", "1"), str("element", "z")}, stringType, true, false, `["z", "b", "a"]`, migrate.Approximated, "v2 fails on an index outside 1..Size(seq), which v1 requires"},
		{"Subsequence", []seqPin{seq("seq", aba...), num("lower", "2"), num("upper", "3")}, stringType, true, false, `["b", "a"]`, migrate.Approximated, "the bounds are clamped to 1..Size(seq) as in v1"},
		{"Subsequence", []seqPin{seq("seq", aba...), num("lower", "0"), num("upper", "9")}, stringType, true, false, `["a", "b", "a"]`, migrate.Approximated, ""},
		{"Subsequence", []seqPin{seq("seq", aba...), num("lower", "3"), num("upper", "2")}, stringType, true, false, `[]`, migrate.Approximated, ""},
	} {
		name := c.fragment + " " + strings.ReplaceAll(c.want, " ", "")
		t.Run(name, func(t *testing.T) {
			r := migrateDocument(t, libraryCall(alfSeq+c.fragment, c.pins, c.result, c.many, c.optional), recorderBlock)
			wantClean(t, "t.sysml", r)
			wantNote(t, r, "_call", joined(c.pins, c.verdict), c.note)
			s := session(t, r)
			meta(t, s, "%instantiate Recorder")
			wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"call.result": c.want})
		})
	}
}

// The in-place collection functions hand their result back through the inout
// parameter and the result alike; clear gives the empty sequence.
func TestAlfCollectionFunctionsCompute(t *testing.T) {
	for _, c := range []struct {
		fragment string
		pins     []seqPin
		want     string
		verdict  migrate.Verdict
		note     string
	}{
		{"add", []seqPin{seq("seq", aba...), str("element", "c")}, `["a", "b", "a", "c"]`, migrate.Approximated, "the sequence the inout parameter hands back and the result are one value in v2"},
		{"addAll", []seqPin{seq("seq1", aba...), seq("seq2", bc...), num("index", "1")}, `["a", "b", "a", "b", "c"]`, migrate.Approximated, "the library document declares addAll with the in parameters seq1, seq2 and an unused index"},
		{"addAt", []seqPin{seq("seq", aba...), str("element", "z"), num("index", "2")}, `["a", "z", "b", "a"]`, migrate.Approximated, ""},
		{"addAllAt", []seqPin{seq("seq1", aba...), seq("seq2", bc...), num("index", "4")}, `["a", "b", "a", "b", "c"]`, migrate.Approximated, ""},
		{"remove", []seqPin{seq("seq", aba...), str("element", "a")}, `["b"]`, migrate.Approximated, ""},
		{"removeAll", []seqPin{seq("seq1", aba...), seq("seq2", bc...)}, `["a", "a"]`, migrate.Approximated, ""},
		{"removeAt", []seqPin{seq("seq", aba...), num("index", "1")}, `["b", "a"]`, migrate.Approximated, ""},
		{"replaceAt", []seqPin{seq("seq", aba...), num("index", "2"), str("element", "z")}, `["a", "z", "a"]`, migrate.Approximated, ""},
		{"clear", []seqPin{seq("seq", aba...)}, "null", migrate.Mapped, "the inout parameter hands back the empty sequence"},
	} {
		t.Run(c.fragment, func(t *testing.T) {
			r := migrateDocument(t, libraryCall(alfColl+c.fragment, c.pins, stringType, true, false), recorderBlock)
			wantClean(t, "t.sysml", r)
			wantNote(t, r, "_call", joined(c.pins, c.verdict), c.note)
			s := session(t, r)
			meta(t, s, "%instantiate Recorder")
			wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"call.result": c.want})
		})
	}
}

const fuml = "http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-"

func real(name, value string) seqPin { return seqPin{name: name, typ: realType, value: value} }
func boolean(name, value string) seqPin {
	return seqPin{name: name, typ: booleanType, value: value}
}
func natural(name, value string) seqPin {
	return seqPin{name: name, typ: naturalType, value: value}
}

// The fUML primitive functions compute through the v2 library with the results
// fUML 1.5 §9.3 specifies: Div and Mod truncate toward zero, Round takes the
// larger of two nearest integers, Real ToInteger truncates, Substring counts
// characters from 1, and the conversions read and write the v1 texts.
func TestFUMLFunctionsCompute(t *testing.T) {
	for _, c := range []struct {
		fragment string
		pins     []seqPin
		result   string
		want     string
		verdict  migrate.Verdict
		note     string
	}{
		{"IntegerFunctions-Neg", []seqPin{num("x", "3")}, integerType, "-3", migrate.Mapped, ""},
		{"IntegerFunctions-Abs", []seqPin{num("x", "-3")}, integerType, "3", migrate.Mapped, ""},
		{"IntegerFunctions-plus", []seqPin{num("x", "2"), num("y", "3")}, integerType, "5", migrate.Mapped, ""},
		{"IntegerFunctions-minus", []seqPin{num("x", "2"), num("y", "3")}, integerType, "-1", migrate.Mapped, ""},
		{"IntegerFunctions-times", []seqPin{num("x", "2"), num("y", "3")}, integerType, "6", migrate.Mapped, ""},
		{"IntegerFunctions-divide", []seqPin{num("x", "7"), num("y", "2")}, realType, "3.5", migrate.Approximated, "v2 fails on a divisor of 0 where v1 gives no result"},
		{"IntegerFunctions-Div", []seqPin{num("x", "7"), num("y", "2")}, integerType, "3", migrate.Approximated, "the quotient is truncated toward zero, as in v1"},
		{"IntegerFunctions-Div", []seqPin{num("x", "-7"), num("y", "2")}, integerType, "-3", migrate.Approximated, ""},
		{"IntegerFunctions-Mod", []seqPin{num("x", "-7"), num("y", "2")}, integerType, "-1", migrate.Approximated, "v2 fails on a divisor of 0, which v1 leaves undefined"},
		{"IntegerFunctions-Max", []seqPin{num("x", "2"), num("y", "3")}, integerType, "3", migrate.Mapped, ""},
		{"IntegerFunctions-Min", []seqPin{num("x", "2"), num("y", "3")}, integerType, "2", migrate.Mapped, ""},
		{"IntegerFunctions-lt", []seqPin{num("x", "2"), num("y", "3")}, booleanType, "true", migrate.Mapped, ""},
		{"IntegerFunctions-gt", []seqPin{num("x", "2"), num("y", "3")}, booleanType, "false", migrate.Mapped, ""},
		{"IntegerFunctions-le", []seqPin{num("x", "3"), num("y", "3")}, booleanType, "true", migrate.Mapped, ""},
		{"IntegerFunctions-ge", []seqPin{num("x", "2"), num("y", "3")}, booleanType, "false", migrate.Mapped, ""},
		{"IntegerFunctions-ToString", []seqPin{num("x", "1000000")}, stringType, `"1000000"`, migrate.Mapped, ""},
		{"IntegerFunctions-ToUnlimitedNatural", []seqPin{num("x", "4")}, naturalType, "4", migrate.Approximated, "v2 fails on a negative argument"},
		{"IntegerFunctions-ToInteger", []seqPin{str("x", "-12")}, integerType, "-12", migrate.Approximated, "v2 fails on text that is no decimal integer"},
		{"RealFunctions-Neg", []seqPin{real("x", "2.5")}, realType, "-2.5", migrate.Mapped, ""},
		{"RealFunctions-Abs", []seqPin{real("x", "-2.5")}, realType, "2.5", migrate.Mapped, ""},
		{"RealFunctions-Inv", []seqPin{real("x", "4.0")}, realType, "0.25", migrate.Approximated, "v2 fails on an argument of 0.0"},
		{"RealFunctions-Floor", []seqPin{real("x", "-2.5")}, integerType, "-3", migrate.Mapped, ""},
		{"RealFunctions-Floor-Round", []seqPin{real("x", "2.5")}, integerType, "3", migrate.Mapped, "a half rounds to the larger integer, as in v1 (-2.5 to -2)"},
		{"RealFunctions-Floor-Round", []seqPin{real("x", "-2.5")}, integerType, "-2", migrate.Mapped, ""},
		{"RealFunctions-Floor-Round", []seqPin{real("x", "2.4")}, integerType, "2", migrate.Mapped, ""},
		{"RealFunctions-plus", []seqPin{real("x", "2.5"), real("y", "1.5")}, realType, "4.0", migrate.Mapped, ""},
		{"RealFunctions-minus", []seqPin{real("x", "2.5"), real("y", "1.5")}, realType, "1.0", migrate.Mapped, ""},
		{"RealFunctions-times", []seqPin{real("x", "2.5"), real("y", "2.0")}, realType, "5.0", migrate.Mapped, ""},
		{"RealFunctions-divide", []seqPin{real("x", "1.0"), real("y", "4.0")}, realType, "0.25", migrate.Approximated, "v2 fails on a divisor of 0.0"},
		{"RealFunctions-Max", []seqPin{real("x", "2.5"), real("y", "1.5")}, realType, "2.5", migrate.Mapped, ""},
		{"RealFunctions-Min", []seqPin{real("x", "2.5"), real("y", "1.5")}, realType, "1.5", migrate.Mapped, ""},
		{"RealFunctions-lt", []seqPin{real("x", "2.5"), real("y", "1.5")}, booleanType, "false", migrate.Mapped, ""},
		{"RealFunctions-gt", []seqPin{real("x", "2.5"), real("y", "1.5")}, booleanType, "true", migrate.Mapped, ""},
		{"RealFunctions-le", []seqPin{real("x", "2.5"), real("y", "2.5")}, booleanType, "true", migrate.Mapped, ""},
		{"RealFunctions-ge", []seqPin{real("x", "2.5"), real("y", "1.5")}, booleanType, "true", migrate.Mapped, ""},
		{"RealFunctions-ToString", []seqPin{real("x", "0.1")}, stringType, `"0.1"`, migrate.Approximated, "v1 leaves the text of a real unspecified beyond reading back as the same value"},
		{"RealFunctions-ToInteger", []seqPin{real("x", "-2.7")}, integerType, "-2", migrate.Mapped, ""},
		{"RealFunctions-ToInteger", []seqPin{real("x", "2.7")}, integerType, "2", migrate.Mapped, ""},
		{"RealFunctions-ToReal", []seqPin{str("x", "2.5")}, realType, "2.5", migrate.Approximated, "v2 fails on text that is no real number"},
		{"UnlimitedNaturalFunctions-Max", []seqPin{natural("x", "2"), natural("y", "3")}, naturalType, "3", migrate.Approximated, "v2 Natural has no unbounded value"},
		{"UnlimitedNaturalFunctions-Min", []seqPin{natural("x", "2"), natural("y", "3")}, naturalType, "2", migrate.Approximated, ""},
		{"UnlimitedNaturalFunctions-lt", []seqPin{natural("x", "2"), natural("y", "3")}, booleanType, "true", migrate.Approximated, ""},
		{"UnlimitedNaturalFunctions-gt", []seqPin{natural("x", "2"), natural("y", "3")}, booleanType, "false", migrate.Approximated, ""},
		{"UnlimitedNaturalFunctions-le", []seqPin{natural("x", "3"), natural("y", "3")}, booleanType, "true", migrate.Approximated, ""},
		{"UnlimitedNaturalFunctions-ge", []seqPin{natural("x", "2"), natural("y", "3")}, booleanType, "false", migrate.Approximated, ""},
		{"UnlimitedNaturalFunctions-ToString", []seqPin{natural("x", "5")}, stringType, `"5"`, migrate.Approximated, `where v1 writes "*"`},
		{"UnlimitedNaturalFunctions-ToInteger", []seqPin{natural("x", "5")}, integerType, "5", migrate.Approximated, "a v2 Natural is an Integer and passes through unchanged"},
		{"UnlimitedNaturalFunctions-ToUnlimitedNatural", []seqPin{str("x", "5")}, naturalType, "5", migrate.Approximated, `v2 fails on text that is no decimal natural, "*" included`},
		{"BooleanFunctions-Or", []seqPin{boolean("x", "false"), boolean("y", "true")}, booleanType, "true", migrate.Mapped, ""},
		{"BooleanFunctions-Xor", []seqPin{boolean("x", "true"), boolean("y", "true")}, booleanType, "false", migrate.Mapped, ""},
		{"BooleanFunctions-And", []seqPin{boolean("x", "true"), boolean("y", "false")}, booleanType, "false", migrate.Mapped, ""},
		{"BooleanFunctions-Implies", []seqPin{boolean("x", "true"), boolean("y", "false")}, booleanType, "false", migrate.Mapped, ""},
		{"BooleanFunctions-Implies", []seqPin{boolean("x", "false"), boolean("y", "false")}, booleanType, "true", migrate.Mapped, ""},
		{"BooleanFunctions-Not", []seqPin{boolean("x", "true")}, booleanType, "false", migrate.Mapped, ""},
		{"BooleanFunctions-ToString", []seqPin{boolean("x", "true")}, stringType, `"true"`, migrate.Mapped, ""},
		{"BooleanFunctions-ToBoolean", []seqPin{str("x", "false")}, booleanType, "false", migrate.Approximated, `v2 reads "true" and "false" only`},
		{"StringFunctions-Concat", []seqPin{str("x", "ab"), str("y", "cd")}, stringType, `"abcd"`, migrate.Mapped, ""},
		{"StringFunctions-Size", []seqPin{str("x", "hello")}, integerType, "5", migrate.Mapped, ""},
		{"StringFunctions-Substring", []seqPin{str("x", "hello"), num("lower", "2"), num("upper", "4")}, stringType, `"ell"`, migrate.Approximated, "v2 fails on bounds outside 1..Size(x) or a lower bound above the upper"},
	} {
		t.Run(c.fragment+" "+c.want, func(t *testing.T) {
			r := migrateDocument(t, libraryCall(fuml+c.fragment, c.pins, c.result, false, false), recorderBlock)
			wantClean(t, "t.sysml", r)
			wantNote(t, r, "_call", c.verdict, c.note)
			s := session(t, r)
			meta(t, s, "%instantiate Recorder")
			wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"call.result": c.want})
		})
	}
}

// The fUML list functions (fUML 1.5 Table 9.7) take their lists as sequences,
// index from 1, and ListConcat keeps every value of both lists in order.
func TestFUMLListFunctionsCompute(t *testing.T) {
	for _, c := range []struct {
		fragment string
		pins     []seqPin
		result   string
		many     bool
		want     string
		verdict  migrate.Verdict
		note     string
	}{
		{"ListSize", []seqPin{seq("list", aba...)}, integerType, false, "3", migrate.Mapped, ""},
		{"ListSize", []seqPin{seq("list")}, integerType, false, "0", migrate.Mapped, ""},
		{"ListGet", []seqPin{seq("list", aba...), num("index", "2")}, stringType, false, `"b"`, migrate.Approximated, "v2 fails on an index outside 1..ListSize(list)"},
		{"ListConcat", []seqPin{seq("list1", aba...), seq("list2", bc...)}, stringType, true, `["a", "b", "a", "b", "c"]`, migrate.Mapped, ""},
		{"ListConcat", []seqPin{seq("list1", aba...), seq("list2")}, stringType, true, `["a", "b", "a"]`, migrate.Mapped, ""},
	} {
		name := c.fragment + " " + strings.ReplaceAll(c.want, " ", "")
		t.Run(name, func(t *testing.T) {
			r := migrateDocument(t, libraryCall(fuml+"ListFunctions-"+c.fragment, c.pins, c.result, c.many, false), recorderBlock)
			wantClean(t, "t.sysml", r)
			wantNote(t, r, "_call", joined(c.pins, c.verdict), c.note)
			s := session(t, r)
			meta(t, s, "%instantiate Recorder")
			wantValues(t, runValues(t, s, "Recorder::Label", "Recorder"), map[string]string{"call.result": c.want})
		})
	}
}
