package doccounts

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// libraryBlockName names the generated block carrying the per-library table.
const libraryBlockName = "analysis-libraries"

// libraryTable renders the census as the Markdown the block carries.
func libraryTable(census fixtures.LibraryCensus) string {
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

const libraryBlockTemplateText = "**Measured by `{{.Library.Command}}`,** which writes [`analysis-library-census.json`]({{.LinkPrefix}}analysis-library-census.json); `make docs-counts` renders this block from that file and `go run -C tools ./cmd/doc-counts -check` fails when they disagree.\n\n" +
	"{{.Table}}"
