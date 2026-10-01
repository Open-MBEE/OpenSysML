package symbolfacts

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// LookupNamed resolves a symbol ID written in either spelling: the quoted,
// notation-legal form a model author writes ('My Pkg'::Car), or the unquoted
// spelling the index records (My Pkg::Car), which keeps working as it did.
// Model elements come before library homonyms: an ID naming both denotes the
// model's own element, which is what the client asked about.
func LookupNamed(idx *symbols.Index, id string) []*symbols.Symbol {
	if syms := idx.LookupQualified(id); len(syms) > 0 {
		return modelFirst(idx, syms)
	}
	if plain, ok := UnquotedName(id); ok && plain != id {
		return modelFirst(idx, idx.LookupQualified(plain))
	}
	return nil
}

// modelFirst reorders matches so the ones the model declares precede the ones
// standard-library content declares, each group keeping its index order.
func modelFirst(idx *symbols.Index, syms []*symbols.Symbol) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(syms))
	var lib []*symbols.Symbol
	for _, sym := range syms {
		if idx.Library(sym) {
			lib = append(lib, sym)
			continue
		}
		out = append(out, sym)
	}
	return append(out, lib...)
}

// UnquotedName is the name a notation-legal qualified name states, with the
// quoting of its unrestricted segments removed, or false for an ID the notation
// does not read as one whole name.
func UnquotedName(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	p := parser.New(source.New("<symbol-id>", []byte(id)))
	expr := p.ParseExpression()
	if len(p.Diagnostics) > 0 || p.Offset() != len(id) {
		return "", false
	}
	ref, ok := expr.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ref.Name.Global || len(ref.Name.Parts) == 0 {
		return "", false
	}
	segments := make([]string, 0, len(ref.Name.Parts))
	for _, part := range ref.Name.Parts {
		if part.Text == "" {
			return "", false
		}
		segments = append(segments, part.Text)
	}
	return strings.Join(segments, "::"), true
}
