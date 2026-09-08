package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evalDeclaredExpr evaluates expr as the value of an attribute declared beside src.
func evalDeclaredExpr(t *testing.T, src, expr string) (*Context, Value, error) {
	t.Helper()
	ctx, scope, decl := declaredExpr(t, src, expr)
	got, err := NewEvalContext(ctx, scope).Eval(decl)
	return ctx, got, err
}

// declaredExpr is expr as the value of an attribute declared beside src, with
// the context and scope to evaluate it in, so one context can evaluate it twice.
func declaredExpr(t *testing.T, src, expr string) (*Context, *symbols.Scope, ast.Node) {
	t.Helper()
	full := src + "\npackage probe {\n\tattribute result = " + expr + ";\n}"
	model, resolver, root := parseAndBuildLibraryModel(t, full)
	pkg, ok := root.LookupLocal("probe")
	if !ok || pkg == nil || pkg.Scope == nil {
		t.Fatal("package probe has no scope")
	}
	sym, ok := pkg.Scope.LookupLocal("result")
	if !ok || sym == nil {
		t.Fatal("attribute result not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("result declares %T, want a usage", sym.Decl)
	}
	return NewContext(model, resolver, 10000), pkg.Scope, decl.Value
}

// TestInfinityValue evaluates `*`: its own scalar value, printed as written.
func TestInfinityValue(t *testing.T) {
	_, got, err := evalDeclaredExpr(t, "package test {}", "*")
	if err != nil {
		t.Fatalf("* failed: %v", err)
	}
	if got.Kind != ValConst || !got.Const.IsUnbounded() {
		t.Fatalf("* evaluated to %v (%v), want the unbounded constant", got.Kind, got.Const.Kind)
	}
	if text := FormatValue(got); text != "*" {
		t.Errorf("FormatValue = %q, want %q", text, "*")
	}
	if text := FormatTraceValue(got); text != "*" {
		t.Errorf("FormatTraceValue = %q, want %q", text, "*")
	}
}

// TestInfinityComparison orders `*` above every finite number and equal to itself.
func TestInfinityComparison(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"* > 1", true},
		{"* > 1.5", true},
		{"* > -100", true},
		{"1 < *", true},
		{"* >= 7", true},
		{"* <= 1", false},
		{"* < 1", false},
		{"* == *", true},
		{"* >= *", true},
		{"* <= *", true},
		{"* > *", false},
		{"* < *", false},
		{"* != *", false},
		{"* == 1", false},
		{"* != 1", true},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, got, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.expr, err)
			}
			if got.Kind != ValConst || got.Const.Kind != semantics.ValBool {
				t.Fatalf("%s evaluated to %v, want a Boolean", tc.expr, got.Kind)
			}
			if got.Const.Bool != tc.want {
				t.Errorf("%s = %v, want %v", tc.expr, got.Const.Bool, tc.want)
			}
		})
	}
}

// TestInfinityArithmeticRefused refuses arithmetic over `*` with a typed error
// naming the operation rather than answering with a number.
func TestInfinityArithmeticRefused(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"* + 1", "operator '+' is not defined for the unbounded value '*'"},
		{"1 + *", "operator '+' is not defined for the unbounded value '*'"},
		{"* - 1", "operator '-' is not defined for the unbounded value '*'"},
		{"2 * *", "operator '*' is not defined for the unbounded value '*'"},
		{"* / 2", "operator '/' is not defined for the unbounded value '*'"},
		{"* % 2", "operator '%' is not defined for the unbounded value '*'"},
		{"* ** 2", "operator '**' is not defined for the unbounded value '*'"},
		{"-*", "unary '-' is not defined for *"},
		{"+*", "unary '+' is not defined for *"},
		{`* > "a"`, "'>' is not defined"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, _, err := evalDeclaredExpr(t, "package test {}", tc.expr)
			if err == nil {
				t.Fatalf("%s succeeded, want a typed error", tc.expr)
			}
			if !errors.Is(err, ErrTypeMismatch) {
				t.Errorf("%s: error %v, want a type mismatch", tc.expr, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: error %q, want it to contain %q", tc.expr, err, tc.want)
			}
			if strings.Contains(err.Error(), "∞") || strings.Contains(err.Error(), "Inf") {
				t.Errorf("%s: error %q names the value as an infinity, want `*`", tc.expr, err)
			}
		})
	}
}

// TestInfinityDirectType classifies `*` as the Positive it is typed as in
// expression position, and so as a Natural, but as nothing unrelated.
func TestInfinityDirectType(t *testing.T) {
	cases := map[string]string{
		"* istype ScalarValues::Positive":  "true",
		"* hastype ScalarValues::Positive": "true",
		"* istype ScalarValues::Natural":   "true",
		// hastype demands the direct type itself, which Natural is only above.
		"* hastype ScalarValues::Natural": "false",
		"* istype ScalarValues::String":   "false",
		"* hastype ScalarValues::Boolean": "false",
		"* @ ScalarValues::Positive":      "true",
		"* @ ScalarValues::String":        "false",
	}
	for expr, want := range cases {
		_, got, err := evalDeclaredExpr(t, "package test {}", expr)
		if err != nil {
			t.Errorf("%s failed: %v", expr, err)
			continue
		}
		if text := FormatValue(got); text != want {
			t.Errorf("%s = %s, want %s", expr, text, want)
		}
	}
}
