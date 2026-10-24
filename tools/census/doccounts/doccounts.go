// Package doccounts is the one census of the compliance map's status markers and
// the one statement of which documentation lines derive from the oracle baselines:
// the guard in tools/referee/diff checks those lines, tools/cmd/doc-counts rewrites them. The
// rule census itself is counted at documentation-build time (scripts/mkdocs_census.py)
// and is never written into a committed file.
package doccounts

import (
	"encoding/json"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// Paths of the compliance map and of the files carrying a derived line, relative
// to the repository root.
const (
	SpecCompliancePath = "docs/project/spec-compliance.md"
	// ValidationConstraintsPath is the census page whose table the manifest renders.
	ValidationConstraintsPath = "docs/project/validation-constraints.md"
	ReadmePath                = "README.md"
	ArchitecturePath          = "docs/internals/architecture.md"
)

// refereedBlockName names the generated block: the prose census the Markdown pages share.
const refereedBlockName = "refereed-figures"

// The blocks rendered from the constraint manifest: the census table of the
// validation-constraints page, the census summary the compliance map quotes,
// and the formal documents' coverage figures on the validation-constraints page.
const (
	censusTableBlockName   = "validation-census-table"
	censusSummaryBlockName = "validation-census-summary"
	specCoverageBlockName  = "validation-spec-coverage"
)

// statusMarkers are the row statuses the compliance map uses. '⚠' is matched
// without its variation selector, as the map writes both spellings.
var statusMarkers = []string{"✅", "🧩", "⚠", "❌", "⛔", "🚧"}

// RuleCounts is the census of the compliance map's rule rows: a row is one table
// row, its status the one marker in its table's Status column.
type RuleCounts struct {
	Total          int
	Faithful       int
	Extension      int
	Approximate    int
	NotImplemented int
	Deliberate     int
	KnownFailure   int
}

// RefereedCounts is the five-figure census read from the committed baselines.
type RefereedCounts struct {
	Files                  int
	FilesAgreeing          int
	OursOnly               int
	PilotOnly              int
	DeclaredErrors         int
	Silent                 int
	DeclaredAgree          int
	WordingOnly            int
	LocationOnly           int
	SeverityDiffers        int
	Elsewhere              int
	ScopeExact             int
	ScopeTotal             int
	RejectCases            int
	RejectPilotOnly        int
	RejectBoth             int
	RejectDefaultPilotOnly int
	RejectDefaultBoth      int
	RejectStrictOnly       int
	PilotTag               string
	PilotArtifact          string
	// Errata is the same census with the declared corrections applied. It is a
	// secondary diagnostic figure; the fields above stay the conformance ones.
	Errata ErrataCounts
}

// ErrataCounts is the errata-applied census, read from the same baselines'
// errata sections so the two figures cannot come from different runs.
type ErrataCounts struct {
	Registry        int
	Corrections     int
	Documented      int
	Files           int
	FilesAgreeing   int
	OursOnly        int
	PilotOnly       int
	Silent          int
	RejectCases     int
	RejectPilotOnly int
}

// differentialTotals is the differential's counts, shared by the as-published
// totals and the errata-applied ones.
type differentialTotals struct {
	Files         int `json:"files"`
	FilesAgreeing int `json:"filesFullyAgreeing"`
	OursOnly      int `json:"openSysMLOnly"`
	PilotOnly     int `json:"pilotOnly"`
}

// errataProvenance is the registry census every oracle baseline restates.
type errataProvenance struct {
	Registry    int `json:"registryEntries"`
	Corrections int `json:"corrections"`
	Documented  int `json:"documentedWithoutCorrection"`
}

type differentialBaseline struct {
	PilotRelease string             `json:"pilotRelease"`
	Totals       differentialTotals `json:"totals"`
	Errata       *struct {
		errataProvenance
		Totals differentialTotals `json:"totals"`
	} `json:"errata"`
}

// xpectKind is one assertion kind's counts, as published or with the errata.
type xpectKind struct {
	Kind            string `json:"kind"`
	Assertions      int    `json:"assertions"`
	Rows            int    `json:"rows"`
	Agree           int    `json:"agree"`
	WordingOnly     int    `json:"wordingOnly"`
	SameLocation    int    `json:"sameLocation"`
	SameLine        int    `json:"sameLine"`
	SeverityDiffers int    `json:"severityDiffers"`
	Elsewhere       int    `json:"elsewhereInFile"`
}

type xpectBaseline struct {
	Kinds  []xpectKind `json:"kinds"`
	Errata *struct {
		Kinds []xpectKind `json:"kinds"`
	} `json:"errata"`
}

type rejectionTotals struct {
	Cases            int `json:"cases"`
	BothReject       int `json:"bothReject"`
	PilotOnlyRejects int `json:"pilotOnlyRejects"`
}

type rejectionBaseline struct {
	Totals               rejectionTotals `json:"totals"`
	StrictOnlyAgreements []string        `json:"strictOnlyAgreements"`
	Errata               *struct {
		Totals rejectionTotals `json:"totals"`
	} `json:"errata"`
}

// ReadRefereedCounts reads and derives all five headline figures.
func ReadRefereedCounts(root string) (RefereedCounts, error) {
	var differential differentialBaseline
	if err := readJSON(root, "docs/project/pilot-differential-baseline.json", &differential); err != nil {
		return RefereedCounts{}, err
	}
	var xpect xpectBaseline
	if err := readJSON(root, "docs/project/pilot-xpect-baseline.json", &xpect); err != nil {
		return RefereedCounts{}, err
	}
	var rejection rejectionBaseline
	if err := readJSON(root, "docs/project/pilot-rejection-baseline.json", &rejection); err != nil {
		return RefereedCounts{}, err
	}
	pilotTag, pilotArtifact, err := parsePilotRelease(differential.PilotRelease)
	if err != nil {
		return RefereedCounts{}, fmt.Errorf("docs/project/pilot-differential-baseline.json: %w", err)
	}
	counts := RefereedCounts{
		Files:            differential.Totals.Files,
		FilesAgreeing:    differential.Totals.FilesAgreeing,
		OursOnly:         differential.Totals.OursOnly,
		PilotOnly:        differential.Totals.PilotOnly,
		RejectCases:      rejection.Totals.Cases,
		RejectPilotOnly:  rejection.Totals.PilotOnlyRejects,
		RejectBoth:       rejection.Totals.BothReject,
		RejectStrictOnly: len(rejection.StrictOnlyAgreements),
		PilotTag:         pilotTag,
		PilotArtifact:    pilotArtifact,
	}
	counts.RejectDefaultBoth = counts.RejectBoth - counts.RejectStrictOnly
	counts.RejectDefaultPilotOnly = counts.RejectPilotOnly + counts.RejectStrictOnly
	if counts.RejectDefaultBoth < 0 {
		return RefereedCounts{}, fmt.Errorf("docs/project/pilot-rejection-baseline.json: more strict-only agreements than agreements")
	}
	var foundErrors, foundScope bool
	for _, kind := range xpect.Kinds {
		switch kind.Kind {
		case "errors":
			foundErrors = true
			counts.DeclaredErrors = kind.Rows
			counts.DeclaredAgree = kind.Agree - kind.WordingOnly
			counts.WordingOnly = kind.WordingOnly
			counts.LocationOnly = kind.SameLine
			counts.SeverityDiffers = kind.SeverityDiffers
			counts.Elsewhere = kind.Elsewhere
			counts.Silent = kind.Rows - kind.Agree - kind.SameLocation - kind.SameLine - kind.SeverityDiffers - kind.Elsewhere
			if counts.DeclaredAgree < 0 || counts.Silent < 0 {
				return RefereedCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: errors agreements and tolerances exceed %d rows", kind.Rows)
			}
		case "scope":
			foundScope = true
			counts.ScopeExact = kind.Agree
			counts.ScopeTotal = kind.Assertions
		}
	}
	if !foundErrors || !foundScope {
		return RefereedCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: baseline states no errors or scope kind to derive the headline from")
	}
	if counts.Errata, err = readErrataCounts(differential, xpect, rejection); err != nil {
		return RefereedCounts{}, err
	}
	return counts, nil
}

// readErrataCounts derives the errata-applied census from the same baselines.
// A baseline without an errata section is stale: rerun the oracle.
func readErrataCounts(differential differentialBaseline, xpect xpectBaseline, rejection rejectionBaseline) (ErrataCounts, error) {
	if differential.Errata == nil {
		return ErrataCounts{}, fmt.Errorf("docs/project/pilot-differential-baseline.json: no errata section; rerun `go run -C tools ./cmd/pilot-diff`")
	}
	if xpect.Errata == nil {
		return ErrataCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: no errata section; rerun `go run -C tools ./cmd/pilot-xpect`")
	}
	if rejection.Errata == nil {
		return ErrataCounts{}, fmt.Errorf("docs/project/pilot-rejection-baseline.json: no errata section; rerun `go run -C tools ./cmd/pilot-reject`")
	}
	counts := ErrataCounts{
		Registry:        differential.Errata.Registry,
		Corrections:     differential.Errata.Corrections,
		Documented:      differential.Errata.Documented,
		Files:           differential.Errata.Totals.Files,
		FilesAgreeing:   differential.Errata.Totals.FilesAgreeing,
		OursOnly:        differential.Errata.Totals.OursOnly,
		PilotOnly:       differential.Errata.Totals.PilotOnly,
		RejectCases:     rejection.Errata.Totals.Cases,
		RejectPilotOnly: rejection.Errata.Totals.PilotOnlyRejects,
	}
	if counts.Registry != counts.Corrections+counts.Documented {
		return ErrataCounts{}, fmt.Errorf("docs/project/pilot-differential-baseline.json: %d errata entries are neither corrected nor documented-only", counts.Registry-counts.Corrections-counts.Documented)
	}
	for _, kind := range xpect.Errata.Kinds {
		if kind.Kind != "errors" {
			continue
		}
		counts.Silent = kind.Rows - kind.Agree - kind.SameLocation - kind.SameLine - kind.SeverityDiffers - kind.Elsewhere
		if counts.Silent < 0 {
			return ErrataCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: errata agreements and tolerances exceed %d rows", kind.Rows)
		}
		return counts, nil
	}
	return ErrataCounts{}, fmt.Errorf("docs/project/pilot-xpect-baseline.json: the errata section states no errors kind")
}

func readJSON(root, path string, into any) error {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) // #nosec G304 -- the path is a fixed baseline file under the requested repository root
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, into); err != nil {
		return fmt.Errorf("%s: parse baseline: %w", path, err)
	}
	return nil
}

