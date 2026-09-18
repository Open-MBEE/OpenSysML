// Package errata is the declared overlay of defects in OMG-published material,
// and the entries for the bundled standard library; published bytes are never rewritten.
package errata

import (
	"fmt"
	"sort"
	"strings"
)

// IssuesPath is the page every entry must be documented in.
const IssuesPath = "docs/project/omg-issues.md"

// LibraryRoot is the repository path of the bundled standard library, whose
// files are OMG-published material too. Entries under it are keyed for
// LibrarySource by the path a libs.Source lists them under.
const LibraryRoot = "internal/core/libs/stdlib"

// Entry is one defect in published material. The span is one whole line:
// AsPublished is that line's exact bytes without its terminator, which is what
// lets a re-vendored corpus invalidate the entry instead of silently rotting it.
type Entry struct {
	// ID is the entry's stable internal label, as the oracles report it.
	ID string
	// Heading titles the docs/project/omg-issues.md section documenting the
	// defect, without its `### ` marker.
	Heading string
	// Path is the defective file, relative to the repository root.
	Path string
	// Line is the 1-based line the entry covers.
	Line int
	// AsPublished is the published line, verbatim.
	AsPublished string
	// Corrected replaces AsPublished when non-empty. Empty means documented
	// without a correction: nothing is substituted, and the defect stands.
	Corrected string
	// Citation is the specification clause the published text violates.
	Citation string
	// Derivation states in one line why that clause is violated.
	Derivation string
}

// Corrects reports whether the entry substitutes text.
func (e Entry) Corrects() bool { return e.Corrected != "" }

// Validate reports why an entry may not be accepted, short of where it lies:
// New checks the path against the published roots it is given.
func (e Entry) Validate() error {
	switch {
	case e.ID == "":
		return fmt.Errorf("entry for %s:%d names no %s row", e.Path, e.Line, IssuesPath)
	case e.Heading == "":
		return fmt.Errorf("entry %s names no %s section documenting it", e.ID, IssuesPath)
	case e.Path == "":
		return fmt.Errorf("entry %s names no file", e.ID)
	case e.Line < 1:
		return fmt.Errorf("entry %s: line %d is not a line", e.ID, e.Line)
	case strings.TrimSpace(e.AsPublished) == "":
		return fmt.Errorf("entry %s: no as-published text to match against the corpus", e.ID)
	case e.Citation == "":
		return fmt.Errorf("entry %s: no specification citation makes the published text wrong", e.ID)
	case e.Derivation == "":
		return fmt.Errorf("entry %s: no derivation from %s is written down", e.ID, e.Citation)
	case e.Corrected == e.AsPublished:
		return fmt.Errorf("entry %s: the correction repeats the published text", e.ID)
	case strings.Contains(e.AsPublished, "\n") || strings.Contains(e.Corrected, "\n"):
		return fmt.Errorf("entry %s: an entry covers one line", e.ID)
	}
	return nil
}

