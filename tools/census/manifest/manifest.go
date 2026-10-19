// Package manifest reads and renders the constraint manifest, the single
// machine-readable record of every normative validation constraint the pinned
// pilot names: its spec identity, implementation, diagnostic codes, probe
// evidence and census status, plus the errata and the passes beyond the spec.
// Beside the pilot's rows it records every check*, derive* and validate*
// constraint the pinned formal documents state and the pilot does not name,
// as spec-sourced rows outside the census counts.
package manifest

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

// Path is the manifest's repository-relative path.
const Path = "docs/project/constraint-manifest.json"

// SchemaVersion is the shape this package reads; it moves with the shape.
const SchemaVersion = 1

// The census statuses, as the manifest records them.
const (
	StatusFaithful       = "faithful"
	StatusApproximate    = "approximate"
	StatusNotImplemented = "not-implemented"
	StatusDeliberate     = "deliberate"
	StatusKnownFailure   = "known-failure"
	StatusUnknown        = "unknown"
)

// StatusMarkers maps each status to the exact text of its status cell, in the
// order the census summary states them.
var StatusMarkers = []struct {
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

// MarkerFor returns the status cell text of a recorded status.
func MarkerFor(status string) (string, bool) {
	for _, m := range StatusMarkers {
		if m.Status == status {
			return m.Marker, true
		}
	}
	return "", false
}

// Implemented reports whether a status claims OpenSysML reports the constraint.
func Implemented(status string) bool {
	return status == StatusFaithful || status == StatusApproximate
}

// LanguageBySource names the census table's language column per validator source.
var LanguageBySource = map[string]string{"kerml": "KerML", "sysml": "SysML", "both": "both"}

// SourceSpec is the source of a row the formal documents state and no pilot
// validator names; such rows stay outside the pilot census.
const SourceSpec = "spec"

// Kinds are the constraint families of the formal documents, in the order the
// coverage figures state them.
var Kinds = []string{"check", "derive", "validate"}

// Manifest is the committed record. The pilot provenance and the extraction
// record are what the census tool re-extracts from the pinned jar.
type Manifest struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	PilotTag       string                   `json:"pilotTag"`
	PilotCommit    string                   `json:"pilotCommit"`
	PilotArtifact  string                   `json:"pilotArtifact"`
	Jar            JarRecord                `json:"jar"`
	Extraction     ExtractionRecord         `json:"extraction"`
	Recorded       string                   `json:"recorded,omitempty"`
	Specifications map[string]Specification `json:"specifications"`
	// FormalExtraction is how the spec-sourced rows were read from the documents.
	FormalExtraction *FormalExtractionRecord `json:"formalExtraction,omitempty"`
	Constraints      []Constraint            `json:"constraints"`
	Extensions       []Extension             `json:"extensions"`
	Errata           []Erratum               `json:"errata"`
	SpecIssues       []SpecIssue             `json:"specIssues"`
}

// JarRecord is the pinned pilot artifact the names were read from.
type JarRecord struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// ExtractionRecord is how the names were read from the jar.
type ExtractionRecord struct {
	Classes []string `json:"classes"`
	Method  string   `json:"method"`
}

// FormalExtractionRecord is how the spec-sourced rows were read from the pinned
// documents: the extraction program's method and the number of constraints each
// document states, pilot-named rows included.
type FormalExtractionRecord struct {
	Method      string         `json:"method"`
	Constraints map[string]int `json:"constraints"`
}