func parsePilotRelease(release string) (string, string, error) {
	match := pilotReleasePattern.FindStringSubmatch(release)
	if match == nil {
		return "", "", fmt.Errorf("pilotRelease %q does not match `TAG (jupyter-sysml-kernel ARTIFACT)`", release)
	}
	return match[1], match[2], nil
}

// statusColumnHeading names the compliance tables' status column.
const statusColumnHeading = "Status"

// splitCells splits a table row into cells as the documentation site's Markdown
// renderer (Python-Markdown's tables extension) does: a pipe escaped with a
// backslash or inside a code span does not split.
func splitCells(row string) []string {
	row = strings.Trim(row, " ")
	row = strings.TrimPrefix(row, "|")
	row = trimEndBorder(row)
	pipes, tics := scanRow(row)
	spans := codeSpans(tics)
	var cells []string
	start := 0
	for _, pipe := range pipes {
		if inCodeSpan(pipe, spans) {
			continue
		}
		cells = append(cells, strings.TrimSpace(row[start:pipe]))
		start = pipe + 1
	}
	return append(cells, strings.TrimSpace(row[start:]))
}

// ticRun is a run of backticks in a row: its first and last offsets, whether a
// backslash escapes its first tick, and how many ticks it holds.
type ticRun struct{ start, end, escape, size int }

