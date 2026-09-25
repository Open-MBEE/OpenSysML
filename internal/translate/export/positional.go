package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// DeclarationNames maps declarations to their RDF qualified names by span.
// Unnamed declarations use the export's positional `@N` identity.
func DeclarationNames(file *source.SourceFile, root *ast.RootNamespace) (map[source.Span]string, error) {
	e, err := newEncoder(file, root, "", IDQualifiedName)
	if err != nil {
		return nil, err
	}
	names := make(map[source.Span]string, len(e.fqn))
	ambiguous := make(map[source.Span]bool)
	for node, name := range e.fqn {
		span := node.Span()
		if previous, exists := names[span]; exists && previous != name {
			delete(names, span)
			ambiguous[span] = true
			continue
		}
		if !ambiguous[span] {
			names[span] = name
		}
	}
	return names, nil
}