// Specification is one pinned OMG document the clauses cite.
type Specification struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Document string `json:"document"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
}

// Constraint is one row: a validation constraint as the pilot names it, or a
// constraint of the formal documents the pilot does not name (source "spec").
type Constraint struct {
	Name string `json:"name"`
	// Raw is the pilot's compiled identifier when it differs from Name.
	Raw string `json:"raw,omitempty"`
	// Source is the validator declaring it, kerml, sysml or both, or spec for a
	// row only the formal documents state.
	Source string `json:"source"`
	// Kind is the spec's constraint family: validate, check or derive.
	Kind      string `json:"kind"`
	Metaclass string `json:"metaclass"`
	// Spec cites the pinned document's clause stating the constraint, and the
	// document's own name for it when the document states it — the pilot's name,
	// except where the pilot renames or misspells it.
	Spec *SpecRef `json:"spec,omitempty"`
	// Checks, Implementation, Message and NegativeCase are the census table's
	// cells, in Markdown; a spec-sourced row's Checks is the document's prose.
	Checks string `json:"checks"`
	// OCL is the formal document's OCL for a spec-sourced row, verbatim.
	OCL            string `json:"ocl,omitempty"`
	Implementation string `json:"implementation"`
	Message        string `json:"message"`
	NegativeCase   string `json:"negativeCase"`
	// Codes and Passes are the diagnostic codes and the pass identifiers the
	// probes observe for this constraint.
	Codes  []string `json:"codes"`
	Passes []string `json:"passes"`
	Status string   `json:"status"`
	// Rationale is required for every status but faithful.
	Rationale string `json:"rationale,omitempty"`
}

// SpecRef locates a constraint in a pinned specification.
type SpecRef struct {
	Document string `json:"document"`
	Clause   string `json:"clause"`
	Name     string `json:"name,omitempty"`
}

// Extension is a constraint-tier pass no pilot-named row cites: Basis is
// "spec" for a rule of the specification the pilot names no constraint for,
// "extension" for a rule beyond the specification.
type Extension struct {
	File  string `json:"file"`
	Pass  string `json:"pass"`
	Basis string `json:"basis"`
	Note  string `json:"note"`
}

// Erratum is one declared defect in published OMG material, with the OMG
// issue reporting it when one is filed.
type Erratum struct {
	ID       string `json:"id"`
	Heading  string `json:"heading"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	OMGIssue string `json:"omgIssue,omitempty"`
}

// SpecIssue is an OMG issue against the specification that the census bears
// on, with the rows it affects and the page discussing it.
type SpecIssue struct {
	ID      string   `json:"id"`
	URL     string   `json:"url"`
	Summary string   `json:"summary"`
	BearsOn []string `json:"bearsOn"`
	Page    string   `json:"page"`
}

// Load reads the manifest under root.
func Load(root string) (*Manifest, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path))) // #nosec G304 -- fixed repository path
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	return &m, nil
}

// Write stores the manifest under root in its canonical encoding.
func Write(root string, m *Manifest) error {
	return os.WriteFile(filepath.Join(root, filepath.FromSlash(Path)), Encode(m), 0o644) // #nosec G306 -- documentation record
}

// Encode is the canonical encoding Write stores.
func Encode(m *Manifest) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		panic(err) // the types encode
	}
	return buf.Bytes()
}

var (
	constraintNamePattern = regexp.MustCompile(`^(validate|check|derive)[A-Z][A-Za-z0-9]*$`)
	clausePattern         = regexp.MustCompile(`^8\.3(\.\d+)+$`)
	omgIssuePattern       = regexp.MustCompile(`^[A-Z0-9]+-\d+$`)
)

// Validate checks the manifest on its own: the schema version, sorted unique
// names with the pilot's rows before the spec-sourced ones, known sources, kinds
// and statuses, a rationale wherever the status is not faithful, a clause in a
// declared specification wherever one is cited, and for every spec-sourced row
// the document's own name, clause and text.
func (m *Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%s: schema version %d, this program reads %d", Path, m.SchemaVersion, SchemaVersion)
	}
	if err := m.validateSpecifications(); err != nil {
		return err
	}
	seen, err := m.validateConstraints()
	if err != nil {
		return err
	}
	if err := m.validateExtensions(); err != nil {
		return err
	}
	if err := m.validateErrata(); err != nil {
		return err
	}
	return m.validateSpecIssues(seen)
}

