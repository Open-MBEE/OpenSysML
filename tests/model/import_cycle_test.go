package model_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/testbudget"
)

// importCycleLimit bounds each analysis: thirty packages take seconds, tens of them under
// the race detector, and do not finish at all when every route through them is searched.
// The host's contention budget is the only thing that moves; the analysis is unchanged.
func importCycleLimit(t *testing.T) time.Duration {
	return testbudget.Deadline(t, 3*time.Minute)
}

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

				limit := importCycleLimit(t)
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
				case <-time.After(limit):
					t.Fatalf("analysing %d packages importing each other in a cycle did not finish in %v", n, limit)
				}
			})
		}
	}
}

// A package with a `filter` member that one of the cycle imports privately is read on no route but that
// package's own, where every route into what it imports runs through it: the cycle stays polynomial.
func TestImportCycleWithAPrivatelyImportedFilterAnalysesPromptly(t *testing.T) {
	for _, n := range []int{15, 30} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			var src strings.Builder
			src.WriteString("package Marked {\n\tmetadata def Safety;\n\t#Safety part def Kept;\n\tpart def Dropped;\n}\n")
			src.WriteString("package Filtered {\n\tpublic import Marked::*;\n\tfilter @Marked::Safety;\n}\n")
			for i := 1; i <= n; i++ {
				fmt.Fprintf(&src, "package P%d {\n", i)
				for k := 1; k <= n; k++ {
					if k != i {
						fmt.Fprintf(&src, "\tpublic import P%d::*;\n", k)
					}
				}
				if i == 1 {
					src.WriteString("\tprivate import Filtered::*;\n\tpart kept : Kept;\n\tpart dropped : Dropped;\n")
				}
				fmt.Fprintf(&src, "\tpart def D%d;\n}\n", i)
			}
			fmt.Fprintf(&src, "package Use {\n\tpublic import P1::*;\n\tpart x : D%d;\n\tpart y : Kept;\n}\n", n)

			limit := importCycleLimit(t)
			done := make(chan []diag.Diagnostic, 1)
			go func() {
				ws := model.NewWorkspace()
				ws.Open("cycle.sysml", []byte(src.String()), 1)
				done <- ws.Diagnostics("cycle.sysml")
			}()
			select {
			case diags := <-done:
				// The filter keeps Dropped out of P1, and the private import keeps Kept out of Use.
				var unresolved []string
				for _, d := range diags {
					name, _ := strings.CutPrefix(d.Message, "unresolved reference: ")
					name, _, _ = strings.Cut(name, " ")
					unresolved = append(unresolved, name)
				}
				if want := []string{"Dropped", "Kept"}; !slices.Equal(unresolved, want) {
					t.Fatalf("unresolved names %q, want %q: %v", unresolved, want, diags)
				}
			case <-time.After(limit):
				t.Fatalf("analysing %d packages in a cycle, one privately importing a filtered package, did not finish in %v", n, limit)
			}
		})
	}
}

// A package of the cycle that another document declares too imports there what its filtered route
// keeps out: that second route is one of the routes into the namespace (KerML 8.3.2.4.5).
func TestImportCycleReadsTheUnfilteredRouteAnotherDocumentStates(t *testing.T) {
	const cycle = `package M { metadata def Safe; }
package S { #M::Safe part def Kept; part def Extra; }
package F { public import S::*; filter @M::Safe; }
package P { public import F::*; public import Q::*; part def PD; }
package Q { public import P::*; part def QD; }
package U { private import %s::*; part k : Kept; part e : Extra; part p : PD; part q : QD; }
package V { part e : P::Extra; part e2 : Q::Extra; part k : Q::Kept; }
`
	for _, reopened := range []string{"P", "Q"} {
		for _, read := range []string{"P", "Q"} {
			for _, first := range []string{"cycle.sysml", "reopened.sysml"} {
				t.Run(fmt.Sprintf("%s reopened, %s read, %s first", reopened, read, first), func(t *testing.T) {
					docs := map[string]string{
						"cycle.sysml":    fmt.Sprintf(cycle, read),
						"reopened.sysml": fmt.Sprintf("package %s { public import S::*; }\n", reopened),
					}
					ws := model.NewWorkspace()
					ws.Open(first, []byte(docs[first]), 1)
					for name := range docs {
						if name != first {
							ws.Open(name, []byte(docs[name]), 1)
						}
					}
					for name := range docs {
						if diags := ws.Diagnostics(name); len(diags) != 0 {
							t.Errorf("%s: %v", name, diags)
						}
					}
				})
			}
		}
	}
}

