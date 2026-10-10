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
