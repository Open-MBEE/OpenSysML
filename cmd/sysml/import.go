package main

import (
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

var (
	dataImports  stringSlice
	importAs     string
	importMap    string
	importFormat string
	importDryRun bool
)

// importOptions are the -import-map, -import-format and -import-dry-run flags.
func importOptions() repl.ImportOptions {
	return repl.ImportOptions{Map: importMap, Format: importFormat, DryRun: importDryRun}
}

// importMisuse is why the -import flags given import nothing, "" when they do.
func importMisuse() string {
	switch {
	case importAs != "values":
		return fmt.Sprintf("-import-as %q is not supported; only values is", importAs)
	case len(dataImports) == 0 && (importMap != "" || importFormat != "" || importDryRun):
		return "-import-map, -import-format and -import-dry-run accompany -import; write `sysml model.sysml -import data.csv -convert sysml -o out.sysml`"
	case len(dataImports) == 0:
		return ""
	case importDryRun && convertFormat != "":
		return "-import-dry-run reports what -import would set and writes nothing; drop -convert, or -import-dry-run to write the imported model"
	case !importDryRun && convertFormat == "":
		return "-import writes the imported model with -convert sysml -o <file>; preview it with -import-dry-run"
	case importDryRun && (modelChecks.requested() || renderView != "" || renderDoc != "" || queryText != "" || len(evalExprs) > 0):
		return "-import-dry-run reports what -import would set; check or render the imported model in its own run"
	}
	return ""
}

// importInto sets the values each -import file assigns in the session, as one
// edit per file; a file the model refuses stops the run.
func importInto(sess *repl.Session) error {
	for _, path := range dataImports {
		verdict := sess.ImportData(path, importOptions())
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
	status := exitHolds
	for _, path := range dataImports {
		verdict := sess.ImportData(path, importOptions())
		writeLines(os.Stdout, verdict.Lines)
		if verdict.Status != repl.VerdictHolds {
			status = exitFailed
		}
	}
	return status
}
