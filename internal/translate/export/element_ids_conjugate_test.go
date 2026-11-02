package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The ids the reader answers for the conjugated port definition a port
// definition implies, its owning membership and its PortConjugation are the
// ones the conversion writes for them.
func TestElementIDReaderSpellsTheConjugateAsTheConversionWritesIt(t *testing.T) {
	sf, index, doc := readerIndex(t, "ports.sysml", "package P { port def Q { attribute x; } }\n")
	graph, err := ToRDF(sf, parser.New(sf).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	written := map[string]string{}
	for _, subject := range graph.Subjects() {
		for _, class := range graph.Objects(subject, rdf.RDFType) {
			switch class.Value {
			case rdf.SysML + "ConjugatedPortDefinition", rdf.SysML + "PortConjugation":
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					written[class.Value] = id.Value
				}
			}
		}
	}
	if len(written) != 2 {
		t.Fatalf("the conversion writes %v, want a ConjugatedPortDefinition and a PortConjugation", written)
	}
	conjugate := written[rdf.SysML+"ConjugatedPortDefinition"]
	var membership string
	for _, subject := range graph.Subjects() {
		for _, member := range graph.Objects(subject, rdf.SysML+"ownedMemberElement") {
			if rdf.LocalName(member.Value) == conjugate {
				for _, id := range graph.Objects(subject, rdf.SysML+"elementId") {
					membership = id.Value
				}
			}
		}
	}
	if membership == "" {
		t.Fatalf("the conversion writes no owning membership for %s", conjugate)
	}

	matches := index.LookupQualified("P::Q")
	if len(matches) != 1 {
		t.Fatalf("P::Q names %d declarations, want 1", len(matches))
	}
	sym, ok := matches[0].Scope.LookupLocal("~Q")
	if !ok || !sym.IsConjugatedPortDefinition() {
		t.Fatalf("P::Q::'~Q' = %v, want the implied conjugate", sym)
	}
	if _, ok := symbols.ImpliedDeclOf(sym); !ok {
		t.Fatalf("the conjugate's declaration is %T, want an ImpliedDecl", sym.Decl)
	}
	reader := NewElementIDReader([]ElementIDDocument{doc})
	if got, err := reader.Read(sym.DocName, sym.Decl, "P::Q::~Q"); err != nil || got != conjugate {
		t.Errorf("Read conjugate = %q, %v; want %q", got, err, conjugate)
	}
	if got, err := reader.ReadOwningMembership(sym.DocName, sym.Decl, "P::Q::~Q"); err != nil || got != membership {
		t.Errorf("ReadOwningMembership conjugate = %q, %v; want %q", got, err, membership)
	}
	if got, err := reader.ReadRelationship(sym.DocName, sym.Decl); err != nil || got != written[rdf.SysML+"PortConjugation"] {
		t.Errorf("ReadRelationship conjugation = %q, %v; want %q", got, err, written[rdf.SysML+"PortConjugation"])
	}
}

func TestConversionNamesConjugatesFromOriginalDeclaredNames(t *testing.T) {
	for _, tc := range []struct {
		name, source, predicate, address, declaredName string
	}{
		{
			name: "short only", source: "package P { port def <s>; }",
			predicate: rdf.OpenSysML + xPositionalName, address: "P::@0::@conjugated",
		},
		{
			name: "short and name", source: "package P { port def <q> Q; }",
			predicate: rdf.SysML + "qualifiedName", address: "P::Q::'~Q'", declaredName: "~Q",
		},
		{
			name: "name", source: "package P { port def Q; }",
			predicate: rdf.SysML + "qualifiedName", address: "P::Q::'~Q'", declaredName: "~Q",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := source.New(tc.name+".sysml", []byte(tc.source))
			p := parser.New(file)
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			graph, err := ToRDF(file, root)
			if err != nil {
				t.Fatalf("ToRDF: %v", err)
			}
			conjugate := typedSubject(t, graph, rdf.SysML+"ConjugatedPortDefinition")
			if got := graph.Objects(conjugate, rdf.SysML+"declaredName"); tc.declaredName == "" && len(got) != 0 || tc.declaredName != "" && (len(got) != 1 || got[0].Value != tc.declaredName) {
				t.Errorf("declaredName = %v, want %q", got, tc.declaredName)
			}
			if got := graph.Objects(conjugate, rdf.SysML+"declaredShortName"); len(got) != 0 {
				t.Errorf("declaredShortName = %v, want none", got)
			}
			if got := graph.Objects(conjugate, tc.predicate); len(got) != 1 || got[0].Value != tc.address {
				t.Errorf("%s = %v, want %q", tc.predicate, got, tc.address)
			}
			other := rdf.SysML + "qualifiedName"
			if tc.predicate == other {
				other = rdf.OpenSysML + xPositionalName
			}
			if got := graph.Objects(conjugate, other); len(got) != 0 {
				t.Errorf("unexpected alternate address %s = %v", other, got)
			}
		})
	}
}

