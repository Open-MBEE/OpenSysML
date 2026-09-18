package fixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LibraryCensusPath is the committed measurement of the analysis libraries, written
// by the runtime test TestAnalysisLibraryCensus and rendered into the compliance map
// by the doc-counts gate.
const LibraryCensusPath = "docs/project/analysis-library-census.json"

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
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(LibraryCensusPath))) // #nosec G304 -- fixed path under the requested repository root
	if err != nil {
		return LibraryCensus{}, err
	}
	var census LibraryCensus
	if err := json.Unmarshal(content, &census); err != nil {
		return LibraryCensus{}, fmt.Errorf("%s: parse census: %w", LibraryCensusPath, err)
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
