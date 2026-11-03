package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

const (
	clocksTimeOfOp     = "Clocks::TimeOf"
	clocksDurationOfOp = "Clocks::DurationOf"
	timeTimeOfOp       = "Time::TimeOf"
	timeDurationOfOp   = "Time::DurationOf"
)

// TimeOf and DurationOf read the shared clock when an occurrence began and ended:
// Clocks.kerml declares them NumericalValue, Time.sysml quantities of time.
func init() {
	for name, fn := range map[string]builtinFunc{
		clocksTimeOfOp:     builtinTimeOf(clocksTimeOfOp, false),
		clocksDurationOfOp: builtinDurationOf(clocksDurationOfOp, false),
		timeTimeOfOp:       builtinTimeOf(timeTimeOfOp, true),
		timeDurationOfOp:   builtinDurationOf(timeDurationOfOp, true),
	} {
		builtins[name] = fn
		builtinSignatures[name] = []declaredParam{param("o"), optionalParam("clock")}
	}
}

// clockReading resolves the occurrence and clock arguments of a time function:
// the occurrence's lifetime, or the reason the reading is undetermined.
func clockReading(ec *EvalContext, op string, args []Value) (*Instance, life, *Value, error) {
	inst, err := occurrenceArgument(ec, op, "o", args[0])
	if err != nil {
		return nil, life{}, nil, err
	}
	l, err := ec.ctx.lifeOf(op, inst)
	if err != nil {
		return nil, life{}, nil, err
	}
	if !isEmptyValue(args[1]) {
		shared, err := ec.ctx.universalClockObject(universalClockFQN)
		if err != nil {
			return nil, life{}, nil, err
		}
		if args[1].Kind != ValInstance || args[1].Instance != shared.Instance {
			reason := fmt.Sprintf("%s reads only the shared clock %s; %s is another clock", writtenName(op), universalClockFQN, FormatValue(args[1]))
			undetermined := NewUndeterminedValue(reason, semantics.CountRange(1))
			return inst, l, &undetermined, nil
		}
	}
	return inst, l, nil, nil
}

// builtinTimeOf is `TimeOf(o, clock)`: the shared clock's time when o began,
// undetermined until o has started; a quantity of seconds or a bare number.
func builtinTimeOf(op string, quantity bool) builtinFunc {
	return func(ec *EvalContext, args []Value) (Value, error) {
		inst, l, undetermined, err := clockReading(ec, op, args)
		if err != nil {
			return Value{}, err
		}
		if undetermined != nil {
			return *undetermined, nil
		}
		if l.began == 0 {
			return NewUndeterminedValue(fmt.Sprintf("object #%d (%s) has not started: its startShot has not happened", inst.ID, SymbolText(inst.Type)), semantics.CountRange(1)), nil
		}
		return ec.ctx.timeValue(l.beganAt, quantity), nil
	}
}

// builtinDurationOf is `DurationOf(o, clock)`: the shared clock's time when o
// ended minus when it began, undetermined until o has ended.
func builtinDurationOf(op string, quantity bool) builtinFunc {
	return func(ec *EvalContext, args []Value) (Value, error) {
		inst, l, undetermined, err := clockReading(ec, op, args)
		if err != nil {
			return Value{}, err
		}
		if undetermined != nil {
			return *undetermined, nil
		}
		switch {
		case l.began == 0:
			return NewUndeterminedValue(fmt.Sprintf("object #%d (%s) has not started: its startShot has not happened", inst.ID, SymbolText(inst.Type)), semantics.CountRange(1)), nil
		case l.ended == 0:
			return NewUndeterminedValue(fmt.Sprintf("object #%d (%s) has not ended: its endShot has not happened", inst.ID, SymbolText(inst.Type)), semantics.CountRange(1)), nil
		}
		return ec.ctx.timeValue(l.endedAt-l.beganAt, quantity), nil
	}
}

// timeValue is the clock's time t as a quantity of seconds, or as the bare
// NumericalValue the Kernel's Clock reads.
func (ctx *Context) timeValue(t float64, quantity bool) Value {
	if quantity {
		return ctx.instantValue(t)
	}
	return realConst(t)
}
