package runtime

import (
	"fmt"
	"testing"
)

// BenchmarkStartFailures reports the start failures of one object in a context
// crowded with others: a cost of the failures on record, not of the objects held.
func BenchmarkStartFailures(b *testing.B) {
	for _, crowd := range []struct {
		name string
		of   string
		n    int
	}{
		{"plain", "test::plain", 0},
		{"plain", "test::plain", 1000},
		{"plain", "test::plain", 10000},
		{"failed", "test::relay", 1000},
	} {
		b.Run(fmt.Sprintf("%s=%d", crowd.name, crowd.n), func(b *testing.B) {
			ctx, idx := benchRuntime(b, crowdModel)
			for i := 0; i < crowd.n; i++ {
				benchInstantiate(b, ctx, idx, crowd.of)
			}
			plain := benchInstantiate(b, ctx, idx, "test::plain")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if failed := ctx.StartFailures(plain); len(failed) != 0 {
					b.Fatalf("plain has %d failed behaviors, want none", len(failed))
				}
			}
		})
	}
}
