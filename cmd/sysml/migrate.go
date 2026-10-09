//go:build !sysml_prod && !sysml_nov1

package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

func init() {
	v1Feature.link(func(fs *flag.FlagSet) {
		fs.StringVar(&migrateFormat, "migrate", "", "Migrate the SysML v1 model (.xmi, .uml or .mdzip; -from names the format when the extension does not) to SysML v2, written in this format: sysml, kerml, ttl, turtle, rdf or api-json. Ledgered, not lossless: every element is mapped, approximated or left unmapped, and -migration-report says which (experimental)")
		fs.StringVar(&migrationReport, "migration-report", "", "With -migrate, write the element-by-element migration report to this file: JSON when it ends in .json, text otherwise")
		fs.StringVar(&migrationResults, "migration-results", "", "With -migrate, write the run configurations and the result snapshots the simulation tool stored for them to this JSON file, for -compare-results to read against the migrated model")
		fs.StringVar(&layoutPath, "layout", "", "With -migrate, read this MTIP export (HUDS XML) and write the diagram geometry it records as DiagramLayout annotations in the migrated views")
		fs.StringVar(&imageBaseURL, "image-base-url", "", "With -migrate, the http(s) URL a comment's relative <img src> is resolved against, such as the View Editor server")
	})
}

// runMigrate migrates the SysML v1 model named on the command line to v2,
// written in the format -migrate asks for, to -o or to stdout. The migration
// is ledgered, not lossless: every v1 element is mapped, approximated or left
// unmapped, and the report (-migration-report, or its summary on stderr)
// says which. -o may name a Flexo branch URL, the place a Turtle migration is
// pushed.
func runMigrateExit(files []string) int {
	status, err := runMigrate(files)
	if err != nil {
		return fail(err)
	}
	return status
}

func runMigrate(files []string) (int, error) {
	to, err := parseTargetFormat(migrateFormat, "-migrate")
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, errors.New("no model to migrate; name the SysML v1 model to migrate, as `sysml Model.mdzip -migrate sysml -o Model.sysml`")
	}
	if len(files) > 1 {
		return 0, fmt.Errorf("-migrate migrates one SysML v1 model per run, and %d files were named; a .mdzip or .xmi export holds the whole project", len(files))
	}
	input := files[0]
	if status, handled, err := migrateBranch(input, to, migrateInput); handled {
		return status, err
	}
	if syncState != "" {
		return 0, fmt.Errorf("-sync-state records a repository branch's head; -o %s does not name a branch", outputPath)
	}

	from, err := resolveFormat(fromFormat, input)
	if err != nil {
		return 0, err
	}
	if err := requireV1(from, input, to); err != nil {
		return 0, err
	}
	name, data, err := project.ReadFile(input)
	if err != nil {
		return 0, err
	}
	for _, notice := range convert.Notices(from, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	if err := migrateFlagsMisuse(input); err != nil {
		return 0, err
	}
	made, err := migrateInput(name, data, from, to)
	if err != nil {
		return 0, err
	}
	if err := writeConverted(input, to, made); err != nil {
		return 0, err
	}
	return exitHolds, nil
}

// migrateFlagsMisuse refuses the flag combinations that would lose the v1
// model, its report, its results or its layout export: none may be written
// over another, and the migration may not replace the model it reads.
func migrateFlagsMisuse(input string) error {
	if migrationReport != "" && outputPath != "" && samePath(migrationReport, outputPath) {
		return fmt.Errorf("-migration-report and -o both name %s; the report would be replaced by the model", outputPath)
	}
	if migrationReport != "" && input != "-" && samePath(migrationReport, input) {
		return fmt.Errorf("-migration-report names the model being migrated, %s; the report would replace it", input)
	}
	if err := migrationResultsMisuse(input); err != nil {
		return err
	}
	if err := layoutMisuse(input); err != nil {
		return err
	}
	if outputPath != "" && input != "-" && samePath(outputPath, input) {
		return fmt.Errorf("-o names the model being migrated, %s; the v1 model would be replaced by its migration", input)
	}
	return nil
}

// migrateInput runs the migration -migrate asks for. Its report and results
// are written by the sidecars, once the destination has taken the model.
func migrateInput(name string, data []byte, _ convert.Format, to convert.Format) (produced, error) {
	migOpts, err := migrationOptions()
	if err != nil {
		return produced{}, err
	}
	migrated, err := convert.Migrate(name, data, to, migOpts)
	if err != nil {
		return produced{}, err
	}
	return produced{
		out:   migrated.Output,
		files: migrated.Files,
		sidecars: func() error {
			if err := writeMigrationReport(migrated.Report); err != nil {
				return err
			}
			return writeMigrationResults(migrated.Results)
		},
	}, nil
}

// migrationOptions reads the -layout MTIP export into the migration's
// options; none were given when the flag was not passed.
func migrationOptions() (convert.MigrateOptions, error) {
	if layoutPath == "" {
		return convert.MigrateOptions{ImageBaseURL: imageBaseURL, Strict: strictMode, Portable: portableMode}, nil
	}
	data, err := os.ReadFile(layoutPath)
	if err != nil {
		return convert.MigrateOptions{}, err
	}
	layout, err := mtip.Parse(data)
	if err != nil {
		return convert.MigrateOptions{}, fmt.Errorf("%s: %w", layoutPath, err)
	}
	return convert.MigrateOptions{Layout: layout, LayoutSource: layoutPath, ImageBaseURL: imageBaseURL, Strict: strictMode, Portable: portableMode}, nil
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
