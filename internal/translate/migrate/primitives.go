package migrate

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// primitive is how a call to a behavior of the fUML or Alf standard library is
// written: each result pin takes a v2 library expression over the argument pins.
// A behavior is known by the library document its href names and the fragment
// within it, whatever date the URI carries.
type primitive struct {
	lib      primitiveLib
	family   string   // the v1 package holding the behavior
	name     string   // the behavior's name within its family
	fragment string   // the fragment identifying the behavior in its library document
	ins      []string // the in and inout parameters in v1 order; a * prefix marks a sequence, which fires on no value
	outs     []string // one v2 expression per out, inout or return parameter in v1 order, naming the arguments $1, $2, ...; nil refuses the call
	verdict  Verdict  // Mapped, Approximated, or Unmapped when outs is nil
	note     string   // the semantic difference, or why the call is refused
}

// primitiveLib is one of the two standard-library documents whose behaviors are mapped.
type primitiveLib int

const (
	fumlLib primitiveLib = iota
	alfLib
)

// String is the name of the library, as its specification calls it.
func (l primitiveLib) String() string {
	if l == alfLib {
		return "Alf"
	}
	return "fUML"
}

// libraryHref recognizes the href of an element of a standard-library document:
// the fUML library, http://www.omg.org/spec/FUML/<date>/fUML_Library.xmi, or the
// Alf library, http://www.omg.org/spec/ALF/<date>/Alf-Library.xmi, keyed on the
// library and the document name whatever the date, then by fragment.
var libraryHref = regexp.MustCompile(`^https?://www\.omg\.org/spec/(FUML/[^/]+/fUML_Library\.xmi|ALF/[^/]+/Alf-Library\.xmi)#(.+)$`)

// primitiveAt returns the primitive an href names, or nil when the href points
// elsewhere or the two libraries hold no such behavior.
func primitiveAt(href string) *primitive {
	m := libraryHref.FindStringSubmatch(href)
	if m == nil {
		return nil
	}
	lib := fumlLib
	if strings.HasPrefix(m[1], "ALF/") {
		lib = alfLib
	}
	return primitiveIndex[primitiveKey{lib, m[2]}]
}

// primitiveKey identifies a primitive by its library and the fragment naming it there.
type primitiveKey struct {
	lib      primitiveLib
	fragment string
}

var primitiveIndex = indexPrimitives()

func indexPrimitives() map[primitiveKey]*primitive {
	index := make(map[primitiveKey]*primitive, len(primitives))
	for i := range primitives {
		p := &primitives[i]
		index[primitiveKey{p.lib, p.fragment}] = p
	}
	return index
}

// qualified is the behavior's name as its library qualifies it, for notes.
func (p *primitive) qualified() string {
	return p.lib.String() + " " + p.family + "::" + p.name
}

// arguments lists the in parameters of the primitive with their names and
// whether each must hold a value: a sequence parameter fires on none.
func (p *primitive) arguments() []primitiveArg {
	args := make([]primitiveArg, len(p.ins))
	for i, in := range p.ins {
		seq := strings.HasPrefix(in, "*")
		args[i] = primitiveArg{name: strings.TrimPrefix(in, "*"), required: !seq}
	}
	return args
}

// primitiveArg is an in parameter of a primitive: its v1 name, and whether the
// call waits for a value on its pin before it fires.
type primitiveArg struct {
	name     string
	required bool
}

// result writes the v2 expression of the primitive's i-th result over the given
// argument expressions, one per in parameter in v1 order.
func (p *primitive) result(i int, args []string) string {
	expr := p.outs[i]
	for j := len(args); j >= 1; j-- {
		expr = strings.ReplaceAll(expr, "$"+strconv.Itoa(j), args[j-1])
	}
	return expr
}

// primitiveCalled returns the standard-library primitive a call behavior action
// calls, by the href of its behavior: a proxy for a document the model does not
// bundle, or the raw href kept when the model bundles the library and the href
// resolved to its copy. nil when the call names no primitive.
func (m *migration) primitiveCalled(n *sysmlv1.Element) *primitive {
	for _, id := range n.RefIDs("behavior") {
		if p := primitiveAt(id); p != nil {
			return p
		}
	}
	return nil
}

