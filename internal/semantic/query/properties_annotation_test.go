package query

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// annotatedModel declares a documented part, a part with a textual
// representation, a localized comment and an alias of the latter part.
const annotatedModel = `package Model {
	metadata def m {
		attribute a default "a";
	}
	part <a> aaaa {
		doc /*Documentation*/
		@m {
			a := "aa";
		}
		port p;
	}
	part <b> bbb {
		rep language "rus"
			/*Textual representation*/
		comment c locale "fr" /* Un commentaire
		                       * sur deux lignes. */
		port p;
	}
	alias ccc for bbb;
	interface connect (a.p, b.p);
}`

// annotatedReader reads the model's properties through a semantic model that
// can see its notation, as the service and the CLI do.
func annotatedReader(t *testing.T) (*PropertyReader, func(string) *symbols.Symbol) {
	t.Helper()
	file := source.New("annotated.sysml", []byte(annotatedModel))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	model.SetSourceText(func(doc string, span source.Span) string {
		if doc != file.Name() {
			return ""
		}
		return file.Text(span)
	})
	lookup := func(fqn string) *symbols.Symbol {
		t.Helper()
		matches := index.LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("lookup %s: %d symbols", fqn, len(matches))
		}
		return matches[0]
	}
	return NewPropertyReader(index, resolver, model), lookup
}

// anonymousMember is the one unnamed member of kind under owner.
func anonymousMember(t *testing.T, owner *symbols.Symbol, kind symbols.SymbolKind) *symbols.Symbol {
	t.Helper()
	var found *symbols.Symbol
	owner.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == kind {
			found = member
		}
		return true
	})
	if found == nil {
		t.Fatalf("%s owns no %s", owner.Name, kind)
	}
	return found
}

func TestAnnotationPropertiesReadTheBodiesExactly(t *testing.T) {
	reader, lookup := annotatedReader(t)
	aaaa, bbb := lookup("Model::aaaa"), lookup("Model::bbb")
	doc := anonymousMember(t, aaaa, symbols.SymbolDocumentation)
	rep := anonymousMember(t, bbb, symbols.SymbolTextualRepresentation)
	comment := lookup("Model::bbb::c")
	for _, tc := range []struct {
		sym      *symbols.Symbol
		property string
		want     []string
	}{
		{aaaa, PropertyDocumentation, []string{"Documentation"}},
		{doc, PropertyBody, []string{"Documentation"}},
		{bbb, PropertyTextualRepresentation, []string{"Textual representation"}},
		{rep, PropertyBody, []string{"Textual representation"}},
		{rep, PropertyLanguage, []string{"rus"}},
		{comment, PropertyBody, []string{"Un commentaire\nsur deux lignes. "}},
		{comment, PropertyLocale, []string{"fr"}},
		// Comment::body is exact: the space before `*/` stays. An alias reads its target's reps.
		{lookup("Model::ccc"), PropertyTextualRepresentation, []string{"Textual representation"}},
	} {
		got, ok := reader.Values(tc.sym, tc.property)
		if !ok || !slices.Equal(got, tc.want) {
			t.Errorf("%s of %s = %q, %v; want %q", tc.property, tc.sym.Name, got, ok, tc.want)
		}
	}
}

func TestAnnotationPropertiesAreAbsentNotEmpty(t *testing.T) {
	reader, lookup := annotatedReader(t)
	aaaa, bbb := lookup("Model::aaaa"), lookup("Model::bbb")
	doc := anonymousMember(t, aaaa, symbols.SymbolDocumentation)
	rep := anonymousMember(t, bbb, symbols.SymbolTextualRepresentation)
	for _, tc := range []struct {
		sym      *symbols.Symbol
		property string
	}{
		{aaaa, PropertyTextualRepresentation},
		{aaaa, PropertyBody},
		{bbb, PropertyDocumentation},
		{bbb, PropertyBody},
		{lookup("Model::m"), PropertyTextualRepresentation},
		{lookup("Model::m"), PropertyLanguage},
		{doc, PropertyLocale},
		{doc, PropertyLanguage},
		{rep, PropertyLocale},
		{lookup("Model::bbb::c"), PropertyLanguage},
	} {
		if got, ok := reader.Values(tc.sym, tc.property); ok || len(got) != 0 {
			t.Errorf("%s of %s = %q, %v; want absent", tc.property, tc.sym.Name, got, ok)
		}
	}
}

// Without the notation the bodies come from, the body properties are absent
// rather than empty, while the declared strings still read.
func TestAnnotationBodiesNeedTheNotation(t *testing.T) {
	file := source.New("annotated.sysml", []byte(annotatedModel))
	root := parser.New(file).ParseFile()
	index := symbols.NewIndex()
	index.AddDocument(file.Name(), root)
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	reader := NewPropertyReader(index, resolver, model)
	bbb := index.LookupQualified("Model::bbb")[0]
	rep := anonymousMember(t, bbb, symbols.SymbolTextualRepresentation)
	if got, ok := reader.Values(bbb, PropertyTextualRepresentation); ok || len(got) != 0 {
		t.Errorf("textualRepresentation without notation = %q, %v; want absent", got, ok)
	}
	if got, ok := reader.Values(rep, PropertyBody); ok || len(got) != 0 {
		t.Errorf("body without notation = %q, %v; want absent", got, ok)
	}
	if got, ok := reader.Values(rep, PropertyLanguage); !ok || !slices.Equal(got, []string{"rus"}) {
		t.Errorf("language without notation = %q, %v; want [rus]", got, ok)
	}
}
