package migrate_test

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

var standardDeclaration = regexp.MustCompile(`(?m)^standard library package`)

const inlinedLibrariesMark = "\n// OpenSysML library packages the model refers to"

// A portable migration appends every OpenSysML library package its notation
// refers to, as a library package a user file may declare, and accounts for
// them; the default migration appends none and accounts for none.
func TestPortableMigrationInlinesTheLibrariesItRefersTo(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "xmi", "*.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixture found")
	}
	libs := openSysMLLibraries(t)
	for _, f := range fixtures {
		name := strings.TrimSuffix(filepath.Base(f), ".xmi")
		t.Run(name, func(t *testing.T) {
			r := migrateFixtureFileOptions(t, name, migrate.Options{Portable: true})
			model, appended, _ := strings.Cut(string(r.Notation), inlinedLibrariesMark)
			if r.Report.Libraries == nil {
				t.Fatal("report accounts for no libraries")
			}
			got := r.Report.Libraries.Inlined
			var want []string
			for _, lib := range libs {
				if strings.Contains(model, lib+"::") {
					want = append(want, lib)
				}
			}
			for _, lib := range want {
				if !slices.Contains(got, lib) {
					t.Errorf("%s referred to but not accounted for in %v", lib, got)
				}
				if !strings.Contains(appended, "library package "+lib+" {") {
					t.Errorf("%s referred to but not inlined", lib)
				}
			}
			if len(want) == 0 && (appended != "" || len(got) > 0) {
				t.Errorf("nothing referred to, yet inlined %v", got)
			}
			if standardDeclaration.MatchString(appended) {
				t.Error("an inlined package is marked standard")
			}
			if summary := r.Report.Summary(); len(want) > 0 && !strings.Contains(summary, "inlined "+itoa(len(got))+" OpenSysML library package(s): ") {
				t.Errorf("summary does not account for the libraries: %s", summary)
			}
			wantClean(t, name+".sysml", r)
		})
	}
	if r := migrateFixtureFile(t, "vehicle"); r.Report.Libraries != nil || strings.Contains(string(r.Notation), inlinedLibrariesMark) {
		t.Error("a default migration inlined libraries")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// A KerML library is inlined in its SysML spelling, its functions as calc defs.
func TestPortableMigrationSpellsAKerMLLibraryInSysML(t *testing.T) {
	r := migrateDocumentOptions(t, missionActivity, missionApplications, migrate.Options{Portable: true})
	wantLine(t, r.Notation, "action wait2 accept after RandomFunctions::uniform(1.0, 8.0) [SI::s];")
	wantLine(t, r.Notation, "library package RandomFunctions {")
	wantLine(t, r.Notation, "calc def uniform {")
	wantNoLine(t, r.Notation, "function uniform {")
	if got := r.Report.Libraries.Inlined; !slices.Contains(got, "RandomFunctions") {
		t.Errorf("inlined %v, want RandomFunctions among them", got)
	}
	wantClean(t, "t.sysml", r)
}
