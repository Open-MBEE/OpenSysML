package model_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// importCycleLimit bounds each analysis: thirty packages take seconds, tens of them under
// the race detector, and do not finish at all when every route through them is searched.
const importCycleLimit = 3 * time.Minute

// Packages that publicly import one another in a cycle have to analyse in time polynomial in them
// (issue #633): a search per route doubles with each package, which 15 and 30 show where 6 and 8 do not.
func TestPackagesImportingEachOtherInACycleAnalysePromptly(t *testing.T) {
	for _, n := range []int{6, 8, 15, 30} {
		// Packages that declare something are enumerated as well as searched;
		// ones holding only their imports are searched, for the import targets.
		for _, declaring := range []bool{true, false} {
			t.Run(fmt.Sprintf("n=%d/declaring=%v", n, declaring), func(t *testing.T) {
				var src strings.Builder
				for i := 1; i <= n; i++ {
					fmt.Fprintf(&src, "package P%d {\n", i)
					for k := 1; k <= n; k++ {
						if k != i {
							fmt.Fprintf(&src, "\tpublic import P%d::*;\n", k)
						}
					}
					if declaring {
						fmt.Fprintf(&src, "\tpart def D%d;\n", i)
					}
					src.WriteString("}\n")
				}
				want := 1
				if declaring {
					fmt.Fprintf(&src, "package Use {\n\tpublic import P1::*;\n\tpart x : D%d;\n\tpart y : Missing;\n}\n", n)
				} else {
					// Nothing declares D, so both names are unresolved.
					src.WriteString("package Use {\n\tpublic import P1::*;\n\tpart x : D;\n\tpart y : Missing;\n}\n")
					want = 2
				}

				done := make(chan []diag.Diagnostic, 1)
				go func() {
					ws := model.NewWorkspace()
					ws.Open("cycle.sysml", []byte(src.String()), 1)
					done <- ws.Diagnostics("cycle.sysml")
				}()
				select {
				case diags := <-done:
					if len(diags) != want {
						t.Fatalf("want %d unresolved names; got %d diagnostics: %v", want, len(diags), diags)
					}
					if last := diags[len(diags)-1].Message; !strings.Contains(last, "Missing") {
						t.Fatalf("the last diagnostic should name `Missing`; got %q", last)
					}
				case <-time.After(importCycleLimit):
					t.Fatalf("analysing %d packages importing each other in a cycle did not finish in %v", n, importCycleLimit)
				}
			})
		}
	}
}

// A filter clause's own names resolve while the import carrying it is being
// searched: a nested lookup of the same name over the same import edge must
// not be cut short by the visit set of the enclosing search (KerML 8.2.4).
func TestFilteredImportOverMembershipImportsRejectsUnmarked(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
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
			name: "filter names the resolved name",
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
