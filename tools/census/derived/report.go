package derived

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/errata"
	"github.com/Open-MBEE/OpenSysML/tools/referee/diff"
)

const (
	toolName = "derived-census"
	// CommittedBaseline is the recorded census, beside the rules it measures.
	CommittedBaseline = "internal/semantic/metamodel/derived_census_baseline.json"
	refreshCommand    = "go run -C tools ./cmd/derived-census -update"
	bridgeSource      = "scripts/pilot-json-exporter/ExportJSON.java"
	bridgeName        = "pilot-json-exporter"
	evaluatorSource   = "internal/semantic/metamodel"
	evaluatorName     = "metamodel-evaluator"
)

// manualExamples is the handbook's worked examples, measured as the api-json
// differential measures them.
var manualExamples = diff.Root{Name: "manual-examples", Dir: "docs/manual/examples"}

// roots are the corpus roots measured, in report order.
func roots() []diff.Root { return append(diff.Roots(), manualExamples) }

// rootNames lists the measured roots' names, in report order.
func rootNames() []string {
	names := make([]string, 0, len(roots()))
	for _, root := range roots() {
		names = append(names, root.Name)
	}
	return names
}

// Report is the census as written: the run's provenance, every root's counts,
// the totals over the roots and the rules the gate holds against.
type Report struct {
	Provenance baseline.Record `json:"provenance"`
	// Structure names what the evaluator read the elements through.
	Structure string `json:"structure"`
	// Policy states the deliberate differences the counts carry.
	Policy  []string              `json:"policy"`
	Roots   []RootReport          `json:"roots"`
	Totals  map[string]RuleCounts `json:"totals"`
	Summary Summary               `json:"summary"`
	// Silent lists the served rules that answer nothing on every carrier while
	// the pilot answers on at least one, over all roots; the gate's class.
	Silent []string `json:"silent"`
}

// RootReport is one root's census.
type RootReport struct {
	Name          string `json:"name"`
	Dir           string `json:"dir"`
	Files         int    `json:"files"`
	Rejected      int    `json:"rejected"`
	Elements      int    `json:"elements"`
	Implied       int    `json:"implied"`
	PilotElements int    `json:"pilotElements"`
	PilotImplied  int    `json:"pilotImplied"`
	// PilotDuplicates counts the copies the pilot wrote of elements counted once.
	PilotDuplicates int `json:"pilotDuplicates"`
	PilotFailed     int `json:"pilotFailed"`
	// Rules keys every served rule carried by at least one element on either side.
	Rules map[string]RuleCounts `json:"rules"`
}

// Summary classifies the served rules over all roots the way the census the
// gate grew out of did: answering somewhere, empty everywhere, never derived.
type Summary struct {
	Served int `json:"served"`
	// Answering rules answer a value on at least one element.
	Answering int `json:"answering"`
	// Empty rules are derived on at least one element and answer a value on none.
	Empty int `json:"empty"`
	// NotDerived rules are declined on every carrier.
	NotDerived int `json:"notDerived"`
	// Uncarried rules apply to no element of any root.
	Uncarried int `json:"uncarried"`
	// Silent is len(Report.Silent).
	Silent int `json:"silent"`
	// PilotAnswering rules answer a value on at least one of the pilot's elements.
	PilotAnswering int `json:"pilotAnswering"`
}

const structureRead = "metamodel.SymbolStructure (symbol-backed; metamodel.New over metamodel.NewSymbolStructure)"

var policy = []string{
	"Implied relationships are a deliberate difference: the bridge exports without materializing the relationships the pilot implies (isImplied is false on every element it writes), while the semantic model synthesizes the ones it implies and the walk counts them as carriers; each root's implied count records how many, and no detector compares the two populations.",
	"Library elements the pilot writes beside a file's own are not counted; our walk starts at each document root and never reaches them.",
	"The pilot writes an element in every file whose traversal reaches it; a root's elements are counted once by id.",
	"A property the pilot's payload leaves out is counted as not derived on its side; one it writes null or empty is counted as empty.",
}

