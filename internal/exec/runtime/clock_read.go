package runtime

import "fmt"

// universalClockFQN names the Kernel Semantic Library's universal clock, which
// every occurrence's localClock defaults to.
const universalClockFQN = "Clocks::universalClock"

// timeUniversalClockFQN names the Time domain library's universal clock, whose
// currentTime is a TimeInstantValue.
const timeUniversalClockFQN = "Time::universalClock"

// clockFQN names the Kernel's Clock, whose currentTime every clock advances.
const clockFQN = "Clocks::Clock"

// timeClockFQN names the Time library's Clock, whose currentTime is a quantity.
const timeClockFQN = "Time::Clock"

// currentTimeName is the feature of a Clock that advances over its lifetime.
const currentTimeName = "currentTime"

func init() {
	registerLibraryFeature(universalClockFQN, func(ctx *Context) (Value, error) {
		return ctx.universalClockObject(universalClockFQN)
	})
	registerLibraryFeature(timeUniversalClockFQN, func(ctx *Context) (Value, error) {
		return ctx.universalClockObject(timeUniversalClockFQN)
	})
}

// universalClockObject is the run's universal clock as the one object the
// library usage denotes, materialized once per context.
func (ctx *Context) universalClockObject(fqn string) (Value, error) {
	sym := ctx.librarySymbol(fqn)
	if sym == nil {
		return Value{}, fmt.Errorf("%w: no library declares %s", ErrUnresolvedReference, fqn)
	}
	inst, err := ctx.occurrenceOf(sym)
	if err != nil {
		return Value{}, err
	}
	ctx.beganWithRun(inst)
	return Value{Kind: ValInstance, Instance: inst.ID}, nil
}

// beganWithRun dates the universal clock's life from the run's first activation:
// the clock the run is timed against exists throughout it, not from its first read.
func (ctx *Context) beganWithRun(inst *Instance) {
	l := ctx.lives[inst.ID]
	if l.began <= 1 && l.reached <= 1 {
		return
	}
	l.reached, l.began, l.beganAt = 1, 1, 0
	ctx.lives[inst.ID] = l
	ctx.livesChanged()
}

// clockMember answers a Clock object's currentTime from the shared clock every
// executor of the context advances: a Time::Clock reads the instant on its own
// scale, seconds since the run began as `accept at` waits for it; any other
// Clock the Kernel's bare number of seconds. Other members are not answered.
// The instant is the run's, not the shape's: nothing derived over it is shared.
func (ctx *Context) clockMember(inst *Instance, name string) (Value, bool, error) {
	if !ctx.isClockTime(inst, name) {
		return Value{}, false, nil
	}
	ctx.unshareTraces()
	if ctx.conformsToLibrary(ctx.objectType(inst), timeClockFQN) {
		return ctx.ClockValue(), true, nil
	}
	return realConst(ctx.clock.Now()), true, nil
}

// isClock reports whether inst is an object of a Clock, as the Kernel declares one.
func (ctx *Context) isClock(inst *Instance) bool {
	return inst != nil && ctx.conformsToLibrary(ctx.objectType(inst), clockFQN)
}

// isClockTime reports whether name, on inst, is a Clock's currentTime: the
// feature itself or a redefinition of it (`attribute now :>> currentTime;`),
// which is the same feature under another name.
func (ctx *Context) isClockTime(inst *Instance, name string) bool {
	if !ctx.isClock(inst) {
		return false
	}
	if name == currentTimeName {
		return true
	}
	typ := ctx.objectType(inst)
	sym, ok := ctx.model.semantics.LookupMember(typ, name)
	if !ok {
		return false
	}
	for _, redefined := range ctx.redefinedFeatures(sym, typ) {
		if redefined.Name != currentTimeName || redefined.OwnerScope == nil {
			continue
		}
		if ctx.conformsToLibrary(redefined.OwnerScope.Owner(), clockFQN) {
			return true
		}
	}
	return false
}
