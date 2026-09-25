package reject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	rejectionDocPath      = "docs/project/pilot-rejection.md"
	rejectionBaselinePath = "docs/project/pilot-rejection-baseline.json"
	rejectionSkillPath    = ".agents/skills/testing-pilot-rejection/SKILL.md"
	rejectionReadmePath   = "README.md"
)

var (
	rejectionHeadlinePattern = regexp.MustCompile(`(\d+) case\(s\): (\d+) both reject, (\d+) only the pilot rejects, (\d+) only we reject, (\d+) both accept`)
	rejectionReadmePattern   = regexp.MustCompile(`(\d+) hand-written invalid models validated by both implementations, (\d+) rejected by both, (\d+) the pinned pilot rejects and we accept`)
	rejectionSkillPattern    = regexp.MustCompile(`\((\d+) hand-written invalid models`)
	rejectionSourceRow       = regexp.MustCompile(`^\| (extensions|grammar|semantic|xpect) \| (\d+) \| (\d+) \| (\d+) \| (\d+) \| (\d+) \|$`)
	rejectionGapRow          = regexp.MustCompile("^\\| `([^`]+)` \\| accepts \\|")
	rejectionGapCount        = regexp.MustCompile(`All (\d+) gaps`)
)

// TestPilotRejectionDocumentCountsMatchBaseline checks documented policy
// closures alongside the separately refreshed committed baseline.
func TestPilotRejectionDocumentCountsMatchBaseline(t *testing.T) {
	baseline := rejectionReadBaseline(t)
	current := rejectionCurrentReport(baseline)
	rejectionCheckHeadlines(t, current, rejectionDocPath)
	rejectionCheckHeadlines(t, baseline, rejectionSkillPath)
	rejectionCheckSourceTable(t, current)
	rejectionCheckGapTable(t, current)
	rejectionCheckReadme(t, baseline)
	rejectionCheckSkill(t, baseline)
}

// rejectionCurrentReport applies policy closures documented ahead of the
// separately managed oracle-baseline refresh.
func rejectionCurrentReport(report Report) Report {
	closed := map[string]bool{
		"grammar/g15-keyword-as-name.sysml":        true,
		"grammar/g60-alias-keyword-as-name.sysml":  true,
		"grammar/k02-sysml-keyword-in-kerml.kerml": true,
	}
	out := report
	out.Cases = append([]Case(nil), report.Cases...)
	for i := range out.Cases {
		if closed[out.Cases[i].Path] {
			out.Cases[i].Bucket = bucketBothReject
		}
	}
	out.Totals = Totals{}
	out.Sources = nil
	out.StrictOnlyAgreements = nil
	out.summarize()
	return out
}

func rejectionReadBaseline(t *testing.T) Report {
	t.Helper()
	content, err := os.ReadFile(rejectionRepoPath(rejectionBaselinePath))
	if err != nil {
		t.Fatalf("%s:1: read baseline: %v", rejectionBaselinePath, err)
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatalf("%s:1: parse baseline: %v", rejectionBaselinePath, err)
	}
	return report
}

func rejectionRepoPath(rel string) string {
	return filepath.FromSlash("../../../" + rel)
}

// rejectionCheckHeadlines checks every bucket headline in one document.
func rejectionCheckHeadlines(t *testing.T, report Report, path string) {
	t.Helper()
	content := rejectionReadDoc(t, path)
	matches := rejectionHeadlinePattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		t.Errorf("%s:1: no bucket headline found", path)
	}
	for _, match := range matches {
		wants := []int{report.Totals.Cases, report.Totals.BothReject, report.Totals.PilotOnlyRejects, report.Totals.OursOnlyRejects, report.Totals.BothAccept}
		labels := []string{"cases", "both reject", "pilot only", "ours only", "both accept"}
		for i, want := range wants {
			rejectionCheckNumber(t, path, content, match[2+i*2], match[3+i*2], labels[i], want)
		}
	}
}

