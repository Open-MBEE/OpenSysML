package perf

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/repl"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// benchModelEnv names a directory of .sysml/.kerml files to load as one REPL
// session, so a whole real model's load is timed. See docs/internals/performance.md.
//
//	OPENSYSML_BENCH_MODEL=/path/to/model go test ./tests/perf -run '^$' -bench REPLLoadModel -benchmem
const benchModelEnv = "OPENSYSML_BENCH_MODEL"

func BenchmarkREPLLoadModel(b *testing.B) {
	dir := os.Getenv(benchModelEnv)
	if dir == "" {
		b.Skipf("%s is not set", benchModelEnv)
	}
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || source.KindOf(path) == source.KindUnknown {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
	if len(paths) == 0 {
		b.Fatalf("no .sysml or .kerml file under %s", dir)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := repl.NewSession()
		if _, err := s.LoadFilesSummary(paths); err != nil {
			b.Fatal(err)
		}
		if s.LocatedDiagnostics(); s.HasErrors() {
			b.Fatalf("errors: %v", s.DiagnosticLines()[:1])
		}
	}
	b.ReportMetric(float64(len(paths)), "files")
}
