package edit

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (m Model) deleteSplices(i int, op Operation) ([]splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return nil, err
	}
	targets := m.cascadeTargets(sym)
	if len(targets) > 1 && !op.Cascade {
		referring := make([]string, 0, len(targets)-1)
		for _, referrer := range targets[1:] {
			referring = append(referring, m.Index.GetFQN(referrer))
		}
		return nil, &Error{
			Failure:        FailureDeleteReferenced,
			OperationIndex: i,
			Referring:      referring,
			Message: op.Target + " is referenced by " + strings.Join(referring, ", ") +
				"; delete it with cascade to remove those declarations",
		}
	}
	out := make([]splice, 0, len(targets))
	for _, target := range targets {
		span := m.deleteSpan(target)
		out = append(out, splice{
			span: span, opIndex: i, target: m.Index.GetFQN(target),
		})
	}
	return out, nil
}

// cascadeTargets is sym followed by every declaration removing it would leave
// dangling: referrers of sym or of anything it contains, then their referrers,
// until none remain. A declaration nested in another target is left to it.
func (m Model) cascadeTargets(sym *symbols.Symbol) []*symbols.Symbol {
	targets := []*symbols.Symbol{sym}
	seen := map[string]bool{m.Index.GetFQN(sym): true}
	for frontier := targets; len(frontier) > 0; {
		var next []*symbols.Symbol
		for _, referrer := range m.referringSymbols(targets, frontier) {
			fqn := m.Index.GetFQN(referrer)
			if !seen[fqn] {
				seen[fqn] = true
				next = append(next, referrer)
			}
		}
		targets = append(targets, next...)
		frontier = next
	}
	return withoutNested(targets)
}

// withoutNested drops every symbol declared inside another's span.
func withoutNested(syms []*symbols.Symbol) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(syms))
	for _, sym := range syms {
		nested := false
		for _, other := range syms {
			if !symbols.SameElement(sym, other) && declaredWithin(sym, other) {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, sym)
		}
	}
	return out
}

// declaredWithin reports whether sym's declaration lies inside outer's.
func declaredWithin(sym, outer *symbols.Symbol) bool {
	return sym.DocName == outer.DocName && outer.DeclSpan.Len > 0 &&
		outer.DeclSpan.Offset <= sym.DeclSpan.Offset &&
		outer.DeclSpan.End() >= sym.DeclSpan.End()
}

// referringSymbols returns the declarations outside every target that refer to
// one of frontier or to a declaration inside one, sorted by qualified name.
func (m Model) referringSymbols(targets, frontier []*symbols.Symbol) []*symbols.Symbol {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if rootScope == nil {
		return nil
	}
	within := func(sym *symbols.Symbol, set []*symbols.Symbol) bool {
		for _, member := range set {
			if symbols.SameElement(sym, member) || declaredWithin(sym, member) {
				return true
			}
		}
		return false
	}
	r, _ := m.resolver()
	seen := map[string]bool{}
	var out []*symbols.Symbol
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for i, part := range ref.QN.Parts {
			seg, ok := r.PartSymbol(ref.QN, i)
			if !ok || part.Span == seg.NameSpan || !within(seg, frontier) {
				continue
			}
			referrer := m.symbolContaining(part.Span.Offset)
			if referrer == nil || within(referrer, targets) {
				continue
			}
			fqn := m.Index.GetFQN(referrer)
			if fqn != "" && !seen[fqn] {
				seen[fqn] = true
				out = append(out, referrer)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return m.Index.GetFQN(out[i]) < m.Index.GetFQN(out[j])
	})
	return out
}

// symbolContaining is the innermost declaration of this document whose span
// covers offset.
func (m Model) symbolContaining(offset int) *symbols.Symbol {
	var found *symbols.Symbol
	for _, fqn := range m.Index.FQNs() {
		for _, candidate := range m.Index.LookupQualified(fqn) {
			if candidate == nil || candidate.DocName != m.Source.Name() {
				continue
			}
			span := candidate.DeclSpan
			if span.Len == 0 || offset < span.Offset || offset >= span.End() {
				continue
			}
			if found == nil || span.Len < found.DeclSpan.Len {
				found = candidate
			}
		}
	}
	return found
}

func (m Model) deleteSpan(sym *symbols.Symbol) source.Span {
	span := sym.DeclSpan
	if span.Len == 0 && sym.Decl != nil {
		span = sym.Decl.Span()
	}
	content := m.Source.Bytes()
	start := span.Offset
	end := m.declarationEnd(sym)
	if end <= start || end > len(content) {
		end = span.End()
	}
	lineStart := start
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}
	// A declaration on its own line owns that line, including its newline.
	if onlyWhitespace(content[lineStart:start]) && onlyWhitespace(content[end:lineEnd]) {
		if lineEnd < len(content) {
			lineEnd++
		}
		start = lineStart
		end = lineEnd
	}
	// Include contiguous leading comment lines, but stop at a blank line so a
	// neighboring declaration's comment ownership is never consumed.
	for {
		cursor := start - 1
		if cursor < 0 {
			break
		}
		if content[cursor] == '\n' {
			cursor--
		}
		if cursor < 0 {
			break
		}
		prevLineEnd := cursor + 1
		prevLineStart := prevLineEnd
		for prevLineStart > 0 && content[prevLineStart-1] != '\n' {
			prevLineStart--
		}
		line := strings.TrimSpace(string(content[prevLineStart:prevLineEnd]))
		if line == "" {
			// A blank line immediately before a leading comment belongs to
			// that comment block and is removed with it.
			start = prevLineStart
			break
		}
		if len(line) < 2 || (!strings.HasPrefix(line, "//") &&
			!strings.HasPrefix(line, "/*") && !strings.HasPrefix(line, "*")) {
			break
		}
		start = prevLineStart
	}
	return source.Span{Offset: start, Len: end - start}
}

func (m Model) declarationEnd(sym *symbols.Symbol) int {
	start := sym.NameSpan.End()
	if start <= 0 {
		start = sym.DeclSpan.Offset
	}
	depth := 0
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.End() <= start {
			continue
		}
		switch tok.Kind {
		case lexer.LBrace:
			depth++
		case lexer.RBrace:
			if depth > 0 {
				depth--
				if depth == 0 {
					return tok.Span.End()
				}
			}
		case lexer.Semicolon:
			if depth == 0 {
				return tok.Span.End()
			}
		}
	}
	return 0
}

func onlyWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}
