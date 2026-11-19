package view

import (
	"fmt"
	"strings"
	"testing"
)

func TestRepeatedExpansionFoldReleasesPlans(t *testing.T) {
	for _, kind := range []string{"asTreeDiagram", "asInterconnectionDiagram"} {
		for _, threshold := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/%d", kind, threshold), func(t *testing.T) {
				src := strings.Replace(sharedQueryModel(3, 2), "asTreeDiagram", kind, 1)
				src = strings.Replace(src, "metadata Layout about root { x = 0; y = 0; }", "", 1)
				r, idx := loadSources(t, []string{"lifetime.sysml"}, [][]byte{[]byte(src)})
				r.expansionFoldThreshold = threshold
				v := lookup(t, idx, "P::v")
				renderingKind, _, err := r.KindOf(v)
				if err != nil {
					t.Fatal(err)
				}
				exposed, err := r.model.ExposedElements(v)
				if err != nil {
					t.Fatal(err)
				}
				for _, implicit := range []bool{false, true} {
					t.Run(fmt.Sprintf("exposed=%v", implicit), func(t *testing.T) {
						var got *Rendering
						var err error
						if implicit {
							got, err = r.RenderExposed(exposed, renderingKind, "")
						} else {
							got, err = r.Render(v)
						}
						if err != nil {
							t.Fatal(err)
						}
						wantNotice := ""
						if threshold == 1 {
							if got.foldedOccurrences == 0 {
								t.Fatal("no occurrence folded")
							}
							wantNotice = fmt.Sprintf("Repeated definition expansions folded: %d later occurrences of 1 definitions. Each definition's features are shown at an earlier occurrence expanded at least as deeply; later occurrences retain their own and redefined features.", got.foldedOccurrences)
						}
						for _, completed := range []*Rendering{got, got.Clone()} {
							if completed.expansionPlans != nil || completed.foldedDefinitions != nil {
								t.Fatal("completed rendering retains fold plans or definitions")
							}
							if completed.ExpansionFoldNotice != wantNotice {
								t.Fatalf("notice changed: %q; want %q", completed.ExpansionFoldNotice, wantNotice)
							}
						}
					})
				}
			})
		}
	}
}