// scanRow finds the unescaped pipes and the backtick runs of row.
func scanRow(row string) (pipes []int, tics []ticRun) {
	for i := 0; i < len(row); {
		switch {
		case strings.HasPrefix(row[i:], `\\`), strings.HasPrefix(row[i:], `\|`):
			i += 2
		case strings.HasPrefix(row[i:], "\\`"):
			end := i + 1
			for end < len(row) && row[end] == '`' {
				end++
			}
			tics = append(tics, ticRun{start: i, end: end - 1, escape: 1, size: end - i - 1})
			i = end
		case row[i] == '`':
			end := i
			for end < len(row) && row[end] == '`' {
				end++
			}
			tics = append(tics, ticRun{start: i, end: end - 1, size: end - i})
			i = end
		case row[i] == '|':
			pipes = append(pipes, i)
			i++
		default:
			i++
		}
	}
	return pipes, tics
}

// codeSpans pairs each backtick run with the next run of its size, which
// closes the code span it opens; a run no later run matches is plain text.
func codeSpans(tics []ticRun) [][2]int {
	var spans [][2]int
	for pos := 0; pos < len(tics); {
		size := tics[pos].size - tics[pos].escape
		closer := -1
		if size != 0 {
			for j := pos + 1; j < len(tics); j++ {
				if tics[j].size == size {
					closer = j
					break
				}
			}
		}
		if closer < 0 {
			pos++
			continue
		}
		spans = append(spans, [2]int{tics[pos].start, tics[closer].end})
		pos = closer + 1
	}
	return spans
}

