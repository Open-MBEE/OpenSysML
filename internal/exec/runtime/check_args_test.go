package runtime

import (
	"errors"
	"strings"
	"testing"
)

// checkArgsModel declares a requirement and a constraint with `in` parameters,
// one defaulted and one not, and an object to check them against.
const checkArgsModel = `package test {
	private import ScalarValues::*;
	requirement def Under {
		in v : Integer = 2;
		in limit : Integer;
		in slack : Integer = 0;
		require constraint { v + slack < limit }
	}
	constraint def Between {
		in low : Integer;
		in high : Integer = 10;
		low < high
	}
}`

func TestCheckRequirementWithBindsArguments(t *testing.T) {
	m := parseLibraryModel(t, checkArgsModel)
	ctx, _ := m.fresh()
	under := m.idx.LookupQualified("test::Under")[0]
	scope := under.OwnerScope

	cases := []struct {
		name  string
		args  CheckArgs
		holds bool
		err   error
		msg   string
	}{
		{"named holds", CheckArgs{Named: map[string]Value{"limit": integerValue(5)}}, true, nil, ""},
		{"named fails", CheckArgs{Named: map[string]Value{"limit": integerValue(2)}}, false, nil, ""},
		{"positional in declaration order", CheckArgs{Positional: []Value{integerValue(1), integerValue(6), integerValue(4)}}, true, nil, ""},
		{"positional beyond the default", CheckArgs{Positional: []Value{integerValue(1), integerValue(5), integerValue(5)}}, false, nil, ""},
		{"positional after named", CheckArgs{Positional: []Value{integerValue(1)}, Named: map[string]Value{"limit": integerValue(5)}}, true, nil, ""},
		{"unknown parameter", CheckArgs{Named: map[string]Value{"bound": integerValue(1)}}, false, ErrUnknownParameter, `no input parameter "bound"`},
		{"too many arguments", CheckArgs{Positional: []Value{integerValue(1), integerValue(2), integerValue(3), integerValue(4)}}, false, ErrCalcArity, "takes 3 argument(s), got 4"},
		{"unbound without default", CheckArgs{}, false, ErrUnboundParameter, `parameter "limit" has no argument and no default`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ctx.CheckRequirementWith(under, scope, nil, tc.args)
			if tc.err != nil {
				if !errors.Is(err, tc.err) || !strings.Contains(err.Error(), tc.msg) {
					t.Fatalf("err = %v, want %v containing %q", err, tc.err, tc.msg)
				}
				return
			}
			if err != nil && !errors.Is(err, ErrViolated) {
				t.Fatal(err)
			}
			if result.Holds != tc.holds {
				t.Errorf("holds = %v, want %v", result.Holds, tc.holds)
			}
		})
	}
}

func TestCheckConstraintWithBindsArguments(t *testing.T) {
	m := parseLibraryModel(t, checkArgsModel)
	ctx, _ := m.fresh()
	between := m.idx.LookupQualified("test::Between")[0]
	scope := between.OwnerScope

	result, err := ctx.CheckConstraintWith(between, scope, nil, CheckArgs{Positional: []Value{integerValue(3)}})
	if err != nil || !result.Holds {
		t.Fatalf("Between(3) = %v, %v; want holds", result.Holds, err)
	}
	result, err = ctx.CheckConstraintWith(between, scope, nil, CheckArgs{Named: map[string]Value{"low": integerValue(3), "high": integerValue(1)}})
	if (err != nil && !errors.Is(err, ErrViolated)) || result.Holds {
		t.Fatalf("Between(low = 3, high = 1) = %v, %v; want fails", result.Holds, err)
	}
	_, err = ctx.CheckConstraintWith(between, scope, nil, CheckArgs{Named: map[string]Value{"low": NewStringValue("x")}})
	if err == nil {
		t.Fatal("a String bound to an Integer parameter was held")
	}
	if _, err = ctx.CheckConstraintOn(between, scope, nil); !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("Between with low unbound: err = %v, want ErrUnboundParameter", err)
	}
}

