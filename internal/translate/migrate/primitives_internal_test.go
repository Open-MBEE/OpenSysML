package migrate

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// documented is a behavior as its library document declares it.
type documented struct {
	fragment string
	ins      string // the in and inout parameters, in order
	outs     int    // the out, inout and return parameters
}

// fumlDocument lists the behaviors the fUML_Library.xmi document holds, plus ListConcat from
// fUML 1.5 Table 9.7: fragment, in and inout parameters in order, and how many out, inout and return parameters they have.
var fumlDocument = []documented{
	{"PrimitiveBehaviors-IntegerFunctions-ToInteger", "x", 1},
	{"PrimitiveBehaviors-IntegerFunctions-lt", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-plus", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-minus", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-times", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-Div", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-Neg", "x", 1},
	{"PrimitiveBehaviors-IntegerFunctions-Mod", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-Max", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-Min", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-Abs", "x", 1},
	{"PrimitiveBehaviors-IntegerFunctions-gt", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-le", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-ge", "x y", 1},
	{"PrimitiveBehaviors-IntegerFunctions-ToString", "x", 1},
	{"PrimitiveBehaviors-IntegerFunctions-ToUnlimitedNatural", "x", 1},
	{"PrimitiveBehaviors-IntegerFunctions-divide", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-ToInteger", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-lt", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-plus", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-minus", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-times", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-divide", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-Neg", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-Max", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-Min", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-Abs", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-gt", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-le", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-ge", "x y", 1},
	{"PrimitiveBehaviors-RealFunctions-ToString", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-Floor", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-Floor-Round", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-Inv", "x", 1},
	{"PrimitiveBehaviors-RealFunctions-ToReal", "x", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-ToUnlimitedNatural", "x", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-lt", "x y", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-Max", "x y", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-Min", "x y", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-gt", "x y", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-le", "x y", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-ge", "x y", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-ToString", "x", 1},
	{"PrimitiveBehaviors-UnlimitedNaturalFunctions-ToInteger", "x", 1},
	{"PrimitiveBehaviors-BooleanFunctions-ToBoolean", "x", 1},
	{"PrimitiveBehaviors-BooleanFunctions-ToString", "x", 1},
	{"PrimitiveBehaviors-BooleanFunctions-Or", "x y", 1},
	{"PrimitiveBehaviors-BooleanFunctions-Xor", "x y", 1},
	{"PrimitiveBehaviors-BooleanFunctions-And", "x y", 1},
	{"PrimitiveBehaviors-BooleanFunctions-Implies", "x y", 1},
	{"PrimitiveBehaviors-BooleanFunctions-Not", "x", 1},
	{"PrimitiveBehaviors-StringFunctions-Size", "x", 1},
	{"PrimitiveBehaviors-StringFunctions-Concat", "x y", 1},
	{"PrimitiveBehaviors-StringFunctions-Substring", "x lower upper", 1},
	{"PrimitiveBehaviors-ListFunctions-ListSize", "list", 1},
	{"PrimitiveBehaviors-ListFunctions-ListGet", "list index", 1},
	{"PrimitiveBehaviors-ListFunctions-ListConcat", "list1 list2", 1},
	{"BasicInputOutput-WriteLine", "value", 1},
	{"BasicInputOutput-ReadLine", "", 2},
}

// alfDocument lists the behaviors the Alf-Library.xmi document holds: fragment, in and inout parameters in order, and how many out, inout and return parameters they have.
var alfDocument = []documented{
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-IsSet", "b n", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-BitLength", "", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-ToBitString", "n", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-ToInteger", "b", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-ToHexString", "b", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-ToOctalString", "b", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-tilde", "b", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-amp", "b1 b2", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-caret", "b1 b2", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-bar", "b1 b2", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-ltlt", "b n", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-gtgt", "b n", 1},
	{"Alf-Library-PrimitiveBehaviors-BitStringFunctions-gtgtgt", "b n", 1},
	{"Alf-Library-PrimitiveBehaviors-IntegerFunctions-ToNatural", "x", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Size", "seq", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Includes", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Excludes", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Count", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-IsEmpty", "seq", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-NotEmpty", "seq", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-IncludesAll", "seq1 seq2", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-ExcludesAll", "seq1 seq2", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Equals", "seq1 seq2", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-At", "seq index", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-IndexOf", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-First", "seq", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Last", "seq", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Union", "seq1 seq2", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Intersection", "seq1 seq2", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Difference", "seq1 seq2", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Including", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-IncludeAt", "seq element index", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-InsertAt", "seq element index", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-IncludeAllAt", "seq1 seq2 index", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Excluding", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-ExcludingOne", "seq element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-ExcludeAt", "seq index", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Replacing", "seq element newElement", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-ReplacingAt", "seq index element", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-ReplacingOne", "seq element newElement", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-Subsequence", "seq lower upper", 1},
	{"Alf-Library-PrimitiveBehaviors-SequenceFunctions-ToOrderedSet", "seq", 1},
	{"Alf-Library-CollectionFunctions-at", "seq index", 1},
	{"Alf-Library-CollectionFunctions-count", "seq element", 1},
	{"Alf-Library-CollectionFunctions-difference", "seq1 seq2", 1},
	{"Alf-Library-CollectionFunctions-equals", "seq1 seq2", 1},
	{"Alf-Library-CollectionFunctions-excludeAt", "seq index", 1},
	{"Alf-Library-CollectionFunctions-excludes", "seq element", 1},
	{"Alf-Library-CollectionFunctions-excludesAll", "seq1 seq2", 1},
	{"Alf-Library-CollectionFunctions-excluding", "seq element", 1},
	{"Alf-Library-CollectionFunctions-excludingOne", "seq element", 1},
	{"Alf-Library-CollectionFunctions-first", "seq", 1},
	{"Alf-Library-CollectionFunctions-includeAllAt", "seq1 seq2 index", 1},
	{"Alf-Library-CollectionFunctions-includeAt", "seq element index", 1},
	{"Alf-Library-CollectionFunctions-includes", "seq element", 1},
	{"Alf-Library-CollectionFunctions-includesAll", "seq1 seq2", 1},
	{"Alf-Library-CollectionFunctions-including", "seq element", 1},
	{"Alf-Library-CollectionFunctions-indexOf", "seq element", 1},
	{"Alf-Library-CollectionFunctions-insertAt", "seq element index", 1},
	{"Alf-Library-CollectionFunctions-intersection", "seq1 seq2", 1},
	{"Alf-Library-CollectionFunctions-isEmpty", "seq", 1},
	{"Alf-Library-CollectionFunctions-last", "seq", 1},
	{"Alf-Library-CollectionFunctions-notEmpty", "seq", 1},
	{"Alf-Library-CollectionFunctions-replacing", "seq element newElement", 1},
	{"Alf-Library-CollectionFunctions-replacingAt", "seq index element", 1},
	{"Alf-Library-CollectionFunctions-replacingOne", "seq element newElement", 1},
	{"Alf-Library-CollectionFunctions-size", "seq", 1},
	{"Alf-Library-CollectionFunctions-subsequence", "seq lower upper", 1},
	{"Alf-Library-CollectionFunctions-toOrderedSet", "seq", 1},
	{"Alf-Library-CollectionFunctions-union", "seq1 seq2", 1},
	{"Alf-Library-CollectionFunctions-add", "seq element", 2},
	{"Alf-Library-CollectionFunctions-addAll", "seq1 seq2 index", 1},
	{"Alf-Library-CollectionFunctions-addAt", "seq element index", 2},
	{"Alf-Library-CollectionFunctions-addAllAt", "seq1 seq2 index", 2},
	{"Alf-Library-CollectionFunctions-remove", "seq element", 2},
	{"Alf-Library-CollectionFunctions-removeAll", "seq1 seq2", 2},
	{"Alf-Library-CollectionFunctions-removeOne", "seq element", 2},
	{"Alf-Library-CollectionFunctions-removeAt", "seq index", 2},
	{"Alf-Library-CollectionFunctions-replace", "seq element newElement", 2},
	{"Alf-Library-CollectionFunctions-replaceAt", "seq index element", 2},
	{"Alf-Library-CollectionFunctions-replaceOne", "seq element newElement", 2},
	{"Alf-Library-CollectionFunctions-clear", "seq", 1},
}

// TestPrimitivesCoverBothLibraries checks that every behavior either library
// document holds is mapped or refused with the parameters the document declares,
// and nothing beyond them is listed.
func TestPrimitivesCoverBothLibraries(t *testing.T) {
	seen := map[primitiveKey]bool{}
	for lib, behaviors := range map[primitiveLib][]documented{fumlLib: fumlDocument, alfLib: alfDocument} {
		for _, b := range behaviors {
			k := primitiveKey{lib, b.fragment}
			seen[k] = true
			p := primitiveIndex[k]
			if p == nil {
				t.Errorf("%s %s: the library document holds it, the mapping does not", lib, b.fragment)
				continue
			}
			var names []string
			for _, arg := range p.arguments() {
				names = append(names, arg.name)
			}
			if got := strings.Join(names, " "); got != b.ins {
				t.Errorf("%s %s: parameters %q, the document declares %q", lib, b.fragment, got, b.ins)
			}
			if p.outs != nil && len(p.outs) != b.outs {
				t.Errorf("%s %s: %d result(s), the document declares %d", lib, b.fragment, len(p.outs), b.outs)
			}
		}
	}
	for k, p := range primitiveIndex {
		if !seen[k] {
			t.Errorf("%s %s: mapped, yet neither library document holds it", k.lib, p.fragment)
		}
	}
	if len(primitives) != len(primitiveIndex) {
		t.Errorf("%d primitives listed, %d distinct: a fragment is listed twice", len(primitives), len(primitiveIndex))
	}
}

var placeholder = regexp.MustCompile(`\$(\d+)`)

// TestPrimitivesAreWellFormed checks the shape of each mapping: a refusal is
// Unmapped with a reason, a mapping gives each result an expression over the
// arguments it has, and an approximation says how it differs.
func TestPrimitivesAreWellFormed(t *testing.T) {
	if len(primitivePaths) != len(primitives) || len(primitiveIndex) != len(primitives) {
		t.Errorf("%d primitives index as %d fragments and %d paths", len(primitives), len(primitiveIndex), len(primitivePaths))
	}
	for i := range primitives {
		p := &primitives[i]
		name := p.lib.String() + " " + p.fragment
		if p.family == "" || p.name == "" || !strings.Contains(p.fragment, p.family) {
			t.Errorf("%s: family %q and name %q do not identify it", name, p.family, p.name)
		}
		if p.outs == nil {
			if p.verdict != Unmapped || p.note == "" {
				t.Errorf("%s: a refusal must be Unmapped with a reason; got %v %q", name, p.verdict, p.note)
			}
			continue
		}
		switch p.verdict {
		case Mapped:
		case Approximated:
			if p.note == "" {
				t.Errorf("%s: an approximation must say how the v2 function differs", name)
			}
		default:
			t.Errorf("%s: a mapped call is Mapped or Approximated, not %v", name, p.verdict)
		}
		for j, out := range p.outs {
			if out == "" {
				t.Errorf("%s: result %d has no expression", name, j)
			}
			for _, m := range placeholder.FindAllStringSubmatch(out, -1) {
				if n, _ := strconv.Atoi(m[1]); n < 1 || n > len(p.ins) {
					t.Errorf("%s: result %d names argument %s of %d", name, j, m[0], len(p.ins))
				}
			}
		}
		for _, arg := range p.arguments() {
			if arg.name == "" || strings.HasPrefix(arg.name, "*") {
				t.Errorf("%s: argument %q is misnamed", name, arg.name)
			}
		}
	}
}

// TestPrimitiveAt checks that a primitive is found by its library document and
// fragment whatever the date the href carries, and by nothing else.
func TestPrimitiveAt(t *testing.T) {
	for href, want := range map[string]string{
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat":                "fUML StringFunctions::Concat",
		"http://www.omg.org/spec/FUML/20130801/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat":                "fUML StringFunctions::Concat",
		"https://www.omg.org/spec/FUML/1.5/fUML_Library.xmi#PrimitiveBehaviors-RealFunctions-Floor-Round":                 "fUML RealFunctions::Round",
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#BasicInputOutput-WriteLine":                               "fUML BasicInputOutput::WriteLine",
		"http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-Including": "Alf SequenceFunctions::Including",
		"http://www.omg.org/spec/ALF/20130801/Alf-Library.xmi#Alf-Library-CollectionFunctions-addAll":                     "Alf CollectionFunctions::addAll",
		// the fragment is exact
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-concat": "",
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#StringFunctions-Concat":                    "",
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#":                                          "",
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi":                                           "",
		// the fragments of one library do not name behaviors of the other
		"http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#PrimitiveBehaviors-StringFunctions-Concat":                    "",
		"http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-Including": "",
		// only the two library documents at their OMG locations count
		"http://www.omg.org/spec/FUML/20180501/fUML_Semantics.xmi#PrimitiveBehaviors-StringFunctions-Concat":           "",
		"http://www.omg.org/spec/FUML/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat":                      "",
		"http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#PrimitiveBehaviors-StringFunctions-Concat":            "",
		"http://www.example.com/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat":         "",
		"http://www.omg.org.example.com/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat": "",
		"fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat":                                                   "",
		"_concat": "",
		"":        "",
	} {
		got := ""
		if p := primitiveAt(href); p != nil {
			got = p.qualified()
		}
		if got != want {
			t.Errorf("primitiveAt(%q) = %q, want %q", href, got, want)
		}
	}
}

// nested builds an element chain from a qualified name, the last of the given type.
func nested(typ, qualified string) *sysmlv1.Element {
	var e *sysmlv1.Element
	for _, name := range strings.Split(qualified, "::") {
		e = &sysmlv1.Element{Type: "Package", Name: name, Parent: e}
	}
	e.Type = typ
	return e
}

// proxy builds the proxy an href with a tool's referentPath resolves to when the
// document is not bundled.
func proxy(href, typ, qualified string) *sysmlv1.Element {
	return &sysmlv1.Element{ID: href, Href: href, Type: typ, QualifiedName: qualified}
}

// TestPrimitiveNamed checks that an href into a library's own document names a
// primitive by the qualified name of its target, bundled or recorded beside the
// href, and that no other document does, whatever its packages are called.
func TestPrimitiveNamed(t *testing.T) {
	const (
		listSize = "fUML_Library::PrimitiveBehaviors::ListFunctions::ListSize"
		mdHref   = "fUML-Library.mdzip#_jJIy63OeEd2TgN94jve35g"
	)
	for _, tc := range []struct {
		name       string
		href       string
		target     *sysmlv1.Element
		want, prov string
	}{
		{"MagicDraw referentPath", mdHref, proxy(mdHref, "FunctionBehavior", listSize),
			"fUML ListFunctions::ListSize", "the behavior is known by the referentPath " + listSize + " recorded beside its href into the library module fUML-Library.mdzip"},
		{"MagicDraw referentPath under a path", "../modelLibraries/fUML-Library.mdzip#_x", proxy("../modelLibraries/fUML-Library.mdzip#_x", "", "fUML_Library::PrimitiveBehaviors::StringFunctions::Concat"),
			"fUML StringFunctions::Concat", "the behavior is known by the referentPath fUML_Library::PrimitiveBehaviors::StringFunctions::Concat recorded beside its href into the library module ../modelLibraries/fUML-Library.mdzip"},
		{"MagicDraw referentPath at the root", mdHref, proxy(mdHref, "Activity", "fUML_Library::BasicInputOutput::WriteLine"),
			"fUML BasicInputOutput::WriteLine", ""},
		{"bundled MagicDraw copy", mdHref, nested("FunctionBehavior", listSize),
			"fUML ListFunctions::ListSize", "the behavior is known by the copy of the library the model bundles as fUML-Library.mdzip, which its href resolves to"},
		{"bundled OMG document", "fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-ToString", nested("FunctionBehavior", "FoundationalModelLibrary::PrimitiveBehaviors::IntegerFunctions::ToString"),
			"fUML IntegerFunctions::ToString", ""},
		{"bundled Alf document", "Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-Including", nested("FunctionBehavior", "Alf::Library::PrimitiveBehaviors::SequenceFunctions::Including"),
			"Alf SequenceFunctions::Including", ""},
		{"Alf referentPath", "Alf-Library.mdzip#_y", proxy("Alf-Library.mdzip#_y", "FunctionBehavior", "Alf::Library::CollectionFunctions::addAll"),
			"Alf CollectionFunctions::addAll", ""},
		// the name is exact
		{"wrong case", mdHref, proxy(mdHref, "FunctionBehavior", "fUML_Library::PrimitiveBehaviors::ListFunctions::listSize"), "", ""},
		{"family missing", mdHref, proxy(mdHref, "FunctionBehavior", "fUML_Library::PrimitiveBehaviors::ListSize"), "", ""},
		{"root missing", mdHref, proxy(mdHref, "FunctionBehavior", "PrimitiveBehaviors::ListFunctions::ListSize"), "", ""},
		{"no path", mdHref, proxy(mdHref, "FunctionBehavior", ""), "", ""},
		{"Alf root under fUML", mdHref, proxy(mdHref, "FunctionBehavior", "Alf::Library::PrimitiveBehaviors::SequenceFunctions::Including"), "", ""},
		{"no behavior", mdHref, proxy(mdHref, "Class", listSize), "", ""},
		// the document must be the library's own
		{"used project named like the library", "Helpers.mdzip#_x", proxy("Helpers.mdzip#_x", "FunctionBehavior", listSize), "", ""},
		{"user package named like the library", "Model.mdzip#_x", nested("FunctionBehavior", listSize), "", ""},
		{"user package named like the library, in the document", "_userListSize", nested("FunctionBehavior", "Model::"+listSize), "", ""},
		{"Alf document naming a fUML root", "Alf-Library.mdzip#_x", proxy("Alf-Library.mdzip#_x", "FunctionBehavior", listSize), "", ""},
		{"unresolved", mdHref, nil, "", ""},
	} {
		got, prov := "", ""
		if p := primitiveNamed(tc.href, tc.target); p != nil {
			got, prov = p.qualified(), p.provenance
		}
		if got != tc.want {
			t.Errorf("%s: primitiveNamed(%q) = %q, want %q", tc.name, tc.href, got, tc.want)
		}
		if tc.prov != "" && prov != tc.prov {
			t.Errorf("%s: provenance %q, want %q", tc.name, prov, tc.prov)
		}
	}
}

// TestPrimitiveResult checks that a result expression takes the arguments in
// the order the v1 parameters have, however the v2 function permutes them.
func TestPrimitiveResult(t *testing.T) {
	p := primitiveAt("http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-ReplacingAt")
	if p == nil {
		t.Fatal("ReplacingAt is not mapped")
	}
	if got, want := strings.Join(p.ins, " "), "*seq index element"; got != want {
		t.Fatalf("ReplacingAt parameters %q, want %q", got, want)
	}
	if got, want := p.result(0, []string{"a", "b", "c"}), "SequenceFunctions::includingAt(SequenceFunctions::excludingAt(a, b), c, b)"; got != want {
		t.Errorf("ReplacingAt result = %q, want %q", got, want)
	}
	// An argument named $1 is not confused with $10 and later.
	q := &primitive{ins: make([]string, 11), outs: []string{"f($1, $10, $11)"}}
	args := make([]string, 11)
	for i := range args {
		args[i] = "a" + strconv.Itoa(i+1)
	}
	if got, want := q.result(0, args), "f(a1, a10, a11)"; got != want {
		t.Errorf("result = %q, want %q", got, want)
	}
}
