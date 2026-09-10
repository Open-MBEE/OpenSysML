package repl

import (
	"fmt"

	corequery "github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Query evaluates OSLC element-identification query text and renders one
// matched element per line for the interactive frontend.
func (s *Session) Query(text string) ([]string, error) {
	defer s.enter()()
	return s.query(text)
}

func (s *Session) query(text string) ([]string, error) {
	q, err := corequery.ParseOSLC(text)
	if err != nil {
		return nil, err
	}
	if len(s.sessionMembers()) == 0 || s.hasAnalysisErrors() {
		return nil, &corequery.Error{Kind: corequery.ErrNoModel, Message: "no model loaded"}
	}
	idx := s.browseIndex()
	if idx == nil {
		return nil, &corequery.Error{Kind: corequery.ErrNoModel, Message: "no model loaded"}
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	model.SetSourceText(s.sessionSourceText())
	adapter := &replQueryModel{
		session: s,
		index:   idx,
		reader:  corequery.NewPropertyReader(idx, resolver, model),
	}
	elements, err := corequery.Evaluate(adapter, q)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(elements))
	for _, element := range elements {
		line := fmt.Sprintf("%s  %s", element.ID, element.Type)
		for _, name := range q.Select {
			if value, ok := element.Properties[name]; ok {
				line += fmt.Sprintf("  %s=%s", q.SpellingOf(name), value)
			}
		}
		lines = append(lines, line)
	}
	return lines, nil
}

type replQueryModel struct {
	session *Session
	index   *symbols.Index
	reader  *corequery.PropertyReader
}

func (m *replQueryModel) Candidates(scope []string) ([]*symbols.Symbol, error) {
	if len(scope) == 0 {
		var out []*symbols.Symbol
		seen := map[*symbols.Symbol]bool{}
		for _, root := range m.session.docScopes() {
			collectQueryScope(root, &out, seen)
		}
		return out, nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, name := range scope {
		roots := m.index.LookupQualified(name)
		if len(roots) == 0 {
			return nil, fmt.Errorf("query scope names an element the model does not have: %q", name)
		}
		for _, root := range roots {
			collectQuerySymbol(root, &out, seen, m.index)
		}
	}
	return out, nil
}

func collectQueryScope(scope *symbols.Scope, out *[]*symbols.Symbol, seen map[*symbols.Symbol]bool) {
	if scope == nil {
		return
	}
	for _, sym := range append(scope.Members(), scope.AnonymousMembers()...) {
		collectQuerySymbol(sym, out, seen, nil)
	}
	for _, child := range scope.Children() {
		collectQueryScope(child, out, seen)
	}
}

func collectQuerySymbol(sym *symbols.Symbol, out *[]*symbols.Symbol, seen map[*symbols.Symbol]bool, index *symbols.Index) {
	if sym == nil || seen[sym] {
		return
	}
	seen[sym] = true
	fqn := ""
	if index != nil {
		fqn = index.GetFQN(sym)
	}
	if fqn != "" || sym.Name != "" {
		*out = append(*out, sym)
	}
	if sym.Scope != nil {
		collectQueryScope(sym.Scope, out, seen)
	} else if index != nil {
		for _, child := range index.LookupDirectChildren(index.GetFQN(sym)) {
			collectQuerySymbol(child, out, seen, index)
		}
	}
}

func (m *replQueryModel) Value(sym *symbols.Symbol, property string) ([]string, bool) {
	return m.reader.Values(sym, property)
}

func (m *replQueryModel) Identity(sym *symbols.Symbol) string { return m.index.GetFQN(sym) }
func (m *replQueryModel) Type(sym *symbols.Symbol) string {
	return corequery.MetamodelTypeNameOf(sym)
}
