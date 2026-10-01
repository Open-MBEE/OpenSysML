package pssm

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// libraryBehavior is a behavior of the suite's libraries the driver evaluates
// when the tester traces a value computed with it: its arity and its function.
type libraryBehavior struct {
	arity int
	eval  func(args []runtime.Value) (runtime.Value, error)
}

// libraryBehaviors are the behaviors the tester's traces apply, by qualified
// name: the fUML primitives (fUML 9.2) and the suite's own formatting activity.
var libraryBehaviors = map[string]libraryBehavior{
	"StringFunctions::Concat":             {2, concat},
	"BooleanFunctions::ToString":          {1, toString},
	"IntegerFunctions::ToString":          {1, toString},
	"UnlimitedNaturalFunctions::ToString": {1, toString},
	"Util::Tracing::formatParameterValue": {2, formatParameterValue},
}

// concat is fUML StringFunctions::Concat.
func concat(args []runtime.Value) (runtime.Value, error) {
	x, err := stringArg("Concat", args[0])
	if err != nil {
		return runtime.Value{}, err
	}
	y, err := stringArg("Concat", args[1])
	if err != nil {
		return runtime.Value{}, err
	}
	return runtime.NewStringValue(x + y), nil
}

// toString is fUML ToString of a Boolean, Integer or UnlimitedNatural.
func toString(args []runtime.Value) (runtime.Value, error) {
	s, ok := primitiveText(args[0])
	if !ok {
		return runtime.Value{}, fmt.Errorf("ToString of a %s", valueKind(args[0]))
	}
	return runtime.NewStringValue(s), nil
}

// formatParameterValue is the suite's Util::Tracing::formatParameterValue: the
// value bracketed as an input ("[in=") when input is null or true, an output
// ("[out=") otherwise, spelled by ToString of its primitive type, "??" for any other.
func formatParameterValue(args []runtime.Value) (runtime.Value, error) {
	input, value := args[0], args[1]
	prefix := "[out="
	switch {
	case input.Kind == runtime.ValNull:
		prefix = "[in="
	case input.Kind == runtime.ValConst && input.Const.Kind == semantics.ValBool:
		if input.Const.Bool {
			prefix = "[in="
		}
	default:
		return runtime.Value{}, fmt.Errorf("formatParameterValue input is a %s, not a Boolean", valueKind(input))
	}
	text := "??"
	if value.Kind == runtime.ValString {
		text = value.Str()
	} else if s, ok := primitiveText(value); ok {
		text = s
	}
	return runtime.NewStringValue(prefix + text + "]"), nil
}

// primitiveText spells a Boolean, Integer or UnlimitedNatural as fUML ToString does.
func primitiveText(v runtime.Value) (string, bool) {
	if v.Kind != runtime.ValConst {
		return "", false
	}
	switch v.Const.Kind {
	case semantics.ValBool:
		return strconv.FormatBool(v.Const.Bool), true
	case semantics.ValInt:
		return strconv.FormatInt(v.Const.Int, 10), true
	case semantics.ValInfinity:
		return "*", true
	}
	return "", false
}

func stringArg(fn string, v runtime.Value) (string, error) {
	if v.Kind != runtime.ValString {
		return "", fmt.Errorf("%s of a %s, not a String", fn, valueKind(v))
	}
	return v.Str(), nil
}

// valueKind names a value's kind for a diagnostic.
func valueKind(v runtime.Value) string {
	if v.Kind == runtime.ValConst {
		switch v.Const.Kind {
		case semantics.ValBool:
			return "Boolean"
		case semantics.ValInt:
			return "Integer"
		case semantics.ValReal:
			return "Real"
		case semantics.ValInfinity:
			return "UnlimitedNatural"
		}
	}
	return v.Kind.String()
}
