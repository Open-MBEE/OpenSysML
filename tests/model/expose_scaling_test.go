package model_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Analysing a view resolves the names in its body against its exposes, and each
// expose's target is found by resolving a name in that same view. Before import
// targets were memoized, every such resolution searched the other exposes'
// unresolved targets again, in every order: a view with 12 exposes and one
// metadata usage did not finish (issue #636). Twenty must analyse at once.
func TestViewWithManyExposesAnalysesPromptly(t *testing.T) {
	const n = 20
	var src strings.Builder
	src.WriteString("metadata def Tag;\n")
	for k := 1; k <= n; k++ {
		fmt.Fprintf(&src, "package P%d { }\n", k)
	}
	src.WriteString("view v {\n")
	for k := 1; k <= n; k++ {
		fmt.Fprintf(&src, "\texpose P%d::*;\n", k)
	}
	src.WriteString("\t@Tag;\n}\n")

	done := make(chan int, 1)
	go func() {
		ws := model.NewWorkspace()
		ws.Open("views.sysml", []byte(src.String()), 1)
		done <- len(ws.Diagnostics("views.sysml"))
	}()
	select {
	case count := <-done:
		if count != 0 {
			t.Fatalf("the view is valid SysML; got %d diagnostics", count)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("analysing a view with %d exposes did not finish in 10s", n)
	}
}
