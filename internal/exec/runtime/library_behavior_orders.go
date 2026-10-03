package runtime

import (
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

type libraryOrdersKey struct{}

var libraryOrderWalks atomic.Int64

func libraryBehaviorOrders(base *symbols.Index) map[*symbols.Scope][]lower.BehaviorOrder {
	if base == nil || !base.Frozen() {
		return nil
	}
	return base.Derived(libraryOrdersKey{}, func() any {
		libraryOrderWalks.Add(1)
		resolver := resolve.New(base)
		model := semantics.NewModel(resolver)
		resolver.SetModel(model)
		byRoot := make(map[*symbols.Scope][]lower.BehaviorOrder)
		for _, name := range base.Documents() {
			root := base.DocumentRoot(name)
			byRoot[root] = lower.BehaviorOrders(model, root)
		}
		return byRoot
	}).(map[*symbols.Scope][]lower.BehaviorOrder)
}
