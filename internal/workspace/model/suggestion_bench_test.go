package model

import (
	"fmt"
	"strings"
	"testing"
)

// benchSrc is a file of 50 typings, either all unresolvable or all resolved:
// what a half-typed file costs to analyse next to one that resolves.
func benchSrc(unresolved bool) string {
	var b strings.Builder
	b.WriteString("private import ScalarValues::*;\npart def P {\n")
	for i := 0; i < 50; i++ {
		if unresolved {
			fmt.Fprintf(&b, "  attribute a%d : Zqxwv%d;\n", i, i)
		} else {
			fmt.Fprintf(&b, "  attribute a%d : Integer;\n", i)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func benchAnalyse(b *testing.B, unresolved bool) {
	src := benchSrc(unresolved)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws := NewWorkspace()
		ws.Open("bench.sysml", []byte(src), 1)
		ws.Diagnostics("bench.sysml")
	}
}

func BenchmarkAnalyseUnresolved(b *testing.B) { benchAnalyse(b, true) }
func BenchmarkAnalyseResolved(b *testing.B)   { benchAnalyse(b, false) }
