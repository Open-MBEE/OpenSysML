package runtime

import (
	"fmt"
)

const (
	occSameOp     = "OccurrenceFunctions::==="
	occIsDuringOp = "OccurrenceFunctions::isDuring"
	occCreateOp   = "OccurrenceFunctions::create"
	occDestroyOp  = "OccurrenceFunctions::destroy"
	occAddNewOp   = "OccurrenceFunctions::addNew"
	occAddNewAtOp = "OccurrenceFunctions::addNewAt"
)

// The OccurrenceFunctions are built-ins over the objects' lifetimes: the call is
// the performance they happen during, entered at the activation `entered` marks.
func init() {
	for name, fn := range map[string]builtinFunc{
		occSameOp:     builtinOccurrenceSame,
		occIsDuringOp: builtinOccurrenceIsDuring,
		occCreateOp:   builtinOccurrenceCreate,
		occDestroyOp:  builtinOccurrenceDestroy,
		occAddNewOp:   builtinOccurrenceAddNew,
		occAddNewAtOp: builtinOccurrenceAddNewAt,
	} {
		builtins[name] = fn
	}
	for name, params := range map[string][]declaredParam{
		occSameOp:     {optionalParam("x"), optionalParam("y")},
		occIsDuringOp: {param("occ")},
		occCreateOp:   {param("occ")},
		occDestroyOp:  {optionalParam("occ")},
		occAddNewOp:   {optionalParam("group"), param("occ")},
		occAddNewAtOp: {optionalParam("group"), param("occ"), param("index")},
	} {
		builtinSignatures[name] = params
	}
}

// occurrenceArgument answers the one object the argument bound to param
// denotes; several values, none, or a data value are reported.
func occurrenceArgument(ec *EvalContext, name, param string, val Value) (*Instance, error) {
	elements := elementsOf(val)
	if len(elements) != 1 {
		return nil, fmt.Errorf("%w: function %s parameter %q holds %d values, exactly 1 required",
			ErrMultiplicityViolation, writtenName(name), param, len(elements))
	}
	return occurrenceOf(ec, name, param, elements[0])
}

// occurrenceOf answers the object one value denotes, when it is an occurrence.
func occurrenceOf(ec *EvalContext, name, param string, val Value) (*Instance, error) {
	id, ok := val.Object()
	if !ok || ec.ctx.isDataValue(val) {
		return nil, fmt.Errorf("%w: function %s parameter %q is %s, not an occurrence",
			ErrNotAnOccurrence, writtenName(name), param, describeOperand(val))
	}
	inst, found := ec.ctx.getInstance(id)
	if !found {
		return nil, fmt.Errorf("%w: function %s parameter %q names object #%d, which this context does not hold",
			ErrNotAnOccurrence, writtenName(name), param, id)
	}
	return inst, nil
}

// occurrenceGroup checks that every element of the group bound to param is an
// occurrence.
func occurrenceGroup(ec *EvalContext, name, param string, group Value) error {
	for _, element := range elementsOf(group) {
		if _, err := occurrenceOf(ec, name, param, element); err != nil {
			return err
		}
	}
	return nil
}

// builtinOccurrenceSame is `'==='(x, y)`: whether x and y are one occurrence.
// Nothing is the same occurrence as nothing.
func builtinOccurrenceSame(ec *EvalContext, args []Value) (Value, error) {
	const op = occSameOp
	occurrences := make([]*Instance, 2)
	for i, param := range []string{"x", "y"} {
		elements := elementsOf(args[i])
		switch len(elements) {
		case 0:
			continue
		case 1:
		default:
			return Value{}, fmt.Errorf("%w: function %s parameter %q holds %d values, at most 1 allowed",
				ErrMultiplicityViolation, writtenName(op), param, len(elements))
		}
		inst, err := occurrenceOf(ec, op, param, elements[0])
		if err != nil {
			return Value{}, err
		}
		occurrences[i] = inst
	}
	return boolValue(occurrences[0] == occurrences[1]), nil
}

// builtinOccurrenceIsDuring is `isDuring(occ)`: whether this performance of the
// function happens during occ, which it does while occ has begun and not ended.
func builtinOccurrenceIsDuring(ec *EvalContext, args []Value) (Value, error) {
	const op = occIsDuringOp
	inst, err := occurrenceArgument(ec, op, "occ", args[0])
	if err != nil {
		return Value{}, err
	}
	l, err := ec.ctx.lifeOf(op, inst)
	if err != nil {
		return Value{}, err
	}
	return boolValue(l.alive()), nil
}

// builtinOccurrenceCreate is `create(occ)`: occ starts during the call, which
// is where the runtime first reached it; one reached before began already.
func builtinOccurrenceCreate(ec *EvalContext, args []Value) (Value, error) {
	const op = occCreateOp
	inst, err := occurrenceArgument(ec, op, "occ", args[0])
	if err != nil {
		return Value{}, err
	}
	if err := ec.ctx.createDuring(op, inst, ec.entered); err != nil {
		return Value{}, err
	}
	return args[0], nil
}

// builtinOccurrenceDestroy is `destroy(occ)`: occ ends during the call, with
// every object it holds as a portion of itself; `destroy()` of nothing is nothing.
func builtinOccurrenceDestroy(ec *EvalContext, args []Value) (Value, error) {
	const op = occDestroyOp
	if isEmptyValue(args[0]) {
		return nullValue(), nil
	}
	inst, err := occurrenceArgument(ec, op, "occ", args[0])
	if err != nil {
		return Value{}, err
	}
	if err := ec.ctx.destroy(inst); err != nil {
		return Value{}, fmt.Errorf("function %s: %w", writtenName(op), err)
	}
	return args[0], nil
}

// builtinOccurrenceAddNew is `addNew(group, occ)`: occ, created during the
// call, is appended to group. The value is what `inout group` holds after, as
// `SequenceFunctions::including` answers it: a call rebinds no argument.
func builtinOccurrenceAddNew(ec *EvalContext, args []Value) (Value, error) {
	const op = occAddNewOp
	if err := occurrenceGroup(ec, op, "group", args[0]); err != nil {
		return Value{}, err
	}
	inst, err := occurrenceArgument(ec, op, "occ", args[1])
	if err != nil {
		return Value{}, err
	}
	group, err := builtinSequenceUnion(ec, []Value{args[0], args[1]})
	if err != nil {
		return Value{}, err
	}
	if err := ec.ctx.createDuring(op, inst, ec.entered); err != nil {
		return Value{}, err
	}
	return group, nil
}

// builtinOccurrenceAddNewAt is `addNewAt(group, occ, index)`: occ, created
// during the call, is inserted into group at the positive index, which may be
// one past its last element and no further. The value is what group holds after.
func builtinOccurrenceAddNewAt(ec *EvalContext, args []Value) (Value, error) {
	const op = occAddNewAtOp
	if err := occurrenceGroup(ec, op, "group", args[0]); err != nil {
		return Value{}, err
	}
	inst, err := occurrenceArgument(ec, op, "occ", args[1])
	if err != nil {
		return Value{}, err
	}
	group, err := ec.insertAt(op, args[0], args[1], args[2])
	if err != nil {
		return Value{}, err
	}
	if err := ec.ctx.createDuring(op, inst, ec.entered); err != nil {
		return Value{}, err
	}
	return group, nil
}
