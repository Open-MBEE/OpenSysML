//go:build !sysml_prod && !sysml_nov1

package convert

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/exec/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// Migration is a v1 model written in another format, with the report of what
// each v1 element became and the results its simulation tool stored.
type Migration struct {
	Output  []byte
	Report  *migrate.Report
	Results *simresults.Results
	// Files are the attached image files the migration wrote for its document
	// Image blocks, by the relative path they belong under; a caller writes
	// them beside Output, empty when none was attached.
	Files map[string][]byte
}

// MigrateOptions carries a migration's augments, as Migrate takes them: an MTIP
// export whose diagram records lay out the views, the server a comment's
// relative image is resolved against, and strictness. It is migrate.Options,
// named here so a caller drives the migration through this package alone.
type MigrateOptions = migrate.Options

// MigrationReport is the ledger a migration writes: one entry per v1 element
// with its Verdict.
type MigrationReport = migrate.Report

// Verdict is what the report records for one v1 element.
type Verdict = migrate.Verdict

// The verdicts a migration report records.
const (
	Mapped       = migrate.Mapped
	Approximated = migrate.Approximated
	Unmapped     = migrate.Unmapped
	Skipped      = migrate.Skipped
)

// Migrate reads a SysML v1 model in XMI and writes it in the to format. opts
// carries the migration's augments: an MTIP export whose diagram records lay
// out the views the migration writes.
func Migrate(name string, data []byte, to Format, opts MigrateOptions) (*Migration, error) {
	if !to.Writable() {
		return nil, &NotWritableError{Format: to, Migrating: true}
	}
	result, err := migrate.MigrateOptions(name, data, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	out, _, err := convert(name+sysmlExt, result.Notation, FormatSysML, to, false, Options{})
	if err != nil {
		return nil, fmt.Errorf("the migrated notation could not be written: %w", err)
	}
	return &Migration{Output: out, Report: result.Report, Results: result.Results, Files: result.Files}, nil
}

// migrateTo writes a SysML v1 model in the to format, without augments.
func migrateTo(name string, data []byte, to Format) ([]byte, error) {
	m, err := Migrate(name, data, to, migrate.Options{})
	if err != nil {
		return nil, err
	}
	return m.Output, nil
}