// validateSpecifications checks the declared specifications and the formal
// extraction record, which counts only against declared ones.
func (m *Manifest) validateSpecifications() error {
	for key, spec := range m.Specifications {
		if spec.Name == "" || spec.Document == "" || spec.SHA256 == "" {
			return fmt.Errorf("%s: specification %q names no document", Path, key)
		}
	}
	if m.FormalExtraction != nil {
		if strings.TrimSpace(m.FormalExtraction.Method) == "" {
			return fmt.Errorf("%s: the formal extraction record states no method", Path)
		}
		for key := range m.FormalExtraction.Constraints {
			if _, ok := m.Specifications[key]; !ok {
				return fmt.Errorf("%s: the formal extraction record counts specification %q, which is not declared", Path, key)
			}
		}
	}
	return nil
}

// validateConstraints checks the rows' names, sources and order, then each row
// on its own; it returns the names listed.
func (m *Manifest) validateConstraints() (map[string]bool, error) {
	seen := make(map[string]bool, len(m.Constraints))
	for i, c := range m.Constraints {
		if !constraintNamePattern.MatchString(c.Name) {
			return nil, fmt.Errorf("%s: %q is not a normalized constraint name", Path, c.Name)
		}
		if seen[c.Name] {
			return nil, fmt.Errorf("%s: %s is listed twice", Path, c.Name)
		}
		seen[c.Name] = true
		if _, ok := LanguageBySource[c.Source]; !ok && c.Source != SourceSpec {
			return nil, fmt.Errorf("%s: %s has source %q", Path, c.Name, c.Source)
		}
		if i > 0 {
			if err := validateOrder(m.Constraints[i-1], c); err != nil {
				return nil, err
			}
		}
		if err := m.validateConstraint(c); err != nil {
			return nil, err
		}
	}
	return seen, nil
}

// validateOrder checks that c follows previous: the pilot's rows precede the
// spec-sourced rows, and each group is sorted by name.
func validateOrder(previous, c Constraint) error {
	switch {
	case previous.Source == SourceSpec && c.Source != SourceSpec:
		return fmt.Errorf("%s: %s is out of order (the pilot's rows precede the spec-sourced rows)", Path, c.Name)
	case (previous.Source == SourceSpec) == (c.Source == SourceSpec) && previous.Name > c.Name:
		return fmt.Errorf("%s: %s is out of order (each group is sorted by name)", Path, c.Name)
	}
	return nil
}

// validateConstraint checks one row: its kind, what its source allows, its
// metaclass, status and rationale, the clause it cites, and its cells.
func (m *Manifest) validateConstraint(c Constraint) error {
	if !knownKind(c.Kind) {
		return fmt.Errorf("%s: %s has kind %q", Path, c.Name, c.Kind)
	}
	if !strings.HasPrefix(c.Name, c.Kind) {
		return fmt.Errorf("%s: %s has kind %q, which its name does not state", Path, c.Name, c.Kind)
	}
	if c.Source == SourceSpec {
		if err := validateSpecSourced(c); err != nil {
			return err
		}
	} else if c.OCL != "" {
		return fmt.Errorf("%s: %s records OCL, which only a spec-sourced row carries", Path, c.Name)
	}
	if c.Metaclass == "" {
		return fmt.Errorf("%s: %s names no metaclass", Path, c.Name)
	}
	if _, ok := MarkerFor(c.Status); !ok {
		return fmt.Errorf("%s: %s has status %q", Path, c.Name, c.Status)
	}
	if c.Status != StatusFaithful && strings.TrimSpace(c.Rationale) == "" {
		return fmt.Errorf("%s: %s is %s but states no rationale", Path, c.Name, c.Status)
	}
	if c.Spec != nil {
		if err := m.validateCitation(c); err != nil {
			return err
		}
	}
	for _, cell := range []string{c.Checks, c.Implementation, c.Message, c.NegativeCase, c.Rationale} {
		if strings.ContainsAny(cell, "|\n") {
			return fmt.Errorf("%s: %s holds a table cell with a pipe or line break", Path, c.Name)
		}
	}
	return nil
}