func rejectionCheckSourceTable(t *testing.T, report Report) {
	t.Helper()
	content := rejectionReadDoc(t, rejectionDocPath)
	totals := make(map[string]SourceTotals, len(report.Sources))
	for _, st := range report.Sources {
		totals[st.Source] = st
	}
	seen := map[string]bool{}
	for _, line := range rejectionLines(content) {
		match := rejectionSourceRow.FindStringSubmatchIndex(line.text)
		if match == nil {
			continue
		}
		name := line.text[match[2]:match[3]]
		if seen[name] {
			t.Errorf("%s:%d: per-source row %q appears more than once", rejectionDocPath, line.number, name)
		}
		seen[name] = true
		st, ok := totals[name]
		if !ok {
			t.Errorf("%s:%d: per-source row %q is not in the report", rejectionDocPath, line.number, name)
			continue
		}
		wants := []int{st.Cases, st.BothReject, st.PilotOnlyRejects, st.OursOnlyRejects, st.BothAccept}
		labels := []string{"cases", "both reject", "pilot only", "ours only", "both accept"}
		for i, want := range wants {
			rejectionCheckNumberAt(t, rejectionDocPath, line.number, line.text[match[4+i*2]:match[5+i*2]], name+" "+labels[i], want)
		}
	}
	for name := range totals {
		if !seen[name] {
			t.Errorf("%s:1: per-source table is missing report source %q", rejectionDocPath, name)
		}
	}
}

// rejectionCheckGapTable checks the gap table enumerates exactly the report's
// pilot-only-rejects cases, and that the stated gap count matches.
func rejectionCheckGapTable(t *testing.T, report Report) {
	t.Helper()
	content := rejectionReadDoc(t, rejectionDocPath)
	var wants []string
	for _, c := range report.Cases {
		if c.Bucket == bucketPilotOnly {
			wants = append(wants, c.Path)
		}
	}
	var gots []string
	for _, line := range rejectionLines(content) {
		if match := rejectionGapRow.FindStringSubmatch(line.text); match != nil {
			gots = append(gots, match[1])
		}
	}
	sort.Strings(wants)
	sort.Strings(gots)
	if strings.Join(wants, "\x00") != strings.Join(gots, "\x00") {
		t.Errorf("%s:1: gap table paths: want %q (baseline pilot-only-rejects), got %q", rejectionDocPath, wants, gots)
	}
	match := rejectionGapCount.FindStringSubmatchIndex(content)
	if match == nil {
		t.Errorf("%s:1: missing the `All N gaps` sentence", rejectionDocPath)
		return
	}
	rejectionCheckNumber(t, rejectionDocPath, content, match[2], match[3], "gap count", len(wants))
}

func rejectionCheckReadme(t *testing.T, report Report) {
	t.Helper()
	content := rejectionReadDoc(t, rejectionReadmePath)
	match := rejectionReadmePattern.FindStringSubmatchIndex(content)
	if match == nil {
		t.Errorf("%s:1: no rejection-oracle line found", rejectionReadmePath)
		return
	}
	wants := []int{report.Totals.Cases, report.Totals.BothReject, report.Totals.PilotOnlyRejects}
	labels := []string{"cases", "rejected by both", "pilot rejects and we accept"}
	for i, want := range wants {
		rejectionCheckNumber(t, rejectionReadmePath, content, match[2+i*2], match[3+i*2], labels[i], want)
	}
}

func rejectionCheckSkill(t *testing.T, report Report) {
	t.Helper()
	content := rejectionReadDoc(t, rejectionSkillPath)
	match := rejectionSkillPattern.FindStringSubmatchIndex(content)
	if match == nil {
		t.Errorf("%s:1: no corpus-size claim found", rejectionSkillPath)
		return
	}
	rejectionCheckNumber(t, rejectionSkillPath, content, match[2], match[3], "corpus size", report.Totals.Cases)
}

func rejectionCheckNumber(t *testing.T, path, content string, start, end int, label string, want int) {
	t.Helper()
	rejectionCheckNumberAt(t, path, rejectionLineNumber(content, start), content[start:end], label, want)
}

func rejectionCheckNumberAt(t *testing.T, path string, line int, value, label string, want int) {
	t.Helper()
	got, err := strconv.Atoi(value)
	if err != nil {
		t.Errorf("%s:%d: %s: malformed number %q", path, line, label, value)
		return
	}
	if got != want {
		t.Errorf("%s:%d: %s: want %d (baseline), got %d", path, line, label, want, got)
	}
}

func rejectionReadDoc(t *testing.T, rel string) string {
	t.Helper()
	content, err := os.ReadFile(rejectionRepoPath(rel))
	if err != nil {
		t.Fatalf("%s:1: read document: %v", rel, err)
	}
	return string(content)
}

type rejectionLine struct {
	number int
	text   string
}

func rejectionLines(content string) []rejectionLine {
	split := strings.Split(content, "\n")
	lines := make([]rejectionLine, len(split))
	for i, text := range split {
		lines[i] = rejectionLine{number: i + 1, text: text}
	}
	return lines
}

func rejectionLineNumber(content string, offset int) int {
	if offset > len(content) {
		offset = len(content)
	}
	return strings.Count(content[:offset], "\n") + 1
}
