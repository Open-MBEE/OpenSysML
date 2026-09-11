package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// HeldStateError reports an object that is no longer as its declaration materializes
// it, so a fresh object of the declaration would not stand for it: what it carries is
// the reason.
type HeldStateError struct {
	ID     int64
	Type   *symbols.Symbol
	Reason string
}

func (e *HeldStateError) Error() string {
	return fmt.Sprintf("object #%d (%s) %s", e.ID, symbolText(e.Type), e.Reason)
}

// Pristine reports whether inst stands as its declaration materializes it — nothing
// written to it or to an object it holds, every behavior it runs as its start left it,
// not destroyed — so a fresh object of the declaration is inst as it stands. Otherwise
// the HeldStateError names the first thing inst carries that a fresh object would not.
func (ctx *Context) Pristine(inst *Instance) error {
	return ctx.pristine(inst, make(map[int64]bool))
}

func (ctx *Context) pristine(inst *Instance, seen map[int64]bool) error {
	if seen[inst.ID] {
		return nil
	}
	seen[inst.ID] = true
	if l, ok := ctx.lives[inst.ID]; ok && l.destroyed {
		return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: "was destroyed"}
	}
	for _, b := range inst.behaviors {
		if !b.Moved() {
			continue
		}
		name := b.Name
		if name == "" {
			name = symbolText(b.Symbol)
		}
		return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: fmt.Sprintf("runs %s %s, an execution that has moved since its start", b.Kind, name)}
	}
	for _, name := range slices.Sorted(maps.Keys(inst.FeatureValues)) {
		fv := inst.FeatureValues[name]
		if fv.Written {
			return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: fmt.Sprintf("had %s written by a run", name)}
		}
		if !fv.Materialized {
			continue
		}
		for _, held := range heldInstances(ctx, fv) {
			if err := ctx.pristine(held, seen); err != nil {
				return err
			}
		}
	}
	return nil
}
