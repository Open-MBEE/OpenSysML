package main

import (
	"flag"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

// The shipped page is generated, so a flag or a section added to the
// description without regenerating it is a failure here rather than a page that
// documents a command that no longer exists.
func TestTheShippedManualPageIsWhatTheCommandRenders(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")
	if err := usage.CheckShippedPage("../..", doc(), docFlags()); err != nil {
		t.Error(err)
	}
}

func TestTheManualPageDocumentsEveryFlag(t *testing.T) {
	page := string(usage.Page(doc(), docFlags()))
	docFlags().VisitAll(func(f *flag.Flag) {
		// A hyphen means itself in roff, so every one of them is escaped.
		name := "\\-" + strings.ReplaceAll(f.Name, "-", "\\-")
		if !strings.Contains(page, name+" ") && !strings.Contains(page, name+"\n") {
			t.Errorf("the manual page does not document -%s", f.Name)
		}
	})
}

// The catalog tells a caller of a CLI-only option what sysml-mcp has instead,
// so each flag and environment variable it names must be one the usage defines.
func TestTheCatalogNamesOnlyFlagsAndVariablesTheUsageDefines(t *testing.T) {
	flags := map[string]bool{}
	docFlags().VisitAll(func(f *flag.Flag) { flags[f.Name] = true })
	variables := map[string]bool{}
	for _, item := range usage.BudgetEnvironment() {
		variables[item.Label] = true
	}
	flagRef := regexp.MustCompile(`sysml-mcp -([a-z][a-z0-9-]*)`)
	variableRef := regexp.MustCompile(`\bOPENSYSML_[A-Z_]+\b`)
	for option, text := range catalog.CLIOnly {
		for _, m := range flagRef.FindAllStringSubmatch(text, -1) {
			if !flags[m[1]] {
				t.Errorf("CLIOnly[%q] names sysml-mcp -%s, which the usage does not define", option, m[1])
			}
		}
		for _, name := range variableRef.FindAllString(text, -1) {
			if !variables[name] {
				t.Errorf("CLIOnly[%q] names %s, which the usage does not document", option, name)
			}
		}
	}
}
