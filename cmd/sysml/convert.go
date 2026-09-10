package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/core/project"
)

// deprecatedFlag rejects a flag that has been replaced, so the old spelling
// reports what to write instead of "flag provided but not defined".
type deprecatedFlag struct{ instead string }

func (f *deprecatedFlag) String() string { return "" }

func (f *deprecatedFlag) Set(string) error { return errors.New(f.instead) }

// runConvert converts the model named on the command line to the format
// -convert asks for, writing to -o or to stdout.
//
// The input format is taken from -from when given and from the file extension
// otherwise; the model itself is a positional argument, as it is for every other
// mode of the command, and a lone "-" names standard input.
//
// A SysML v1 model (-from xmi, or a .xmi/.uml file) is migrated to v2 on the
// way in; see writeMigrationReport for where its report goes.
func runConvert(files []string) error {
	to, err := parseTargetFormat(convertFormat)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no model to convert; name the file to convert, as `sysml model.sysml -convert ttl`")
	}
	if len(files) > 1 {
		return fmt.Errorf("-convert converts one file; unexpected extra argument %q", files[1])
	}
	input := files[0]

	from, err := resolveFormat(fromFormat, input)
	if err != nil {
		return err
	}

	name, data, err := project.ReadFile(input)
	if err != nil {
		return err
	}
	// Reported before the conversion, so a refusal carries it too, and on stderr,
	// where it cannot land in the converted model written to stdout.
	for _, notice := range export.Notices(from, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	if migrationReport != "" && from != export.FormatXMI {
		return fmt.Errorf("-migration-report describes a SysML v1 migration, and %s input is not migrated; pass it with -from xmi or a .xmi/.uml file", from)
	}
	if migrationReport != "" && outputPath != "" && samePath(migrationReport, outputPath) {
		return fmt.Errorf("-migration-report and -o both name %s; the report would be replaced by the model", outputPath)
	}
	if migrationReport != "" && input != "-" && samePath(migrationReport, input) {
		return fmt.Errorf("-migration-report names the model being migrated, %s; the report would replace it", input)
	}
	// A v2 model may be rewritten in place; a v1 model would be lost.
	if from == export.FormatXMI && outputPath != "" && input != "-" && samePath(outputPath, input) {
		return fmt.Errorf("-o names the model being migrated, %s; the v1 model would be replaced by its migration", input)
	}
	var out []byte
	if from == export.FormatXMI {
		var report *migrate.Report
		out, report, err = export.Migrate(name, data, to)
		if err != nil {
			return err
		}
		if err := writeMigrationReport(report); err != nil {
			return err
		}
	} else {
		out, err = export.Convert(name, data, from, to)
		if err != nil {
			return err
		}
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return err
	}
	replaced, err := export.WriteFile(outputPath, out)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", outputPath, to, len(out), what)
	return nil
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

// samePath reports whether a and b name one file, following symbolic links,
// including a dangling link to a file neither has written yet.
func samePath(a, b string) bool {
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	ra, errA := resolvePath(a)
	rb, errB := resolvePath(b)
	return errA == nil && errB == nil && ra == rb
}

// resolvePath returns the absolute path a write to path lands on: every
// symbolic link on the way is followed, whether or not its target exists.
func resolvePath(path string) (string, error) {
	for range 64 {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Abs(resolved)
		}
		dir, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			return "", err
		}
		path = filepath.Join(dir, filepath.Base(path))
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			return filepath.Abs(path)
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		path = target
	}
	return "", fmt.Errorf("%s: too many levels of symbolic links", path)
}

// parseTargetFormat resolves the -convert value, explaining the flag when a file
// name was passed where a format belongs — the spelling this flag used to take.
func parseTargetFormat(value string) (export.Format, error) {
	f, err := export.ParseFormat(value)
	if err != nil && namesAFile(value) {
		return 0, fmt.Errorf("%w; -convert names the format to convert to, so write `sysml %s -convert ttl`", err, value)
	}
	if err == nil && !f.Writable() {
		return 0, &export.NotWritableError{Format: f}
	}
	return f, err
}

// namesAFile reports whether the value looks like a path rather than a format
// name: it exists on disk, or is written with a directory or an extension.
func namesAFile(value string) bool {
	if _, err := os.Stat(value); err == nil {
		return true
	}
	return filepath.Ext(value) != "" || filepath.Dir(value) != "."
}

// resolveFormat returns the format named by the flag, or the one the path's
// extension implies. Standard input carries no extension to read it from, so
// -from is the only thing that can name its format.
func resolveFormat(flagValue, path string) (export.Format, error) {
	if flagValue != "" {
		return export.ParseFormat(flagValue)
	}
	if project.IsStdin(path) {
		return 0, errors.New("standard input carries no file name to take the format from; name it with -from, as `-from sysml`")
	}
	f, err := export.FormatOfPath(path)
	return f, export.Advise(err, "pass -from, or "+export.ExtensionAdvice)
}
