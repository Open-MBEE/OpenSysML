package runtime

import (
	"strings"
	"testing"
)

// TestConstructorExpressionIsAValue: `new T(…)` evaluates to the object of T
// its arguments bind, and its arity is checked as a send's is.
func TestConstructorExpressionIsAValue(t *testing.T) {
	ctx := libraryContextOver(t, `
package Demo {
	private import ScalarValues::*;
	item def Foo { attribute v : Real; attribute w : Real; }
	attribute made : Real = new Foo(2.0, 3.0).w;
}`)
	scope := lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
	for expr, want := range map[string]string{
		"new Foo(2.0).v":          "2.0",
		"new Foo(w = 3.0).w":      "3.0",
		"new Foo(2.0, 3.0).v + 1": "3.0",
		"made":                    "3.0",
	} {
		got, err := evalIn(t, ctx, scope, expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if FormatValue(got) != want {
			t.Errorf("%s = %s, want %s", expr, FormatValue(got), want)
		}
	}
	for expr, want := range map[string]string{
		"new Foo(1.0, 2.0, 3.0)":    "new Foo: new Foo takes 2 argument(s), found 3",
		"new Missing(1.0)":          "new Missing: unresolved reference: Missing",
		"new Foo(v = 1.0, v = 2.0)": "new Foo: v is bound twice",
	} {
		_, err := evalIn(t, ctx, scope, expr)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s = %v, want an error containing %q", expr, err, want)
		}
	}
}
