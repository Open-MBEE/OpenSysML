package perf

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/format"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// largestLibraryFile is the standard library's biggest file, copies times over.
func largestLibraryFile(b *testing.B, copies int) string {
	b.Helper()
	src := libs.DefaultSource()
	var largest []byte
	for _, name := range src.List() {
		content, err := src.Read(name)
		if err != nil {
			b.Fatal(err)
		}
		if len(content) > len(largest) {
			largest = content
		}
	}
	return strings.Repeat(string(largest), copies)
}

// unformat changes every line: indentation shifted, trailing spaces and blank lines added.
func unformat(src string) string {
	var b strings.Builder
	for i, line := range strings.SplitAfter(src, "\n") {
		switch i % 4 {
		case 0:
			b.WriteString(strings.TrimLeft(line, " \t"))
		case 1:
			b.WriteString("\t" + line)
		case 2:
			b.WriteString(strings.TrimSuffix(line, "\n") + "  \n")
		default:
			b.WriteString(line + "\n\n")
		}
	}
	return b.String()
}

// BenchmarkFormattingEdits reports what the editor's Format Document costs beyond the
// formatter's own pass (diff/format): the edits must stay the smaller part at every size.
func BenchmarkFormattingEdits(b *testing.B) {
	opts := protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}
	layout := format.Options{IndentWidth: 4, BreakStatements: true}
	for _, copies := range []int{1, 8} {
		b.Run(fmt.Sprintf("library-x%d", copies), func(b *testing.B) {
			content := []byte(unformat(largestLibraryFile(b, copies)))
			ws := model.NewWorkspace()
			name := uri.File("/tmp/perf-format.sysml").FsPath()
			ws.Open(name, content, 1)
			server := lsp.NewServer(ws)
			params := &protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)}, Options: opts}
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			var formatTime, requestTime time.Duration
			var edits int
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				if _, err := format.Source(name, content, layout); err != nil {
					b.Fatal(err)
				}
				formatted := time.Now()
				got, err := server.Formatting(context.Background(), params)
				if err != nil {
					b.Fatal(err)
				}
				formatTime += formatted.Sub(start)
				requestTime += time.Since(formatted)
				edits = len(got)
			}
			if edits == 0 {
				b.Fatal("no edits: the document did not parse or was already formatted")
			}
			b.ReportMetric(float64(edits), "edits")
			b.ReportMetric(float64(formatTime.Nanoseconds())/float64(b.N), "format-ns/op")
			b.ReportMetric(float64((requestTime-formatTime).Nanoseconds())/float64(b.N), "diff-ns/op")
			b.ReportMetric(float64(requestTime-formatTime)/float64(formatTime), "diff/format")
		})
	}
}
