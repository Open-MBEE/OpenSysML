package edit

import (
	"bytes"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// modelTokenCache lazily stores lexer data for one edit model.
type modelTokenCache struct {
	built   bool
	tokens  []lexer.Token
	tabs    int
	hasTabs bool
}

func (m Model) tokenData() *modelTokenCache {
	if m.tokens == nil {
		m.tokens = new(modelTokenCache)
	}
	cache := m.tokens
	if !cache.built {
		content := m.Source.Bytes()
		cache.tabs = bytes.Count(content, []byte{'\t'})
		cache.hasTabs = cache.tabs > 0
		lx := lexer.New(m.Source)
		for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
			cache.tokens = append(cache.tokens, tok)
		}
		cache.built = true
	}
	return cache
}

func (m Model) lastToken(span source.Span, kind lexer.Kind) lexer.Token {
	tokens := m.tokenData().tokens
	end := sort.Search(len(tokens), func(i int) bool {
		return tokens[i].Span.Offset >= span.End()
	})
	for i := end - 1; i >= 0; i-- {
		if tokens[i].Kind == kind {
			return tokens[i]
		}
	}
	return lexer.Token{}
}
