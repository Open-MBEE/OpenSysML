package lower

import (
	"fmt"
	"strings"
	"testing"
)

// starOfSuccessions is an action whose step a leads to count others over
// successions stating end multiplicities, each restated by the specialization.
func starOfSuccessions(count int) string {
	var base, restated strings.Builder
	for i := range count {
		fmt.Fprintf(&base, "action b%d;\nsuccession first [1] a then [1] b%d;\n", i, i)
		fmt.Fprintf(&restated, "succession first [1] a then [1] b%d;\n", i)
	}
	return "action def Base {\naction a[1];\n" + base.String() + "}\naction def S :> Base {\n" + restated.String() + "}\n"
}

func TestToActionGraphDeduplicatesAStarOfRestatedSuccessions(t *testing.T) {
	const count = 40
	decl, scope, _ := inheritedActionDecl(t, starOfSuccessions(count), "S")
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("lower S: %v", err)
	}
	if got := len(graph.Edges[namedNode(graph, "a")]); got != count {
		t.Fatalf("a has %d out-edges, want one per restated succession (%d)", got, count)
	}
}

func BenchmarkToActionGraphStarOfSuccessions(b *testing.B) {
	for _, count := range []int{50, 200, 800} {
		decl, scope, _ := inheritedActionDecl(b, starOfSuccessions(count), "S")
		b.Run(fmt.Sprintf("successions=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ToActionGraph(decl, scope); err != nil {
					b.Fatalf("lower S: %v", err)
				}
			}
		})
	}
}
