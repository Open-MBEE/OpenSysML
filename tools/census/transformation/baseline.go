package transformation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	baselinePath  = "docs/project/sysml-v1-transformation-census-baseline.json"
	censusDocPath = "docs/project/sysml-v1-transformation-census.md"
)

// Census statuses as the baseline records them. The document shows each one
// with the marker spec-compliance.md uses for the same meaning.
const (
	StatusFaithful       = "faithful"
	StatusApproximate    = "approximate"
	StatusNotImplemented = "not-implemented"
	StatusDeliberate     = "deliberate"
	StatusKnownFailure   = "known-failure"
	StatusUnknown        = "unknown"
)

// statusMarkers maps each recorded status to the exact text of its status
// cell, in the order the summary line states them.
var statusMarkers = []struct {
	Status string
	Marker string
}{
	{StatusFaithful, "✅ faithful"},
	{StatusApproximate, "⚠️ approximate"},
	{StatusNotImplemented, "❌ not implemented"},
	{StatusDeliberate, "⛔ deliberate"},
	{StatusKnownFailure, "🚧 known failure"},
	{StatusUnknown, "❔ unknown"},
}

func markerFor(status string) (string, bool) {
	for _, m := range statusMarkers {
		if m.Status == status {
			return m.Marker, true
		}
	}
	return "", false
}

// backed reports whether a status claims the mapping exists in the migrator,
// which is what implementation and test cites must back.
func backed(status string) bool {
	return status == StatusFaithful || status == StatusApproximate || status == StatusKnownFailure
}

// scoped reports whether a status must name the measurement tokens whose
// frequency makes its absence or approximation visible.
func scoped(status string) bool {
	return status == StatusNotImplemented || status == StatusApproximate
}

// scopeToken is a measured corpus token: `uml:<Name>` or `sysml:<Name>`.
var scopeToken = regexp.MustCompile(`^(uml|sysml):[A-Za-z_]\w*$`)

// Baseline is the committed record: the pin and the counts the rows came from,
// the measured corpora, each mapping's extracted fields and verdict, and the
// migrator behaviours no OMG mapping class covers.
type Baseline struct {
	Source      Source        `json:"source"`
	Measurement Measurement   `json:"measurement"`
	Mappings    []Mapping     `json:"mappings"`
	Beyond      []BeyondEntry `json:"beyond"`
}

// BeyondEntry is one migrator behaviour that has no OMG mapping class to cite.
type BeyondEntry struct {
	Behaviour      string   `json:"behaviour"`
	Implementation []string `json:"implementation"`
	Tests          []string `json:"tests"`
}

// Source identifies the document the census enumerates and its dimensions.
type Source struct {
	Document string `json:"document"`
	Version  string `json:"version"`
	URL      string `json:"url"`
	Digest   string `json:"digest"`
	File     string `json:"file"`
	Packages int    `json:"packages"`
	Classes  int    `json:"classes"`
	Mappings int    `json:"mappings"`
	// OCLBodies counts OCL2.0 operation bodies (bodyCondition specifications);
	// OCLSpecifications counts every OCL2.0 specification, the rest of which
	// split into postconditions and owned rules as recorded.
	OCLBodies         int `json:"oclBodies"`
	OCLSpecifications int `json:"oclSpecifications"`
	OCLPostconditions int `json:"oclPostconditions"`
	OCLOwnedRules     int `json:"oclOwnedRules"`
}

// Measurement records the corpora the scope tokens were counted over and the
// per-corpus count of each token a row's scope names.
type Measurement struct {
	Corpora []Corpus              `json:"corpora"`
	Counts  map[string]TokenCount `json:"counts"`
}

// Corpus names one corpus a scope token is counted over.
type Corpus struct {
	Name     string `json:"name"`
	Document string `json:"document,omitempty"`
	Digest   string `json:"digest,omitempty"`
	Files    int    `json:"files,omitempty"`
}

// TokenCount is one scope token's element count per corpus.
type TokenCount struct {
	PSSM     int `json:"pssm"`
	Fixtures int `json:"fixtures"`
}

// Mapping is one OMG mapping class: what the XMI carries plus the adjudicated
// verdict fields (status, implementation, tests, reason, scope).
type Mapping struct {
	Name           string   `json:"name"`
	Package        string   `json:"package"`
	Qualified      string   `json:"qualified"`
	Abstract       bool     `json:"abstract"`
	From           string   `json:"from"`
	To             string   `json:"to"`
	Generals       []string `json:"generals"`
	Operations     []string `json:"operations"`
	OCL            string   `json:"ocl"`
	Status         string   `json:"status"`
	Implementation []string `json:"implementation"`
	Tests          []string `json:"tests"`
	Reason         string   `json:"reason"`
	Scope          []string `json:"scope"`
}

func loadBaseline(root string) (*Baseline, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return nil, err
	}
	var base Baseline
	if err := json.Unmarshal(content, &base); err != nil {
		return nil, fmt.Errorf("%s: %w", baselinePath, err)
	}
	return &base, nil
}

func writeBaseline(root string, base *Baseline) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(base); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(baselinePath)), buf.Bytes(), 0o644) // #nosec G306 -- documentation baseline
}

