package diff

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sysideScope is recorded in every report that has a third column, because what
// SysIDE can and cannot be read as evidence for is not obvious from the numbers.
const sysideScope = "static checking only: parsing, name resolution, expression typing and the KerML/SysML validation rules. " +
	"SysIDE executes nothing, so it is not evidence for or against any behavioural rule (actions, state machines, " +
	"classifier behaviours, body-expression scope). It corroborates; it never adjudicates."

// SysideInfo is the third implementation's provenance and its aggregate
// three-way tallies. Absent from the report when SysIDE is not provisioned.
type SysideInfo struct {
	Validator string       `json:"validator"`
	Version   string       `json:"version"`
	Library   string       `json:"standardLibrary"`
	Pilot     string       `json:"pilotRelease"`
	Scope     string       `json:"scope"`
	Totals    SysideTotals `json:"totals"`
}

// SysideTotals counts compared tuples by which implementations reported them.
// The two-way totals are untouched by design: this is a second, independent
// partition of the same tuples, not a re-adjudication of the first.
type SysideTotals struct {
	Files                   int `json:"files"`
	FilesAgreeing           int `json:"filesAllThreeAgreeing"`
	Diagnostics             int `json:"sysideDiagnostics"`
	AllThree                int `json:"allThree"`
	WithOpenSysML           int `json:"withOpenSysMLAgainstPilot"`
	WithPilot               int `json:"withPilotAgainstOpenSysML"`
	SysideOnly              int `json:"sysideOnly"`
	OpenSysMLUncorroborated int `json:"openSysMLOnlyUncorroborated"`
	PilotUncorroborated     int `json:"pilotOnlyUncorroborated"`
	AgreedWithoutSyside     int `json:"openSysMLAndPilotOnly"`
}

// SysideFile is one file's three-way partition.
type SysideFile struct {
	Diagnostics int             `json:"sysideDiagnostics"`
	Entries     []ThreeWayEntry `json:"entries,omitempty"`
}

// ThreeWayEntry is a compared tuple plus which implementations reported it, so a
// reader can always see which tool said what.
type ThreeWayEntry struct {
	Line     int      `json:"line"`
	Severity string   `json:"severity"`
	Category Category `json:"category"`
	Count    int      `json:"count"`
	Sides    string   `json:"sides"`
	Examples []string `json:"examples,omitempty"`
}

// The side labels a ThreeWayEntry can carry.
const (
	sidesAll               = "opensysml+pilot+syside"
	sidesOpenSysMLSyside   = "opensysml+syside"
	sidesPilotSyside       = "pilot+syside"
	sidesSyside            = "syside"
	sidesOpenSysML         = "opensysml"
	sidesPilot             = "pilot"
	sidesOpenSysMLAndPilot = "opensysml+pilot"
)

// sysideRelease reads the pin the provisioning script recorded next to the
// launcher, so a report always names the SysIDE release and standard library it
// was produced with.
func sysideRelease(validator string) (version, library string, err error) {
	path := filepath.Join(filepath.Dir(validator), "syside-pin.txt")
	// #nosec G304 -- the launcher to run is named on the command line.
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read the SysIDE pin: %w", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch name {
		case "SYSIDE_VERSION":
			version = value
		case "SYSIDE_SPEC":
			library = value
		}
	}
	if version == "" || library == "" {
		return "", "", fmt.Errorf("%s does not pin SYSIDE_VERSION/SYSIDE_SPEC", path)
	}
	return version, library, nil
}

// sysideDiagnostics runs SysIDE over a root's files in one workspace, both
// languages together: SysIDE loads SysML and KerML into a single index, so
// splitting them would only measure the split.
func sysideDiagnostics(validator, repo, dir string, files []string, timeout time.Duration, log io.Writer) (map[string][]diagnostic, error) {
	root, err := filepath.Abs(filepath.Join(repo, dir))
	if err != nil {
		return nil, fmt.Errorf("resolve corpus root: %w", err)
	}

	byPath := make(map[string]string, len(files))
	args := []string{"--root", root}
	for _, rel := range files {
		byPath[rel] = rel
		args = append(args, filepath.Join(root, filepath.FromSlash(rel)))
	}

	out := make(map[string][]diagnostic, len(files))
	if err := runPilot(validator, args, byPath, out, timeout, categorizeSyside, log); err != nil {
		return nil, err
	}
	return out, nil
}

