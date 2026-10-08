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

// scopeSpec is one parsed scope token: the matched element plus an optional
// qualifier `[name]` (an attribute or a direct child element of that name, any
// value) or `[name=value]` (the attribute exactly).
type scopeSpec struct {
	token    string
	prefix   string
	name     string
	qualName string
	qualVal  string
	hasQual  bool
	hasVal   bool
}

// parseScopeSpec splits a validated token; callers run scopeToken first.
func parseScopeSpec(tok string) scopeSpec {
	spec := scopeSpec{token: tok}
	name := tok[strings.Index(tok, ":")+1:]
	if i := strings.Index(name, "["); i >= 0 {
		spec.hasQual = true
		qual := name[i+1 : len(name)-1]
		name = name[:i]
		if j := strings.Index(qual, "="); j >= 0 {
			spec.hasVal = true
			spec.qualName, spec.qualVal = qual[:j], qual[j+1:]
		} else {
			spec.qualName = qual
		}
	}
	spec.prefix, spec.name = tok[:strings.Index(tok, ":")], name
	return spec
}

// tokenFrame tracks one open element's candidate tokens: spec -> satisfied.
type tokenFrame map[*scopeSpec]bool

// countTokens counts each scope token over one XMI document: `uml:X` counts
// elements with xmi:type="uml:X" at any depth, `sysml:X` counts
// stereotype-application elements whose local name is X and whose namespace
// URI contains "SysML". A `[name]` qualifier is satisfied by an attribute or a
// direct child of that name (child presence is only known once the child's
// start is seen), `[name=value]` by the attribute alone; an element counts on
// its end tag so nested same-type elements cannot leak the flag.
func countTokens(path string, tokens map[string]bool) (map[string]int, error) {
	f, err := os.Open(path) // #nosec G304 -- corpora are fixed repository paths or the located suite
	if err != nil {
		return nil, err
	}
	defer f.Close()
	byKey := map[string][]*scopeSpec{}
	for tok := range tokens {
		spec := parseScopeSpec(tok)
		byKey[spec.prefix+":"+spec.name] = append(byKey[spec.prefix+":"+spec.name], &spec)
	}
	dec := xml.NewDecoder(f)
	counts := make(map[string]int, len(tokens))
	var stack []tokenFrame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) > 0 {
				for spec, ok := range stack[len(stack)-1] {
					if !ok && spec.hasQual && !spec.hasVal && t.Name.Local == spec.qualName {
						stack[len(stack)-1][spec] = true
					}
				}
			}
			frame := tokenFrame{}
			for _, a := range t.Attr {
				if a.Name.Local == "type" && strings.Contains(a.Name.Space, "/XMI/") {
					key := a.Value
					if i := strings.LastIndex(key, " "); i >= 0 {
						key = key[i+1:]
					}
					for _, spec := range byKey[key] {
						frame[spec] = qualifierSatisfied(spec, t)
					}
				}
			}
			if strings.Contains(strings.ToLower(t.Name.Space), "sysml") {
				for _, spec := range byKey["sysml:"+t.Name.Local] {
					frame[spec] = qualifierSatisfied(spec, t)
				}
			}
			stack = append(stack, frame)
		case xml.EndElement:
			frame := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for spec, ok := range frame {
				if ok {
					counts[spec.token]++
				}
			}
		}
	}
	return counts, nil
}

// qualifierSatisfied reports whether the element's own attributes satisfy the
// qualifier; a bare `[name]` may still be satisfied later by a direct child.
func qualifierSatisfied(s *scopeSpec, start xml.StartElement) bool {
	if !s.hasQual {
		return true
	}
	for _, a := range start.Attr {
		if a.Name.Local == s.qualName && (!s.hasVal || a.Value == s.qualVal) {
			return true
		}
	}
	return false
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
	// Every used token is recorded, even one absent from both corpora: validate
	// requires a count for every scope token, and a {0,0} is the honest count.
	counts := make(map[string]TokenCount, len(tokens))
	for tok := range tokens {
		counts[tok] = TokenCount{}
	}
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