func TestElementIDReaderSeparatesShortPortAndAnonymousConjugate(t *testing.T) {
	file, index, document := readerIndex(t, "short-port.sysml", "package P { port def <s>; }\n")
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	pkg := index.LookupQualified("P")[0]
	short, ok := pkg.Scope.LookupLocal("s")
	if !ok {
		t.Fatal("P has no short-named port definition")
	}
	var conjugate *symbols.Symbol
	for _, member := range short.Scope.AllMembers() {
		if member.IsConjugatedPortDefinition() {
			conjugate = member
			break
		}
	}
	if conjugate == nil {
		t.Fatal("short-named port definition has no conjugate")
	}
	reader := NewElementIDReader([]ElementIDDocument{document})
	originalID, err := reader.Read(short.DocName, short.Decl, "P::@0")
	if err != nil {
		t.Fatalf("Read original: %v", err)
	}
	conjugateID, err := reader.Read(conjugate.DocName, conjugate.Decl, "P::@0::@conjugated")
	if err != nil {
		t.Fatalf("Read conjugate: %v", err)
	}
	if got := elementIDOf(t, graph, rdf.SysML+"PortDefinition"); originalID != got {
		t.Errorf("Read original = %q, graph writes %q", originalID, got)
	}
	if got := elementIDOf(t, graph, rdf.SysML+"ConjugatedPortDefinition"); conjugateID != got {
		t.Errorf("Read conjugate = %q, graph writes %q", conjugateID, got)
	}
	if originalID == "" || conjugateID == "" || originalID == conjugateID {
		t.Errorf("original id %q and conjugate id %q must be distinct", originalID, conjugateID)
	}
}

func TestShortNamedPortDefinitionRoundTripsThroughRDF(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		contains     []string
	}{
		{
			name: "short only", source: "package P { port def <s>; }",
			contains: []string{"port def <s>"},
		},
		{
			name: "short and name", source: "package P { port def <q> Q; }",
			contains: []string{"port def <q> Q"},
		},
		{
			name: "name", source: "package P { port def Q; }",
			contains: []string{"port def Q"},
		},
		{
			name: "conjugated typings", source: `package P {
	port def <s>;
	port def <q> Q;
	port def R;
	part def U {
		port a : ~s;
		port b : ~Q;
		port c : ~R;
	}
}`,
			contains: []string{
				"port def <s>", "port def <q> Q", "port def R",
				"port a : ~s", "port b : ~Q", "port c : ~R",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := source.New(tc.name+".sysml", []byte(tc.source))
			p := parser.New(file)
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			first, err := ToRDF(file, root)
			if err != nil {
				t.Fatalf("ToRDF: %v", err)
			}
			out, err := ToSysML(withoutSourceLayout(first))
			if err != nil {
				t.Fatalf("ToSysML: %v", err)
			}
			for _, want := range tc.contains {
				if !strings.Contains(string(out), want) {
					t.Errorf("round trip lacks %q:\n%s", want, out)
				}
			}
			roundTripFile := source.New("round-trip.sysml", out)
			roundTripParser := parser.New(roundTripFile)
			roundTripRoot := roundTripParser.ParseFile()
			if len(roundTripParser.Diagnostics) != 0 {
				t.Fatalf("round-tripped notation does not parse: %v\n%s", roundTripParser.Diagnostics, out)
			}
			second, err := ToRDF(roundTripFile, roundTripRoot)
			if err != nil {
				t.Fatalf("ToRDF of round-tripped notation: %v", err)
			}
			if missing, extra := graphDifference(withoutSourceLayout(first), withoutSourceLayout(second)); len(missing) != 0 || len(extra) != 0 {
				t.Errorf("RDF graph changed across the source-text-free round trip:\nmissing: %v\nextra: %v", missing, extra)
			}
		})
	}
}

