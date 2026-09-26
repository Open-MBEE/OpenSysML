package repl

import (
	"fmt"
	"sync"

	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
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
	}
	adapter.reader = corequery.NewPropertyReader(idx, resolver, model).WithIdentity(adapter.Identity)
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

	positionalOnce sync.Once
	positional     map[*symbols.Symbol]string
	byPositional   map[string]*symbols.Symbol
}

func (m *replQueryModel) Candidates(scope []string) ([]*symbols.Symbol, error) {
	if len(scope) == 0 {
		var out []*symbols.Symbol
		seen := map[*symbols.Symbol]bool{}
		walked := map[*symbols.Scope]bool{}
		for _, doc := range m.session.sessionDocs() {
			if root := m.index.DocumentRoot(doc.Name); root != nil {
				m.collectQueryScope(root, &out, seen, walked)
			}
		}
		return out, nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, name := range scope {
		roots := m.index.LookupQualified(name)
		if len(roots) == 0 {
			m.positionalNames()
			if sym := m.byPositional[name]; sym != nil {
				roots = []*symbols.Symbol{sym}
			}
		}
		if len(roots) == 0 {
			return nil, fmt.Errorf("query scope names an element the model does not have: %q", name)
		}
		for _, root := range roots {
			m.collectQuerySymbol(root, &out, seen, map[*symbols.Scope]bool{})
		}
	}
	return out, nil
}

func (m *replQueryModel) collectQueryScope(scope *symbols.Scope, out *[]*symbols.Symbol, seen map[*symbols.Symbol]bool, walked map[*symbols.Scope]bool) {
	if scope == nil || walked[scope] {
		return
	}
	walked[scope] = true
	for _, sym := range scope.AllMembers() {
		m.collectQuerySymbol(sym, out, seen, walked)
	}
	for _, child := range scope.Children() {
		m.collectQueryScope(child, out, seen, walked)
	}
}

func (m *replQueryModel) collectQuerySymbol(sym *symbols.Symbol, out *[]*symbols.Symbol, seen map[*symbols.Symbol]bool, walked map[*symbols.Scope]bool) {
	if sym == nil || seen[sym] {
		return
	}
	seen[sym] = true
	if m.Identity(sym) != "" {
		*out = append(*out, sym)
	}
	if sym.Scope != nil {
		m.collectQueryScope(sym.Scope, out, seen, walked)
	} else if fqn := m.index.GetFQN(sym); fqn != "" {
		for _, child := range m.index.LookupDirectChildren(fqn) {
			m.collectQuerySymbol(child, out, seen, walked)
		}
	}
}

func (m *replQueryModel) Value(sym *symbols.Symbol, property string) ([]string, bool) {
	return m.reader.Values(sym, property)
}

func (m *replQueryModel) Identity(sym *symbols.Symbol) string {
	if identity := corequery.QualifiedIdentity(m.index, sym); identity != "" {
		return identity
	}
	m.positionalNames()
	return m.positional[sym]
}

func (m *replQueryModel) positionalNames() {
	m.positionalOnce.Do(func() {
		if m.index == nil {
			m.positional, m.byPositional = export.PositionalIdentities(nil, nil, nil)
			return
		}
		docs := make([]export.PositionalDocument, 0)
		for _, doc := range m.session.sessionDocs() {
			root := m.index.DocumentRoot(doc.Name)
			if root == nil {
				continue
			}
			docs = append(docs, export.PositionalDocument{
				File: source.New(doc.Name, doc.Content),
				Root: root,
			})
		}
		m.positional, m.byPositional = export.PositionalIdentities(
			m.index,
			docs,
			func(sym *symbols.Symbol) bool {
				return corequery.QualifiedIdentity(m.index, sym) != ""
			},
		)
	})
}

func (m *replQueryModel) Type(sym *symbols.Symbol) string {
	return corequery.MetamodelTypeNameOf(sym)
}
