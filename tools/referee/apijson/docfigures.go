package apijson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// recordPath is the page adjudicating the differential; its figures are doc-counts
// blocks the documentation build renders from the baseline, never committed.
const recordPath = "docs/project/api-json-differential.md"

// The record's blocks: a sentence in the tree for the scorecard and the roots,
// the hand-written rows keyed by (metaclass, property) for the tables.
const (
	scorecardBlock        = "apijson-scorecard"
	rootsBlock            = "apijson-roots"
	presentDifferentBlock = "apijson-present-different"
	pilotMissingBlock     = "apijson-pilot-missing"
	launchReadyBlock      = "apijson-launch-ready-gaps"
)

// bucketColumns maps a figure column's heading to its index in a scorecard row.
var bucketColumns = map[string]int{
	"present-equal": 0, "present-different": 1, "ours-missing": 2, "pilot-missing": 3,
	"absent-both": 4, "indeterminate": 5, "pilot-nondeterministic": 6,
}

// tableBlock is a keyed table: committed rows carry the key and the written
// columns; figures render after the key, and a ranked table is sorted and numbered.
type tableBlock struct {
	name    string
	written []string
	figures []string
	ranked  string
}

var tableBlocks = []tableBlock{
	{name: presentDifferentBlock, written: []string{"metaclass", "property"}, figures: []string{"present-equal", "present-different"}},
	{name: pilotMissingBlock, written: []string{"metaclass", "property", "clause"}, figures: []string{"pilot-missing"}},
	{name: launchReadyBlock, written: []string{"metaclass", "property", "in-flight overlap", "derivation clause"}, figures: []string{"ours-missing", "present-different"}, ranked: "ours-missing"},
}

var (
	tableSeparatorPattern = regexp.MustCompile(`^\|(\s*:?-+:?\s*\|)+$`)
	keyCellPattern        = regexp.MustCompile("^`([A-Za-z]+)`$")
)

// readCommittedReport folds the committed directory into the report the
// monolithic baseline once stated: totals, roots and one scorecard.
func readCommittedReport(checkout string) (Report, error) {
	committed, err := readCommitted(checkout)
	if err != nil {
		return Report{}, err
	}
	report := Report{Provenance: committed.Provenance, Exporter: committed.Summary.Exporter, Normalizations: committed.Summary.Normalizations,
		PilotNondeterministic: committed.Summary.PilotNondeterministic, Totals: committed.Summary.Totals, Roots: committed.Summary.Roots}
	report.setScorecard(scorecardOf(committed.Fragments))
	return report, nil
}

// RenderSiteBlocks renders the record's blocks from the committed baseline and
// the record's own rows, by page then block name, as doc-counts -site-blocks does.
func RenderSiteBlocks(checkout string) (map[string]map[string]string, error) {
	report, err := readCommittedReport(checkout)
	if err != nil {
		return nil, err
	}
	record, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(recordPath))) // #nosec G304 -- the committed record
	if err != nil {
		return nil, err
	}
	blocks, err := renderBlocks(report, string(record))
	if err != nil {
		return nil, err
	}
	return map[string]map[string]string{recordPath: blocks}, nil
}

// renderBlocks renders every block of the record from the report; a sentence
// block stating a figure or a row the baseline has no verdict for is an error.
func renderBlocks(report Report, record string) (map[string]string, error) {
	blocks := map[string]string{}
	for _, name := range []string{scorecardBlock, rootsBlock} {
		body, err := committedBlock(record, name)
		if err != nil {
			return nil, err
		}
		if strings.ContainsAny(body, "0123456789") {
			return nil, fmt.Errorf("%s: the block named %q states a figure; the build renders it", recordPath, name)
		}
	}
	blocks[scorecardBlock] = scorecardLine(report.Totals)
	blocks[rootsBlock] = rootsTable(report.Roots)
	for _, spec := range tableBlocks {
		body, err := committedBlock(record, spec.name)
		if err != nil {
			return nil, err
		}
		rows, err := parseTable(spec, body)
		if err != nil {
			return nil, err
		}
		text, err := spec.render(report, rows)
		if err != nil {
			return nil, err
		}
		blocks[spec.name] = text
	}
	return blocks, nil
}

// committedBlock returns what the record holds between the block's markers.
func committedBlock(record, name string) (string, error) {
	begin, end := "<!-- doc-counts:begin "+name+" -->", "<!-- doc-counts:end "+name+" -->"
	if strings.Count(record, begin) != 1 || strings.Count(record, end) != 1 {
		return "", fmt.Errorf("%s: exactly one block named %q required", recordPath, name)
	}
	start, stop := strings.Index(record, begin)+len(begin), strings.Index(record, end)
	if stop < start {
		return "", fmt.Errorf("%s: the block named %q ends before it begins", recordPath, name)
	}
	return record[start:stop], nil
}

func scorecardLine(t Totals) string {
	return fmt.Sprintf("**Scorecard:** %d files; %d elements matched (%d ours-only, %d pilot-only, %d of them implied); properties: %d present-equal, %d present-different, %d ours-missing, %d pilot-missing, %d absent-both, %d indeterminate, %d pilot-nondeterministic.",
		t.Files, t.Elements.Matched, t.Elements.OursOnly, t.Elements.PilotOnly, t.Elements.PilotOnlyImplied,
		t.Properties.PresentEqual, t.Properties.PresentDifferent, t.Properties.OursMissing, t.Properties.PilotMissing, t.Properties.AbsentBoth, t.Properties.Indeterminate, t.Properties.PilotNondeterministic)
}

