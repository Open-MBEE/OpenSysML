//go:build !sysml_prod && !sysml_nov1

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
)

func init() {
	v1Feature.link(func(fs *flag.FlagSet) {
		fs.StringVar(&migrationReport, "migration-report", "", "With -convert from xmi, write the element-by-element migration report to this file: JSON when it ends in .json, text otherwise")
		fs.StringVar(&migrationResults, "migration-results", "", "With -convert from xmi, write the run configurations and the result snapshots the simulation tool stored for them to this JSON file, for -compare-results to read against the migrated model")
		fs.StringVar(&layoutPath, "layout", "", "With -convert from xmi, read this MTIP export (HUDS XML) and write the diagram geometry it records as DiagramLayout annotations in the migrated views")
		fs.StringVar(&imageBaseURL, "image-base-url", "", "With -convert from xmi, the http(s) URL a comment's relative <img src> is resolved against, such as the View Editor server")
	})
}

// migrateInput migrates a SysML v1 model to the to format, writing its report
// and results where the flags say; files are the image files it attached.
func migrateInput(name string, data []byte, to convert.Format) ([]byte, map[string][]byte, error) {
	migOpts, err := migrationOptions()
	if err != nil {
		return nil, nil, err
	}
	migrated, err := convert.Migrate(name, data, to, migOpts)
	if err != nil {
		return nil, nil, err
	}
	if err := writeMigrationReport(migrated.Report); err != nil {
		return nil, nil, err
	}
	if err := writeMigrationResults(migrated.Results); err != nil {
		return nil, nil, err
	}
	return migrated.Output, migrated.Files, nil
}

// migrationOptions reads the -layout MTIP export into the migration's
// options; none were given when the flag was not passed.
func migrationOptions() (migrate.Options, error) {
	if layoutPath == "" {
		return migrate.Options{ImageBaseURL: imageBaseURL, Strict: strictMode}, nil
	}
	data, err := os.ReadFile(layoutPath)
	if err != nil {
		return migrate.Options{}, err
	}
	layout, err := mtip.Parse(data)
	if err != nil {
		return migrate.Options{}, fmt.Errorf("%s: %w", layoutPath, err)
	}
	return migrate.Options{Layout: layout, LayoutSource: layoutPath, ImageBaseURL: imageBaseURL, Strict: strictMode}, nil
}

// writeMigrationReport writes the report to the -migration-report file (JSON when
// it ends in .json), or just its summary to stderr when the flag was not given.
func writeMigrationReport(report *migrate.Report) error {
	if migrationReport == "" {
		fmt.Fprintf(os.Stderr, "migration: %s; pass -migration-report FILE for the element-by-element report\n", report.Summary())
		return nil
	}
	var body bytes.Buffer
	write := report.WriteText
	if strings.EqualFold(filepath.Ext(migrationReport), ".json") {
		write = report.WriteJSON
	}
	if err := write(&body); err != nil {
		return err
	}
	if _, err := export.WriteFile(migrationReport, body.Bytes()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (migration report: %s)\n", migrationReport, report.Summary())
	return nil
}

// writeMigrationResults writes the result snapshots the migration indexed to the
// -migration-results file as JSON, for -compare-results to read against the migrated model.
func writeMigrationResults(results *simresults.Results) error {
	if migrationResults == "" {
		return nil
	}
	body, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if _, err := export.WriteFile(migrationResults, append(body, '\n')); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s)\n", migrationResults, results.Summary())
	return nil
}
