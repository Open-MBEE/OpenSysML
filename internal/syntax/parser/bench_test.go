package parser

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// benchModelEnv names a directory of .sysml/.kerml files to parse, so the parser
// alone is timed over a real model. See docs/internals/performance.md.
//
//	OPENSYSML_BENCH_MODEL=/path/to/model go test ./internal/syntax/parser -run '^$' -bench ParseModel -benchmem
const benchModelEnv = "OPENSYSML_BENCH_MODEL"

func BenchmarkParseModel(b *testing.B) {
	dir := os.Getenv(benchModelEnv)
	if dir == "" {
		b.Skipf("%s is not set", benchModelEnv)
	}
	type file struct {
		path string
		data []byte
	}
	var files []file
	var lines, size int
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || source.KindOf(path) == source.KindUnknown {
			return err
		}
		data, err := os.ReadFile(path) // #nosec G304 -- the directory is the one the environment names.
		if err != nil {
			return err
		}
		files = append(files, file{path, data})
		lines += bytes.Count(data, []byte("\n"))
		size += len(data)
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
	if len(files) == 0 {
		b.Fatalf("no .sysml or .kerml file under %s", dir)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range files {
			// A fresh SourceFile each time, so its lazy text conversion is paid as a real load pays it.
			p := New(source.New(f.path, f.data))
			p.ParseFile()
			if len(p.Diagnostics) > 0 {
				b.Fatalf("%s: %s", f.path, p.Diagnostics[0].Message)
			}
		}
	}
	b.ReportMetric(float64(len(files)), "files")
	b.ReportMetric(float64(lines), "lines")
	b.ReportMetric(float64(size), "bytes")
}
