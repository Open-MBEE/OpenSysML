package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func (m Model) addMetadataPrefixSplice(i int, op Operation) (splice, error) {
	if m.Source.Kind() != source.KindSysML {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("metadata prefix is not legal in %s source %q", m.Source.Kind(), m.Source.Name()),
		}
	}
	if err := checkQualifiedReference(i, "metadata type", op.MetadataType); err != nil {
		return splice{}, err
	}
	sym, err := m.target(i, op)
	if err != nil {
		return splice{}, err
	}
	prefixes, _, ok := ast.DeclaredMetadata(sym.Decl)
	if !ok {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("%q cannot carry prefix metadata", op.Target),
		}
	}
	if _, ok := sym.Decl.(*ast.TransitionMember); ok {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("transition %q cannot carry prefix metadata", op.Target),
		}
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		if usage.IsStateAction() {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("state action %q cannot carry prefix metadata", op.Target),
			}
		}
		if usage.Kind == ast.UsageTransition {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("transition %q cannot carry prefix metadata", op.Target),
			}
		}
	}

	if _, ok := metadataQualifiedName(op.MetadataType); !ok {
		return splice{}, &Error{
			Failure: FailureInvalidName, OperationIndex: i,
			Message: fmt.Sprintf("metadata type %q is not a qualified name", op.MetadataType),
		}
	}

	declSpan := sym.Decl.Span()
	for _, diagnostic := range parseDiagnostics(m.ParseDiags) {
		if diagnostic.Span.Offset >= declSpan.Offset && diagnostic.Span.Offset < declSpan.End() {
			return splice{}, &Error{
				Failure: FailureResultInvalid, OperationIndex: i,
				Diagnostics: []diag.Diagnostic{diagnostic}, Diagnosed: m.Source,
				Message: fmt.Sprintf("cannot add metadata prefix to %q: %s", op.Target, diagnostic.Message),
			}
		}
	}

	var insertion int
	if len(prefixes) > 0 {
		last := prefixes[len(prefixes)-1]
		insertion = m.tokenSpan(last.Span()).End()
		return m.deferPrefix(i, op, splice{
			span: source.Span{Offset: insertion}, text: " #" + op.MetadataType,
			opIndex: i, target: op.Target,
		}), nil
	}
	insertion, ok = metadataPrefixInsertion(m, sym.Decl)
	if !ok {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("%q has no prefix metadata insertion point", op.Target),
		}
	}
	return m.deferPrefix(i, op, splice{
		span: source.Span{Offset: insertion}, text: "#" + op.MetadataType + " ",
		opIndex: i, target: op.Target,
	}), nil
}

// deferPrefix has the prefix sp writes judged against the model the batch leaves.
func (m Model) deferPrefix(i int, op Operation, sp splice) splice {
	m.resolveLater(sp, func(after Model, region source.Span) []deferredRef {
		var refs []deferredRef
		inspectWithin(after.Root, region, func(n ast.Node) {
			if prefix, ok := n.(*ast.PrefixMetadata); ok && prefix.Type != nil && prefix.Type.Text() == op.MetadataType {
				refs = append(refs, deferredRef{offset: prefix.Span().Offset, check: func(final Model, offset int) error {
					return final.checkMetadataPrefix(i, op.Target, offset)
				}})
			}
		})
		return refs
	})
	return sp
}

// checkMetadataPrefix refuses the metadata prefix written at offset in m — the
// model the batch leaves — unless it names a metadata definition, from the
// declaration it prefixes, that no other prefix of that declaration names too;
// a byte no prefix is written at any more leaves the verdict to validation.
func (m Model) checkMetadataPrefix(i int, target string, offset int) error {
	sym, prefix := m.metadataPrefixAt(offset)
	if prefix == nil || prefix.Type == nil {
		return nil
	}
	name := prefix.Type.Text()
	r, _ := m.resolver()
	scope := r.PrefixScope(sym.OwnerScope, sym.Decl)
	metadataType, resolved := r.ResolveQualified(scope, prefix.Type)
	if resolved && metadataType != nil {
		metadataType, resolved = r.ResolveAliasTarget(metadataType)
	}
	if !resolved || metadataType == nil {
		return &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: fmt.Sprintf("metadata type %q does not resolve from %s", name, notationName(sym)),
		}
	}
	if metadataType.Kind != symbols.SymbolMetadataDef {
		return &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: fmt.Sprintf("%q is a %s, not a metadata definition", name, metadataType.Kind),
		}
	}
	prefixes, _, _ := ast.DeclaredMetadata(sym.Decl)
	for _, other := range prefixes {
		if other == nil || other == prefix || other.Type == nil {
			continue
		}
		otherType, found := r.ResolveQualified(scope, other.Type)
		if found && otherType != nil {
			otherType, found = r.ResolveAliasTarget(otherType)
		}
		if found && otherType == metadataType {
			return &Error{
				Failure: FailureInvalidValue, OperationIndex: i,
				Message: fmt.Sprintf("%q already carries #%s", target, name),
			}
		}
	}
	return nil
}

func metadataQualifiedName(text string) (*ast.QualifiedName, bool) {
	global := strings.HasPrefix(text, "$::")
	if global {
		text = text[3:]
	}
	segments, ok := source.QualifiedNameSegments(text)
	if !ok || len(segments) == 0 {
		return nil, false
	}
	qn := &ast.QualifiedName{Global: global}
	qn.Parts = make([]ast.NameSegment, len(segments))
	for i, segment := range segments {
		qn.Parts[i].Text = segment
	}
	return qn, true
}

func metadataPrefixInsertion(m Model, decl ast.Node) (int, bool) {
	span := decl.Span()
	end := span.End()
	nonModifierKeywords := map[string]bool{
		"standard": true, "library": true, "timeslice": true,
		"subject": true, "actor": true, "stakeholder": true, "objective": true,
		"variant": true, "assume": true, "require": true, "then": true,
		"verify": true, "frame": true, "render": true, "return": true,
	}
	var skipThrough int
	if usage, ok := decl.(*ast.Usage); ok && usage.IsEnd && usage.CrossFeature != nil {
		skipThrough = m.tokenSpan(usage.CrossFeature.Span()).End()
	}
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset < span.Offset {
			continue
		}
		if tok.Span.Offset >= end {
			break
		}
		if tok.IsTrivia() || tok.Kind == lexer.RegularComment || tok.Span.End() <= skipThrough {
			continue
		}
		if tok.Kind == lexer.Keyword &&
			(parser.IsPrefixModifierKeyword(tok.KeywordID) || nonModifierKeywords[tok.KeywordID]) {
			continue
		}
		return tok.Span.Offset, true
	}
	return 0, false
}
