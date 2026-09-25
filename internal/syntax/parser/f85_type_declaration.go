package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// KerML `type` (KerML.xtext Type:319) introduces a declaration; in SysML the
// same word is an ordinary name, e.g. a parameter called `type`.
var (
	kermlDefinitionKindKeywords = map[string]ast.DefinitionKind{"type": ast.DefClass}
	kermlUsageKindKeywords      = map[string]ast.UsageKind{"type": ast.UsageClass}
)

// definitionKind reports the DefinitionKind a kind keyword introduces here.
func (p *Parser) definitionKind(kw string) (ast.DefinitionKind, bool) {
	if k, ok := definitionKindKeywords[kw]; ok {
		return k, true
	}
	if p.src.Kind() == source.KindKerML {
		k, ok := kermlDefinitionKindKeywords[kw]
		return k, ok
	}
	return 0, false
}

// usageKind reports the UsageKind a kind keyword introduces here.
func (p *Parser) usageKind(kw string) (ast.UsageKind, bool) {
	if k, ok := usageKindKeywords[kw]; ok {
		return k, true
	}
	if p.src.Kind() == source.KindKerML {
		k, ok := kermlUsageKindKeywords[kw]
		return k, ok
	}
	return 0, false
}

func (p *Parser) definitionKindOf(kw string) ast.DefinitionKind {
	k, _ := p.definitionKind(kw)
	return k
}

func (p *Parser) usageKindOf(kw string) ast.UsageKind {
	k, _ := p.usageKind(kw)
	return k
}

// checkTypeDeclarationSpecialization reports a `type` declaring neither a
// specialization nor a conjugation, which KerML.xtext TypeDeclaration:324 requires.
func (p *Parser) checkTypeDeclarationSpecialization(u *ast.Usage, keyword string) {
	if keyword != "type" || p.src.Kind() != source.KindKerML {
		return
	}
	for _, r := range u.Relationships {
		switch r.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
			return
		}
	}
	span := u.Ident.NameSpan
	if span.Len == 0 {
		span = p.peek().Span
	}
	p.error(span, "a 'type' declaration must specialize or conjugate a type")
}
