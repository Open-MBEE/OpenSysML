package repl

import "sync"

// parseMemo remembers what command text parsed to, keyed by the exact text.
// Command text is literal, so its parse depends on nothing the session holds;
// only evaluation does, and that still happens on every call. It locks itself.
type parseMemo[T any] struct {
	mu      sync.Mutex
	entries map[string]T
}

// parseMemoLimit bounds a memo's entries; reaching it starts the memo afresh.
const parseMemoLimit = 256

// get returns what text parsed to, parsing it on the first request.
func (m *parseMemo[T]) get(text string, parse func(string) T) T {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.entries[text]; ok {
		return v
	}
	if m.entries == nil || len(m.entries) >= parseMemoLimit {
		m.entries = make(map[string]T)
	}
	v := parse(text)
	m.entries[text] = v
	return v
}

// parsedArgs is what an argument list parsed to: its expressions, or the error
// the parse reports.
type parsedArgs struct {
	exprs []argExpr
	err   error
}

// argExprs is parseExprList for the session: the parse is reused across calls,
// each of which evaluates the expressions afresh against the current state.
func (s *Session) argExprs(text string) ([]argExpr, error) {
	p := s.argMemo.get(text, func(text string) parsedArgs {
		exprs, err := parseExprList(text)
		return parsedArgs{exprs: exprs, err: err}
	})
	return p.exprs, p.err
}

// parseName is parseName for the session, reused across calls.
func (s *Session) parseName(text string) parsedName {
	return s.nameMemo.get(text, parseName)
}
