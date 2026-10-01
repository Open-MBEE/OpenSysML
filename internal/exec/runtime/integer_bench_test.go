package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// integerBenchModel holds Integer-heavy calcs: a tight arithmetic loop, the
// same loop over operands beyond int64, and a sum over a range.
const integerBenchModel = `
package bench {
	private import ScalarValues::*;
	private import NumericalFunctions::*;
	calc def Loop {
		in n : Integer;
		in start : Integer;
		attribute acc : Integer = start;
		attribute i : Integer = 0;
		while i < n {
			assign acc := acc + i * 3 - 1;
			assign i := i + 1;
		}
		acc
	}
	calc def RangeSum {
		in n : Integer;
		sum(1..n)
	}
}
`

func benchIntegerCalc(b *testing.B, calc string, args ...Value) {
	b.Helper()
	ctx, idx := benchRuntime(b, integerBenchModel)
	matches := idx.LookupQualified("bench::" + calc)
	if len(matches) != 1 {
		b.Fatalf("bench::%s: %d matching symbols, want 1", calc, len(matches))
	}
	scope := idx.LookupQualified("bench")[0].Scope
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ctx.InvokeCalc(matches[0], args, scope); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIntegerArithmeticLoop runs a while loop of Integer arithmetic whose
// values stay within int64, the fast path every ordinary model takes.
func BenchmarkIntegerArithmeticLoop(b *testing.B) {
	benchIntegerCalc(b, "Loop", constInt(1000), constInt(0))
}

// BenchmarkIntegerArithmeticLoopBeyondInt64 runs the same loop over an
// accumulator starting at 2^70, so every addition is arbitrary-precision.
func BenchmarkIntegerArithmeticLoopBeyondInt64(b *testing.B) {
	start, ok := semantics.ParseInteger("1180591620717411303424")
	if !ok {
		b.Fatal("2^70 does not parse as an Integer")
	}
	benchIntegerCalc(b, "Loop", constInt(1000), Value{Kind: ValConst, Const: start})
}

// BenchmarkIntegerCollectionSum sums a materialized Integer range.
func BenchmarkIntegerCollectionSum(b *testing.B) {
	benchIntegerCalc(b, "RangeSum", constInt(10000))
}
