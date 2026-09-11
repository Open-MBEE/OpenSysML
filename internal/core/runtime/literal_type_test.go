package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// literalTypeContext builds a library-backed model with a package that imports nothing
// from ScalarValues and one that declares its own types under the library's names.
func literalTypeContext(t *testing.T) (*Context, *symbols.Scope, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package NoImport {
			attribute n = 2;
		}
		package Shadow {
			attribute def Integer;
			attribute def Real;
			attribute def Boolean;
			attribute def String;
			attribute def Complex;
		}
	`))
	root := idx.DocumentRoot("<test>")
	scopes := make([]*symbols.Scope, 0, 2)
	for _, name := range []string{"NoImport", "Shadow"} {
		pkg, ok := root.LookupLocal(name)
		if !ok || pkg.Scope == nil {
			t.Fatalf("package %s not indexed", name)
		}
		scopes = append(scopes, pkg.Scope)
	}
	return ctx, scopes[0], scopes[1]
}

func evalBoolIn(t *testing.T, ctx *Context, scope *symbols.Scope, src string) bool {
	t.Helper()
	got, err := evalIn(t, ctx, scope, src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	if got.Kind != ValConst || got.Const.Kind != semantics.ValBool {
		t.Fatalf("%s = %s, want a Boolean", src, FormatValue(got))
	}
	return got.Const.Bool
}

// A literal is of its ScalarValues type whether or not the evaluating scope imports
// ScalarValues: an integer is an Integer and so a Real, no Natural; a finite real is a
// Rational (KerML 1.0 §8.4.4.9.2); `hastype` names the one direct type.
func TestLiteralTypeNeedsNoScalarValuesImport(t *testing.T) {
	ctx, noImport, _ := literalTypeContext(t)
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{"2 istype ScalarValues::Integer", true},
		{"2 hastype ScalarValues::Integer", true},
		{"2 istype ScalarValues::Real", true},
		{"2 hastype ScalarValues::Real", false},
		{"2 istype ScalarValues::Natural", false},
		{"2 istype ScalarValues::Complex", true},
		{"2 hastype ScalarValues::Complex", false},
		{"2.5 istype ScalarValues::Real", true},
		{"2.5 hastype ScalarValues::Real", false},
		{"2.5 istype ScalarValues::Rational", true},
		{"2.5 hastype ScalarValues::Rational", true},
		{"2.5 istype ScalarValues::Integer", false},
		{"true istype ScalarValues::Boolean", true},
		{"true hastype ScalarValues::Boolean", true},
		{`"s" istype ScalarValues::String`, true},
		{`"s" hastype ScalarValues::String`, true},
		{"2 @ ScalarValues::Integer", true},
		{"2.5 @ ScalarValues::Rational", true},
		{"n istype ScalarValues::Integer", true},
		{"n hastype ScalarValues::Integer", true},
	} {
		if got := evalBoolIn(t, ctx, noImport, tc.src); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.src, got, tc.want)
		}
	}
}

// A type a model declares under a library name is what a written `Integer` resolves
// to, and a literal is not of it: the literal's type is the library's, found qualified.
func TestLiteralTypeIsNotAShadowingUserType(t *testing.T) {
	ctx, _, shadow := literalTypeContext(t)
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{"2 istype Integer", false},
		{"2 hastype Integer", false},
		{"2 @ Integer", false},
		{"2 istype ScalarValues::Integer", true},
		{"2 hastype ScalarValues::Integer", true},
		{"2 @ ScalarValues::Integer", true},
		{"2 istype Real", false},
		{"2 istype ScalarValues::Real", true},
		{"2.5 istype Real", false},
		{"2.5 hastype Real", false},
		{"2.5 istype ScalarValues::Real", true},
		{"true istype Boolean", false},
		{"true hastype Boolean", false},
		{"true istype ScalarValues::Boolean", true},
		{`"s" istype String`, false},
		{`"s" hastype String`, false},
		{`"s" istype ScalarValues::String`, true},
	} {
		if got := evalBoolIn(t, ctx, shadow, tc.src); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.src, got, tc.want)
		}
	}
}

// The direct type of a scalar is the library definition itself, in a scope that sees a
// same-named user type as in one that imports nothing; the written name stays the user's.
func TestScalarDirectTypeIsTheLibraryDefinition(t *testing.T) {
	ctx, noImport, shadow := literalTypeContext(t)
	for _, tc := range []struct {
		value Value
		want  string
	}{
		{intArg(2), "ScalarValues::Integer"},
		{realArg(2.5), "ScalarValues::Rational"},
		{realArg(2.0), "ScalarValues::Rational"},
		{boolArg(true), "ScalarValues::Boolean"},
		{NewStringValue("s"), "ScalarValues::String"},
		{NewComplex(complex(1, 2)), "ScalarValues::Complex"},
	} {
		want := ctx.librarySymbol(tc.want)
		if want == nil {
			t.Fatalf("%s not loaded", tc.want)
		}
		for _, scope := range []*symbols.Scope{noImport, shadow} {
			typ, err := ctx.directValueType(scope, tc.value)
			if err != nil {
				t.Fatalf("directValueType(%s): %v", FormatValue(tc.value), err)
			}
			if typ != want {
				t.Errorf("directValueType(%s) = %s, want %s", FormatValue(tc.value), symbolText(typ), tc.want)
			}
		}
		name := tc.want[len("ScalarValues::"):]
		if name == "Rational" {
			continue
		}
		user := ctx.resolveType(shadow, name)
		if user == nil || user == want {
			t.Errorf("resolveType(Shadow, %s) = %s, want the type Shadow declares", name, symbolText(user))
		}
	}
}

// A model that declares a scalar type under a library name stands in for the library
// only while no library is loaded at all; with any library document present, a scalar
// whose ScalarValues definition is missing is of no determinable type.
func TestUserScalarTypeStandsInOnlyWithoutALibrary(t *testing.T) {
	const model = `
		package Shadow {
			attribute def Integer;
			attribute def Real;
		}
	`
	build := func(t *testing.T, lib string) (*Context, *symbols.Scope) {
		t.Helper()
		idx := symbols.NewIndex()
		idx.AddDocument("<test>", parseAndBuild(t, model))
		if lib != "" {
			idx.AddDocument("<lib>", parseAndBuild(t, lib))
			idx.MarkLibrary("<lib>")
		}
		resolver := resolve.New(idx)
		sem := semantics.NewModel(resolver)
		ctx := NewContext(NewModel(sem, resolver), 10000)
		pkg, ok := idx.DocumentRoot("<test>").LookupLocal("Shadow")
		if !ok || pkg.Scope == nil {
			t.Fatal("package Shadow not indexed")
		}
		return ctx, pkg.Scope
	}

	ctx, shadow := build(t, "")
	if got := evalBoolIn(t, ctx, shadow, "2 istype Integer"); !got {
		t.Errorf("without a library, 2 istype Integer = false, want the model's Integer to stand in")
	}
	typ, err := ctx.directValueType(shadow, intArg(2))
	if err != nil || typ != ctx.resolveType(shadow, "Integer") {
		t.Errorf("without a library, directValueType(2) = %s, %v, want Shadow::Integer", symbolText(typ), err)
	}

	ctx, shadow = build(t, "package Base { class Anything; }")
	if _, err := ctx.directValueType(shadow, intArg(2)); !errors.Is(err, ErrUndeterminedValueType) {
		t.Errorf("with a library lacking ScalarValues, directValueType(2) error = %v, want ErrUndeterminedValueType", err)
	}
	if _, err := evalIn(t, ctx, shadow, "2 istype Integer"); !errors.Is(err, ErrUndeterminedValueType) {
		t.Errorf("with a library lacking ScalarValues, 2 istype Integer error = %v, want ErrUndeterminedValueType", err)
	}
}
