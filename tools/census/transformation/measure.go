package transformation

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/referee/pssm"
)

// The corpora a scope token is measured over: the pinned PSSM suite and the
// migration fixtures committed under tests/ and conformance/.
const (
	pssmSuiteRel   = "build/pssm/PSSM_TestSuite.xmi"
	fixturesGlob   = "tests/migrate/testdata/xmi/*.xmi"
	vehicleFixture = "conformance/fixtures/vehicle.xmi"
)

// pssmRequireEnv turns an absent PSSM suite into a measurement failure, as CI sets it.
const pssmRequireEnv = "OPENSYSML_REQUIRE_PSSM_SUITE"

// fixtureFiles lists the fixture corpus: every migration XMI plus the
// conformance vehicle model, sorted for a stable record.
func fixtureFiles(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(fixturesGlob)))
	if err != nil {
		return nil, err
	}
	files := append(matches, filepath.Join(root, filepath.FromSlash(vehicleFixture)))
	sort.Strings(files)
	return files, nil
}

// countTokens counts each scope token over one XMI document: `uml:X` counts
// elements with xmi:type="uml:X" at any depth, `sysml:X` counts
// stereotype-application elements whose local name is X and whose namespace
// URI contains "SysML".
func countTokens(path string, tokens map[string]bool) (map[string]int, error) {
	f, err := os.Open(path) // #nosec G304 -- corpora are fixed repository paths or the located suite
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	counts := make(map[string]int, len(tokens))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Local == "type" && strings.Contains(a.Name.Space, "/XMI/") {
				key := a.Value
				if i := strings.LastIndex(key, " "); i >= 0 {
					key = key[i+1:]
				}
				if tokens[key] {
					counts[key]++
				}
			}
		}
		if strings.Contains(strings.ToLower(start.Name.Space), "sysml") {
			key := "sysml:" + start.Name.Local
			if tokens[key] {
				counts[key]++
			}
		}
	}
	return counts, nil
}

// measure recomputes every scope token the baseline's rows name over both
// corpora and records the corpora the counts came from.
func measure(root string, b *Baseline, out io.Writer) error {
	tokens := map[string]bool{}
	for _, m := range b.Mappings {
		for _, tok := range m.Scope {
			tokens[tok] = true
		}
	}
	fixtures, err := fixtureFiles(root)
	if err != nil {
		return err
	}
	counts := make(map[string]TokenCount, len(tokens))
	suite := filepath.Join(root, filepath.FromSlash(pssmSuiteRel))
	if _, err := os.Stat(suite); err != nil {
		return fmt.Errorf("the PSSM suite is needed to measure scope tokens: run ./scripts/download-pssm-suite.sh (%v)", err)
	}
	fresh, err := countTokens(suite, tokens)
	if err != nil {
		return err
	}
	for tok, n := range fresh {
		c := counts[tok]
		c.PSSM = n
		counts[tok] = c
	}
	for _, f := range fixtures {
		fresh, err := countTokens(f, tokens)
		if err != nil {
			return err
		}
		for tok, n := range fresh {
			c := counts[tok]
			c.Fixtures += n
			counts[tok] = c
		}
	}
	pin, err := pssm.ReadPin(root)
	if err != nil {
		return err
	}
	digest, err := pssm.Digest(suite)
	if err != nil {
		return err
	}
	b.Measurement = Measurement{
		Corpora: []Corpus{
			{Name: "pssm", Document: pin.Document, Digest: "sha256:" + digest},
			{Name: "fixtures", Files: len(fixtures)},
		},
		Counts: counts,
	}
	fmt.Fprintf(out, "transformation-census: measured %d scope tokens over the PSSM suite and %d fixtures\n", len(tokens), len(fixtures))
	return nil
}

// checkMeasurement recomputes the recorded counts when the PSSM suite is
// provisioned; an absent suite skips unless OPENSYSML_REQUIRE_PSSM_SUITE is set.
func checkMeasurement(root string, b *Baseline, out io.Writer) error {
	suite := filepath.Join(root, filepath.FromSlash(pssmSuiteRel))
	if _, err := os.Stat(suite); err != nil {
		if os.Getenv(pssmRequireEnv) == "1" {
			return fmt.Errorf("the PSSM suite is required (%s=1): run ./scripts/download-pssm-suite.sh", pssmRequireEnv)
		}
		fmt.Fprintf(out, "transformation-census: PSSM suite not provisioned at %s; skipping the measurement comparison\n", suite)
		return nil
	}
	fresh := *b
	if err := measure(root, &fresh, io.Discard); err != nil {
		return err
	}
	var problems []string
	if len(fresh.Measurement.Corpora) != len(b.Measurement.Corpora) {
		problems = append(problems, "the recorded corpora do not match a fresh measurement")
	} else {
		for i := range fresh.Measurement.Corpora {
			if fresh.Measurement.Corpora[i] != b.Measurement.Corpora[i] {
				problems = append(problems, fmt.Sprintf("corpus %s: recorded %+v, measured %+v",
					b.Measurement.Corpora[i].Name, b.Measurement.Corpora[i], fresh.Measurement.Corpora[i]))
			}
		}
	}
	for tok, recorded := range b.Measurement.Counts {
		if fresh.Measurement.Counts[tok] != recorded {
			problems = append(problems, fmt.Sprintf("token %s: recorded %+v, measured %+v", tok, recorded, fresh.Measurement.Counts[tok]))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s: the recorded measurement is stale (re-measure with -measure):\n  %s",
		baselinePath, strings.Join(problems, "\n  "))
}
