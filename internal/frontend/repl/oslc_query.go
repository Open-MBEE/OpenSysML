package repl

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/doc/querytranslate"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Query translates OSLC query text to a document-query plan, runs it over the
// session's model and the objects it holds, and renders one matched row per
// line for the interactive frontend.
func (s *Session) Query(text string) ([]string, error) {
	defer s.enter()()
	return s.query(text)
}

// errQueryNotLinked is Query's answer in a build that links no positional namer.
var errQueryNotLinked = errors.New("OSLC query is not linked into this build")

// errNoQueryModel is Query's answer while the session holds no model to query.
var errNoQueryModel = &querytranslate.Error{Kind: querytranslate.ErrNoModel, Message: "no model loaded"}

func (s *Session) query(text string) ([]string, error) {
	if replext.Positional() == nil {
		return nil, errQueryNotLinked
	}
	if len(s.sessionMembers()) == 0 || s.hasAnalysisErrors() {
		return nil, errNoQueryModel
	}
	idx := s.browseIndex()
	if idx == nil {
		return nil, errNoQueryModel
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}
	context := s.queryContext(ctx)
	namer := &replQueryModel{session: s, index: idx}
	context.Identity = namer.Identity
	table := querytranslate.NewTable(context.Index, context.Model)
	q, err := querytranslate.ParseOSLC(text, table)
	if err != nil {
		return nil, err
	}
	plan, err := querytranslate.Translate(q, querytranslate.Context{Index: context.Index, Model: context.Model, Table: table}, "%query")
	if err != nil {
		return nil, err
	}
	bindings := queryexec.Bindings{}
	if !plan.Objects {
		candidates, err := namer.Candidates(q.Scope)
		if err != nil {
			return nil, err
		}
		values := make([]queryexec.Value, len(candidates))
		for i, sym := range candidates {
			values[i] = queryexec.ElementValue(sym)
		}
		bindings[querytranslate.SourceParameter] = values
	}
	result, err := queryexec.Execute(plan.Program, context, bindings, queryexec.Options{})
	if err != nil {
		return nil, err
	}
	rows := result.Rows()
	columns := result.Columns()
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		value := row.Element()
		var line strings.Builder
		if _, label, ok := value.Object(); ok {
			line.WriteString(label)
			line.WriteString("  ")
		}
		if decl := value.Declaration(); decl != nil {
			fmt.Fprintf(&line, "%s  %s", namer.Identity(decl), corequery.MetamodelTypeNameOf(decl))
		}
		if len(q.Select) == 0 {
			lines = append(lines, line.String())
			continue
		}
		for i, cell := range row.Cells() {
			values := cell.Values()
			if i >= len(columns) || len(values) == 0 {
				continue
			}
			if text, ok := context.PropertyText(values[0]); ok {
				fmt.Fprintf(&line, "  %s=%s", q.SpellingOf(columns[i].Name()), text)
			}
		}
		lines = append(lines, line.String())
	}
	return lines, nil
}

// replQueryModel names the session's elements as %query reports them — by
// qualified name, else by positional identity — and enumerates what a scope considers.
type replQueryModel struct {
	session *Session
	index   *symbols.Index

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
			return nil, &querytranslate.Error{Kind: querytranslate.ErrUnknownScope,
				Message: fmt.Sprintf("query scope names an element the model does not have: %q", name)}
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
			m.positional, m.byPositional = replext.Positional()(nil, nil, nil)
			return
		}
		docs := make([]replext.PositionalDocument, 0)
		for _, doc := range m.session.sessionDocs() {
			root := m.index.DocumentRoot(doc.Name)
			if root == nil {
				continue
			}
			docs = append(docs, replext.PositionalDocument{
				File:  sourceForKind(doc.Name, doc.Content, doc.Kind()),
				Root:  doc.AST,
				Scope: root,
			})
		}
		m.positional, m.byPositional = replext.Positional()(
			m.index,
			docs,
			func(sym *symbols.Symbol) bool {
				return corequery.QualifiedIdentity(m.index, sym) != ""
			},
		)
	})
}