func TestConjugatedPortDefinitionVerificationAcceptsLegacyShortName(t *testing.T) {
	file := source.New("legacy.sysml", []byte("package P { port def <q> Q; }\n"))
	p := parser.New(file)
	graph, err := ToRDF(file, p.ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	conjugate := typedSubject(t, graph, rdf.SysML+"ConjugatedPortDefinition")
	legacy := replaceElementProperty(withoutSourceLayout(graph), conjugate, rdf.SysML+"declaredShortName", rdf.String("~q"))
	if _, err := ToSysML(legacy); err != nil {
		t.Errorf("ToSysML with legacy conjugate declaredShortName: %v", err)
	}
}

func TestConjugatedPortDefinitionVerificationRejectsIncorrectNames(t *testing.T) {
	namedFile := source.New("named.sysml", []byte("package P { port def <q> Q; }\n"))
	namedParser := parser.New(namedFile)
	namedGraph, err := ToRDF(namedFile, namedParser.ParseFile())
	if err != nil {
		t.Fatalf("ToRDF named: %v", err)
	}
	namedConjugate := typedSubject(t, namedGraph, rdf.SysML+"ConjugatedPortDefinition")
	incorrectName := replaceElementProperty(withoutSourceLayout(namedGraph), namedConjugate, rdf.SysML+"declaredName", rdf.String("~wrong"))
	if _, err := ToSysML(incorrectName); err == nil {
		t.Error("ToSysML accepted an incorrect conjugate declaredName")
	}
	incorrectShort := replaceElementProperty(withoutSourceLayout(namedGraph), namedConjugate, rdf.SysML+"declaredShortName", rdf.String("~wrong"))
	if _, err := ToSysML(incorrectShort); err == nil {
		t.Error("ToSysML accepted an incorrect conjugate declaredShortName")
	}

	shortFile := source.New("short.sysml", []byte("package P { port def <s>; }\n"))
	shortParser := parser.New(shortFile)
	shortGraph, err := ToRDF(shortFile, shortParser.ParseFile())
	if err != nil {
		t.Fatalf("ToRDF short: %v", err)
	}
	shortConjugate := typedSubject(t, shortGraph, rdf.SysML+"ConjugatedPortDefinition")
	namedAnonymous := replaceElementProperty(withoutSourceLayout(shortGraph), shortConjugate, rdf.SysML+"declaredName", rdf.String("~s"))
	if _, err := ToSysML(namedAnonymous); err == nil {
		t.Error("ToSysML accepted a declaredName on the anonymous conjugate")
	}
}

func typedSubject(t *testing.T, graph *rdf.Graph, class string) rdf.Term {
	t.Helper()
	for _, subject := range graph.Subjects() {
		for _, typ := range graph.Objects(subject, rdf.RDFType) {
			if typ.Value == class {
				return subject
			}
		}
	}
	t.Fatalf("graph has no element of type %s", class)
	return rdf.Term{}
}

func elementIDOf(t *testing.T, graph *rdf.Graph, class string) string {
	t.Helper()
	ids := graph.Objects(typedSubject(t, graph, class), rdf.SysML+"elementId")
	if len(ids) != 1 {
		t.Fatalf("%s elementId = %v, want one", class, ids)
	}
	return ids[0].Value
}

func replaceElementProperty(graph *rdf.Graph, subject rdf.Term, predicate string, objects ...rdf.Term) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if triple.Subject != subject || triple.Predicate.Value != predicate {
			out.AddTriple(triple)
		}
	}
	for _, object := range objects {
		out.Add(subject, rdf.IRI(predicate), object)
	}
	return out
}

func graphDifference(before, after *rdf.Graph) (missing, extra []rdf.Triple) {
	beforeSet, afterSet := map[rdf.Triple]bool{}, map[rdf.Triple]bool{}
	for _, triple := range before.Triples() {
		beforeSet[triple] = true
	}
	for _, triple := range after.Triples() {
		afterSet[triple] = true
	}
	for triple := range beforeSet {
		if !afterSet[triple] {
			missing = append(missing, triple)
		}
	}
	for triple := range afterSet {
		if !beforeSet[triple] {
			extra = append(extra, triple)
		}
	}
	return missing, extra
}
