package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// atDeclarationConjugation reports whether the cursor is at the conjugation part
// of a KerML type or feature declaration: `type C conjugates T;`, `feature f ~ g`
// (KerML.xtext ClassifierConjugationPart:468, FeatureConjugationPart:730).
func (p *Parser) atDeclarationConjugation() bool {
	if p.src.Kind() != source.KindKerML {
		return false
	}
	if p.atKeyword("conjugates") {
		return true
	}
	return p.at(lexer.Tilde) && p.peekN(1).Kind != lexer.EOF
}

// parseDeclarationConjugation parses the conjugation part, recorded as a
// conjugated generalization edge.
func (p *Parser) parseDeclarationConjugation() *ast.Relationship {
	start := p.peek().Span.Offset
	p.advance() // 'conjugates' or '~'
	r := &ast.Relationship{
		Kind:       ast.RelSpecializes,
		Target:     p.parseRelationshipTarget(),
		Conjugated: true,
	}
	r.NodeSpan = p.spanFrom(start)
	return r
}
