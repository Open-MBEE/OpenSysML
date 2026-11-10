package perf

import (
	"testing"

	_ "github.com/Open-MBEE/OpenSysML/internal/doc/queryexec/elementids" // the CLI's elementId reader
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/positional" // the CLI's positional namer
)

// oslcQueryModels are the models BenchmarkOSLCQuery runs over: the largest
// self-model package (pipeline loads execution too) and a small fixture, so a
// cost fixed per query shows apart from one growing with the model.
var oslcQueryModels = []struct{ name, path string }{
	{"pipeline", "../../examples/self-model/pipeline.sysml"},
	{"basic", "../../tests/testdata/resolve/basic.sysml"},
}

// oslcQueries are the shapes `sysml -query` is driven with: a one-property
// projection, a filter with the default projection, and a type test.
var oslcQueries = []struct{ name, text string }{
	{"select-name", "oslc.select=sysml:name"},
	{"where-default", "oslc.where=sysml:isAbstract=false"},
	{"where-type", "oslc.where=rdf:type=sysml:PartUsage&oslc.select=sysml:name,sysml:owner"},
}

// BenchmarkOSLCQuery times Session.Query alone, as `sysml -query` runs it: a
// fresh session per iteration, loaded outside the timer, so nothing a query
// memoizes survives into the next.
func BenchmarkOSLCQuery(b *testing.B) {
	for _, m := range oslcQueryModels {
		for _, q := range oslcQueries {
			b.Run(m.name+"/"+q.name, func(b *testing.B) {
				b.ReportAllocs()
				rows := 0
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					s := repl.NewSession()
					if _, err := s.LoadFilesSummary([]string{m.path}); err != nil {
						b.Fatal(err)
					}
					b.StartTimer()
					lines, err := s.Query(q.text)
					if err != nil {
						b.Fatal(err)
					}
					rows = len(lines)
				}
				b.ReportMetric(float64(rows), "rows")
			})
		}
	}
}
