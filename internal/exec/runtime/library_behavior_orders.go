package runtime

import (
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

var libraryOrders sync.Map // *symbols.Index (frozen) → *libraryOrderSet

type libraryOrderSet struct {
	once   sync.Once
	byRoot map[*symbols.Scope][]lower.BehaviorOrder
}

var libraryOrderWalks atomic.Int64

func libraryBehaviorOrders(base *symbols.Index) map[*symbols.Scope][]lower.BehaviorOrder {
	if base == nil || !base.Frozen() {
		return nil
	}
	value, _ := libraryOrders.LoadOrStore(base, &libraryOrderSet{})
	set := value.(*libraryOrderSet)
	set.once.Do(func() {
		libraryOrderWalks.Add(1)
		resolver := resolve.New(base)
		model := semantics.NewModel(resolver)
		resolver.SetModel(model)
		set.byRoot = make(map[*symbols.Scope][]lower.BehaviorOrder)
		for _, name := range base.Documents() {
			root := base.DocumentRoot(name)
			set.byRoot[root] = lower.BehaviorOrders(model, root)
		}
	})
	return set.byRoot
}