// The fUML notes below follow the fUML 1.5 specification, §9.3 (the primitive
// behaviors) and §9.4 (basic input and output); the Alf notes follow Alf 1.1,
// §11.4 (primitive behaviors), §11.6 (collection functions) and Annex B.

const (
	unlimitedNote     = "v2 Natural has no unbounded value, so an argument of * has no v2 rendering; bounded values compare as in v1"
	noResultOnFailure = " where v1 gives no result"
	bitStringNote     = "v2 has no BitString type: no library function performs bitwise operations"
	indexOfNote       = "the v2 library has no function giving the position of an element in a sequence"
	excludingOneNote  = "the v2 library removes every occurrence of an element from a sequence (excluding); none removes the first alone"
	replacingNote     = "the v2 library has no function replacing the occurrences of an element in a sequence"
	replacingOneNote  = "the v2 library has no function replacing the first occurrence of an element in a sequence"
	orderedSetNote    = "the v2 library has no function removing the repeated elements of a sequence"
	indexNote         = "v2 fails on an index outside 1..Size(seq)" + noResultOnFailure
	removeIndexNote   = "v2 fails on an index outside 1..Size(seq), where v1 gives seq unchanged"
	insertIndexNote   = "v2 fails on an index outside 1..Size(seq)+1, where v1 gives seq unchanged"
	subsequenceNote   = "the bounds are clamped to 1..Size(seq) as in v1; v2 fails on a lower bound above Size(seq), which v1 leaves undefined"
	subsequenceExpr   = "SequenceFunctions::subsequence($1, IntegerFunctions::max($2, 1), IntegerFunctions::min($3, SequenceFunctions::size($1)))"
	countExpr         = "IntegerFunctions::'-'(SequenceFunctions::size($1), SequenceFunctions::size(SequenceFunctions::excluding($1, $2)))"
	replacingAtExpr   = "SequenceFunctions::includingAt(SequenceFunctions::excludingAt($1, $2), $3, $2)"
	realToStringNote  = "v1 leaves the text of a real unspecified beyond reading back as the same value; v2 writes the shortest such text, with an exponent for large or small magnitudes (1e+21)"
	toBooleanNote     = "v2 reads \"true\" and \"false\" only, where v1 reads them in any letter case, and fails on other text" + noResultOnFailure
	toNaturalNote     = "v2 reads decimal text only, where v1 also reads the 0b, 0o and 0x forms of a natural literal, and fails on other text" + noResultOnFailure
	inPlaceNote       = "the sequence the inout parameter hands back and the result are one value in v2, as the in-place function assigns them in v1"
	addAllNote        = "the library document declares addAll with the in parameters seq1, seq2 and an unused index, and one result: seq1 with seq2 appended"
	inPlaceIndexNote  = removeIndexNote + "; " + inPlaceNote
	replaceIndexNote  = "v2 fails on an index outside 1..Size(seq), which v1 requires; " + inPlaceNote
	inPlaceInsertNote = insertIndexNote + "; " + inPlaceNote
)

// The argument lists the library behaviors share.
var (
	argX      = []string{"x"}
	argXY     = []string{"x", "y"}
	argSeq    = []string{"*seq"}
	argSeqEl  = []string{"*seq", "element"}
	argSeqs   = []string{"*seq1", "*seq2"}
	argSeqAt  = []string{"*seq", "index"}
	argInsert = []string{"*seq", "element", "index"}
	argInsAll = []string{"*seq1", "*seq2", "index"}
	argReplAt = []string{"*seq", "index", "element"}
	argRepl   = []string{"*seq", "element", "newElement"}
)

// fumlP builds an fUML primitive, whose fragment is PrimitiveBehaviors-<family>-<name>.
func fumlP(family, name string, ins []string, out string, v Verdict, note string) primitive {
	return primitive{lib: fumlLib, family: family, name: name, fragment: "PrimitiveBehaviors-" + family + "-" + name, ins: ins, outs: []string{out}, verdict: v, note: note}
}

