package query

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const kindAndTextModel = `package Demo {
	requirement def Mass { doc /* The mass shall be small. */ }
	requirement def Silent;
	requirement limit : Mass { doc /* First sentence. */ doc /* Second sentence. */ }
	requirement open;
	part rig { doc /* A rig, which is no requirement. */ }
	state def Lamp {
		entry; then off;
		state off;
		state on;
		transition off_on first off then on;
		transition first on then off;
		action { }
	}
}`

func kindAndTextReader(t *testing.T) (*symbols.Index, *PropertyReader) {
	t.Helper()
	file := source.New("kinds.sysml", []byte(kindAndTextModel))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %v", p.Diagnostics)
	}
	// Requirement text is read of the metaclass, so the model needs the library that declares it.
	index := symbols.NewIndex()
	library := filepath.Join("..", "..", "workspace", "libs", "stdlib")
	err := filepath.WalkDir(library, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := filepath.Ext(path); ext != ".sysml" && ext != ".kerml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		index.AddDocument(path, parser.New(source.New(path, data)).ParseFile())
		return nil
	})
	if err != nil {
		t.Fatalf("walk the standard library: %v", err)
	}
	index.AddDocument(file.Name(), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	model.SetSourceText(func(doc string, span source.Span) string {
		if doc != file.Name() {
			return ""
		}
		return file.Text(span)
	})
	return index, NewPropertyReader(index, resolver, model)
}

func namedSymbol(t *testing.T, index *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	found := index.LookupQualified(fqn)
	if len(found) != 1 {
		t.Fatalf("LookupQualified(%q) = %d symbols, want 1", fqn, len(found))
	}
	return found[0]
}

// RequirementDefinition::text and RequirementUsage::text are the requirement's doc
// bodies in order, absent of a requirement that states none and of any other element.
func TestTextReadsARequirementsDocBodies(t *testing.T) {
	index, reader := kindAndTextReader(t)
	for fqn, want := range map[string][]string{
		"Demo::Mass":   {"The mass shall be small."},
		"Demo::limit":  {"First sentence.", "Second sentence."},
		"Demo::Silent": nil,
		"Demo::open":   nil,
		"Demo::rig":    nil,
	} {
		values, ok := reader.Values(namedSymbol(t, index, fqn), PropertyText)
		if ok != (want != nil) || !slices.Equal(values, want) {
			t.Errorf("%s: text = %q, %v; want %q, %v", fqn, values, ok, want, want != nil)
		}
	}
	if values, ok := reader.Values(namedSymbol(t, index, "Demo::rig"), PropertyDocumentation); !ok || len(values) != 1 {
		t.Errorf("Demo::rig: documentation = %q, %v; want its one doc body", values, ok)
	}
}

// symbolKind is the symbol's own kind as SymbolInfo.kind spells it, so a member no name
// reaches answers it too; a transition is an action usage there, TransitionUsage by @type.
func TestSymbolKindReadsNamedAndAnonymousMembers(t *testing.T) {
	index, reader := kindAndTextReader(t)
	for fqn, want := range map[string]string{
		"Demo":               "package",
		"Demo::Mass":         "requirementDef",
		"Demo::limit":        "requirementUsage",
		"Demo::Lamp":         "stateDef",
		"Demo::Lamp::off":    "stateUsage",
		"Demo::Lamp::off_on": "actionUsage",
	} {
		values, ok := reader.Values(namedSymbol(t, index, fqn), PropertySymbolKind)
		if !ok || len(values) != 1 || values[0] != want {
			t.Errorf("%s: symbolKind = %q, %v; want [%s], true", fqn, values, ok, want)
		}
	}
	var anonymous []string
	namedSymbol(t, index, "Demo::Lamp").Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Name == "" {
			values, _ := reader.Values(member, PropertySymbolKind)
			anonymous = append(anonymous, values...)
		}
		return true
	})
	want := []string{"actionUsage", "successionUsage", "actionUsage", "actionUsage"}
	if !slices.Equal(anonymous, want) {
		t.Errorf("anonymous members of Demo::Lamp: symbolKind = %q, want %q", anonymous, want)
	}
}
