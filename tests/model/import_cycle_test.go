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

// A filter clause's own names resolve while the import carrying it is being
// searched: a nested lookup of the same name over the same import edge must
// not be cut short by the visit set of the enclosing search (KerML 8.2.4).
func TestFilteredImportOverMembershipImportsRejectsUnmarked(t *testing.T) {
	for _, tc := range []struct {
		name      string
		src       string
		ambiguous bool
	}{
		{
			name: "filter names another member",
			src: `package S {
	metadata def Marker;
	#Marker part def Good;
	part def Bad;
}
package R {
	public import S::Marker;
	public import S::Good;
	public import S::Bad;
}
package Q {
	public import R::*[@Marker];
}
package P {
	public import Q::*;
	part g : Good;
	part b : Bad;
}
`,
		},
		{
			name:      "filter and imported type have conflicting names",
			ambiguous: true,
			src: `package M { metadata def Good; }
package S {
	#M::Good part def Good;
	part def Bad;
}
package R {
	public import S::*;
}
package Q {
	public import R::*[@Good];
	public import M::*;
}
package P {
	public import Q::*;
	part g : Good;
	part b : Bad;
}
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan []diag.Diagnostic, 1)
			go func() {
				ws := model.NewWorkspace()
				ws.Open("filter.sysml", []byte(tc.src), 1)
				done <- ws.Diagnostics("filter.sysml")
			}()
			select {
			case diags := <-done:
				if tc.ambiguous {
					if len(diags) != 2 {
						t.Fatalf("filter and consumer must both reject Good: %v", diags)
					}
					for _, d := range diags {
						if d.Code != "unresolved" || !strings.HasPrefix(d.Message, "unresolved reference: Good") {
							t.Fatalf("unexpected diagnostic: %v", d)
						}
					}
					// Qualify the metadata type and remove its competing wildcard import.
					qualified := strings.Replace(tc.src, "R::*[@Good]", "R::*[@M::Good]", 1)
					qualified = strings.Replace(qualified, "\n\tpublic import M::*;", "", 1)
					ws := model.NewWorkspace()
					ws.Open("qualified.sysml", []byte(qualified), 1)
					diags = ws.Diagnostics("qualified.sysml")
				}
				if len(diags) != 1 {
					t.Fatalf("only `Bad` is unresolved; got %d diagnostics: %v", len(diags), diags)
				}
				if !strings.Contains(diags[0].Message, "Bad") {
					t.Fatalf("the one diagnostic should name `Bad`; got %q", diags[0].Message)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("analysing a filtered import over membership imports did not finish in 5s")
			}
		})
	}
}