func underRoot(path string, roots []string) bool {
	for _, root := range roots {
		if strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// LibraryEntries are the declared errata of the bundled standard library, in report order.
func LibraryEntries() []Entry {
	return []Entry{{
		ID:          "SI-137",
		Heading:     "`'eV⋅m⁻²/kg' : TotalMassStoppingPowerUnit = eV*m^-2/kg` is T^-2",
		Path:        siPath,
		Line:        137,
		AsPublished: "    attribute <'eV⋅m⁻²/kg'> 'electronvolt metre to the power minus 2 per kilogram' : TotalMassStoppingPowerUnit = eV*m^-2/kg;",
		Corrected:   "    attribute <'eV⋅m⁻²/kg'> 'electronvolt metre to the power minus 2 per kilogram' : TotalMassStoppingPowerUnit = eV*m^2/kg;",
		Citation:    "KerML 7.4.9",
		Derivation:  "ISO 80000-10 item 10-55 defines mass stopping power as energy × area per mass, L^4·T^-2, which is the dimension TotalMassStoppingPowerUnit declares and the dimension of the file's own `J*m^2/kg` (line 147); `eV*m^-2/kg` is T^-2, and `eV*m^2/kg` is the only reading of an electronvolt spelling with that dimension. The name stays as published so the element's identity does.",
	}, {
		ID:          "SI-149",
		Heading:     "`'J⋅s⋅eV⋅s' : TotalAngularMomentumUnit = J*s*eV*s` squares an angular momentum",
		Path:        siPath,
		Line:        149,
		AsPublished: "    attribute <'J⋅s⋅eV⋅s'> 'joule second electronvolt second' : TotalAngularMomentumUnit = J*s*eV*s;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`J*s` and `eV*s` are each an angular momentum (L^2·M·T^-1, ISO 80000-10 item 10-11), and their product is L^4·M^2·T^-2; the two are different units, so which one the line means cannot be inferred and the defect is documented without a correction.",
	}, {
		ID:          "SI-163",
		Heading:     "`'J⁻¹⋅m⁻³⋅eV⁻¹⋅m⁻³' : EnergyDensityOfStatesUnit = J^-1*m^-3*eV^-1*m^-3` squares a density of states",
		Path:        siPath,
		Line:        163,
		AsPublished: "    attribute <'J⁻¹⋅m⁻³⋅eV⁻¹⋅m⁻³'> 'joule to the power minus 1 metre to the power minus 3 electronvolt to the power minus 1 metre to the power minus 3' : EnergyDensityOfStatesUnit = J^-1*m^-3*eV^-1*m^-3;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`J^-1*m^-3` and `eV^-1*m^-3` are each an energy density of states (L^-5·M^-1·T^2, ISO 80000-12 item 12-16), and their product is L^-10·M^-2·T^4; the two are different units, so the defect is documented without a correction.",
	}, {
		ID:          "SI-233",
		Heading:     "`'m²⋅A' : MagneticDipoleMomentUnit = m^2*A` names the electromagnetic unit for the atomic one",
		Path:        siPath,
		Line:        233,
		AsPublished: "    attribute <'m²⋅A'> 'metre squared ampere' : MagneticDipoleMomentUnit = m^2*A;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`ISQ::*` re-exports two MagneticDipoleMomentUnits, the electromagnetic one (L^3·M·T^-2·I^-1, IEC 80000-6 item 6-30) and the atomic one (L^2·I, ISO 80000-10 item 10-9.1); `m^2*A` is the atomic unit, the unqualified name resolves to the electromagnetic one, and the ISQ library rather than this line is where the name clash is fixed, so the defect is documented without a correction.",
	}, {
		ID:          "SI-239",
		Heading:     "`'m²⋅s⁻³' : DoseEquivalentUnit = m^2*s^-3` types a dose-equivalent rate as a dose equivalent",
		Path:        siPath,
		Line:        239,
		AsPublished: "    attribute <'m²⋅s⁻³'> 'metre squared second to the power minus 3' : DoseEquivalentUnit = m^2*s^-3;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`m^2*s^-3` is L^2·T^-3, a dose-equivalent rate (ISO 80000-10 item 10-83.2), while DoseEquivalentUnit is L^2·T^-2; ISQAtomicNuclear declares no rate unit to retype the line by, so the defect is documented without a correction.",
	}, {
		ID:          "SI-247",
		Heading:     "`'m³/C⋅m³⋅s⁻¹⋅A⁻¹' : HallCoefficientUnit = m^3/C*m^3*s^-1*A^-1` squares a Hall coefficient",
		Path:        siPath,
		Line:        247,
		AsPublished: "    attribute <'m³/C⋅m³⋅s⁻¹⋅A⁻¹'> 'metre cubed per coulomb cubic metre second to the power minus 1 ampere to the power minus 1' : HallCoefficientUnit = m^3/C*m^3*s^-1*A^-1;",
		Corrected:   "    attribute <'m³/C⋅m³⋅s⁻¹⋅A⁻¹'> 'metre cubed per coulomb cubic metre second to the power minus 1 ampere to the power minus 1' : HallCoefficientUnit = m^3/C;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`m^3/C` and `m^3*s^-1*A^-1` are one coherent unit spelled twice (`C = A*s` in the same file), each the Hall coefficient of ISO 80000-12 item 12-19 (L^3·T^-1·I^-1) that HallCoefficientUnit declares; their product is L^6·T^-2·I^-2, and either spelling alone is the same unit, so `m^3/C` is substituted. The name stays as published so the element's identity does.",
	}, {
		ID:          "SI-286",
		Heading:     "`'Sv/s' : DoseEquivalentUnit = Sv/s` types a dose-equivalent rate as a dose equivalent",
		Path:        siPath,
		Line:        286,
		AsPublished: "    attribute <'Sv/s'> 'sievert per second' : DoseEquivalentUnit = Sv/s;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`Sv/s` is L^2·T^-3, a dose-equivalent rate (ISO 80000-10 item 10-83.2), while DoseEquivalentUnit is L^2·T^-2; as at line 239, no rate unit exists to retype the line by, so the defect is documented without a correction.",
	}, {
		ID:          "SI-299",
		Heading:     "`'W/kg' : DoseEquivalentUnit = W/kg` types a dose-equivalent rate as a dose equivalent",
		Path:        siPath,
		Line:        299,
		AsPublished: "    attribute <'W/kg'> 'watt per kilogram' : DoseEquivalentUnit = W/kg;",
		Citation:    "KerML 7.4.9",
		Derivation:  "`W/kg` is L^2·T^-3, a dose-equivalent rate (ISO 80000-10 item 10-83.2), while DoseEquivalentUnit is L^2·T^-2; as at line 239, no rate unit exists to retype the line by, so the defect is documented without a correction.",
	}, {
		ID:          "USCustomaryUnits-255",
		Heading:     "`zeroDegreeFahrenheitInKelvin = 229835/900 [K]` divides by a temperature",
		Path:        LibraryRoot + "/Domain Libraries/Quantities and Units/USCustomaryUnits.sysml",
		Line:        255,
		AsPublished: "        private attribute zeroDegreeFahrenheitInKelvin: ThermodynamicTemperatureValue = 229835/900 [K];",
		Corrected:   "        private attribute zeroDegreeFahrenheitInKelvin: ThermodynamicTemperatureValue = (229835/900) [K];",
		Citation:    "SysML v2 §9.8.9.1",
		Derivation:  "the unit postfix binds to PrimaryExpression (KerMLExpressions.xtext:308), below MultiplicativeExpression, so `[K]` qualifies 900 alone and the value is Θ^-1 where ThermodynamicTemperatureValue is Θ; the evident intent, 0 °F as a temperature in kelvin, is `(229835/900) [K]`.",
	}}
}

const siPath = LibraryRoot + "/Domain Libraries/Quantities and Units/SI.sysml"

// Overlay is a registry indexed by file, ready to apply.
type Overlay struct {
	entries []Entry
	byPath  map[string][]Entry
}

// Library returns the bundled standard library's declared errata as an overlay.
func Library() (*Overlay, error) { return New([]string{LibraryRoot}, LibraryEntries()) }

// New validates entries and indexes them by path; each must lie under one of
// the published roots, the repository paths holding OMG-published material.
func New(published []string, entries []Entry) (*Overlay, error) {
	overlay := &Overlay{byPath: make(map[string][]Entry, len(entries))}
	ids := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		if !underRoot(entry.Path, published) {
			return nil, fmt.Errorf("entry %s: %s is not OMG-published material (%s)", entry.ID, entry.Path, strings.Join(published, ", "))
		}
		if ids[entry.ID] {
			return nil, fmt.Errorf("entry %s is declared twice", entry.ID)
		}
		for _, other := range overlay.byPath[entry.Path] {
			// One entry per line keeps application order irrelevant.
			if other.Line == entry.Line {
				return nil, fmt.Errorf("entries %s and %s both cover %s:%d", other.ID, entry.ID, entry.Path, entry.Line)
			}
		}
		ids[entry.ID] = true
		overlay.byPath[entry.Path] = append(overlay.byPath[entry.Path], entry)
		overlay.entries = append(overlay.entries, entry)
	}
	return overlay, nil
}

// Entries returns every declared entry.
func (o *Overlay) Entries() []Entry {
	if o == nil {
		return nil
	}
	return o.entries
}

// Corrections returns the entries that substitute text.
func (o *Overlay) Corrections() []Entry {
	var out []Entry
	for _, entry := range o.Entries() {
		if entry.Corrects() {
			out = append(out, entry)
		}
	}
	return out
}

// Documented returns the entries documented without a correction.
func (o *Overlay) Documented() []Entry {
	var out []Entry
	for _, entry := range o.Entries() {
		if !entry.Corrects() {
			out = append(out, entry)
		}
	}
	return out
}

// Under returns the correcting entries whose file lies under a repository path,
// keyed by the path relative to it, each file's entries in line order.
func (o *Overlay) Under(dir string) map[string][]Entry {
	return byRelativePath(dir, o.Corrections())
}

// EntriesUnder is Under for every entry, the documented-only ones included, so
// a reader can verify each declared line and not only substitute the corrected.
func (o *Overlay) EntriesUnder(dir string) map[string][]Entry {
	return byRelativePath(dir, o.Entries())
}

func byRelativePath(dir string, entries []Entry) map[string][]Entry {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	out := map[string][]Entry{}
	for _, entry := range entries {
		if rel, ok := strings.CutPrefix(entry.Path, prefix); ok {
			out[rel] = append(out[rel], entry)
		}
	}
	for _, entries := range out {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Line < entries[j].Line })
	}
	return out
}

