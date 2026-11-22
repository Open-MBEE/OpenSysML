package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const subclassificationsQuery = `package Q {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	calc def Subclassifications :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "Subclassification", exact = true),
			properties = ("elementId")
		)
	}
}
`

// TestRunQueryReadsAKerMLClassifiersSubclassificationByItsExportedID: for each KerML classifier
// keyword, the Subclassification reflection reads carries the id the Turtle export writes it at.
func TestRunQueryReadsAKerMLClassifiersSubclassificationByItsExportedID(t *testing.T) {
	binary := buildCLI(t)
	var kernel strings.Builder
	var want []string
	kernel.WriteString("package K {\n\tclass C;\n")
	for i, keyword := range ast.KerMLTypeKeywords() {
		if keyword == "type" {
			// A Type is no Classifier: its specializes is not a Subclassification.
			continue
		}
		body := ";"
		if strings.HasPrefix(keyword, "assoc") || keyword == "interaction" {
			body = " { end feature a : C; end feature b : C; }"
		}
		fmt.Fprintf(&kernel, "\tpackage P%d {\n\t\t%s A%s\n\t\t%s X specializes A%s\n\t}\n", i, keyword, body, keyword, body)
		want = append(want, fmt.Sprintf("K__P%d__X_sc0", i))
	}
	kernel.WriteString("}\n")
	slices.Sort(want)

	got := checkFiles(t, binary, map[string]string{"q.sysml": subclassificationsQuery, "k.kerml": kernel.String()},
		"-run-query", "Q::Subclassifications root=K", "-json")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	var report struct {
		Checks []struct {
			Table *struct {
				Rows []struct {
					Cells [][]string `json:"cells"`
				} `json:"rows"`
			} `json:"table"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.stdout)
	}
	if len(report.Checks) != 1 || report.Checks[0].Table == nil {
		t.Fatalf("want one check with a table:\n%s", got.stdout)
	}
	var reflected []string
	for _, row := range report.Checks[0].Table.Rows {
		if len(row.Cells) != 1 || len(row.Cells[0]) != 1 {
			t.Fatalf("cells = %v, want one elementId", row.Cells)
		}
		id, _ := strconv.Unquote(row.Cells[0][0])
		reflected = append(reflected, id)
	}
	slices.Sort(reflected)
	if !slices.Equal(reflected, want) {
		t.Errorf("reflection reads the Subclassifications %v, want %v", reflected, want)
	}

	path := filepath.Join(t.TempDir(), "k.kerml")
	if err := os.WriteFile(path, []byte(kernel.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var exported []string
	for _, match := range regexp.MustCompile(`(?m)^elmt:(\S+)\n    a sysml:Subclassification ;`).FindAllStringSubmatch(run(t, binary, "-convert", "ttl", path), -1) {
		exported = append(exported, match[1])
	}
	slices.Sort(exported)
	if !slices.Equal(exported, want) {
		t.Errorf("the export writes the Subclassifications %v, want %v", exported, want)
	}
}