// fumlNo builds an fUML library behavior whose calls are refused.
func fumlNo(fragment, family, name string, ins []string, note string) primitive {
	return primitive{lib: fumlLib, family: family, name: name, fragment: fragment, ins: ins, verdict: Unmapped, note: note}
}

// alfP builds an Alf primitive, whose fragment is Alf-Library-PrimitiveBehaviors-<family>-<name>.
func alfP(family, name string, ins []string, out string, v Verdict, note string) primitive {
	return primitive{lib: alfLib, family: family, name: name, fragment: "Alf-Library-PrimitiveBehaviors-" + family + "-" + name, ins: ins, outs: []string{out}, verdict: v, note: note}
}

// alfNo builds an Alf primitive whose calls are refused; its fragment is that of alfP.
func alfNo(family, name string, ins []string, note string) primitive {
	return primitive{lib: alfLib, family: family, name: name, fragment: "Alf-Library-PrimitiveBehaviors-" + family + "-" + name, ins: ins, verdict: Unmapped, note: note}
}

// alfBits builds a BitStringFunctions primitive, refused; its fragment names the
// operator in words where the name is punctuation.
func alfBits(fragment, name string, ins []string) primitive {
	return primitive{lib: alfLib, family: "BitStringFunctions", name: name, fragment: "Alf-Library-PrimitiveBehaviors-BitStringFunctions-" + fragment, ins: ins, verdict: Unmapped, note: bitStringNote}
}

// alfC builds a CollectionFunctions template behavior, whose fragment is
// Alf-Library-CollectionFunctions-<name>; each out and return parameter takes
// the same expression, and none refuses the call.
func alfC(name string, ins []string, outs []string, v Verdict, note string) primitive {
	return primitive{lib: alfLib, family: "CollectionFunctions", name: name, fragment: "Alf-Library-CollectionFunctions-" + name, ins: ins, outs: outs, verdict: v, note: note}
}

// inPlace is the outs of an Alf in-place collection function: the inout
// sequence handed back, then the return, both the same value.
func inPlace(expr string) []string { return []string{expr, expr} }

