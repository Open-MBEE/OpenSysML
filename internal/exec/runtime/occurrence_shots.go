package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const (
	occurrenceFQN          = "Occurrences::Occurrence"
	occurrenceStartShotFQN = occurrenceFQN + "::startShot"
	occurrenceEndShotFQN   = occurrenceFQN + "::endShot"
)

// occurrenceShots are the snapshots of an occurrence the runtime has handed out:
// its start shot once it began and its end shot once it ended, each materialized
// on the first read of it and kept for every later one, as the universal clock
// object is.
type occurrenceShots struct {
	start int64
	end   int64
}

// shotValue answers a read of a member of inst subsetting `startShot` or
// `endShot` (Occurrences.kerml: the snapshots the occurrence starts and ends
// with; `Actions::Action::start`/`done` redefine them): the snapshot object once
// the shot has happened, undetermined with the reason while it has not; false
// for a member that is neither.
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
		// A snapshot is a portion of the occurrence, not a suboccurrence it owns;
		// a plain KerML feature's shot is an object of the feature's type.
		typ := sym
		if semantics.IsValueType(sym) {
			if types := ctx.model.semantics.FeatureTypes(sym); len(types) > 0 {
				typ = types[0]
			}
		}
		shot, err := ctx.materialize(typ, 0, nil, "")
		if err != nil {
			return Value{}, err
		}
		prior := ctx.lives[shot.ID]
		ctx.lives[shot.ID] = life{reached: prior.reached, began: at, ended: at, beganAt: atClock, endedAt: atClock}
		ctx.livesChanged()
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

// shotsCollection reports a collection member every shot is in: one `startShot`
// subsets transitively (`snapshots`, `portions`, `timeEnclosedOccurrences`, …).
func (ctx *Context) shotsCollection(sym *symbols.Symbol) bool {
	start := ctx.librarySymbol(occurrenceStartShotFQN)
	occurrence := ctx.librarySymbol(occurrenceFQN)
	return start != nil && occurrence != nil && ctx.subsetsTransitively(start, occurrence, sym)
}

// happenedShots are inst's snapshots that have happened, start shot first, read
// through the members of its type that stand for them.
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

// shotMemberOf is the member standing for the shot (startShot or endShot) on an
// object of typ: the nearest member of typ or what it specializes that subsets
// the shot, past a node a body declares under the member's name; nil for none.
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

// shotFeatureValue reads a shape feature of inst standing for its start or end
// snapshot (`start`/`done` a body steps through) from the occurrence's lifetime
// rather than a value of its own, which nothing gives it; nil for any other.
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
