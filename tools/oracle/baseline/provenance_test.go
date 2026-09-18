package baseline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPinResolvesTheRepositorysPin(t *testing.T) {
	pin, err := ReadPin("../..")
	if err != nil {
		t.Fatal(err)
	}
	if pin.Tag == "" || pin.Commit == "" || pin.Artifact == "" {
		t.Fatalf("%s resolved to %+v", PinPath, pin)
	}
	if want := pin.Tag + " (jupyter-sysml-kernel " + pin.Artifact + ")"; pin.Release() != want {
		t.Errorf("release: want %q, got %q", want, pin.Release())
	}
}

func TestReadPinFailsOnAPinItCannotResolve(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"no defaults":    "PILOT_TAG=2026-05\n",
		"tag only":       "PILOT_TAG=\"${PILOT_TAG:-2026-05}\"\nPILOT_ARTIFACT_VERSION=\"${PILOT_ARTIFACT_VERSION:-0.60.1}\"\n",
		"commit not sha": "PILOT_TAG=\"${PILOT_TAG:-2026-05}\"\nPILOT_COMMIT=\"${PILOT_COMMIT:-2026-05}\"\nPILOT_ARTIFACT_VERSION=\"${PILOT_ARTIFACT_VERSION:-0.60.1}\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(PinPath)), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := ReadPin(root)
			if err == nil || !strings.Contains(err.Error(), "pins no PILOT_TAG") {
				t.Fatalf("want a pin-resolution failure, got %v", err)
			}
		})
	}
}