// Apply returns content with the entry's correction substituted. It fails
// unless the declared line still reads as published, so a re-vendored corpus is
// a hard error rather than a silently skipped correction.
func Apply(entry Entry, content []byte) ([]byte, error) {
	return ApplyAll([]Entry{entry}, content)
}

// ApplyAll is Apply for every entry of one file: each line is checked against
// its entry as published before any is substituted.
func ApplyAll(entries []Entry, content []byte) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	for _, entry := range entries {
		if entry.Line > len(lines) {
			return nil, fmt.Errorf("%s: %s has %d lines, entry names line %d", entry.ID, entry.Path, len(lines), entry.Line)
		}
		if got := lines[entry.Line-1]; got != entry.AsPublished {
			return nil, fmt.Errorf("%s: %s:%d reads %q, the entry records %q", entry.ID, entry.Path, entry.Line, got, entry.AsPublished)
		}
	}
	changed := false
	for _, entry := range entries {
		if entry.Corrects() {
			lines[entry.Line-1] = entry.Corrected
			changed = true
		}
	}
	if !changed {
		return content, nil
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// SortedPaths returns the keys of an Under result in order.
func SortedPaths(applied map[string][]Entry) []string {
	paths := make([]string, 0, len(applied))
	for rel := range applied {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}

// Count returns how many entries an Under result holds.
func Count(applied map[string][]Entry) int {
	n := 0
	for _, entries := range applied {
		n += len(entries)
	}
	return n
}

// Source is a library source in the shape of libs.Source: the relative paths
// of the files it serves, and their bytes.
type Source interface {
	List() []string
	Read(name string) ([]byte, error)
}

// LibrarySource serves the bundled standard library as published, with the
// corrections declared under LibraryRoot applied to the files they name. The
// published source is only ever read; a file whose declared line, corrected or
// documented only, no longer reads as published fails to load rather than
// loading unverified.
func (o *Overlay) LibrarySource(published Source) Source {
	return &librarySource{published: published, entries: o.EntriesUnder(LibraryRoot)}
}

type librarySource struct {
	published Source
	entries   map[string][]Entry
}

func (s *librarySource) List() []string { return s.published.List() }

func (s *librarySource) Read(name string) ([]byte, error) {
	content, err := s.published.Read(name)
	if err != nil {
		return nil, err
	}
	entries, ok := s.entries[name]
	if !ok {
		return content, nil
	}
	return ApplyAll(entries, content)
}
