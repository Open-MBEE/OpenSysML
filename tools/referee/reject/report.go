package reject

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// Case is one negative model's adjudication. The error messages are kept only
// for the disagreeing buckets, where they are the evidence.
type Case struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Rule   string `json:"rule"`
	// Mode is the conformance mode our verdict was taken under.
	Mode        string `json:"mode"`
	Bucket      string `json:"bucket"`
	OursErrors  int    `json:"oursErrors"`
	PilotErrors int    `json:"pilotErrors"`
	// DefaultBucket and DefaultErrors are what the default mode says, recorded
	// only for a case asked strictly: agreement there is not agreement here.
	DefaultBucket string   `json:"defaultBucket,omitempty"`
	DefaultErrors int      `json:"defaultErrors,omitempty"`
	Ours          []string `json:"ours,omitempty"`
	Pilot         []string `json:"pilot,omitempty"`
}

// Totals is one scope's bucket counts. PilotOnlyRejects are the permissiveness
// gaps this harness exists to surface.
type Totals struct {
	Cases            int `json:"cases"`
	BothReject       int `json:"bothReject"`
	PilotOnlyRejects int `json:"pilotOnlyRejects"`
	OursOnlyRejects  int `json:"oursOnlyRejects"`
	BothAccept       int `json:"bothAccept"`
}

// SourceTotals is the bucket counts for one derivation source of the corpus.
type SourceTotals struct {
	Source string `json:"source"`
	Totals
}

// Report is the whole run. It carries no timestamp and no absolute path, so
// two runs over the same pin produce byte-identical output.
type Report struct {
	Pilot     string `json:"pilot"`
	Validator string `json:"validator"`
	Corpus    string `json:"corpus"`
	// Conformance is the policy our verdicts were taken under.
	Conformance string `json:"conformance"`
	// Provenance identifies the pin, bridges and corpus this run measured, so a
	// committed baseline can be checked without the validators.
	Provenance baseline.Record `json:"provenance"`
	Totals     Totals          `json:"totals"`
	// StrictOnlyAgreements are the cases both sides reject only because ours was
	// asked strictly: the default mode accepts them, by design.
	StrictOnlyAgreements []string       `json:"strictOnlyAgreements,omitempty"`
	Sources              []SourceTotals `json:"sources"`
	Cases                []Case         `json:"cases"`
	// Errata is the same run with the declared corrections applied; Totals
	// above stays the as-published headline.
	Errata *ErrataReport `json:"errata,omitempty"`
}

func (t *Totals) count(bucket string) {
	t.Cases++
	switch bucket {
	case bucketBothReject:
		t.BothReject++
	case bucketPilotOnly:
		t.PilotOnlyRejects++
	case bucketOursOnly:
		t.OursOnlyRejects++
	case bucketBothAccept:
		t.BothAccept++
	}
}

// summarize aggregates the totals and per-source totals from the cases.
func (r *Report) summarize() {
	bySource := map[string]*SourceTotals{}
	var names []string
	for _, c := range r.Cases {
		r.Totals.count(c.Bucket)
		if c.Bucket == bucketBothReject && c.DefaultBucket == bucketPilotOnly {
			r.StrictOnlyAgreements = append(r.StrictOnlyAgreements, c.Path)
		}
		st := bySource[c.Source]
		if st == nil {
			st = &SourceTotals{Source: c.Source}
			bySource[c.Source] = st
			names = append(names, c.Source)
		}
		st.count(c.Bucket)
	}
	sort.Strings(names)
	for _, name := range names {
		r.Sources = append(r.Sources, *bySource[name])
	}
}

func writeReports(dir string, report *Report) ([]byte, error) {
	files, err := reports.Open(dir, "pilot-reject")
	if err != nil {
		return nil, err
	}
	encoded, err := files.JSON(report)
	if err != nil {
		return nil, err
	}
	if err := files.Text(renderText(report)); err != nil {
		return nil, err
	}
	files.Announce(os.Stderr, headline(report.Totals))
	return encoded, nil
}

func headline(t Totals) string {
	return fmt.Sprintf("%d case(s): %d both reject, %d only the pilot rejects, %d only we reject, %d both accept",
		t.Cases, t.BothReject, t.PilotOnlyRejects, t.OursOnlyRejects, t.BothAccept)
}

// writeErrata states the second figure after the headline, never instead of it.
func writeErrata(b *strings.Builder, report *ErrataReport) {
	if report == nil {
		return
	}
	fmt.Fprintf(b, "\ndeclared errata: %d entr(ies), %d correction(s), %d documented without one; %d applied here\n",
		report.Registry, report.Corrections, report.Documented, report.Applied)
	for _, entry := range report.Entries {
		shape := "documented, no substitution"
		if entry.Corrected {
			shape = "corrected"
		}
		fmt.Fprintf(b, "  %s %s:%d (%s) — %s\n", entry.ID, entry.Path, entry.Line, entry.Citation, shape)
	}
	fmt.Fprintf(b, "  %s\n", report.Note)
	fmt.Fprintf(b, "TOTAL with errata applied: %s\n", headline(report.Totals))
	for _, change := range report.VerdictChanges {
		fmt.Fprintf(b, "  %s: %s -> %s\n", change.Path, change.Published, change.Corrected)
	}
}

func renderText(report *Report) string {
	var b strings.Builder
	b.WriteString("OpenSysML vs the OMG pilot implementation over a hand-written negative corpus\n")
	fmt.Fprintf(&b, "pilot pin: %s\nvalidator: %s\ncorpus:    %s\nour mode:  %s\n\n",
		report.Pilot, report.Validator, report.Corpus, report.Conformance)
	fmt.Fprintf(&b, "TOTAL: %s\n", headline(report.Totals))
	fmt.Fprintf(&b, "  of which %d agree only because we were asked strictly (the default mode accepts them, by design)\n",
		len(report.StrictOnlyAgreements))
	fmt.Fprintf(&b, "  %-12s %6s %11s %10s %9s %11s\n", "source", "cases", "both-reject", "pilot-only", "ours-only", "both-accept")
	for _, st := range report.Sources {
		fmt.Fprintf(&b, "  %-12s %6d %11d %10d %9d %11d\n",
			st.Source, st.Cases, st.BothReject, st.PilotOnlyRejects, st.OursOnlyRejects, st.BothAccept)
	}
	writeErrata(&b, report.Errata)

	for _, c := range report.Cases {
		fmt.Fprintf(&b, "\n%s\n", c.Path)
		fmt.Fprintf(&b, "  rule:   %s\n", c.Rule)
		fmt.Fprintf(&b, "  bucket: %s (ours %d error(s) in %s mode, pilot %d error(s))\n",
			c.Bucket, c.OursErrors, c.Mode, c.PilotErrors)
		if c.DefaultBucket != "" {
			fmt.Fprintf(&b, "  default mode: %s (ours %d error(s))\n", c.DefaultBucket, c.DefaultErrors)
		}
		for _, msg := range c.Pilot {
			fmt.Fprintf(&b, "  pilot: %s\n", msg)
		}
		for _, msg := range c.Ours {
			fmt.Fprintf(&b, "  ours:  %s\n", msg)
		}
	}
	return b.String()
}