// renamedCheckArgsModel redefines an inherited, defaulted first parameter under a
// new name, so the derived constraint's slots must still read x-then-y.
const renamedCheckArgsModel = `package test {
	private import ScalarValues::*;
	constraint def Base {
		in x : Integer = 1;
		in y : Integer;
		x < y
	}
	constraint def Derived :> Base {
		in z :>> x;
	}
	part def Thing {
		attribute v : Integer = 5;
	}
	part thing : Thing;
}`

func TestCheckArgsRenamedParameterKeepsSlotAndDefault(t *testing.T) {
	m := parseLibraryModel(t, renamedCheckArgsModel)
	ctx, _ := m.fresh()
	derived := m.idx.LookupQualified("test::Derived")[0]
	scope := derived.OwnerScope

	// Positional values bind z (the renamed x) then y.
	result, err := ctx.CheckConstraintWith(derived, scope, nil, CheckArgs{Positional: []Value{integerValue(2), integerValue(9)}})
	if err != nil || !result.Holds {
		t.Fatalf("Derived(2, 9) = %v, %v; want holds", result.Holds, err)
	}
	result, err = ctx.CheckConstraintWith(derived, scope, nil, CheckArgs{Positional: []Value{integerValue(9), integerValue(2)}})
	if (err != nil && !errors.Is(err, ErrViolated)) || result.Holds {
		t.Fatalf("Derived(9, 2) = %v, %v; want fails", result.Holds, err)
	}
	// z keeps x's default when only y is named; it also answers to its old name.
	result, err = ctx.CheckConstraintWith(derived, scope, nil, CheckArgs{Named: map[string]Value{"y": integerValue(3)}})
	if err != nil || !result.Holds {
		t.Fatalf("Derived(y = 3) = %v, %v; want holds by x's default", result.Holds, err)
	}
	result, err = ctx.CheckConstraintWith(derived, scope, nil, CheckArgs{Named: map[string]Value{"z": integerValue(4), "y": integerValue(3)}})
	if (err != nil && !errors.Is(err, ErrViolated)) || result.Holds {
		t.Fatalf("Derived(z = 4, y = 3) = %v, %v; want fails", result.Holds, err)
	}
	if _, err = ctx.CheckConstraintWith(derived, scope, nil, CheckArgs{Positional: []Value{integerValue(1), integerValue(2), integerValue(3)}}); !errors.Is(err, ErrCalcArity) || !strings.Contains(err.Error(), "takes 2 argument(s)") {
		t.Fatalf("Derived(1, 2, 3): err = %v, want arity of 2", err)
	}
}

// Within a verdict-sharing span, checks of one constraint on one object with
// different arguments are each decided: an argument is not something the first
// check read from the object, so its verdict cannot stand for the next.
func TestCheckArgsAreNotSharedAcrossVerdicts(t *testing.T) {
	m := parseLibraryModel(t, renamedCheckArgsModel)
	ctx, _ := m.fresh()
	ctx.SetSharedDefaults(true)
	between := m.idx.LookupQualified("test::Base")[0]
	scope := between.OwnerScope
	thing, err := ctx.Instantiate(m.idx.LookupQualified("test::thing")[0])
	if err != nil {
		t.Fatal(err)
	}
	done := ctx.ShareVerdicts()
	defer done()

	result, err := ctx.CheckConstraintWith(between, scope, thing, CheckArgs{Named: map[string]Value{"y": integerValue(9)}})
	if err != nil || !result.Holds {
		t.Fatalf("Base(y = 9) = %v, %v; want holds", result.Holds, err)
	}
	result, err = ctx.CheckConstraintWith(between, scope, thing, CheckArgs{Named: map[string]Value{"y": integerValue(0)}})
	if (err != nil && !errors.Is(err, ErrViolated)) || result.Holds {
		t.Fatalf("Base(y = 0) after Base(y = 9) = %v, %v; want fails", result.Holds, err)
	}
	if taken := ctx.SharedVerdictsTaken(); taken != 0 {
		t.Errorf("verdicts taken = %d, want 0 for checks with arguments", taken)
	}
}