// TestDigestFilesIdentifiesTheSetNotTheOrder checks the property the corpus
// digests rely on: order does not matter, but a rename or an edit does.
func TestDigestFilesIdentifiesTheSetNotTheOrder(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"a.sysml": "part a;\n", "b.sysml": "part b;\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	forward, err := DigestFiles(dir, []string{"a.sysml", "b.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := DigestFiles(dir, []string{"b.sysml", "a.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if forward != reversed {
		t.Errorf("digest depends on the listing order: %s vs %s", forward, reversed)
	}

	if err := os.Rename(filepath.Join(dir, "b.sysml"), filepath.Join(dir, "c.sysml")); err != nil {
		t.Fatal(err)
	}
	renamed, err := DigestFiles(dir, []string{"a.sysml", "c.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if renamed == forward {
		t.Error("a renamed file leaves the digest unmoved")
	}

	if err := os.WriteFile(filepath.Join(dir, "a.sysml"), []byte("part a2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	edited, err := DigestFiles(dir, []string{"a.sysml", "c.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if edited == renamed {
		t.Error("an edited file leaves the digest unmoved")
	}
}

func TestValidateNamesWhatARecordFailsToState(t *testing.T) {
	complete := Record{
		PilotTag:      "2026-05",
		PilotCommit:   "fa709f28dfd49dfdb7ee83e4e19da2f57e0eb3aa",
		PilotArtifact: "0.60.1",
		Errata:        "sha256:aa",
		Inputs:        []Input{{Name: "corpus", Dir: "examples", Origin: OriginOurs, Files: 1, Digest: "sha256:bb"}},
		Recorded:      "2026-08-26",
	}
	if err := Validate(complete); err != nil {
		t.Fatalf("a complete record is invalid: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Record)
		want   string
	}{
		{"no tag", func(r *Record) { r.PilotTag = "" }, "states no pilot pin"},
		{"no commit", func(r *Record) { r.PilotCommit = "" }, "states no pilot pin"},
		{"no artifact", func(r *Record) { r.PilotArtifact = "" }, "states no pilot pin"},
		{"no errata", func(r *Record) { r.Errata = "" }, "states no errata registry digest"},
		{"no inputs", func(r *Record) { r.Inputs = nil }, "states no compared inputs"},
		{"no date", func(r *Record) { r.Recorded = "" }, "states no ISO recording date"},
		{"unparsable date", func(r *Record) { r.Recorded = "August 2026" }, "states no ISO recording date"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := complete
			tc.mutate(&record)
			err := Validate(record)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

// TestCompareClassifiesEveryMismatchByItsAction is the distinction the guards
// and the scheduled run both report: a moved pin, moved material of ours and a
// pinned input that moved at an unchanged pin call for different actions.
func TestCompareClassifiesEveryMismatchByItsAction(t *testing.T) {
	recorded := Record{
		PilotTag:      "2026-05",
		PilotCommit:   "fa709f28dfd49dfdb7ee83e4e19da2f57e0eb3aa",
		PilotArtifact: "0.60.1",
		Errata:        "sha256:aa",
		Tools:         []Tool{{Name: "bridge", Source: "scripts/x.java", SourceDigest: "sha256:cc", Release: "2026-05"}},
		Inputs: []Input{
			{Name: "ours", Dir: "examples", Origin: OriginOurs, Files: 2, Digest: "sha256:dd"},
			{Name: "pinned", Dir: "build/corpus", Origin: OriginPinned, Files: 3, Digest: "sha256:ee"},
		},
		Recorded: "2026-08-26",
	}
	current := recorded
	current.PilotTag = "2026-11"
	current.PilotCommit = "c7fc737d56da9e2d78f9d7df6d38efbec2e7e965"
	current.Tools = []Tool{{Name: "bridge", Source: "scripts/x.java", SourceDigest: "sha256:c0", Release: "2026-11"}}
	current.Inputs = []Input{
		{Name: "ours", Dir: "examples", Origin: OriginOurs, Files: 2, Digest: "sha256:d0"},
		{Name: "pinned", Dir: "build/corpus", Origin: OriginPinned, Files: 4, Digest: "sha256:ee"},
	}

	want := map[string]Cause{
		"pilotTag":                   CausePin,
		"pilotCommit":                CausePin,
		"tools[bridge].sourceDigest": CauseOurs,
		"tools[bridge].release":      CausePin,
		"inputs[ours].digest":        CauseOurs,
		"inputs[pinned].files":       CausePinned,
	}
	got := map[string]Cause{}
	for _, m := range Compare(recorded, current) {
		got[m.Field] = m.Cause
	}
	if len(got) != len(want) {
		t.Fatalf("want mismatches on %v, got %v", want, got)
	}
	for field, cause := range want {
		if got[field] != cause {
			t.Errorf("%s: want cause %d, got %d", field, cause, got[field])
		}
	}

	explained := Explain("docs/project/x-baseline.json", "go run ./cmd/x -update", Compare(recorded, current))
	for _, fragment := range []string{
		"docs/project/x-baseline.json no longer describes this repository",
		`provenance.pilotTag: baseline records "2026-05", repository now has "2026-11"`,
		"the repository's pilot pin has moved",
		"pinned reference material differs",
		"material this repository owns has changed",
		"re-record it with: go run ./cmd/x -update",
	} {
		if !strings.Contains(explained, fragment) {
			t.Errorf("the explanation does not state %q:\n%s", fragment, explained)
		}
	}
}

// TestCompareIgnoresInputsThisCheckoutDoesNotHave keeps the guard runnable
// without the downloaded corpora, which is the whole point of it being Java-free.
func TestCompareIgnoresInputsThisCheckoutDoesNotHave(t *testing.T) {
	recorded := Record{Inputs: []Input{
		{Name: "ours", Origin: OriginOurs, Files: 1, Digest: "sha256:aa"},
		{Name: "downloaded", Origin: OriginPinned, Files: 9, Digest: "sha256:bb"},
	}}
	current := Record{Inputs: []Input{recorded.Inputs[0]}}
	if got := Compare(recorded, current); len(got) != 0 {
		t.Fatalf("want no mismatch for an absent input, got %+v", got)
	}
}

// TestReproducesSaysWhichActionADifferenceCallsFor covers the scheduled run's
// two verdicts, and that the recording date alone is not a non-reproduction.
func TestReproducesSaysWhichActionADifferenceCallsFor(t *testing.T) {
	dir := t.TempDir()
	committedPath := filepath.Join(dir, "baseline.json")
	committed := `{"provenance":{"pilotTag":"2026-05","recorded":"2026-08-01"},"totals":{"pilotOnly":61}}`
	if err := os.WriteFile(committedPath, []byte(committed), 0o600); err != nil {
		t.Fatal(err)
	}

	sameDayLater := `{"provenance":{"pilotTag":"2026-05","recorded":"2026-09-09"},"totals":{"pilotOnly":61}}`
	if err := Reproduces(committedPath, []byte(sameDayLater)); err != nil {
		t.Errorf("the recording date alone reads as a non-reproduction: %v", err)
	}

	moved := `{"provenance":{"pilotTag":"2026-05","recorded":"2026-08-01"},"totals":{"pilotOnly":60}}`
	err := Reproduces(committedPath, []byte(moved))
	if err == nil {
		t.Fatal("a moved count reproduces")
	}
	if !strings.Contains(err.Error(), "totals.pilotOnly: 61 -> 60") ||
		!strings.Contains(err.Error(), "implementation movement") {
		t.Errorf("a moved count is misreported: %v", err)
	}

	repinned := `{"provenance":{"pilotTag":"2026-11","recorded":"2026-08-01"},"totals":{"pilotOnly":61}}`
	err = Reproduces(committedPath, []byte(repinned))
	if err == nil {
		t.Fatal("a moved pin reproduces")
	}
	if !strings.Contains(err.Error(), "provenance.pilotTag") ||
		!strings.Contains(err.Error(), "Investigate the provisioning") {
		t.Errorf("a moved pin is misreported: %v", err)
	}
}

// TestWriteRefusesAReportWithoutProvenance stops -update from committing a
// baseline the Java-free guard could not check.
func TestWriteRefusesAReportWithoutProvenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	err := Write(path, []byte(`{"totals":{"pilotOnly":61}}`))
	if err == nil || !strings.Contains(err.Error(), "states no pilot pin") {
		t.Fatalf("want a refusal naming the missing provenance, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("a report without provenance was still written")
	}
}