func rootsTable(roots []RootReport) string {
	lines := []string{
		"| root | files | matched | ours-only | pilot-only | present-equal | present-different | ours-missing | pilot-missing |",
		"|---|---:|---:|---:|---:|---:|---:|---:|---:|",
	}
	for _, r := range roots {
		lines = append(lines, fmt.Sprintf("| `%s` | %d | %d | %d | %d | %d | %d | %d | %d |", r.Dir, r.Files,
			r.Elements.Matched, r.Elements.OursOnly, r.Elements.PilotOnly,
			r.Properties.PresentEqual, r.Properties.PresentDifferent, r.Properties.OursMissing, r.Properties.PilotMissing))
	}
	return strings.Join(lines, "\n")
}

// tableRow is one committed row: its key and its cells as written.
type tableRow struct {
	metaclass, property string
	cells               []string
}

func (r tableRow) key() string { return r.metaclass + "." + r.property }

// parseTable reads the committed table of a block: the written header, a
// separator and the rows, each cell count that of the header.
func parseTable(spec tableBlock, body string) ([]tableRow, error) {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 2 {
		return nil, fmt.Errorf("%s: the block named %q holds no table", recordPath, spec.name)
	}
	header, err := splitCells(lines[0])
	if err != nil || strings.Join(header, "|") != strings.Join(spec.written, "|") {
		return nil, fmt.Errorf("%s: the block named %q wants the header | %s |, got %q", recordPath, spec.name, strings.Join(spec.written, " | "), lines[0])
	}
	if !tableSeparatorPattern.MatchString(lines[1]) {
		return nil, fmt.Errorf("%s: the block named %q wants a table separator after its header, got %q", recordPath, spec.name, lines[1])
	}
	var rows []tableRow
	for _, line := range lines[2:] {
		cells, err := splitCells(line)
		if err != nil || len(cells) != len(spec.written) {
			return nil, fmt.Errorf("%s: the block named %q wants %d cells per row, got %q", recordPath, spec.name, len(spec.written), line)
		}
		metaclass, property := keyCellPattern.FindStringSubmatch(cells[0]), keyCellPattern.FindStringSubmatch(cells[1])
		if metaclass == nil || property == nil {
			return nil, fmt.Errorf("%s: the block named %q wants a `metaclass` and `property` key, got %q", recordPath, spec.name, line)
		}
		rows = append(rows, tableRow{metaclass: metaclass[1], property: property[1], cells: cells})
	}
	return rows, nil
}

func splitCells(line string) ([]string, error) {
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil, fmt.Errorf("not a table row: %q", line)
	}
	cells := strings.Split(line[1:len(line)-1], "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells, nil
}

// render prints the rows with their figure columns filled from the report;
// a ranked table is sorted by its ranking column, then by key, and numbered.
func (spec tableBlock) render(report Report, rows []tableRow) (string, error) {
	counts := make(map[string][7]int, len(rows))
	for _, row := range rows {
		c, ok := report.Scorecard[row.key()]
		if !ok {
			return "", fmt.Errorf("%s: the block named %q lists (%s, %s), which has no row in the baseline", recordPath, spec.name, row.metaclass, row.property)
		}
		counts[row.key()] = c
	}
	if spec.ranked != "" {
		by := bucketColumns[spec.ranked]
		sort.SliceStable(rows, func(i, j int) bool {
			if d := counts[rows[i].key()][by] - counts[rows[j].key()][by]; d != 0 {
				return d > 0
			}
			if rows[i].metaclass != rows[j].metaclass {
				return rows[i].metaclass < rows[j].metaclass
			}
			return rows[i].property < rows[j].property
		})
	}
	header, separator := []string{}, []string{}
	if spec.ranked != "" {
		header, separator = append(header, "#"), append(separator, "---:")
	}
	header, separator = append(header, spec.written[:2]...), append(separator, "---", "---")
	for _, figure := range spec.figures {
		header, separator = append(header, figure), append(separator, "---:")
	}
	for range spec.written[2:] {
		separator = append(separator, "---")
	}
	header = append(header, spec.written[2:]...)
	lines := []string{"| " + strings.Join(header, " | ") + " |", "|" + strings.Join(separator, "|") + "|"}
	for i, row := range rows {
		cells := []string{}
		if spec.ranked != "" {
			cells = append(cells, fmt.Sprint(i+1))
		}
		cells = append(cells, row.cells[:2]...)
		for _, figure := range spec.figures {
			cells = append(cells, fmt.Sprint(counts[row.key()][bucketColumns[figure]]))
		}
		cells = append(cells, row.cells[2:]...)
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
	}
	return strings.Join(lines, "\n"), nil
}

// printSiteBlocks prints the record's rendered blocks for the documentation build.
func printSiteBlocks(checkout string, stdout, stderr io.Writer) int {
	rendered, err := RenderSiteBlocks(checkout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rendered); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", toolName, err)
		return 1
	}
	return 0
}
