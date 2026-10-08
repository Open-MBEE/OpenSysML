package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/variables.xmi, activity Buffers: a central buffer with one
// target writes as the flows through it, a fan-out buffer and a data store
// approximate.
func TestBuffersFixture(t *testing.T) {
	r := migrateFixtureFile(t, "variables")
	wantClean(t, "variables.xmi", r)

	wantNote(t, r, "_buf", migrate.Mapped, "writing the flow from its source to its target behaves as the buffer does")
	wantNote(t, r, "_fan", migrate.Approximated, "the buffer offers each token to one of its targets; the flows written give every target the value")
	wantNote(t, r, "_store", migrate.Approximated, "a data store keeps every value and gives each reader a copy; a v2 flow holds no store, so the flows through it are written from its sources to its targets")
}