// A private import another document states for a package of the cycle is on no route a reader outside
// takes, so the cycle is still read once: wherever the namespace it names is declared.
func TestImportCycleWithAPrivateImportAnotherDocumentStatesAnalysesPromptly(t *testing.T) {
	const n = 15
	var cycle strings.Builder
	cycle.WriteString("package Marked {\n\tmetadata def Safety;\n\t#Safety part def Kept;\n\tpart def Dropped;\n}\n")
	cycle.WriteString("package Filtered {\n\tpublic import Marked::*;\n\tfilter @Marked::Safety;\n}\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&cycle, "package P%d {\n", i)
		for k := 1; k <= n; k++ {
			if k != i {
				fmt.Fprintf(&cycle, "\tpublic import P%d::*;\n", k)
			}
		}
		if i == 1 {
			cycle.WriteString("\tprivate import Filtered::*;\n")
		}
		fmt.Fprintf(&cycle, "\tpart def D%d;\n}\n", i)
	}
	fmt.Fprintf(&cycle, "package Use {\n\tpublic import P1::*;\n\tpart x : D%d;\n\tpart y : Missing;\n\tpart z : E;\n}\n", n)
	const elsewhere = "package Elsewhere {\n\tpart def E;\n}\n"
	const reopened = "package P2 {\n\tprivate import Elsewhere::*;\n\tpart e : E;\n}\n"

	for _, tc := range []struct {
		name string
		docs map[string]string
	}{
		{"named in the document of the cycle", map[string]string{"cycle.sysml": elsewhere + cycle.String(), "reopened.sysml": reopened}},
		{"named in the document stating it", map[string]string{"cycle.sysml": cycle.String(), "reopened.sysml": reopened + elsewhere}},
		{"named in a third document", map[string]string{"cycle.sysml": cycle.String(), "reopened.sysml": reopened, "elsewhere.sysml": elsewhere}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limit := importCycleLimit(t)
			done := make(chan map[string][]diag.Diagnostic, 1)
			go func() {
				ws := model.NewWorkspace()
				for _, name := range slices.Sorted(maps.Keys(tc.docs)) {
					ws.Open(name, []byte(tc.docs[name]), 1)
				}
				diags := map[string][]diag.Diagnostic{}
				for name := range tc.docs {
					diags[name] = ws.Diagnostics(name)
				}
				done <- diags
			}()
			select {
			case diags := <-done:
				// The private import brings E to the declaration stating it, and to no reader of the cycle.
				var unresolved []string
				for name, found := range diags {
					for _, d := range found {
						if name != "cycle.sysml" {
							t.Errorf("%s: %v", name, d)
						}
						missing, _ := strings.CutPrefix(d.Message, "unresolved reference: ")
						missing, _, _ = strings.Cut(missing, " ")
						unresolved = append(unresolved, missing)
					}
				}
				if want := []string{"Missing", "E"}; !slices.Equal(unresolved, want) {
					t.Fatalf("unresolved names %q, want %q: %v", unresolved, want, diags)
				}
			case <-time.After(limit):
				t.Fatalf("analysing %d packages in a cycle, one given a private import by another document, did not finish in %v", n, limit)
			}
		})
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
			limit := testbudget.Deadline(t, 5*time.Second)
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
			case <-time.After(limit):
				t.Fatalf("analysing a filtered import over membership imports did not finish in %v", limit)
			}
		})
	}
}
