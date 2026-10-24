package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const (
	occurrenceFQN          = "Occurrences::Occurrence"
	occurrenceStartShotFQN = occurrenceFQN + "::startShot"
	occurrenceEndShotFQN   = occurrenceFQN + "::endShot"
)

// occurrenceShots are the snapshots the runtime handed out for an occurrence:
// its start shot once it began and its end shot once it ended, made on first read.
type occurrenceShots struct {
	start int64
	end   int64
}

// shotValue answers a member of inst subsetting `startShot` or `endShot`: the
// snapshot once it happened, undetermined with the reason until; false for neither.
func (ctx *Context) shotValue(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature) (Value, bool, error) {
	if start := ctx.librarySymbol(occurrenceStartShotFQN); start != nil && ctx.subsetsTransitively(sym, inst.Type, start) {
		val, err := ctx.shotOf(inst, sym, feat, true)
		return val, true, err
	}
	if end := ctx.librarySymbol(occurrenceEndShotFQN); end != nil && ctx.subsetsTransitively(sym, inst.Type, end) {
		val, err := ctx.shotOf(inst, sym, feat, false)
		return val, true, err
	}
	return Value{}, false, nil
}

// shotOf is inst's start or end snapshot read through its member sym.
func (ctx *Context) shotOf(inst *Instance, sym *symbols.Symbol, feat *EffectiveFeature, start bool) (Value, error) {
	ctx.readsLives()
	if _, isShot := ctx.shotOccurrence(inst.ID); isShot {
		// A snapshot's start and end shots are itself (Occurrences.kerml: `snapshots { binding startShot = endShot }`).
		return ctx.objectValue(inst)
	}
	l := ctx.lives[inst.ID]
	shots := ctx.shots[inst.ID]
	id, happened, event, at, atClock := shots.end, l.ended != 0, "ended", l.ended, l.endedAt
	if start {
		id, happened, event, at, atClock = shots.start, l.began != 0, "started", l.began, l.beganAt
	}
	if !happened {
		return undeterminedFeatureValue(fmt.Sprintf("%s has not %s: its %s has not happened", SymbolText(inst.Type), event, sym.Name), feat.Multiplicity, sym), nil
	}
	if id == 0 {
		// A portion of the occurrence is the same thing over an instant (KerML §8.3.3
		// isPortion): an object of the occurrence's own type, owned by nothing.
		shot, err := ctx.materialize(inst.Type, 0, nil, "")
		if err != nil {
			return Value{}, err
		}
		prior := ctx.lives[shot.ID]
		ctx.lives[shot.ID] = life{reached: prior.reached, began: at, ended: at, beganAt: atClock, endedAt: atClock}
		ctx.livesChanged()
		if err := ctx.shotValues(shot, inst, start); err != nil {
			return Value{}, err
		}
		previous := shots
		if start {
			shots.start = shot.ID
		} else {
			shots.end = shot.ID
		}
		ctx.shots[inst.ID] = shots
		ctx.noteProbeUndo(func() { ctx.shots[inst.ID] = previous })
		id = shot.ID
	}
	return ctx.objectValue(ctx.instances[id])
}

// shotOccurrence is the occurrence the object id is a recorded shot of; false for none.
func (ctx *Context) shotOccurrence(id int64) (int64, bool) {
	for occurrence, shots := range ctx.shots {
		if id != 0 && (shots.start == id || shots.end == id) {
			return occurrence, true
		}
	}
	return 0, false
}

// shotsCollection reports a collection member every shot is in: one `startShot`
// subsets transitively (`snapshots`, `portions`, `timeEnclosedOccurrences`, …).
func (ctx *Context) shotsCollection(sym *symbols.Symbol) bool {
	start := ctx.librarySymbol(occurrenceStartShotFQN)
	occurrence := ctx.librarySymbol(occurrenceFQN)
	return start != nil && occurrence != nil && ctx.subsetsTransitively(start, occurrence, sym)
}

// happenedShots are inst's snapshots that have happened, start shot first.
func (ctx *Context) happenedShots(inst *Instance) ([]Value, error) {
	var shots []Value
	for _, fqn := range [...]string{occurrenceStartShotFQN, occurrenceEndShotFQN} {
		member := ctx.shotMemberOf(inst.Type, ctx.librarySymbol(fqn))
		if member == nil {
			continue
		}
		feat := ctx.effectiveFeature(member.Name, member, inst.Type)
		val, err := ctx.shotOf(inst, member, &feat, fqn == occurrenceStartShotFQN)
		if err != nil {
			return nil, err
		}
		if val.Kind == ValInstance {
			shots = append(shots, val)
		}
	}
	return shots, nil
}

// shotMemberOf is the nearest member of typ, or of what it specializes, that
// subsets the shot (startShot or endShot); nil for none.
func (ctx *Context) shotMemberOf(typ, shot *symbols.Symbol) *symbols.Symbol {
	if shot == nil {
		return nil
	}
	for _, src := range append([]*symbols.Symbol{typ}, ctx.model.semantics.AllSupertypes(typ)...) {
		for _, m := range ctx.model.semantics.MembersOf(src) {
			if m == shot || (m != nil && m.IsFeature() && ctx.subsetsTransitively(m, src, shot)) {
				return m
			}
		}
	}
	return nil
}

// shotFeatureValue reads a shape feature standing for inst's start or end shot
// (`start`/`done` a body steps through) from its lifetime; nil for any other.
func (inst *Instance) shotFeatureValue(ctx *Context, name string) (*FeatureValue, error) {
	fv := inst.FeatureValues[name]
	if fv.Materialized || fv.HeldValue().Kind != ValInvalid || fv.Feature == nil || fv.Feature.Symbol == nil || !fv.Feature.Scalar() {
		return nil, nil
	}
	sym, feat := fv.Feature.Symbol, fv.Feature
	if _, initial := sym.Decl.(*ast.InitialNode); initial {
		// A body's `first start;` is a node under the inherited start's name.
		if inherited := ctx.shotMemberOf(inst.Type, ctx.librarySymbol(occurrenceStartShotFQN)); inherited != nil {
			effective := ctx.effectiveFeature(name, inherited, inst.Type)
			sym, feat = inherited, &effective
		}
	}
	val, ok, err := ctx.shotValue(inst, sym, feat)
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &FeatureValue{Feature: fv.Feature, Value: val, Materialized: val.Kind != ValUndetermined}, nil
}

// shotValues gives a shot the values the occurrence held at its instant: an ended
// occurrence's values are its end values, and a value nothing wrote since the start
// is its start value; one written since was not recorded then, so it is undetermined.
func (ctx *Context) shotValues(shot, inst *Instance, start bool) error {
	for name, fv := range inst.FeatureValues {
		target, ok := shot.FeatureValues[name]
		if !ok || !fv.Materialized || fv.Feature == nil || !fv.Feature.Scalar() && fv.Values.Kind == ValInvalid {
			continue
		}
		value := fv.HeldValue()
		switch value.Kind {
		case ValInvalid, ValInstance, ValSequence, ValSet, ValArray:
			continue
		}
		if start && fv.Written {
			value = undeterminedFeatureValue(fmt.Sprintf("%s was assigned after %s started: its value then was not recorded", name, SymbolText(inst.Type)), fv.Feature.Multiplicity, fv.Feature.Symbol)
		}
		if err := shot.BindFeatureValue(ctx, name, value); err != nil {
			return err
		}
		target.Written = true
	}
	return nil
}
