package semantics

import "testing"

// A view exposing through a filter on an annotation evaluates that filter
// while the `about` index is still being built — resolving the unqualified
// name one of its `about` usages states looks through the exposure — so the
// verdict on an element annotated only by a later `about` usage is partial and
// must not be kept: once the index is complete, that element is exposed too.
func TestFilterVerdictReachedWhileTheAboutIndexBuildsIsNotKept(t *testing.T) {
	m, _, user := buildOverlayModel(t, `
		metadata def Tag;
	`, `
		package P {
			part def A { @Tag; }
			part def B;
		}
		package V {
			view v {
				expose P::*[@Tag];
				metadata u : Tag about A;
				metadata t : Tag about P::B;
			}
		}
	`)
	p := sym(t, user, "P").Scope
	a, b := sym(t, p, "A"), sym(t, p, "B")
	v := sym(t, sym(t, user, "V").Scope, "v")
	if types := annotationTypes(m, b); len(types) != 1 || types[0] != "Tag" {
		t.Fatalf("annotations of B = %v, want [Tag]", types)
	}
	exposed, err := m.ExposedElements(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(exposed) != 2 || exposed[0] != a || exposed[1] != b {
		t.Fatalf("ExposedElements(v) = %v, want [A B]", exposed)
	}
}