// validateCitation checks the clause c cites: a declared specification, a
// clause number, and a spec name that is a constraint name and no other row's own.
func (m *Manifest) validateCitation(c Constraint) error {
	if _, ok := m.Specifications[c.Spec.Document]; !ok {
		return fmt.Errorf("%s: %s cites specification %q, which is not declared", Path, c.Name, c.Spec.Document)
	}
	if !clausePattern.MatchString(c.Spec.Clause) {
		return fmt.Errorf("%s: %s cites clause %q", Path, c.Name, c.Spec.Clause)
	}
	if c.Spec.Name != "" && c.Spec.Name != c.Name {
		if !constraintNamePattern.MatchString(c.Spec.Name) {
			return fmt.Errorf("%s: %s cites the spec name %q, which is not a constraint name", Path, c.Name, c.Spec.Name)
		}
		if other, ok := m.Constraint(c.Spec.Name); ok {
			return fmt.Errorf("%s: %s cites the spec name %q, which is %s's own name", Path, c.Name, c.Spec.Name, other.Name)
		}
	}
	return nil
}

// validateExtensions checks each extension names a file, pass, note and basis, once.
func (m *Manifest) validateExtensions() error {
	files := make(map[string]bool, len(m.Extensions))
	for _, e := range m.Extensions {
		if e.File == "" || e.Pass == "" || strings.TrimSpace(e.Note) == "" {
			return fmt.Errorf("%s: an extension names no file, pass or note", Path)
		}
		if e.Basis != "spec" && e.Basis != "extension" {
			return fmt.Errorf("%s: extension %s has basis %q, want spec or extension", Path, e.File, e.Basis)
		}
		if files[e.File] {
			return fmt.Errorf("%s: extension %s is listed twice", Path, e.File)
		}
		files[e.File] = true
	}
	return nil
}

// validateErrata checks each erratum is complete, listed once, and cites a
// well-formed OMG issue if any.
func (m *Manifest) validateErrata() error {
	ids := make(map[string]bool, len(m.Errata))
	for _, e := range m.Errata {
		if e.ID == "" || e.Heading == "" || e.Path == "" || e.Line <= 0 {
			return fmt.Errorf("%s: erratum %q names no heading, path or line", Path, e.ID)
		}
		if ids[e.ID] {
			return fmt.Errorf("%s: erratum %s is listed twice", Path, e.ID)
		}
		ids[e.ID] = true
		if e.OMGIssue != "" && !omgIssuePattern.MatchString(e.OMGIssue) {
			return fmt.Errorf("%s: erratum %s cites OMG issue %q", Path, e.ID, e.OMGIssue)
		}
	}
	return nil
}

// validateSpecIssues checks each spec issue is complete, listed once, and bears
// on rows the manifest lists.
func (m *Manifest) validateSpecIssues(rows map[string]bool) error {
	issues := make(map[string]bool, len(m.SpecIssues))
	for _, issue := range m.SpecIssues {
		if !omgIssuePattern.MatchString(issue.ID) || issue.URL == "" || strings.TrimSpace(issue.Summary) == "" || issue.Page == "" {
			return fmt.Errorf("%s: spec issue %q names no OMG issue, URL, summary or page", Path, issue.ID)
		}
		if issues[issue.ID] {
			return fmt.Errorf("%s: spec issue %s is listed twice", Path, issue.ID)
		}
		issues[issue.ID] = true
		for _, name := range issue.BearsOn {
			if !rows[name] {
				return fmt.Errorf("%s: spec issue %s bears on %s, which is not a row", Path, issue.ID, name)
			}
		}
	}
	return nil
}

