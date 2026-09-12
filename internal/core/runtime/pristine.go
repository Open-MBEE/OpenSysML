package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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

// Pristine reports whether inst is as its declaration materializes it in a fresh context:
// nothing written, no behavior moved nor waiting on a clock past zero, no message awaiting
// it or open to it, not destroyed. Else a HeldStateError names the reason.
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
		name := b.Name
		if name == "" {
			name = symbolText(b.Symbol)
		}
		if b.Moved() {
			return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: fmt.Sprintf("runs %s %s, an execution that has moved since its start", b.Kind, name)}
		}
		// A fresh context's clock starts at zero, so a wait on a clock past zero
		// is due at another instant than a fresh declaration's would be.
		if waits := b.armedWaits(); len(waits) > 0 && ctx.clock.now != 0 {
			return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: fmt.Sprintf("runs %s %s, which waits on the clock (%s) with the clock at t=%s, not at zero",
				b.Kind, name, waits[0].What, semantics.FormatReal(ctx.clock.now))}
		}
		// A message addressed to no object in particular is open to this one's executions.
		for _, msg := range ctx.messages {
			if msg.Object == 0 {
				return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: fmt.Sprintf("runs %s %s while a %s addressed to no object in particular awaits dispatch", b.Kind, name, msg.SignalType)}
			}
		}
	}
	for _, msg := range ctx.messages {
		if msg.Object == inst.ID {
			return &HeldStateError{ID: inst.ID, Type: inst.Type, Reason: fmt.Sprintf("has a %s posted to it awaiting dispatch", msg.SignalType)}
		}
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