// inCodeSpan reports whether offset pos lies in one of the spans.
func inCodeSpan(pos int, spans [][2]int) bool {
	for _, span := range spans {
		if span[0] <= pos && pos <= span[1] {
			return true
		}
	}
	return false
}

// trimEndBorder drops a closing pipe that no odd run of backslashes escapes.
func trimEndBorder(row string) string {
	if !strings.HasSuffix(row, "|") {
		return row
	}
	backslashes := 0
	for i := len(row) - 2; i >= 0 && row[i] == '\\'; i-- {
		backslashes++
	}
	if backslashes%2 == 1 {
		return row
	}
	return row[:len(row)-1]
}

// statusColumn reports whether a blank-line-delimited block is a table and, if it
// is, the index of its Status column (-1 for a table without one).
func statusColumn(block []string) (column int, isTable bool) {
	if len(block) < 2 || !strings.HasPrefix(strings.Trim(block[0], " "), "|") {
		return 0, false
	}
	header, separator := splitCells(block[0]), splitCells(block[1])
	if len(header) < 2 || len(separator) != len(header) || strings.Trim(strings.Join(separator, ""), "|:- ") != "" {
		return 0, false
	}
	for i, cell := range header {
		if strings.Trim(cell, "* ") == statusColumnHeading {
			return i, true
		}
	}
	return -1, true
}

