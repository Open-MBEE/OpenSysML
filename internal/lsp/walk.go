package lsp

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// collectRefs gathers every qualified-name reference in a document with the
// scope it resolves in (see resolve.References).
func collectRefs(root *ast.RootNamespace, rootScope *symbols.Scope) []resolve.Reference {
	return resolve.References(root, rootScope)
}

// refAtOffset returns the reference whose qualified-name span contains offset.
func refAtOffset(refs []resolve.Reference, offset int) *resolve.Reference {
	for i := range refs {
		sp := refs[i].QN.Span()
		if offset >= sp.Offset && offset < sp.End() {
			return &refs[i]
		}
	}
	return nil
}

// segmentAt returns the index of the qualified-name segment containing offset,
// or -1 when the cursor is between segments.
func segmentAt(ref resolve.Reference, offset int) int {
	for i, p := range ref.QN.Parts {
		if offset >= p.Span.Offset && offset < p.Span.End() {
			return i
		}
	}
	return -1
}

// onCalledName reports whether offset is on the last segment of ref, the one an
// invocation calls, rather than on a qualifier before it. A `::` between
// segments counts as the whole reference, which names what the last segment does.
func onCalledName(ref resolve.Reference, offset int) bool {
	idx := segmentAt(ref, offset)
	return idx < 0 || idx == len(ref.QN.Parts)-1
}

// referencedSegment resolves the qualified-name segment containing offset with
// that segment's span: a qualifier names the namespace it denotes, the last
// segment the reference's target. false when no segment resolves there.
func (s *Server) referencedSegment(doc string, ref resolve.Reference, offset int) (*symbols.Symbol, source.Span, bool) {
	parts := ref.QN.Parts
	idx := segmentAt(ref, offset)
	if idx < 0 {
		return nil, source.Span{}, false
	}
	if idx == len(parts)-1 {
		target, ok := s.ws.ResolveReferenceInDoc(doc, ref)
		return target, parts[idx].Span, ok
	}
	segs := s.ws.ResolveReferenceSegmentsInDoc(doc, ref)
	if idx >= len(segs) || segs[idx] == nil {
		return nil, source.Span{}, false
	}
	return segs[idx], parts[idx].Span, true
}