// categorizeSyside maps a SysIDE diagnostic to a category by the rule name the
// driver prints as a "[code] " prefix. Deliberately under-mapped: an
// unrecognised finding stays unmapped rather than manufacturing agreement.
func categorizeSyside(message string) Category {
	code, text := sysideCode(message)
	switch code {
	// Chevrotain's lexer and parser: our syntax pass's layer.
	case "parsing-error", "lexing-error":
		return CategorySyntax
	// Langium's linker failing to resolve a cross-reference.
	case "linking-error":
		return CategoryUnresolved
	}

	// SysIDE names a validation after the metamodel constraint it checks.
	if strings.Contains(code, "Multiplicity") {
		return CategoryMultiplicity
	}
	lower := strings.ToLower(text)
	switch {
	// The pilot's units rule: "must have a measurement reference unit".
	case strings.Contains(lower, "measurement reference"):
		return CategoryUnits
	// The metaclass-of-the-type rule the other two also report.
	case strings.Contains(lower, "must be typed by"):
		return CategoryKindMismatch
	}
	return CategoryUnmapped
}

func sysideCode(message string) (code, text string) {
	if !strings.HasPrefix(message, "[") {
		return "", message
	}
	end := strings.Index(message, "] ")
	if end < 0 {
		return "", message
	}
	return message[1:end], message[end+2:]
}

// attachSyside adds the third column to a finished two-way root report, leaving
// every two-way bucket and total exactly as the pilot comparison computed it.
func attachSyside(root *RootReport, files []string, ours, theirs, syside map[string][]diagnostic) {
	totals := &SysideTotals{}
	threeWay := make(map[string]SysideFile, len(files))
	for _, rel := range files {
		file := compareSysideFile(ours[rel], theirs[rel], syside[rel])
		totals.Files++
		totals.Diagnostics += file.Diagnostics
		totals.add(file)
		if len(file.Entries) > 0 {
			threeWay[rel] = file
		}
	}
	root.Syside = totals

	for i, file := range root.Files {
		if three, ok := threeWay[file.Path]; ok {
			root.Files[i].Syside = &three
			delete(threeWay, file.Path)
		}
	}
	// A file both implementations are silent on is omitted from the two-way
	// report; when SysIDE is not silent on it, it has to appear.
	for rel, three := range threeWay {
		file := three
		root.Files = append(root.Files, FileReport{Path: rel, Syside: &file})
	}
	sort.Slice(root.Files, func(i, j int) bool { return root.Files[i].Path < root.Files[j].Path })
}

func (t *SysideTotals) addRoot(root SysideTotals) {
	t.Files += root.Files
	t.FilesAgreeing += root.FilesAgreeing
	t.Diagnostics += root.Diagnostics
	t.AllThree += root.AllThree
	t.WithOpenSysML += root.WithOpenSysML
	t.WithPilot += root.WithPilot
	t.SysideOnly += root.SysideOnly
	t.OpenSysMLUncorroborated += root.OpenSysMLUncorroborated
	t.PilotUncorroborated += root.PilotUncorroborated
	t.AgreedWithoutSyside += root.AgreedWithoutSyside
}

func (t *SysideTotals) add(file SysideFile) {
	agreeing := true
	for _, e := range file.Entries {
		switch e.Sides {
		case sidesAll:
			t.AllThree += e.Count
		case sidesOpenSysMLSyside:
			t.WithOpenSysML += e.Count
		case sidesPilotSyside:
			t.WithPilot += e.Count
		case sidesSyside:
			t.SysideOnly += e.Count
		case sidesOpenSysML:
			t.OpenSysMLUncorroborated += e.Count
		case sidesPilot:
			t.PilotUncorroborated += e.Count
		case sidesOpenSysMLAndPilot:
			t.AgreedWithoutSyside += e.Count
		}
		if e.Sides != sidesAll {
			agreeing = false
		}
	}
	if agreeing {
		t.FilesAgreeing++
	}
}