func carriesMarker(text string) bool {
	for _, marker := range statusMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// rowStatus is the one status marker in a rule row's Status column; a repeated
// marker is refused as two are, so the cell states its status once.
func rowStatus(number int, line string, column int) (string, error) {
	cells := splitCells(line)
	cell := ""
	if column < len(cells) {
		cell = cells[column]
	}
	var found []string
	for _, marker := range statusMarkers {
		for n := strings.Count(cell, marker); n > 0; n-- {
			found = append(found, marker)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("%s:%d: no status marker in the %s column", SpecCompliancePath, number, statusColumnHeading)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("%s:%d: %d status markers in the %s column (%s); state one", SpecCompliancePath, number, len(found), statusColumnHeading, strings.Join(found, ", "))
	}
}

// RuleRow is one rule row of the compliance map: its line number, its text and
// the one marker in its table's Status column.
type RuleRow struct {
	Line   int
	Text   string
	Marker string
}

// RuleRows lists the compliance map's rule rows in page order. A row stating no
// status or several, a row with a marker outside a table or in a table without
// a Status column, is an error.
func RuleRows(content string) ([]RuleRow, error) {
	var rows []RuleRow
	lines := strings.Split(content, "\n")
	for start := 0; start < len(lines); {
		end := start
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		block := lines[start:end]
		column, isTable := statusColumn(block)
		for offset, line := range block {
			number, text := start+offset+1, strings.TrimSpace(line)
			switch {
			case !isTable:
				if strings.HasPrefix(text, "|") && carriesMarker(text) {
					return nil, fmt.Errorf("%s:%d: a row with a status marker outside a table", SpecCompliancePath, number)
				}
			case column < 0:
				if offset >= 2 && carriesMarker(text) {
					return nil, fmt.Errorf("%s:%d: a row with a status marker in a table without a %s column", SpecCompliancePath, number, statusColumnHeading)
				}
			case offset >= 2:
				marker, err := rowStatus(number, line, column)
				if err != nil {
					return nil, err
				}
				rows = append(rows, RuleRow{Line: number, Text: line, Marker: marker})
			}
		}
		if end > start {
			start = end
		} else {
			start++
		}
	}
	return rows, nil
}

// CountRules counts the rule rows of the compliance map by the marker in their
// table's Status column, refusing the drifts RuleRows refuses.
func CountRules(content string) (RuleCounts, error) {
	rows, err := RuleRows(content)
	if err != nil {
		return RuleCounts{}, err
	}
	counts := RuleCounts{}
	for _, row := range rows {
		counts.add(row.Marker)
	}
	counts.Total = counts.Faithful + counts.Extension + counts.Approximate + counts.NotImplemented + counts.Deliberate + counts.KnownFailure
	return counts, nil
}

func (counts *RuleCounts) add(marker string) {
	switch marker {
	case "✅":
		counts.Faithful++
	case "🧩":
		counts.Extension++
	case "⚠":
		counts.Approximate++
	case "❌":
		counts.NotImplemented++
	case "⛔":
		counts.Deliberate++
	case "🚧":
		counts.KnownFailure++
	}
}

var (
	referenceLinePattern = regexp.MustCompile(`^\*\*Reference differential:\*\* ([0-9]+) files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation \(` + "`" + `([^` + "`" + `]+)` + "`" + `\), ([0-9]+) in full agreement;`)
	rejectionLinePattern = regexp.MustCompile(`^\*\*Rejection oracle:\*\* the reverse direction — do we reject what the reference rejects\? ([0-9]+) hand-written invalid models validated by both implementations, ([0-9]+) rejected by both, ([0-9]+) the pinned pilot rejects and we accept;`)
	pilotReleasePattern  = regexp.MustCompile(`^([^ ]+) \(jupyter-sysml-kernel ([^)]+)\)$`)
)

// BaselineLine is a line whose values come from the committed oracle baselines.
type BaselineLine struct {
	Path    string
	Marker  string
	Pattern *regexp.Regexp
	Values  func(RefereedCounts) []string
}

// BaselineLines lists the single-copy oracle lines regenerated from baselines.
func BaselineLines() []BaselineLine {
	return []BaselineLine{{
		Path:    ReadmePath,
		Marker:  "**Reference differential:**",
		Pattern: referenceLinePattern,
		Values: func(counts RefereedCounts) []string {
			return []string{strconv.Itoa(counts.Files), counts.PilotTag, strconv.Itoa(counts.FilesAgreeing)}
		},
	}, {
		Path:    ReadmePath,
		Marker:  "**Rejection oracle:**",
		Pattern: rejectionLinePattern,
		Values: func(counts RefereedCounts) []string {
			return []string{strconv.Itoa(counts.RejectCases), strconv.Itoa(counts.RejectBoth), strconv.Itoa(counts.RejectPilotOnly)}
		},
	}}
}

// Block describes a generated named block and its consumer-relative links. Name
// selects the template the block is rendered from; LinkPrefix is where that
// template's links to the conformance records resolve from this consumer.
//
// A block's markers either stand on lines of their own, around generated lines,
// or sit within one line around a generated span, so a figure can be generated
// in the middle of a sentence or a table cell.
type Block struct {
	Path       string
	Name       string
	LinkPrefix string
}

// Blocks lists every committed generated block and the page carrying it. Each
// moves only when a baseline, the library census or known_failures.txt is
// re-recorded, so a branch adding a test or fixture never rewrites one.
func Blocks() []Block {
	blocks := []Block{
		{Path: ReadmePath, Name: refereedBlockName, LinkPrefix: "docs/project/"},
		{Path: ArchitecturePath, Name: refereedBlockName, LinkPrefix: "../project/"},
		{Path: SpecCompliancePath, Name: libraryBlockName, LinkPrefix: ""},
		{Path: SpecCompliancePath, Name: censusSummaryBlockName, LinkPrefix: ""},
		{Path: ValidationConstraintsPath, Name: censusTableBlockName, LinkPrefix: ""},
		{Path: ValidationConstraintsPath, Name: specCoverageBlockName, LinkPrefix: ""},
	}
	return append(blocks, suiteBlocks()...)
}

// SiteBlocks lists the blocks the documentation build renders and git never
// carries a figure for: the test-suite figures, which move with every fixture
// and test. In the tree each block holds a sentence naming what is counted;
// CheckSiteBlock refuses one holding a digit.
func SiteBlocks() []Block {
	return siteSuiteBlocks()
}

// SitePaths lists the pages carrying a site block, in registration order.
func SitePaths() []string {
	var paths []string
	seen := map[string]bool{}
	for _, block := range SiteBlocks() {
		if !seen[block.Path] {
			seen[block.Path] = true
			paths = append(paths, block.Path)
		}
	}
	return paths
}

// RenderSiteBlocks renders every site block, by page and then by block name.
func RenderSiteBlocks(figures Figures) (map[string]map[string]string, error) {
	rendered := map[string]map[string]string{}
	for _, block := range SiteBlocks() {
		text, err := renderBlock(block, figures)
		if err != nil {
			return nil, err
		}
		if rendered[block.Path] == nil {
			rendered[block.Path] = map[string]string{}
		}
		rendered[block.Path][block.Name] = text
	}
	return rendered, nil
}

// Figures are everything the generated blocks are rendered from: the committed
// oracle baselines and library census, and the test-suite figures counted from the tree.
type Figures struct {
	Refereed RefereedCounts
	Library  fixtures.LibraryCensus
	Suite    SuiteCounts
	Manifest *manifest.Manifest
}

// ReadFigures reads the committed measurements and counts the tree under root.
func ReadFigures(root string) (Figures, error) {
	refereed, err := ReadRefereedCounts(root)
	if err != nil {
		return Figures{}, err
	}
	library, err := fixtures.ReadLibraryCensus(root)
	if err != nil {
		return Figures{}, err
	}
	suite, err := ReadSuiteCounts(root)
	if err != nil {
		return Figures{}, err
	}
	m, err := manifest.Load(root)
	if err != nil {
		return Figures{}, err
	}
	if err := m.Validate(); err != nil {
		return Figures{}, err
	}
	return Figures{Refereed: refereed, Library: library, Suite: suite, Manifest: m}, nil
}

// FindLine returns the index of the first line of content carrying the marker.
func FindLine(content, marker string) (int, bool) {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, marker) {
			return i, true
		}
	}
	return 0, false
}

