package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// benchRuntime builds a runtime over the standard library and src.
func benchRuntime(b *testing.B, src string) (*Context, *symbols.Index) {
	b.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<bench>", parser.New(source.New("<bench>", []byte(src))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return NewContext(NewModel(model, resolver), 1_000_000), idx
}

func benchInstantiate(b *testing.B, ctx *Context, idx *symbols.Index, qualified string) *Instance {
	b.Helper()
	matches := idx.LookupQualified(qualified)
	if len(matches) != 1 {
		b.Fatalf("%s: %d matching symbols, want 1", qualified, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		b.Fatalf("Instantiate %s: %v", qualified, err)
	}
	return inst
}

// plainAttributesModel: an object of many attributes, none derived from another.
func plainAttributesModel(n int) string {
	var sb strings.Builder
	sb.WriteString("package bench {\n\tprivate import ScalarValues::*;\n\tpart def Plain {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "\t\tattribute a%d : Integer default %d;\n", i, i)
	}
	sb.WriteString("\t}\n\tpart plain : Plain;\n}\n")
	return sb.String()
}

// BenchmarkMaterializePlainAttributes reads every attribute of an object that
// derives nothing, the path that must record no dependency.
func BenchmarkMaterializePlainAttributes(b *testing.B) {
	const n = 64
	ctx, idx := benchRuntime(b, plainAttributesModel(n))
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("a%d", i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inst := benchInstantiate(b, ctx, idx, "bench::plain")
		for _, name := range names {
			if _, err := inst.GetFeatureValue(ctx, name); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkSetFeatureValueNoDependents writes one attribute nothing derives
// from, over and over.
func BenchmarkSetFeatureValueNoDependents(b *testing.B) {
	ctx, idx := benchRuntime(b, plainAttributesModel(4))
	inst := benchInstantiate(b, ctx, idx, "bench::plain")
	vals := [2]Value{constInt(1), constInt(2)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := inst.SetFeatureValue(ctx, "a0", vals[i&1]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDerivedReadWriteRead reads a derived value, writes what it read and
// reads it again — one invalidation and one re-derivation per iteration.
func BenchmarkDerivedReadWriteRead(b *testing.B) {
	ctx, idx := benchRuntime(b, `
		package bench {
			private import ScalarValues::*;
			part def Box {
				attribute a : Integer default 3;
				attribute d : Integer = a * 2;
				attribute dd : Integer = d + 1;
			}
			part box : Box;
		}
	`)
	inst := benchInstantiate(b, ctx, idx, "bench::box")
	vals := [2]Value{constInt(1), constInt(2)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := inst.GetFeatureValue(ctx, "dd"); err != nil {
			b.Fatal(err)
		}
		if err := inst.SetFeatureValue(ctx, "a", vals[i&1]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAssignmentLoopStep runs a calc whose loop assigns four attributes each
// step from a calc usage's outputs, the fixed-step integration pattern.
func BenchmarkAssignmentLoopStep(b *testing.B) {
	ctx, idx := benchRuntime(b, loopStepModel)
	matches := idx.LookupQualified("test::Step")
	if len(matches) != 1 {
		b.Fatalf("test::Step: %d matching symbols, want 1", len(matches))
	}
	step := matches[0]
	scope := idx.LookupQualified("test")[0].Scope
	steps := constInt(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ctx.InvokeCalc(step, []Value{steps}, scope); err != nil {
			b.Fatal(err)
		}
	}
}
