package runtime

import "testing"

// rationalBenchModel holds Rational-heavy calcs: a loop of decimal arithmetic
// whose terms stay small, a harmonic sum whose denominator grows past int64,
// and the decimal loop over features declared Real.
const rationalBenchModel = `
package bench {
	private import ScalarValues::*;
	calc def DecimalLoop {
		in n : Integer;
		attribute acc : Rational = 0.0;
		attribute i : Integer = 0;
		while i < n {
			assign acc := acc + 0.25 * i - 0.1;
			assign i := i + 1;
		}
		acc
	}
	calc def HarmonicLoop {
		in n : Integer;
		attribute acc : Rational = 0.0;
		attribute i : Integer = 1;
		while i <= n {
			assign acc := acc + 1 / i;
			assign i := i + 1;
		}
		acc
	}
	calc def RealLoop {
		in n : Integer;
		attribute acc : Real = 0.0;
		attribute i : Integer = 0;
		while i < n {
			assign acc := acc + 0.25 * i - 0.1;
			assign i := i + 1;
		}
		acc
	}
}
`

func benchRationalCalc(b *testing.B, calc string, args ...Value) {
	b.Helper()
	ctx, idx := benchRuntime(b, rationalBenchModel)
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

// BenchmarkRationalDecimalLoop runs exact decimal arithmetic whose terms stay
// within the inline numerator and denominator.
func BenchmarkRationalDecimalLoop(b *testing.B) {
	benchRationalCalc(b, "DecimalLoop", constInt(1000))
}

// BenchmarkRationalHarmonicLoop sums 1/i exactly; its denominator outgrows
// int64 after a few dozen terms, so most additions are arbitrary-precision.
func BenchmarkRationalHarmonicLoop(b *testing.B) {
	benchRationalCalc(b, "HarmonicLoop", constInt(200))
}

// BenchmarkRealDecimalLoop runs the decimal loop over a binary64 Real.
func BenchmarkRealDecimalLoop(b *testing.B) {
	benchRationalCalc(b, "RealLoop", constInt(1000))
}