// RewriteBaselineLine restates one baseline-derived line without other changes.
func RewriteBaselineLine(content string, spec BaselineLine, counts RefereedCounts) (string, error) {
	index, ok := FindLine(content, spec.Marker)
	if !ok {
		return "", fmt.Errorf("%s: no line carries %q", spec.Path, spec.Marker)
	}
	lines := strings.Split(content, "\n")
	match := spec.Pattern.FindStringSubmatchIndex(lines[index])
	if match == nil {
		return "", fmt.Errorf("%s:%d: line carrying %q does not match the derived-line pattern", spec.Path, index+1, spec.Marker)
	}
	values := spec.Values(counts)
	if got := len(match)/2 - 1; got != len(values) {
		return "", fmt.Errorf("%s:%d: line captures %d values, the baseline states %d", spec.Path, index+1, got, len(values))
	}
	rewritten := lines[index]
	for i := len(values) - 1; i >= 0; i-- {
		rewritten = rewritten[:match[2+i*2]] + values[i] + rewritten[match[3+i*2]:]
	}
	lines[index] = rewritten
	return strings.Join(lines, "\n"), nil
}

const (
	blockBeginFormat = "<!-- doc-counts:begin %s -->"
	blockEndFormat   = "<!-- doc-counts:end %s -->"
)

// blockSpan locates a named block in a page: its own lines from beginIndex to
// endIndex, or the one line at inlineIndex carrying both markers.
type blockSpan struct {
	lines                             []string
	beginIndex, endIndex, inlineIndex int
}

func locateBlock(content string, spec Block) (blockSpan, error) {
	begin := fmt.Sprintf(blockBeginFormat, spec.Name)
	end := fmt.Sprintf(blockEndFormat, spec.Name)
	span := blockSpan{lines: strings.Split(content, "\n"), beginIndex: -1, endIndex: -1, inlineIndex: -1}
	for i, line := range span.lines {
		switch strings.TrimSpace(line) {
		case begin:
			if span.beginIndex >= 0 {
				return span, fmt.Errorf("%s: duplicate %q marker", spec.Path, begin)
			}
			span.beginIndex = i
			continue
		case end:
			if span.endIndex >= 0 {
				return span, fmt.Errorf("%s: duplicate %q marker", spec.Path, end)
			}
			span.endIndex = i
			continue
		}
		if !strings.Contains(line, begin) && !strings.Contains(line, end) {
			continue
		}
		if span.inlineIndex >= 0 || strings.Count(line, begin) != 1 || strings.Count(line, end) != 1 {
			return span, fmt.Errorf("%s: duplicate markers of the block named %q", spec.Path, spec.Name)
		}
		if strings.Index(line, end) < strings.Index(line, begin) {
			return span, fmt.Errorf("%s:%d: the block named %q ends before it begins", spec.Path, i+1, spec.Name)
		}
		span.inlineIndex = i
	}
	if span.inlineIndex >= 0 && (span.beginIndex >= 0 || span.endIndex >= 0) {
		return span, fmt.Errorf("%s: duplicate markers of the block named %q", spec.Path, spec.Name)
	}
	if span.inlineIndex < 0 && (span.beginIndex < 0 || span.endIndex < 0 || span.endIndex <= span.beginIndex) {
		return span, fmt.Errorf("%s: named block %q is missing or unterminated", spec.Path, spec.Name)
	}
	return span, nil
}

// body is the text between the markers.
func (s blockSpan) body(spec Block) string {
	begin := fmt.Sprintf(blockBeginFormat, spec.Name)
	end := fmt.Sprintf(blockEndFormat, spec.Name)
	if s.inlineIndex >= 0 {
		line := s.lines[s.inlineIndex]
		return line[strings.Index(line, begin)+len(begin) : strings.Index(line, end)]
	}
	return strings.Join(s.lines[s.beginIndex+1:s.endIndex], "\n")
}

