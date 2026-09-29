package stressmodel

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// BenchmarkConvertAPIJSON writes each network in the API's JSON element form,
// the conversion whose output is largest for its input.
func BenchmarkConvertAPIJSON(b *testing.B) {
	for _, n := range networkSizes {
		src, stats := network(n).Source()
		b.Run(fmt.Sprintf("satellites=%d/elements=%d", stats.Satellites, stats.Elements), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out, err := convert.Convert("satnet.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
				if err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(out)))
			}
		})
	}
}
