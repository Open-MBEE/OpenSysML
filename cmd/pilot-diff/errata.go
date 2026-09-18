package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/errata"
)

// ErrataReport is the second figure of the run: the same comparison over the
// corpus with the declared corrections applied. The published corpus is never
// written to; the corrected text lives in a copy under the output directory.
type ErrataReport struct {
	// Registry and Corrections count the declared entries and the substituting
	// ones; Documented are the defects recorded without a correction.
	Registry    int `json:"registryEntries"`
	Corrections int `json:"corrections"`
	Documented  int `json:"documentedWithoutCorrection"`
	// Applied are the corrections that lie inside this run's corpora.
	Applied int           `json:"correctionsApplied"`
	Entries []ErrataEntry `json:"entries"`
	// Totals is the whole run with the errata applied; Report.Totals stays the
	// as-published headline.
	Totals   Totals          `json:"totals"`
	Findings []ErrataFinding `json:"findings"`
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

// ErrataFinding is what a correction did to the two implementations at its own
// line. A correction that clears our diagnostic while the pilot still reports
// there is a finding, not a fix, so it is stated rather than folded into totals.
type ErrataFinding struct {
	ID              string `json:"id"`
	Root            string `json:"root"`
	Path            string `json:"path"`
	Line            int    `json:"line"`
	OursPublished   int    `json:"openSysMLAsPublished"`
	OursCorrected   int    `json:"openSysMLWithErrata"`
	PilotPublished  int    `json:"pilotAsPublished"`
	PilotCorrected  int    `json:"pilotWithErrata"`
	PilotVerdictNew bool   `json:"pilotVerdictChanged"`
	Note            string `json:"note"`
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

// erratumRun is one root compared a second time with its corrections applied.
type erratumRun struct {
	totals   Totals
	findings []ErrataFinding
	applied  int
}

// runErrata compares one root again over a corrected copy of it. It returns the
// zero run when no correction lies inside the root, in which case the caller
// carries the as-published totals over unchanged.
func runErrata(root corpusRoot, files []string, overlay *errata.Overlay, ours, theirs map[string][]diagnostic,
	opts options) (erratumRun, error) {
	applied := coveredBy(overlay.Under(root.Dir), files)
	if len(applied) == 0 {
		return erratumRun{}, nil
	}
	corrected := filepath.Join(opts.out, "errata-corpora", root.Name)
	if _, err := errata.Materialize(overlay, opts.repo, root.Dir, corrected); err != nil {
		return erratumRun{}, err
	}
	defer func() {
		if err := os.RemoveAll(corrected); err != nil {
			fmt.Fprintf(os.Stderr, "remove the corrected copy: %v\n", err)
		}
		// leaves nothing behind once the last root's copy is gone
		_ = os.Remove(filepath.Dir(corrected))
	}()

	erratumOurs := make(map[string][]diagnostic, len(files))
	erratumTheirs := make(map[string][]diagnostic, len(files))
	for _, batch := range batchByLanguage(files) {
		fmt.Fprintf(os.Stderr, "%s: %d %s file(s) with the errata applied\n", root.Name, len(batch.Files), batch.Kind)
		pilot := opts.validator
		if batch.Kind == source.KindKerML {
			pilot = opts.kermlValidator
		}
		batchOurs, err := openSysMLDiagnostics(corrected, ".", batch.Files)
		if err != nil {
			return erratumRun{}, err
		}
		batchTheirs, err := pilotDiagnostics(pilot, corrected, ".", batch.Files, opts.timeout)
		if err != nil {
			return erratumRun{}, err
		}
		for rel, diagnostics := range batchOurs {
			erratumOurs[rel] = diagnostics
		}
		for rel, diagnostics := range batchTheirs {
			erratumTheirs[rel] = diagnostics
		}
	}

	run := erratumRun{
		totals:  compareRoot(root.Name, root.Dir, files, erratumOurs, erratumTheirs).Totals,
		applied: errata.Count(applied),
	}
	for _, rel := range errata.SortedPaths(applied) {
		for _, entry := range applied[rel] {
			run.findings = append(run.findings, finding(entry, root.Name, rel, ours, theirs, erratumOurs, erratumTheirs))
		}
	}
	return run, nil
}

// coveredBy keeps the entries whose file this root actually walks: a root's
// directory can contain a nested corpus root the walker skips.
func coveredBy(applied map[string][]errata.Entry, files []string) map[string][]errata.Entry {
	walked := make(map[string]bool, len(files))
	for _, rel := range files {
		walked[rel] = true
	}
	out := make(map[string][]errata.Entry, len(applied))
	for rel, entries := range applied {
		if walked[rel] {
			out[rel] = entries
		}
	}
	return out
}

// finding states what the correction changed at its own line on both sides.
func finding(entry errata.Entry, rootName, rel string, ours, theirs, erratumOurs, erratumTheirs map[string][]diagnostic) ErrataFinding {
	f := ErrataFinding{
		ID: entry.ID, Root: rootName, Path: entry.Path, Line: entry.Line,
		OursPublished:  atLine(ours[rel], entry.Line),
		OursCorrected:  atLine(erratumOurs[rel], entry.Line),
		PilotPublished: atLine(theirs[rel], entry.Line),
		PilotCorrected: atLine(erratumTheirs[rel], entry.Line),
	}
	f.PilotVerdictNew = f.PilotPublished != f.PilotCorrected
	switch {
	case f.OursCorrected > 0:
		f.Note = "our diagnostic survives the correction"
	case f.PilotCorrected > 0:
		f.Note = "our diagnostic is cleared while the pilot still reports here: a finding, not a fix"
	case f.PilotPublished > 0:
		f.Note = "both diagnostics are cleared: the correction changes the pilot's verdict too"
	case f.OursPublished == 0:
		f.Note = "neither implementation reported here as published"
	default:
		f.Note = "our diagnostic is cleared and the pilot is silent on both texts"
	}
	return f
}

func atLine(diagnostics []diagnostic, line int) int {
	count := 0
	for _, d := range diagnostics {
		if d.Line == line {
			count++
		}
	}
	return count
}