// CheckSiteBlock reports a site block that is malformed or that states a figure
// in the tree, where only the sentence naming what the build counts belongs.
func CheckSiteBlock(content string, spec Block) error {
	span, err := locateBlock(content, spec)
	if err != nil {
		return err
	}
	body := span.body(spec)
	if strings.ContainsAny(body, "0123456789") {
		return fmt.Errorf("%s: the block named %q states a figure (%q); the documentation build counts it, so the tree names only what is counted", spec.Path, spec.Name, strings.TrimSpace(body))
	}
	return nil
}

// RewriteBlock replaces a named generated block and preserves surrounding bytes.
func RewriteBlock(content string, spec Block, figures Figures) (string, error) {
	renderedBlock, err := renderBlock(spec, figures)
	if err != nil {
		return "", err
	}
	return ReplaceBlock(content, spec, renderedBlock)
}

// BlockBody returns the text between a named block's markers.
func BlockBody(content string, spec Block) (string, error) {
	span, err := locateBlock(content, spec)
	if err != nil {
		return "", err
	}
	return span.body(spec), nil
}

// ReplaceBlock puts body between a named block's markers and preserves every
// other byte; a body of several lines cannot sit within one line.
func ReplaceBlock(content string, spec Block, body string) (string, error) {
	begin := fmt.Sprintf(blockBeginFormat, spec.Name)
	end := fmt.Sprintf(blockEndFormat, spec.Name)
	span, err := locateBlock(content, spec)
	if err != nil {
		return "", err
	}
	lines, beginIndex, endIndex, inlineIndex := span.lines, span.beginIndex, span.endIndex, span.inlineIndex
	if inlineIndex >= 0 {
		if strings.Contains(body, "\n") {
			return "", fmt.Errorf("%s:%d: the block named %q renders several lines and cannot sit within one", spec.Path, inlineIndex+1, spec.Name)
		}
		line := lines[inlineIndex]
		from, to := strings.Index(line, begin), strings.Index(line, end)+len(end)
		lines[inlineIndex] = line[:from] + begin + body + end + line[to:]
		return strings.Join(lines, "\n"), nil
	}
	rendered := append([]string{begin}, strings.Split(strings.TrimSuffix(body, "\n"), "\n")...)
	rendered = append(rendered, end)
	updated := make([]string, 0, len(lines)-endIndex+beginIndex+len(rendered))
	updated = append(updated, lines[:beginIndex]...)
	updated = append(updated, rendered...)
	updated = append(updated, lines[endIndex+1:]...)
	return strings.Join(updated, "\n"), nil
}

// blockTemplateData is the committed figures and the suite figures plus the
// consumer's own link prefix.
type blockTemplateData struct {
	RefereedCounts
	Library fixtures.LibraryCensus
	Table   string
	Suite   suiteFigures
	// CensusTable, CensusSummary and SpecCoverage are rendered from the
	// constraint manifest.
	CensusTable   string
	CensusSummary string
	SpecCoverage  string
	Name          string
	LinkPrefix    string
}

