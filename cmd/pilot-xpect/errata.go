package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/errata"
)

// ErrataReport is the second figure of the run: the same adjudication over the
// suites with the declared corrections applied to a copy, never to the corpus.
type ErrataReport struct {
	Registry    int           `json:"registryEntries"`
	Corrections int           `json:"corrections"`
	Documented  int           `json:"documentedWithoutCorrection"`
	Applied     int           `json:"correctionsApplied"`
	Entries     []ErrataEntry `json:"entries"`
	Totals      Totals        `json:"totals"`
	Kinds       []KindTotals  `json:"kinds"`
	Note        string        `json:"note"`
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

// erratumSuite adjudicates one suite again over a corrected copy of it. It
// returns applied == 0 when no correction lies inside the suite, in which case
// the caller carries the as-published results over unchanged.
func erratumSuite(s suite, overlay *errata.Overlay, repo, out string, jobs int) (results []fileResult, applied int, err error) {
	if len(overlay.Under(s.Dir)) == 0 {
		return nil, 0, nil
	}
	corrected := filepath.Join(out, "errata-corpora", s.Name)
	entries, err := errata.Materialize(overlay, repo, s.Dir, corrected)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if removeErr := os.RemoveAll(corrected); removeErr != nil && err == nil {
			err = removeErr
		}
		// leaves nothing behind once the last suite's copy is gone
		_ = os.Remove(filepath.Dir(corrected))
	}()
	files, err := collectXT(corrected)
	if err != nil {
		return nil, 0, err
	}
	fmt.Fprintf(os.Stderr, "%s: %d .xt file(s) with %d correction(s) applied\n", s.Name, len(files), len(entries))
	return compareAll(corrected, files, jobs), len(entries), nil
}
