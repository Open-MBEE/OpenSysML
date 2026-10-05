package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

var (
	dataImports  stringSlice
	importAs     string
	importMap    = perImport{flag: "import-map"}
	importFormat = perImport{flag: "import-format"}
	importDryRun bool
)

// perImport is a flag that qualifies one -import: the one before it, or the
// first when it precedes every -import.
type perImport struct {
	flag   string
	values map[int]string
}

func (p *perImport) String() string {
	if p == nil {
		return ""
	}
	return p.values[0]
}

func (p *perImport) Set(value string) error {
	i := max(len(dataImports)-1, 0)
	if _, ok := p.values[i]; ok {
		return fmt.Errorf("-%s is already given for this -import; give each -import its own after it", p.flag)
	}
	if p.values == nil {
		p.values = map[int]string{}
	}
	p.values[i] = value
	return nil
}

func (p *perImport) given() bool { return len(p.values) > 0 }

// importOptions are the -import-map, -import-format and -import-dry-run flags
// of the i-th -import.
func importOptions(i int) repl.ImportOptions {
	return repl.ImportOptions{Map: importMap.values[i], Format: importFormat.values[i], DryRun: importDryRun}
}

// importMisuse is why the -import flags given import nothing, "" when they do.
func importMisuse() string {
	switch {
	case importAs != "values":
		return fmt.Sprintf("-import-as %q is not supported; only values is", importAs)
	case len(dataImports) == 0 && (importMap.given() || importFormat.given() || importDryRun):
		return "-import-map, -import-format and -import-dry-run accompany -import; write `sysml model.sysml -import data.csv -convert sysml -o out.sysml`"
	case len(dataImports) == 0:
		return ""
	case importDryRun && convertFormat != "":
		return "-import-dry-run reports what -import would set and writes nothing; drop -convert, or -import-dry-run to write the imported model"
	case !importDryRun && convertFormat == "":
		return "-import writes the imported model with -convert sysml -o <file>; preview it with -import-dry-run"
	case importDryRun && (modelChecks.requested() || renderView != "" || renderAllDir != "" || renderDoc != "" || renderDocsDir != "" ||
		queryText != "" || len(evalExprs) > 0 || migrateFormat != "" || compileCalc != "" || syncDiffWith != "" || syncApplyTo != "" || outputPath != ""):
		return "-import-dry-run reports what -import would set and does nothing else; check, render, migrate, compile, sync or write -o output in its own run"
	}
	return ""
}

// importInto sets the values each -import file assigns in the session, as one
// edit per file; a file the model refuses stops the run.
func importInto(sess *repl.Session) error {
	for i, path := range dataImports {
		verdict := sess.ImportData(path, importOptions(i))
		writeLines(os.Stderr, verdict.Lines)
		if verdict.Status != repl.VerdictHolds {
			return fmt.Errorf("%s was not imported; nothing was converted", path)
		}
	}
	return nil
}

// runImportDryRun loads the model and reports what each -import file would set,
// changing nothing.
func runImportDryRun(files []string) int {
	if len(files) == 0 {
		return fail(fmt.Errorf("no model to import into; write `sysml model.sysml -import data.csv -import-dry-run`"))
	}
	sess := newSession()
	report, err := sess.LoadPathsReport(files)
	if err != nil {
		return fail(err)
	}
	writeLines(os.Stderr, report.Loaded)
	writeLines(os.Stderr, report.Found)
	if report.Errors {
		return fail(fmt.Errorf("the model did not analyse cleanly; nothing was imported"))
	}
	for i, path := range dataImports {
		verdict := sess.ImportData(path, importOptions(i))
		writeLines(os.Stdout, verdict.Lines)
		if verdict.Status != repl.VerdictHolds {
			return exitFailed
		}
		if i < len(dataImports)-1 {
			// Stage the file so the next preview reads the model a real import would.
			staged := importOptions(i)
			staged.DryRun = false
			if verdict := sess.ImportData(path, staged); verdict.Status != repl.VerdictHolds {
				writeLines(os.Stdout, verdict.Lines)
				return exitFailed
			}
		}
	}
	return exitHolds
}
