package doccounts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// LibraryCensusPath is the committed measurement of the analysis libraries, written
// by the runtime test TestAnalysisLibraryCensus and rendered into the compliance map.
const LibraryCensusPath = "docs/project/analysis-library-census.json"

// libraryBlockName names the generated block carrying the per-library table.
const libraryBlockName = "analysis-libraries"

// LibraryCensus is the measurement of every callable declaration the analysis
// libraries make, as one runtime test observed it.
type LibraryCensus struct {
	// Command reproduces the measurement.
	Command  string           `json:"command"`
	Packages []LibraryPackage `json:"packages"`
}

// LibraryPackage is one library package: the callable declarations it makes and
// the verdict of the representative invocation of each.
type LibraryPackage struct {
	Name string `json:"name"`
	// Path locates the library file in the bundled standard library.
	Path string `json:"path"`
	// Declarations are the qualified names of every named callable declaration
	// the package makes, in declaration order.
	Declarations []string `json:"declarations"`
	// Evaluated names the declarations whose invocation produced the checked value.
	Evaluated []string `json:"evaluated"`
	// Refused lists the declarations whose invocation the runtime refused with a typed error.
	Refused []LibraryRefusal `json:"refused"`
	// Wrong lists the declarations whose invocation produced a value that failed its check.
	Wrong []LibraryMismatch `json:"wrong"`
}

// LibraryRefusal is one declaration the runtime refused to run.
type LibraryRefusal struct {
	Declaration string `json:"declaration"`
	// Error is the runtime's sentinel error the refusal is typed by.
	Error string `json:"error"`
	// Message is the refusal as reported.
	Message string `json:"message"`
}

// LibraryMismatch is one declaration whose value disagreed with its check.
type LibraryMismatch struct {
	Declaration string `json:"declaration"`
	// Mismatch is what the check reported about the value.
	Mismatch string `json:"mismatch"`
}

// Counts are the per-package figures the table states.
func (p LibraryPackage) Counts() (declarations, evaluated, refused, wrong int) {
	return len(p.Declarations), len(p.Evaluated), len(p.Refused), len(p.Wrong)
}

// Validate holds the census to its own bookkeeping: every declaration has exactly
// one verdict and every verdict names a declaration.
func (c LibraryCensus) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("%s: states no command that reproduces it", LibraryCensusPath)
	}
	if len(c.Packages) == 0 {
		return fmt.Errorf("%s: states no packages", LibraryCensusPath)
	}
	for _, pkg := range c.Packages {
		verdicts := map[string]int{}
		for _, name := range pkg.Evaluated {
			verdicts[name]++
		}
		for _, refusal := range pkg.Refused {
			verdicts[refusal.Declaration]++
			if refusal.Error == "" || refusal.Message == "" {
				return fmt.Errorf("%s: %s: refusal of %s states no typed error or message", LibraryCensusPath, pkg.Name, refusal.Declaration)
			}
		}
		for _, mismatch := range pkg.Wrong {
			verdicts[mismatch.Declaration]++
			if mismatch.Mismatch == "" {
				return fmt.Errorf("%s: %s: wrong result of %s states no mismatch", LibraryCensusPath, pkg.Name, mismatch.Declaration)
			}
		}
		for _, name := range pkg.Declarations {
			if verdicts[name] != 1 {
				return fmt.Errorf("%s: %s: %s has %d verdicts, want 1", LibraryCensusPath, pkg.Name, name, verdicts[name])
			}
			delete(verdicts, name)
		}
		for name := range verdicts {
			return fmt.Errorf("%s: %s: %s has a verdict and is not a declaration", LibraryCensusPath, pkg.Name, name)
		}
	}
	return nil
}

// ReadLibraryCensus reads and validates the committed census.
func ReadLibraryCensus(root string) (LibraryCensus, error) {
	var census LibraryCensus
	if err := readJSON(root, LibraryCensusPath, &census); err != nil {
		return LibraryCensus{}, err
	}
	if err := census.Validate(); err != nil {
		return LibraryCensus{}, err
	}
	return census, nil
}

// FormatLibraryCensus is the one byte form the census is committed in, so the
// test that writes it and the guard that reads it agree on every byte.
func FormatLibraryCensus(census LibraryCensus) ([]byte, error) {
	if err := census.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(census); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// libraryTable renders the census as the Markdown the block carries.
func libraryTable(census LibraryCensus) string {
	var b strings.Builder
	b.WriteString("| Package | Declarations | Evaluated | Refused | Wrong |\n")
	b.WriteString("|---|---:|---:|---:|---:|\n")
	total := [4]int{}
	for _, pkg := range census.Packages {
		declarations, evaluated, refused, wrong := pkg.Counts()
		total[0] += declarations
		total[1] += evaluated
		total[2] += refused
		total[3] += wrong
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d |\n", pkg.Name, declarations, evaluated, refused, wrong)
	}
	fmt.Fprintf(&b, "| **Total** | **%d** | **%d** | **%d** | **%d** |\n", total[0], total[1], total[2], total[3])
	if total[2] > 0 {
		b.WriteString("\n**Refused, by name** (the typed error the runtime answered with):\n\n")
		for _, pkg := range census.Packages {
			for _, refusal := range pkg.Refused {
				fmt.Fprintf(&b, "- `%s` — `%s`: %s\n", refusal.Declaration, refusal.Error, refusal.Message)
			}
		}
	}
	if total[3] > 0 {
		b.WriteString("\n**Wrong, by name** (the value the runtime produced, against the check):\n\n")
		for _, pkg := range census.Packages {
			for _, mismatch := range pkg.Wrong {
				fmt.Fprintf(&b, "- `%s` — %s\n", mismatch.Declaration, mismatch.Mismatch)
			}
		}
	}
	return b.String()
}

const libraryBlockTemplateText = "**Measured by `{{.Library.Command}}`,** which writes [`analysis-library-census.json`]({{.LinkPrefix}}analysis-library-census.json); `make docs-counts` renders this block from that file and `go run ./cmd/doc-counts -check` fails when they disagree.\n\n" +
	"{{.Table}}"
