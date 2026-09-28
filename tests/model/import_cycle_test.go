package model_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Resolving a name visible through a cycle of packages that publicly import
// one another searched every simple path through the cycle, so six mutually
// importing packages did not finish validating (issue #633). Each import edge
// must be searched once per lookup, as the spec's visible-membership rule
// intends (KerML 8.2.3.5).
func TestPackagesImportingEachOtherInACycleAnalysePromptly(t *testing.T) {
	for _, n := range []int{6, 8} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			var src strings.Builder
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&src, "package P%d {\n", i)
				for k := 1; k <= n; k++ {
					if k != i {
						fmt.Fprintf(&src, "\tpublic import P%d::*;\n", k)
					}
				}
				fmt.Fprintf(&src, "\tpart def D%d;\n}\n", i)
			}
			fmt.Fprintf(&src, "package Use {\n\tpublic import P1::*;\n\tpart x : D%d;\n\tpart y : Missing;\n}\n", n)

			done := make(chan []diag.Diagnostic, 1)
			go func() {
				ws := model.NewWorkspace()
				ws.Open("cycle.sysml", []byte(src.String()), 1)
				done <- ws.Diagnostics("cycle.sysml")
			}()
			select {
			case diags := <-done:
				if len(diags) != 1 {
					t.Fatalf("only `Missing` is unresolved; got %d diagnostics: %v", len(diags), diags)
				}
				if !strings.Contains(diags[0].Message, "Missing") {
					t.Fatalf("the one diagnostic should name `Missing`; got %q", diags[0].Message)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("analysing %d packages importing each other in a cycle did not finish in 5s", n)
			}
		})
	}
}