func knownKind(kind string) bool {
	for _, k := range Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// validateSpecSourced checks what a row only the formal documents state must
// carry: the document's clause and its own name, its prose, and no pilot
// identifier.
func validateSpecSourced(c Constraint) error {
	if c.Raw != "" {
		return fmt.Errorf("%s: %s is spec-sourced but records a pilot identifier %q", Path, c.Name, c.Raw)
	}
	if c.Spec == nil || c.Spec.Name != c.Name {
		return fmt.Errorf("%s: %s is spec-sourced but cites no clause under its own name", Path, c.Name)
	}
	if strings.TrimSpace(c.Checks) == "" {
		return fmt.Errorf("%s: %s is spec-sourced but states no text", Path, c.Name)
	}
	return nil
}

// PilotRows are the rows a pilot validator names: the census counts them.
func (m *Manifest) PilotRows() []Constraint {
	return m.rows(false)
}

// SpecRows are the rows only the formal documents state; they stay outside
// the census counts.
func (m *Manifest) SpecRows() []Constraint {
	return m.rows(true)
}

func (m *Manifest) rows(spec bool) []Constraint {
	var rows []Constraint
	for _, c := range m.Constraints {
		if (c.Source == SourceSpec) == spec {
			rows = append(rows, c)
		}
	}
	return rows
}

// Counts tallies the statuses in the order the summary line states them.
type Counts struct {
	Total   int
	ByState map[string]int
}

// Counts tallies the pilot's rows by status; the spec-sourced rows are
// counted by Coverage instead.
func (m *Manifest) Counts() Counts {
	rows := m.PilotRows()
	c := Counts{Total: len(rows), ByState: make(map[string]int, len(StatusMarkers))}
	for _, constraint := range rows {
		c.ByState[constraint.Status]++
	}
	return c
}

// Implemented is the number of constraints the census claims we report.
func (c Counts) Implemented() int {
	return c.ByState[StatusFaithful] + c.ByState[StatusApproximate]
}

// Constraint returns the row of a name.
func (m *Manifest) Constraint(name string) (Constraint, bool) {
	for _, c := range m.Constraints {
		if c.Name == name {
			return c, true
		}
	}
	return Constraint{}, false
}

// TableColumns are the census table's columns, in order.
var TableColumns = []string{"Constraint", "Language", "Checks", "Implementation", "Our message", "Negative case", "Status"}

// StatusCell renders a row's status cell: the marker, then the rationale.
func StatusCell(c Constraint) string {
	marker, _ := MarkerFor(c.Status)
	if c.Rationale == "" {
		return marker
	}
	return marker + " — " + c.Rationale
}

// TableRows orders the pilot's rows as the census table lists them: the KerML
// validator's constraints first, those both validators declare among them,
// then the SysML validator's, each run by name.
func (m *Manifest) TableRows() []Constraint {
	rows := m.PilotRows()
	sort.SliceStable(rows, func(i, j int) bool {
		return (rows[i].Source == "sysml") != (rows[j].Source == "sysml") && rows[j].Source == "sysml"
	})
	return rows
}

// RenderTable renders the census table from the rows, one line per row.
func (m *Manifest) RenderTable() string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(TableColumns, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat("---|", len(TableColumns)) + "\n")
	for _, c := range m.TableRows() {
		cells := []string{"`" + c.Name + "`", LanguageBySource[c.Source], c.Checks, c.Implementation, c.Message, c.NegativeCase, StatusCell(c)}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return b.String()
}

// Summary renders the census sentence the pages state.
func (m *Manifest) Summary() string {
	c := m.Counts()
	return fmt.Sprintf("**Census:** %d of %d named constraints are reported by OpenSysML — %d ✅ faithful and %d ⚠️ approximate; %d ❌ not implemented, %d ⛔ deliberate, %d 🚧 known failure, %d ❔ unknown.",
		c.Implemented(), c.Total, c.ByState[StatusFaithful], c.ByState[StatusApproximate],
		c.ByState[StatusNotImplemented], c.ByState[StatusDeliberate], c.ByState[StatusKnownFailure], c.ByState[StatusUnknown])
}