// primitives is the mapping of every behavior the fUML and Alf library documents
// hold to the v2 library, or the reason the call is refused.
var primitives = []primitive{
	// fUML IntegerFunctions (fUML 1.5 §9.3.2)
	fumlP("IntegerFunctions", "Neg", argX, "IntegerFunctions::'-'($1)", Mapped, ""),
	fumlP("IntegerFunctions", "Abs", argX, "IntegerFunctions::abs($1)", Mapped, ""),
	fumlP("IntegerFunctions", "plus", argXY, "IntegerFunctions::'+'($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "minus", argXY, "IntegerFunctions::'-'($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "times", argXY, "IntegerFunctions::'*'($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "divide", argXY, "RealFunctions::'/'($1, $2)", Approximated, "v2 fails on a divisor of 0"+noResultOnFailure),
	fumlP("IntegerFunctions", "Div", argXY, "RealFunctions::ToInteger(IntegerFunctions::'/'($1, $2))", Approximated, "the quotient is truncated toward zero, as in v1; v2 fails on a divisor of 0"+noResultOnFailure),
	fumlP("IntegerFunctions", "Mod", argXY, "IntegerFunctions::'%'($1, $2)", Approximated, "v2 fails on a divisor of 0, which v1 leaves undefined"),
	fumlP("IntegerFunctions", "Max", argXY, "IntegerFunctions::max($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "Min", argXY, "IntegerFunctions::min($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "lt", argXY, "IntegerFunctions::'<'($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "gt", argXY, "IntegerFunctions::'>'($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "le", argXY, "IntegerFunctions::'<='($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "ge", argXY, "IntegerFunctions::'>='($1, $2)", Mapped, ""),
	fumlP("IntegerFunctions", "ToString", argX, "IntegerFunctions::ToString($1)", Mapped, ""),
	fumlP("IntegerFunctions", "ToUnlimitedNatural", argX, "IntegerFunctions::ToNatural($1)", Approximated, "v2 fails on a negative argument"+noResultOnFailure),
	fumlP("IntegerFunctions", "ToInteger", argX, "IntegerFunctions::ToInteger($1)", Approximated, "v2 fails on text that is no decimal integer"+noResultOnFailure),

	// fUML RealFunctions (fUML 1.5 §9.3.3)
	fumlP("RealFunctions", "Neg", argX, "RealFunctions::'-'($1)", Mapped, ""),
	fumlP("RealFunctions", "Abs", argX, "RealFunctions::abs($1)", Mapped, ""),
	fumlP("RealFunctions", "Inv", argX, "RealFunctions::'/'(1.0, $1)", Approximated, "v2 fails on an argument of 0.0"+noResultOnFailure),
	fumlP("RealFunctions", "Floor", argX, "RealFunctions::floor($1)", Mapped, ""),
	// Round's fragment is nested under Floor's in the library document.
	{lib: fumlLib, family: "RealFunctions", name: "Round", fragment: "PrimitiveBehaviors-RealFunctions-Floor-Round", ins: argX, outs: []string{"RealFunctions::floor(RealFunctions::'+'($1, 0.5))"}, verdict: Mapped, note: "a half rounds to the larger integer, as in v1 (-2.5 to -2); the v2 round would round it away from zero"},
	fumlP("RealFunctions", "plus", argXY, "RealFunctions::'+'($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "minus", argXY, "RealFunctions::'-'($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "times", argXY, "RealFunctions::'*'($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "divide", argXY, "RealFunctions::'/'($1, $2)", Approximated, "v2 fails on a divisor of 0.0"+noResultOnFailure),
	fumlP("RealFunctions", "Max", argXY, "RealFunctions::max($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "Min", argXY, "RealFunctions::min($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "lt", argXY, "RealFunctions::'<'($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "gt", argXY, "RealFunctions::'>'($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "le", argXY, "RealFunctions::'<='($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "ge", argXY, "RealFunctions::'>='($1, $2)", Mapped, ""),
	fumlP("RealFunctions", "ToString", argX, "RealFunctions::ToString($1)", Approximated, realToStringNote),
	fumlP("RealFunctions", "ToInteger", argX, "RealFunctions::ToInteger($1)", Mapped, ""),
	fumlP("RealFunctions", "ToReal", argX, "RealFunctions::ToReal($1)", Approximated, "v2 fails on text that is no real number"+noResultOnFailure),

	// fUML UnlimitedNaturalFunctions (fUML 1.5 §9.3.4)
	fumlP("UnlimitedNaturalFunctions", "Max", argXY, "NaturalFunctions::max($1, $2)", Approximated, unlimitedNote),
	fumlP("UnlimitedNaturalFunctions", "Min", argXY, "NaturalFunctions::min($1, $2)", Approximated, unlimitedNote),
	fumlP("UnlimitedNaturalFunctions", "lt", argXY, "NaturalFunctions::'<'($1, $2)", Approximated, unlimitedNote),
	fumlP("UnlimitedNaturalFunctions", "gt", argXY, "NaturalFunctions::'>'($1, $2)", Approximated, unlimitedNote),
	fumlP("UnlimitedNaturalFunctions", "le", argXY, "NaturalFunctions::'<='($1, $2)", Approximated, unlimitedNote),
	fumlP("UnlimitedNaturalFunctions", "ge", argXY, "NaturalFunctions::'>='($1, $2)", Approximated, unlimitedNote),
	fumlP("UnlimitedNaturalFunctions", "ToString", argX, "NaturalFunctions::ToString($1)", Approximated, "v2 Natural has no unbounded value, so an argument of * has no v2 rendering, where v1 writes \"*\""),
	fumlP("UnlimitedNaturalFunctions", "ToInteger", argX, "$1", Approximated, "a v2 Natural is an Integer and passes through unchanged; v2 has no unbounded value, for which v1 gives no result"),
	fumlP("UnlimitedNaturalFunctions", "ToUnlimitedNatural", argX, "NaturalFunctions::ToNatural($1)", Approximated, "v2 fails on text that is no decimal natural, \"*\" included,"+noResultOnFailure),

	// fUML BooleanFunctions (fUML 1.5 §9.3.5)
	fumlP("BooleanFunctions", "Or", argXY, "BooleanFunctions::'|'($1, $2)", Mapped, ""),
	fumlP("BooleanFunctions", "Xor", argXY, "BooleanFunctions::xor($1, $2)", Mapped, ""),
	fumlP("BooleanFunctions", "And", argXY, "BooleanFunctions::'&'($1, $2)", Mapped, ""),
	fumlP("BooleanFunctions", "Implies", argXY, "ControlFunctions::implies($1, $2)", Mapped, ""),
	fumlP("BooleanFunctions", "Not", argX, "BooleanFunctions::not($1)", Mapped, ""),
	fumlP("BooleanFunctions", "ToString", argX, "BooleanFunctions::ToString($1)", Mapped, ""),
	fumlP("BooleanFunctions", "ToBoolean", argX, "BooleanFunctions::ToBoolean($1)", Approximated, toBooleanNote),

	// fUML StringFunctions (fUML 1.5 §9.3.6)
	fumlP("StringFunctions", "Concat", argXY, "StringFunctions::'+'($1, $2)", Mapped, ""),
	fumlP("StringFunctions", "Size", argX, "StringFunctions::Length($1)", Mapped, ""),
	fumlP("StringFunctions", "Substring", []string{"x", "lower", "upper"}, "StringFunctions::Substring($1, $2, $3)", Approximated, "v2 fails on bounds outside 1..Size(x) or a lower bound above the upper"+noResultOnFailure),

	// fUML ListFunctions (fUML 1.5 §9.3.7): the library document holds these two
	fumlP("ListFunctions", "ListSize", []string{"*list"}, "SequenceFunctions::size($1)", Mapped, ""),
	fumlP("ListFunctions", "ListGet", []string{"*list", "index"}, "SequenceFunctions::'#'($1, $2)", Approximated, "v2 fails on an index outside 1..ListSize(list)"+noResultOnFailure),

	// fUML BasicInputOutput (fUML 1.5 §9.4): activities over the standard channels
	fumlNo("BasicInputOutput-WriteLine", "BasicInputOutput", "WriteLine", []string{"value"}, "writes a line to the standard output channel, which the v2 library has no function for"),
	fumlNo("BasicInputOutput-ReadLine", "BasicInputOutput", "ReadLine", nil, "reads a line from the standard input channel, which the v2 library has no function for"),

	// Alf IntegerFunctions (Alf 1.1 §11.4.3), beyond those imported from fUML
	alfP("IntegerFunctions", "ToNatural", argX, "NaturalFunctions::ToNatural($1)", Approximated, toNaturalNote),

	// Alf BitStringFunctions (Alf 1.1 §11.4.7)
	alfBits("IsSet", "IsSet", []string{"b", "n"}),
	alfBits("BitLength", "BitLength", nil), // the document declares the return alone
	alfBits("ToBitString", "ToBitString", []string{"n"}),
	alfBits("ToInteger", "ToInteger", []string{"b"}),
	alfBits("ToHexString", "ToHexString", []string{"b"}),
	alfBits("ToOctalString", "ToOctalString", []string{"b"}),
	alfBits("tilde", "~", []string{"b"}),
	alfBits("amp", "&", []string{"b1", "b2"}),
	alfBits("caret", "^", []string{"b1", "b2"}),
	alfBits("bar", "|", []string{"b1", "b2"}),
	alfBits("ltlt", "<<", []string{"b", "n"}),
	alfBits("gtgt", ">>", []string{"b", "n"}),
	alfBits("gtgtgt", ">>>", []string{"b", "n"}),

	// Alf SequenceFunctions (Alf 1.1 §11.4.8)
	alfP("SequenceFunctions", "Size", argSeq, "SequenceFunctions::size($1)", Mapped, ""),
	alfP("SequenceFunctions", "Includes", argSeqEl, "SequenceFunctions::includes($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "Excludes", argSeqEl, "SequenceFunctions::excludes($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "Count", argSeqEl, countExpr, Mapped, ""),
	alfP("SequenceFunctions", "IsEmpty", argSeq, "SequenceFunctions::isEmpty($1)", Mapped, ""),
	alfP("SequenceFunctions", "NotEmpty", argSeq, "SequenceFunctions::notEmpty($1)", Mapped, ""),
	alfP("SequenceFunctions", "IncludesAll", argSeqs, "SequenceFunctions::includes($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "ExcludesAll", argSeqs, "SequenceFunctions::excludes($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "Equals", argSeqs, "SequenceFunctions::equals($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "At", argSeqAt, "SequenceFunctions::'#'($1, $2)", Approximated, indexNote),
	alfNo("SequenceFunctions", "IndexOf", argSeqEl, indexOfNote),
	alfP("SequenceFunctions", "First", argSeq, "SequenceFunctions::head($1)", Mapped, ""),
	alfP("SequenceFunctions", "Last", argSeq, "SequenceFunctions::last($1)", Mapped, ""),
	alfP("SequenceFunctions", "Union", argSeqs, "SequenceFunctions::union($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "Intersection", argSeqs, "SequenceFunctions::intersection($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "Difference", argSeqs, "SequenceFunctions::excluding($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "Including", argSeqEl, "SequenceFunctions::including($1, $2)", Mapped, ""),
	alfP("SequenceFunctions", "IncludeAt", argInsert, "SequenceFunctions::includingAt($1, $2, $3)", Approximated, insertIndexNote),
	alfP("SequenceFunctions", "InsertAt", argInsert, "SequenceFunctions::includingAt($1, $2, $3)", Approximated, insertIndexNote),
	alfP("SequenceFunctions", "IncludeAllAt", argInsAll, "SequenceFunctions::includingAt($1, $2, $3)", Approximated, insertIndexNote),
	alfP("SequenceFunctions", "Excluding", argSeqEl, "SequenceFunctions::excluding($1, $2)", Mapped, ""),
	alfNo("SequenceFunctions", "ExcludingOne", argSeqEl, excludingOneNote),
	alfP("SequenceFunctions", "ExcludeAt", argSeqAt, "SequenceFunctions::excludingAt($1, $2)", Approximated, removeIndexNote),
	alfNo("SequenceFunctions", "Replacing", argRepl, replacingNote),
	alfP("SequenceFunctions", "ReplacingAt", argReplAt, replacingAtExpr, Approximated, "v2 fails on an index outside 1..Size(seq), which v1 requires"),
	alfNo("SequenceFunctions", "ReplacingOne", argRepl, replacingOneNote),
	alfP("SequenceFunctions", "Subsequence", []string{"*seq", "lower", "upper"}, subsequenceExpr, Approximated, subsequenceNote),
	alfNo("SequenceFunctions", "ToOrderedSet", argSeq, orderedSetNote),

	// Alf CollectionFunctions (Alf 1.1 §11.6): the template versions of the sequence functions
	alfC("size", argSeq, []string{"SequenceFunctions::size($1)"}, Mapped, ""),
	alfC("includes", argSeqEl, []string{"SequenceFunctions::includes($1, $2)"}, Mapped, ""),
	alfC("excludes", argSeqEl, []string{"SequenceFunctions::excludes($1, $2)"}, Mapped, ""),
	alfC("count", argSeqEl, []string{countExpr}, Mapped, ""),
	alfC("isEmpty", argSeq, []string{"SequenceFunctions::isEmpty($1)"}, Mapped, ""),
	alfC("notEmpty", argSeq, []string{"SequenceFunctions::notEmpty($1)"}, Mapped, ""),
	alfC("includesAll", argSeqs, []string{"SequenceFunctions::includes($1, $2)"}, Mapped, ""),
	alfC("excludesAll", argSeqs, []string{"SequenceFunctions::excludes($1, $2)"}, Mapped, ""),
	alfC("equals", argSeqs, []string{"SequenceFunctions::equals($1, $2)"}, Mapped, ""),
	alfC("at", argSeqAt, []string{"SequenceFunctions::'#'($1, $2)"}, Approximated, indexNote),
	alfC("indexOf", argSeqEl, nil, Unmapped, indexOfNote),
	alfC("first", argSeq, []string{"SequenceFunctions::head($1)"}, Mapped, ""),
	alfC("last", argSeq, []string{"SequenceFunctions::last($1)"}, Mapped, ""),
	alfC("union", argSeqs, []string{"SequenceFunctions::union($1, $2)"}, Mapped, ""),
	alfC("intersection", argSeqs, []string{"SequenceFunctions::intersection($1, $2)"}, Mapped, ""),
	alfC("difference", argSeqs, []string{"SequenceFunctions::excluding($1, $2)"}, Mapped, ""),
	alfC("including", argSeqEl, []string{"SequenceFunctions::including($1, $2)"}, Mapped, ""),
	alfC("includeAt", argInsert, []string{"SequenceFunctions::includingAt($1, $2, $3)"}, Approximated, insertIndexNote),
	alfC("insertAt", argInsert, []string{"SequenceFunctions::includingAt($1, $2, $3)"}, Approximated, insertIndexNote),
	alfC("includeAllAt", argInsAll, []string{"SequenceFunctions::includingAt($1, $2, $3)"}, Approximated, insertIndexNote),
	alfC("excluding", argSeqEl, []string{"SequenceFunctions::excluding($1, $2)"}, Mapped, ""),
	alfC("excludingOne", argSeqEl, nil, Unmapped, excludingOneNote),
	alfC("excludeAt", argSeqAt, []string{"SequenceFunctions::excludingAt($1, $2)"}, Approximated, removeIndexNote),
	alfC("replacing", argRepl, nil, Unmapped, replacingNote),
	alfC("replacingAt", argReplAt, []string{replacingAtExpr}, Approximated, "v2 fails on an index outside 1..Size(seq), which v1 requires"),
	alfC("replacingOne", argRepl, nil, Unmapped, replacingOneNote),
	alfC("subsequence", []string{"*seq", "lower", "upper"}, []string{subsequenceExpr}, Approximated, subsequenceNote),
	alfC("toOrderedSet", argSeq, nil, Unmapped, orderedSetNote),

	// Alf CollectionFunctions in-place behaviors (Alf 1.1 §11.6, Table 11.9)
	alfC("add", argSeqEl, inPlace("SequenceFunctions::including($1, $2)"), Approximated, inPlaceNote),
	alfC("addAll", argInsAll, []string{"SequenceFunctions::union($1, $2)"}, Approximated, addAllNote),
	alfC("addAt", argInsert, inPlace("SequenceFunctions::includingAt($1, $2, $3)"), Approximated, inPlaceInsertNote),
	alfC("addAllAt", argInsAll, inPlace("SequenceFunctions::includingAt($1, $2, $3)"), Approximated, inPlaceInsertNote),
	alfC("remove", argSeqEl, inPlace("SequenceFunctions::excluding($1, $2)"), Approximated, inPlaceNote),
	alfC("removeAll", argSeqs, inPlace("SequenceFunctions::excluding($1, $2)"), Approximated, inPlaceNote),
	alfC("removeOne", argSeqEl, nil, Unmapped, excludingOneNote),
	alfC("removeAt", argSeqAt, inPlace("SequenceFunctions::excludingAt($1, $2)"), Approximated, inPlaceIndexNote),
	alfC("replace", argRepl, nil, Unmapped, replacingNote),
	alfC("replaceOne", argRepl, nil, Unmapped, replacingOneNote),
	alfC("replaceAt", argReplAt, inPlace(replacingAtExpr), Approximated, replaceIndexNote),
	alfC("clear", argSeq, []string{"()"}, Mapped, "the inout parameter hands back the empty sequence, as v1 assigns null to it"),
}
