package reposync_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
)

// A bare comment and a doc body are anonymous annotating elements with no place
// in the notation for an id, so minting skips them and mints their owner.
func TestMintingSkipsAnonymousAnnotations(t *testing.T) {
	repository := graphOf(t, scoped(vehicle))
	local := graphOf(t, scoped(vehicle+`	/* bare */
	doc /* d */
	part def Z;
`))
	n := 0
	set, err := reposync.Diff(local, repository, reposync.Options{
		MintIDs: true,
		NewID: func() (string, error) {
			n++
			return "minted-uuid-" + string(rune('0'+n)), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mintedOwner := false
	for _, change := range set.Changes {
		if change.MintedID == "" {
			continue
		}
		switch change.Metaclass {
		case "Comment", "Documentation", "TextualRepresentation":
			t.Errorf("the anonymous %s %s was minted an id", change.Metaclass, change.ID)
		}
		if change.ID == "P__Z" {
			mintedOwner = true
		}
	}
	if !mintedOwner {
		t.Errorf("the annotated element was not minted:\n%s", set.Text())
	}
}
