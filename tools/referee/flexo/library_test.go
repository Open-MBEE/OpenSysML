package flexo

import (
	"context"
	"os"
	"testing"
	"time"
)

// The library measurement's fixture and expectation.
const (
	libraryUserFixturePath = "testdata/library_user_model.sysml"
	libraryExpectationPath = "testdata/library_expected.txt"
)

// libraryRequestTimeout bounds one request of the library run: Layer 1 takes
// minutes over a whole-library graph write, and the listing read is as large.
const libraryRequestTimeout = 15 * time.Minute

const libraryExpectationHeader = `# What a real Flexo MMS stack does with the whole bundled standard library,
# measured by TestFlexoInteropLibrary. This is a ratchet: every line is
# adjudicated, and a change to one is a change in interoperability, not a change
# in the harness.
#
# Regenerate against a running stack (see .agents/skills/flexo-interop):
#   FLEXO_INTEROP=1 FLEXO_INTEROP_TOKEN=<jwt> \
#     go test -C tools -count=1 -timeout 30m ./referee/flexo -run TestFlexoInteropLibrary -update-flexo
# against a stack brought up fresh: a loaded library slows every later write.
#
# graph-load is the library as sysml -stdlib -convert ttl writes it, every
# element under its normative id, loaded through Layer 1's graph endpoint into
# one project and read back through the element APIs; elements are counted by
# metaclass rather than listed, a hundred thousand lines being no ratchet a
# reader adjudicates. anchors are named library elements read directly by id.
# user-model is a second project typing parts by library ids: each typing is
# followed into the library project, the delivered elements are re-converted to
# notation, and a ProjectUsage of the library is declared. Server-generated
# commit ids, timestamps and durations are not recorded.
`

// TestFlexoInteropLibrary publishes the standard library to the live stack and
// measures what a user project can make of it.
func TestFlexoInteropLibrary(t *testing.T) {
	stack, _ := liveStack(t)
	cfg := stack.Config()
	if cfg.Timeout < libraryRequestTimeout {
		cfg.Timeout = libraryRequestTimeout
	}
	stack = Connect(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 3*libraryRequestTimeout)
	t.Cleanup(cancel)

	model, err := os.ReadFile(libraryUserFixturePath)
	if err != nil {
		t.Fatalf("read %s: %v", libraryUserFixturePath, err)
	}
	report, err := MeasureLibrary(ctx, stack, libraryUserFixturePath, model)
	if err != nil {
		t.Fatalf("measure the library: %v", err)
	}
	t.Logf("converted the library in %s, loaded it in %s, listed it in %s",
		report.Timings.Convert.Round(time.Millisecond), report.Timings.Load.Round(time.Millisecond), report.Timings.Listing.Round(time.Millisecond))
	for _, finding := range report.Findings {
		t.Log(finding)
	}
	checkExpectation(t, libraryExpectationPath, report.Text(libraryExpectationHeader))
}