// A template renders the text between a block's markers, without them.
const refereedBlockTemplateText = "**Measured against the pinned reference** (`PILOT_TAG={{.PilotTag}}`, artifact `{{.PilotArtifact}}`). Every number below is generated by `make docs-counts` from the committed baselines and gated; none of them is typed in by hand.\n\n" +
	"- **Corpus agreement:** {{.FilesAgreeing}} of {{.Files}} files agree diagnostic-by-diagnostic; {{.OursOnly}} diagnostics are ours alone and {{.PilotOnly}} the reference's alone, and the first number must be read by root: the aggregate includes candidate conformance differences, intentional execution-scope warnings on reference corpora and diagnostics from our own examples ([differential]({{.LinkPrefix}}pilot-differential.md), `go run -C tools ./cmd/pilot-diff`).\n" +
	"- **Declared-diagnostic silence:** of the {{.DeclaredErrors}} declared `errors` rows in the reference's own Xpect suites, we report nothing for {{.Silent}}. {{.DeclaredAgree}} we report word-for-word; {{.WordingOnly}} wording-only and {{.LocationOnly}} location-only differences are agreement in substance and are not counted as gaps; {{.SeverityDiffers}} more we report as a warning and {{.Elsewhere}} elsewhere in the file ([Xpect oracle]({{.LinkPrefix}}pilot-xpect.md), `go run -C tools ./cmd/pilot-xpect`).\n" +
	"- **Scope agreement:** {{.ScopeExact}} of {{.ScopeTotal}} declared scope assertions match exactly (same source).\n" +
	"- **Permissiveness gaps:** of {{.RejectCases}} invalid models we wrote ourselves, the reference rejects {{.RejectDefaultPilotOnly}} that we accept by default, and {{.RejectDefaultBoth}} both reject; {{.RejectStrictOnly}} further cases agree only when we are asked strictly. We authored every one of these cases ourselves, so the denominator measures the reach of our own corpus and not our conformance; agreement reached only under an opt-in strict mode is weaker evidence than agreement by default ([rejection oracle]({{.LinkPrefix}}pilot-rejection.md), `go run -C tools ./cmd/pilot-reject`).\n" +
	"- **Declared errata:** the registry declares {{.Errata.Registry}} defect(s) in the published reference material — {{.Errata.Corrections}} with a specification-derived correction, {{.Errata.Documented}} documented without one, since no intended reading can be inferred ([OMG issues]({{.LinkPrefix}}omg-issues.md), `tools/oracle/errata`). Every figure above is as published and stays the conformance statement; running the same oracles over the corrected text instead reports {{.Errata.FilesAgreeing}} of {{.Errata.Files}} files agreeing, {{.Errata.OursOnly}} diagnostics ours alone and {{.Errata.PilotOnly}} the reference's alone, {{.Errata.Silent}} declared rows we are silent on, and {{.Errata.RejectPilotOnly}} of {{.Errata.RejectCases}} authored cases the reference alone rejects. The corrected figures are diagnostic only: an erratum never reclassifies a divergence category, and the published corpus is never edited.\n" +
	"- **Self-assessed surface:** the action, state-machine and classifier-behavior rows have no external referee at all — the four refereed figures above cannot see them, because the pinned artifact evaluates expressions but executes neither actions nor state machines. [Spec compliance]({{.LinkPrefix}}spec-compliance.md) counts them.\n\n" +
	"What these numbers cannot show: the OMG corpora are demonstrations rather than an official conformance suite; the differential is one-directional, comparing the diagnostics the two implementations report on the same files; the Xpect suites are the pilot authors' test intent rather than a certification oracle; and none of these is a percentage of the specification — no global compliance figure is claimed anywhere.\n\n" +
	"**Row bookkeeping:** the ✅/⚠️/❌/⛔ status of each tracked rule stays in [spec compliance]({{.LinkPrefix}}spec-compliance.md) as a census of our own row list, counted when the documentation site is built rather than committed. It moves when rows are rewritten and does not move when an oracle does, so it is not the progress measure."

// blockTemplates is the one template per generated block name. A block naming no
// template is reported rather than written, so a consumer cannot be added without one.
var blockTemplates = parseBlockTemplates(map[string]string{
	refereedBlockName:      refereedBlockTemplateText,
	libraryBlockName:       libraryBlockTemplateText,
	censusTableBlockName:   "{{.CensusTable}}",
	censusSummaryBlockName: censusSummaryBlockTemplateText,
	specCoverageBlockName:  "{{.SpecCoverage}}",
}, suiteBlockTemplateTexts)

// censusSummaryBlockTemplateText restates the pilot-constraint census where the
// compliance map quotes it; the rows themselves are on the validation page.
const censusSummaryBlockTemplateText = "{{.CensusSummary}} Each row, its implementation, diagnostic code and probe are recorded in [`constraint-manifest.json`]({{.LinkPrefix}}constraint-manifest.json) and rendered in [validation constraints]({{.LinkPrefix}}validation-constraints.md)."

func parseBlockTemplates(texts ...map[string]string) map[string]*template.Template {
	parsed := map[string]*template.Template{}
	for _, group := range texts {
		for name, text := range group {
			parsed[name] = template.Must(template.New(name).Parse(text))
		}
	}
	return parsed
}

func renderBlock(spec Block, figures Figures) (string, error) {
	blockTemplate, ok := blockTemplates[spec.Name]
	if !ok {
		return "", fmt.Errorf("%s: no template renders the block named %q", spec.Path, spec.Name)
	}
	data := blockTemplateData{
		RefereedCounts: figures.Refereed,
		Library:        figures.Library,
		Table:          libraryTable(figures.Library),
		Suite:          figuresOf(figures.Suite),
		Name:           spec.Name,
		LinkPrefix:     spec.LinkPrefix,
	}
	if figures.Manifest != nil {
		data.CensusTable = figures.Manifest.RenderTable()
		data.CensusSummary = figures.Manifest.Summary()
		data.SpecCoverage = figures.Manifest.RenderCoverage()
	}
	var rendered strings.Builder
	if err := blockTemplate.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render %s: %w", spec.Name, err)
	}
	return rendered.String(), nil
}
