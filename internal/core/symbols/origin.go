package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Origin identifies the source declaration behind a derived artifact.
// Synthetic and cache-restored artifacts use the zero value.
type Origin struct {
	Doc  string
	Span source.Span
	Name source.Span
}

// Located reports whether the origin identifies a source location.
func (o Origin) Located() bool { return o.Doc != "" && o.Span.Len > 0 }

// Origin returns the origin of s, or the zero origin when it is synthetic or nil.
func (s *Symbol) Origin() Origin {
	if s == nil {
		return Origin{}
	}
	origin := OriginAt(s.DocName, s.DeclSpan)
	if origin.Located() && s.NameSpan.Len > 0 {
		origin.Name = s.NameSpan
	}
	return origin
}

// NodeOrigin returns the origin of node in doc.
func NodeOrigin(doc string, node ast.Node) Origin {
	if node == nil {
		return Origin{}
	}
	return OriginAt(doc, node.Span())
}

// OriginAt returns the origin of span in doc.
func OriginAt(doc string, span source.Span) Origin {
	if doc == "" || span.Len <= 0 {
		return Origin{}
	}
	return Origin{Doc: doc, Span: span}
}
