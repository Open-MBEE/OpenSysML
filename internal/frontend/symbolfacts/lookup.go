package symbolfacts

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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

// LookupNamedMember is LookupNamed for an ID the index registers no declaration
// under because its last segments name members the type of the segment before
// them contributes: a member a subtype inherits (Sub::inherited) or one reached
// through a feature's type (Outer::inner::b). Such a name is walked through the
// model's member lookup, so the member a redefinition replaces is answered by
// the redefinition, as a reference in the notation resolves.
func LookupNamedMember(idx *symbols.Index, model *semantics.Model, id string) []*symbols.Symbol {
	if syms := LookupNamed(idx, id); len(syms) > 0 {
		return syms
	}
	name := id
	if plain, ok := UnquotedName(id); ok {
		name = plain
	}
	if sym, _ := MemberChain(idx, model, name); sym != nil {
		return []*symbols.Symbol{sym}
	}
	return nil
}

// MemberChain resolves a qualified name, in the unquoted spelling the index
// records, whose later segments are members of the type of the segment before
// them: the longest prefix one declaration answers to is the chain's root, and
// the rest is walked through model's member lookup, which follows typing and
// specialization. A prefix the model declares shadows a library declaration of
// the same name, as the notation resolves the name. The second result is the
// fully-qualified name the member is declared under; nil and "" when no prefix
// is a declaration the rest walks from.
func MemberChain(idx *symbols.Index, model *semantics.Model, name string) (*symbols.Symbol, string) {
	if idx == nil || model == nil {
		return nil, ""
	}
	segments := strings.Split(name, "::")
	// The longest prefix is tried first, so a nested feature is preferred over
	// the type it happens to share a name with.
	for i := len(segments) - 1; i > 0; i-- {
		sym, ambiguous := soleDeclaration(idx, strings.Join(segments[:i], "::"))
		if ambiguous {
			// A shorter prefix would walk into one of the duplicates at random.
			return nil, ""
		}
		if sym == nil {
			continue
		}
		walked := true
		for _, seg := range segments[i:] {
			member, ok := model.LookupMember(sym, seg)
			if !ok || member == nil {
				walked = false
				break
			}
			sym = member
		}
		if !walked {
			continue
		}
		fqn := idx.GetFQN(sym)
		if fqn == "" {
			fqn = name
		}
		return sym, fqn
	}
	return nil, ""
}

// soleDeclaration is the one declaration a qualified name denotes, the model's
// shadowing the library's; ambiguous when the model declares the name twice.
func soleDeclaration(idx *symbols.Index, fqn string) (sym *symbols.Symbol, ambiguous bool) {
	matches := modelFirst(idx, idx.LookupQualified(fqn))
	declared := 0
	for declared < len(matches) && !idx.Library(matches[declared]) {
		declared++
	}
	switch {
	case declared == 1, declared == 0 && len(matches) == 1:
		return matches[0], false
	case declared > 1:
		return nil, true
	}
	return nil, false
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
