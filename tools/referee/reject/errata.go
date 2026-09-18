package reject

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
)

// ErrataReport is the second figure of the run: the same buckets over the
// corpus with the declared corrections applied to a copy, never to the corpus.
type ErrataReport struct {
	Registry    int           `json:"registryEntries"`
	Corrections int           `json:"corrections"`
	Documented  int           `json:"documentedWithoutCorrection"`
	Applied     int           `json:"correctionsApplied"`
	Entries     []ErrataEntry `json:"entries"`
	Totals      Totals        `json:"totals"`
	// VerdictChanges are the cases a correction moved between buckets.
	VerdictChanges []VerdictChange `json:"verdictChanges,omitempty"`
	Note           string          `json:"note"`
}

// ErrataEntry is one declared entry with the provenance the report must carry.
type ErrataEntry struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Citation   string `json:"citation"`
	Corrected  bool   `json:"corrected"`
	Derivation string `json:"derivation"`
}

// VerdictChange is one case whose bucket the errata moved.
type VerdictChange struct {
	Path      string `json:"path"`
	Published string `json:"asPublished"`
	Corrected string `json:"withErrata"`
}

func newErrataReport(overlay *errata.Overlay) *ErrataReport {
	report := &ErrataReport{
		Registry:    len(overlay.Entries()),
		Corrections: len(overlay.Corrections()),
		Documented:  len(overlay.Documented()),
	}
	for _, entry := range overlay.Entries() {
		report.Entries = append(report.Entries, ErrataEntry{
			ID:         entry.ID,
			Path:       entry.Path,
			Line:       entry.Line,
			Citation:   entry.Citation,
			Corrected:  entry.Corrects(),
			Derivation: entry.Derivation,
		})
	}
	return report
}

// runErrata fills the report's errata figure. The hand-written negative corpus
// is ours, so no declared entry lies in it and the two figures coincide; a run
// pointed at a published corpus with -corpus adjudicates a corrected copy.
func runErrata(report *Report, overlay *errata.Overlay, adj adjudication, out string) error {
	applied := overlay.Under(adj.corpusDir)
	if len(applied) == 0 {
		report.Errata.Totals = report.Totals
		report.Errata.Note = fmt.Sprintf(
			"no declared correction lies under %s, so the errata-applied corpus is byte-identical to the published one and both figures coincide", adj.corpusDir)
		return nil
	}

	corrected := filepath.Join(out, "errata-corpus")
	if _, err := errata.Materialize(overlay, adj.repo, adj.corpusDir, corrected); err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(corrected); err != nil {
			fmt.Fprintf(os.Stderr, "remove the corrected copy: %v\n", err)
		}
	}()

	correctedAdj := adj
	correctedAdj.repo, correctedAdj.corpusDir = corrected, "."
	cases, err := adjudicate(correctedAdj)
	if err != nil {
		return err
	}
	published := make(map[string]string, len(report.Cases))
	for _, c := range report.Cases {
		published[c.Path] = c.Bucket
	}
	for _, rel := range adj.files {
		report.Errata.Totals.count(cases[rel].Bucket)
		if was := published[rel]; was != cases[rel].Bucket {
			report.Errata.VerdictChanges = append(report.Errata.VerdictChanges,
				VerdictChange{Path: rel, Published: was, Corrected: cases[rel].Bucket})
		}
	}
	report.Errata.Applied = errata.Count(applied)
	report.Errata.Note = "adjudicated again over a corrected copy of the corpus; the published corpus is unchanged on disk"
	return nil
}