// compareSysideFile partitions one file's tuples as multisets: what all three
// report, then what each pair has left over, then what one reports alone.
// Tuples include severity, so differing severities are not agreement.
func compareSysideFile(ours, theirs, syside []diagnostic) SysideFile {
	ourGroups, theirGroups, sysideGroups := group(ours), group(theirs), group(syside)
	file := SysideFile{Diagnostics: len(syside)}

	for _, k := range sortedKeys(ourGroups, theirGroups, sysideGroups) {
		mine, pilot, third := ourGroups[k], theirGroups[k], sysideGroups[k]

		all := min(len(mine), min(len(pilot), len(third)))
		file.entry(k, sidesAll, all, take(&mine, all), take(&pilot, all), take(&third, all))

		// At most one pair can have anything left: subtracting the minimum
		// leaves at least one of the three at zero.
		withOurs := min(len(mine), len(third))
		file.entry(k, sidesOpenSysMLSyside, withOurs, take(&mine, withOurs), nil, take(&third, withOurs))

		withPilot := min(len(pilot), len(third))
		file.entry(k, sidesPilotSyside, withPilot, nil, take(&pilot, withPilot), take(&third, withPilot))

		agreed := min(len(mine), len(pilot))
		file.entry(k, sidesOpenSysMLAndPilot, agreed, take(&mine, agreed), take(&pilot, agreed), nil)

		file.entry(k, sidesOpenSysML, len(mine), mine, nil, nil)
		file.entry(k, sidesPilot, len(pilot), nil, pilot, nil)
		file.entry(k, sidesSyside, len(third), nil, nil, third)
	}
	return file
}

func (f *SysideFile) entry(k key, sides string, count int, ours, theirs, syside []diagnostic) {
	if count <= 0 {
		return
	}
	e := ThreeWayEntry{Line: k.Line, Severity: k.Severity, Category: k.Category, Count: count, Sides: sides}
	for _, side := range []struct {
		name        string
		diagnostics []diagnostic
	}{{"opensysml", ours}, {"pilot", theirs}, {"syside", syside}} {
		for _, d := range side.diagnostics {
			e.Examples = append(e.Examples, side.name+": "+d.Message)
		}
	}
	f.Entries = append(f.Entries, e)
}

// take removes the first count diagnostics from the group and returns them, so
// each entry's examples are the diagnostics that entry actually accounts for.
func take(group *[]diagnostic, count int) []diagnostic {
	if count > len(*group) {
		count = len(*group)
	}
	taken := (*group)[:count]
	*group = (*group)[count:]
	return taken
}

// countSysideUnmapped adds the SysIDE messages no category rule claimed to the
// report's unmapped table.
func countSysideUnmapped(unmapped map[UnmappedRow]int, file SysideFile) {
	for _, e := range file.Entries {
		if e.Category != CategoryUnmapped {
			continue
		}
		for _, example := range e.Examples {
			if strings.HasPrefix(example, "syside: ") {
				countUnmapped(unmapped, []string{example})
			}
		}
	}
}

func stripSysideExamples(file *SysideFile) *SysideFile {
	if file == nil {
		return nil
	}
	stripped := SysideFile{Diagnostics: file.Diagnostics, Entries: make([]ThreeWayEntry, len(file.Entries))}
	for i, e := range file.Entries {
		e.Examples = nil
		stripped.Entries[i] = e
	}
	return &stripped
}

// writeSysideBucket renders one file's three-way entries, ordered by side so the
// corroborating and contradicting rows are read together.
func writeSysideBucket(b *strings.Builder, file *SysideFile) {
	if file == nil || len(file.Entries) == 0 {
		return
	}
	entries := make([]ThreeWayEntry, len(file.Entries))
	copy(entries, file.Entries)
	sort.SliceStable(entries, func(i, j int) bool { return sideRank(entries[i].Sides) < sideRank(entries[j].Sides) })

	b.WriteString("    syside (third implementation, corroboration only):\n")
	for _, e := range entries {
		fmt.Fprintf(b, "      line %-5d %-8s %-20s x%-4d %s\n", e.Line, e.Severity, e.Category, e.Count, e.Sides)
		for _, example := range e.Examples {
			fmt.Fprintf(b, "        %s\n", example)
		}
	}
}

func sideRank(sides string) int {
	for i, name := range []string{sidesAll, sidesOpenSysMLSyside, sidesPilotSyside, sidesSyside, sidesOpenSysMLAndPilot, sidesOpenSysML, sidesPilot} {
		if name == sides {
			return i
		}
	}
	return 7
}

func writeSysideTotals(b *strings.Builder, totals SysideTotals) {
	fmt.Fprintf(b, "  syside: %d diagnostic(s), %d of %d file(s) where all three agree exactly\n",
		totals.Diagnostics, totals.FilesAgreeing, totals.Files)
	fmt.Fprintf(b, "    all three %d | with us against the pilot %d | with the pilot against us %d | syside alone %d\n",
		totals.AllThree, totals.WithOpenSysML, totals.WithPilot, totals.SysideOnly)
	fmt.Fprintf(b, "    uncorroborated: ours %d | the pilot's %d | ours and the pilot's together %d\n",
		totals.OpenSysMLUncorroborated, totals.PilotUncorroborated, totals.AgreedWithoutSyside)
}
