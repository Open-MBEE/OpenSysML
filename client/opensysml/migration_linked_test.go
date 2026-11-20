//go:build !sysml_re && !sysml_nov1

package opensysml_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// MigrateFile migrates a v1 file whose extension names its form, and
// MigrateSource the same bytes with WithV1Format naming it; both account for
// the migration in the Report, with every element's verdict when asked.
func TestMigrateFileAndSourceMigrateSysMLv1(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	xmi := filepath.Join("..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi")
	data, err := os.ReadFile(xmi)
	if err != nil {
		t.Fatal(err)
	}
	fromFile, err := client.MigrateFile(ctx, xmi, opensysml.FormatSysML)
	if err != nil {
		t.Fatalf("MigrateFile: %v", err)
	}
	fromSource, err := client.MigrateSource(ctx, data, opensysml.FormatSysML,
		opensysml.WithV1Format(opensysml.FormatXMI), opensysml.WithMigrationReport())
	if err != nil {
		t.Fatalf("MigrateSource: %v", err)
	}
	for name, migration := range map[string]*opensysml.Migration{"file": fromFile, "source": fromSource} {
		if migration.From != opensysml.FormatXMI || migration.To != opensysml.FormatSysML {
			t.Errorf("%s: migration = %s to %s, want xmi to sysml", name, migration.From, migration.To)
		}
		if !strings.Contains(migration.Content, "part def Vehicle") {
			t.Errorf("%s: migration does not carry the migrated model:\n%s", name, migration.Content)
		}
		if !strings.Contains(migration.ExperimentalNotice, "SysML v1 migration") {
			t.Errorf("%s: a migration does not report itself as experimental: %q", name, migration.ExperimentalNotice)
		}
		if migration.Report == nil || migration.Report.Mapped == 0 || !strings.HasPrefix(migration.Report.Summary, "migrated ") {
			t.Fatalf("%s: report = %+v, want mapped elements and the summary line", name, migration.Report)
		}
		if migration.Results != "" || len(migration.Files) != 0 {
			t.Errorf("%s: results %q and %d files came back unasked", name, migration.Results, len(migration.Files))
		}
	}
	if fromFile.Content != fromSource.Content {
		t.Error("the file and its bytes migrate differently")
	}
	if len(fromFile.Report.Entries) != 0 || fromFile.Report.Text != "" {
		t.Errorf("entries came back without WithMigrationReport: %d", len(fromFile.Report.Entries))
	}
	report := fromSource.Report
	if len(report.Entries) == 0 || !strings.HasPrefix(report.Text, "# SysML v1 to v2 migration report") {
		t.Fatalf("WithMigrationReport: %d entries, text %q", len(report.Entries), report.Text)
	}
	verdicts := map[string]int{}
	for _, entry := range report.Entries {
		verdicts[entry.Verdict]++
	}
	if verdicts["mapped"] != report.Mapped || verdicts["approximated"] != report.Approximated ||
		verdicts["unmapped"] != report.Unmapped || verdicts["skipped"] != report.Skipped || len(verdicts) > 4 {
		t.Errorf("verdicts %v do not add up to the counts %d/%d/%d/%d",
			verdicts, report.Mapped, report.Approximated, report.Unmapped, report.Skipped)
	}

	_, err = client.MigrateFile(ctx, xmi, opensysml.FormatXMI)
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("writing xmi: err = %v, want CodeInvalidArgument", err)
	}
}

// A v2 model is converted, not migrated: MigrateFile and MigrateSource refuse
// it pointing at the Convert methods.
func TestMigrateRefusesSysMLv2(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"file": func() error {
			_, err := client.MigrateFile(ctx, "Model.sysml", opensysml.FormatSysML)
			return err
		},
		"source": func() error {
			_, err := client.MigrateSource(ctx, []byte(editableSource), opensysml.FormatSysML,
				opensysml.WithV1Format(opensysml.FormatSysML))
			return err
		},
		"source without a format": func() error {
			_, err := client.MigrateSource(ctx, []byte(editableSource), opensysml.FormatSysML)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, opensysml.CodeInvalidArgument) {
				t.Errorf("err = %v, want CodeInvalidArgument", err)
			}
		})
	}
}

// WithLayoutFile and WithLayout lay the migrated views out from an MTIP export,
// and WithMigrationResults indexes the results the v1 tool stored.
func TestMigrateOptionsReachTheMigration(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	dir := filepath.Join("..", "..", "tests", "migrate", "testdata", "xmi")
	layoutPath := filepath.Join(dir, "layout.layout.xml")
	layoutData, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, opt := range map[string]opensysml.MigrateOption{
		"file":   opensysml.WithLayoutFile(layoutPath),
		"inline": opensysml.WithLayout(string(layoutData)),
	} {
		migration, err := client.MigrateFile(ctx, filepath.Join(dir, "layout.xmi"), opensysml.FormatSysML,
			opt, opensysml.WithMigrationResults())
		if err != nil {
			t.Fatalf("%s: MigrateFile: %v", name, err)
		}
		if !strings.Contains(migration.Report.Summary, "laid out") {
			t.Errorf("%s: summary %q does not account for the layout", name, migration.Report.Summary)
		}
		if !strings.HasPrefix(migration.Results, "{") {
			t.Errorf("%s: results = %q, want the JSON index", name, migration.Results)
		}
	}
}