// finish fills the totals, the summary and the silent list from the roots.
func (r *Report) finish() {
	r.Structure = structureRead
	r.Policy = policy
	r.Totals = map[string]RuleCounts{}
	for _, root := range r.Roots {
		for name, counts := range root.Rules {
			total := r.Totals[name]
			total.Ours.add(counts.Ours)
			total.Pilot.add(counts.Pilot)
			r.Totals[name] = total
		}
	}
	names := RuleNames()
	r.Summary = Summary{Served: len(names)}
	r.Silent = nil
	for _, name := range names {
		total, carried := r.Totals[name]
		switch {
		case !carried || total.Ours.Carriers == 0:
			r.Summary.Uncarried++
		case total.Ours.Value > 0:
			r.Summary.Answering++
		case total.Ours.Empty > 0:
			r.Summary.Empty++
		default:
			r.Summary.NotDerived++
		}
		if total.Pilot.Value > 0 {
			r.Summary.PilotAnswering++
		}
		if total.Silent() {
			r.Silent = append(r.Silent, name)
		}
	}
	sort.Strings(r.Silent)
	r.Summary.Silent = len(r.Silent)
}

// provenance identifies the run's inputs: the pin, the bridge, the evaluator
// linked into this executable (digested in built), the roots and the libraries.
func provenance(checkout, built, libraries string) (baseline.Record, error) {
	pin, err := baseline.ReadPin(checkout)
	if err != nil {
		return baseline.Record{}, err
	}
	bridgeDigest, err := baseline.DigestFile(filepath.Join(checkout, filepath.FromSlash(bridgeSource)))
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", bridgeSource, err)
	}
	evaluatorDigest, err := baseline.DigestPackage(filepath.Join(built, filepath.FromSlash(evaluatorSource)))
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", evaluatorSource, err)
	}
	overlay, err := errata.Load()
	if err != nil {
		return baseline.Record{}, err
	}
	record := baseline.Record{
		PilotTag:      pin.Tag,
		PilotCommit:   pin.Commit,
		PilotArtifact: pin.Artifact,
		Errata:        baseline.ErrataDigest(overlay.Entries()),
		Tools: []baseline.Tool{
			{Name: bridgeName, Source: bridgeSource, SourceDigest: bridgeDigest, Release: pin.Release()},
			{Name: evaluatorName, Source: evaluatorSource, SourceDigest: evaluatorDigest},
		},
	}
	for _, root := range roots() {
		files, err := diff.CollectFiles(checkout, root)
		if err != nil {
			return baseline.Record{}, err
		}
		if len(files) == 0 {
			continue
		}
		digest, err := baseline.DigestFiles(filepath.Join(checkout, filepath.FromSlash(root.Dir)), files)
		if err != nil {
			return baseline.Record{}, fmt.Errorf("digest %s: %w", root.Dir, err)
		}
		origin := baseline.OriginOurs
		if root.Pinned {
			origin = baseline.OriginPinned
		}
		record.Inputs = append(record.Inputs, baseline.Input{Name: root.Name, Dir: root.Dir, Origin: origin, Files: len(files), Digest: digest})
	}
	libraryRoot := diff.Root{Name: diff.LibrariesInput, Dir: diff.RelativeTo(checkout, libraries)}
	files, err := diff.CollectFiles(checkout, libraryRoot)
	if err != nil {
		return baseline.Record{}, err
	}
	if len(files) == 0 {
		return baseline.Record{}, fmt.Errorf("no .sysml or .kerml files under the library directory %s", libraries)
	}
	libraryDigest, err := baseline.DigestFiles(libraries, files)
	if err != nil {
		return baseline.Record{}, fmt.Errorf("digest %s: %w", libraries, err)
	}
	record.Inputs = append(record.Inputs, baseline.Input{Name: libraryRoot.Name, Dir: libraryRoot.Dir, Origin: baseline.OriginOurs, Files: len(files), Digest: libraryDigest})
	return record, nil
}