// validate checks the baseline on its own: the source block, sorted unique
// rows, the adjudication rules, and that the measurement records exactly the
// tokens the rows' scopes name.
func (b *Baseline) validate() error {
	if b.Source.Document == "" || !sha256Re.MatchString(strings.TrimPrefix(b.Source.Digest, "sha256:")) {
		return fmt.Errorf("%s: the source block must carry the document and its sha256 digest; re-record with -update", baselinePath)
	}
	seen := make(map[string]bool, len(b.Mappings))
	used := map[string]bool{}
	for i, m := range b.Mappings {
		if seen[m.Name] {
			return fmt.Errorf("%s: %s is listed twice", baselinePath, m.Name)
		}
		seen[m.Name] = true
		if i > 0 && (b.Mappings[i-1].Package > m.Package ||
			(b.Mappings[i-1].Package == m.Package && b.Mappings[i-1].Name > m.Name)) {
			return fmt.Errorf("%s: %s is out of order (the list is sorted by package, then name)", baselinePath, m.Name)
		}
		if _, ok := markerFor(m.Status); !ok {
			return fmt.Errorf("%s: %s has status %q", baselinePath, m.Name, m.Status)
		}
		if m.Status != StatusFaithful && strings.TrimSpace(m.Reason) == "" {
			return fmt.Errorf("%s: %s is %s but states no reason", baselinePath, m.Name, m.Status)
		}
		if backed(m.Status) && (len(m.Implementation) == 0 || len(m.Tests) == 0) {
			return fmt.Errorf("%s: %s is %s but names no implementation or test", baselinePath, m.Name, m.Status)
		}
		if scoped(m.Status) && len(m.Scope) == 0 {
			return fmt.Errorf("%s: %s is %s but names no scope token", baselinePath, m.Name, m.Status)
		}
		for _, tok := range m.Scope {
			if !scopeToken.MatchString(tok) {
				return fmt.Errorf("%s: %s scope %q is not uml:<Name> or sysml:<Name>", baselinePath, m.Name, tok)
			}
			used[tok] = true
		}
	}
	for tok := range used {
		if _, ok := b.Measurement.Counts[tok]; !ok {
			return fmt.Errorf("%s: scope token %s is used but not measured; run -measure", baselinePath, tok)
		}
	}
	for tok := range b.Measurement.Counts {
		if !used[tok] {
			return fmt.Errorf("%s: token %s is measured but no scope uses it; run -measure", baselinePath, tok)
		}
	}
	seenBehaviour := make(map[string]bool, len(b.Beyond))
	for _, e := range b.Beyond {
		if strings.TrimSpace(e.Behaviour) == "" {
			return fmt.Errorf("%s: a beyond entry names no behaviour", baselinePath)
		}
		if seenBehaviour[e.Behaviour] {
			return fmt.Errorf("%s: beyond entry %q is listed twice", baselinePath, e.Behaviour)
		}
		seenBehaviour[e.Behaviour] = true
		if len(e.Implementation) == 0 || len(e.Tests) == 0 {
			return fmt.Errorf("%s: beyond entry %q names no implementation or test", baselinePath, e.Behaviour)
		}
	}
	return nil
}

// counts tallies the statuses in the order the summary line states them.
func (b *Baseline) counts() map[string]int {
	c := make(map[string]int, len(statusMarkers))
	for _, m := range b.Mappings {
		c[m.Status]++
	}
	return c
}

// packageOrder lists each package in baseline order with its row count.
func (b *Baseline) packageOrder() []struct {
	Name string
	Rows []Mapping
} {
	var order []string
	rows := map[string][]Mapping{}
	for _, m := range b.Mappings {
		if _, ok := rows[m.Package]; !ok {
			order = append(order, m.Package)
		}
		rows[m.Package] = append(rows[m.Package], m)
	}
	var out []struct {
		Name string
		Rows []Mapping
	}
	for _, p := range order {
		out = append(out, struct {
			Name string
			Rows []Mapping
		}{p, rows[p]})
	}
	return out
}

// matchesExtracted compares the recorded extracted fields with a fresh
// extraction, naming every name only one side has and every drifted field.
func (b *Baseline) matchesExtracted(list []Mapping) error {
	recorded := make(map[string]Mapping, len(b.Mappings))
	for _, m := range b.Mappings {
		recorded[m.Name] = m
	}
	fresh := make(map[string]Mapping, len(list))
	var problems []string
	for _, e := range list {
		fresh[e.Name] = e
		m, ok := recorded[e.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s is in the model but not in the baseline", e.Name))
			continue
		}
		m.Implementation, m.Tests, m.Reason, m.Scope, m.Status = e.Implementation, e.Tests, e.Reason, e.Scope, e.Status
		if !equalMapping(m, e) {
			problems = append(problems, fmt.Sprintf("%s extracted fields drifted; re-record with -update", e.Name))
		}
	}
	for name := range recorded {
		if _, ok := fresh[name]; !ok {
			problems = append(problems, fmt.Sprintf("%s is in the baseline but not in the model", name))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s does not list what the pinned model contains (re-record with -update, then adjudicate every new name):\n  %s",
		baselinePath, strings.Join(problems, "\n  "))
}

// equalMapping compares the extracted fields only; adjudicated fields are the baseline's own.
func equalMapping(a, b Mapping) bool {
	return a.Name == b.Name && a.Package == b.Package && a.Qualified == b.Qualified &&
		a.Abstract == b.Abstract && a.From == b.From && a.To == b.To && a.OCL == b.OCL &&
		equalStrings(a.Generals, b.Generals) && equalStrings(a.Operations, b.Operations)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
