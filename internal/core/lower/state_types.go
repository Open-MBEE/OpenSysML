package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// StateActionFQN names the library state every state specializes, whose
// content (self, substates, transitions) a consumer of the graph carries natively.
const StateActionFQN = "States::StateAction"

// LibraryStateTypes lowers a state machine through the name-resolution tier, so a
// definition in another document contributes, withholding States::StateAction's content.
type LibraryStateTypes struct {
	*resolve.Resolver
	frame *symbols.Symbol
}

// NewLibraryStateTypes is the LibraryStateTypes over resolver, nil without one:
// lowering then names its endpoints from the scope tree alone.
func NewLibraryStateTypes(resolver *resolve.Resolver) EndpointResolver {
	if resolver == nil {
		return nil
	}
	types := &LibraryStateTypes{Resolver: resolver}
	if idx := resolver.Index(); idx != nil {
		for _, sym := range idx.LookupQualified(StateActionFQN) {
			if idx.Library(sym) {
				types.frame = sym
				break
			}
		}
	}
	return types
}

// WithholdsStateType reports the library's StateAction, whose content lowering
// must not take: TypeDecl still resolves it, so lowering looks no further.
func (s *LibraryStateTypes) WithholdsStateType(decl ast.Node) bool {
	return s.frame != nil && decl == s.frame.Decl
}
