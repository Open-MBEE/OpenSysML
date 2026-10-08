package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
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
// A SysML v1 model (-from xmi, or a .xmi/.uml/.mdzip file) is refused: it is
// migrated, not converted, and -migrate is the verb for that (runMigrate).
//
// Either side may name a Flexo branch URL instead of a file: read as its RDF
// graph, or the place a Turtle conversion is pushed.
func runConvertExit(files []string) int {
	status, err := runConvert(files)
	if err != nil {
		return fail(err)
	}
	return status
}

func runConvert(files []string) (int, error) {
	to, err := parseTargetFormat(convertFormat, "-convert")
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, errors.New("no model to convert; name the file to convert, as `sysml model.sysml -convert ttl`")
	}
	if len(files) > 1 && len(dataImports) > 0 {
		return 0, errors.New("-import sets values in one model file; name a single file to convert")
	}
	if len(files) > 1 {
		return convertModel(files, to)
	}
	input := files[0]

	// A run asked to record puts the records in the session's buffer rather than
	// in the file, so what is converted is that buffer's text, as %save writes it.
	if len(modelChecks.records) > 0 || len(dataImports) > 0 {
		if err := recordedConvertMisuse(input); err != nil {
			return 0, err
		}
		return convertRecorded(input, to)
	}

	if status, handled, err := convertBranch(input, to); handled {
		return status, err
	}
	if syncState != "" {
		return 0, fmt.Errorf("-sync-state records a repository branch's head; neither %s nor -o names a branch", input)
	}

	from, err := resolveFormat(fromFormat, input)
	if err != nil {
		return 0, err
	}
	if err := refuseV1(from, input, to); err != nil {
		return 0, err
	}
	name, data, err := project.ReadFile(input)
	if err != nil {
		return 0, err
	}
	// Reported before the conversion, so a refusal carries it too, and on stderr,
	// where it cannot land in the converted model written to stdout.
	for _, notice := range convert.Notices(from, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	// A v2 model may be rewritten in place; an FMU's archive, read in place,
	// would be lost.
	if from == convert.FormatFMU && outputPath != "" && input != "-" && samePath(outputPath, input) {
		return 0, fmt.Errorf("-o names the FMU being imported, %s; the archive would be replaced by its import", input)
	}
	made, err := convertInput(name, data, from, to)
	if err != nil {
		return 0, err
	}
	if err := writeConverted(input, to, made); err != nil {
		return 0, err
	}
	return exitHolds, nil
}

// requireV1 is why -migrate refuses input that is not a SysML v1 model: a v2
// model, an RDF graph or an FMU is converted, and -convert is the verb for that.
func requireV1(from convert.Format, input string, to convert.Format) error {
	if from == convert.FormatXMI {
		return nil
	}
	remedy := fmt.Sprintf("write `sysml %s -convert %s`", input, to)
	if fromFormat != "" {
		remedy = fmt.Sprintf("write `sysml %s -from %s -convert %s`", input, fromFormat, to)
	}
	return &convert.NotV1Error{Name: inputLabel(input), Format: from, Remedy: remedy}
}

// refuseV1 is why -convert refuses a SysML v1 model: it is migrated, not
// converted, and the remedy names the -migrate run that does it, its report
// beside the model. A build made without the migration has no such run to name.
func refuseV1(from convert.Format, input string, to convert.Format) error {
	if from != convert.FormatXMI {
		return nil
	}
	if !v1Feature.linked {
		return fmt.Errorf("%s is a SysML v1 model, which this build cannot migrate (built without v1)", inputLabel(input))
	}
	stem := "model"
	source := input
	named := ""
	if fromFormat != "" {
		named = " -from " + fromFormat
	}
	if !project.IsStdin(input) {
		stem = strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		if stem == "" {
			stem = "model"
		}
	}
	remedy := fmt.Sprintf("write `sysml %s%s -migrate %s -o %s.%s -migration-report %s.report.txt`", source, named, to, stem, to, stem)
	return &convert.NotMigratedError{Name: inputLabel(input), Remedy: remedy}
}

// inputLabel names the input in a diagnostic: its path, or standard input.
func inputLabel(input string) string {
	if project.IsStdin(input) {
		return "standard input"
	}
	return input
}

// writeConverted writes the converted model to stdout or -o, the images a
// migration wrote beside the file, none allowed to replace an input. The
// migration's sidecars are written once the destination is known to take the
// model, so a refused run leaves no report or results behind.
func writeConverted(input string, to convert.Format, made produced) error {
	if outputPath == "" {
		if len(made.files) > 0 {
			return fmt.Errorf("the migration wrote %d image file(s); -o a local file path is required to write them", len(made.files))
		}
		if err := made.writeSidecars(); err != nil {
			return err
		}
		_, err := os.Stdout.Write(made.out)
		return err
	}
	target, info, err := export.Destination(outputPath)
	if err != nil {
		return err
	}
	if len(made.files) == 0 {
		if err := made.writeSidecars(); err != nil {
			return err
		}
		return writeConversion(outputPath, made.out, to)
	}
	if info != nil && !info.Mode().IsRegular() {
		return fmt.Errorf("-o names %s, which is not a file; the migration writes a model file and the images beside it", outputPath)
	}
	for _, name := range slices.Sorted(maps.Keys(made.files)) {
		dest := filepath.Join(filepath.Dir(target), filepath.FromSlash(name))
		for _, protected := range []string{target, input, migrationReport, migrationResults} {
			if protected != "" && protected != "-" && samePath(dest, protected) {
				return fmt.Errorf("the migration's image %s would replace %s", dest, protected)
			}
		}
	}
	if err := made.writeSidecars(); err != nil {
		return err
	}
	return writeMigrationFiles(outputPath, made.out, to, info != nil, filepath.Dir(target), made.files)
}

// produced is what a producer made of its input: the model in the to format,
// the attached image files a migration wrote for its document Image blocks
// (nil for a conversion), and sidecars, which writes the migration's report
// and results where -migration-report and -migration-results name (nil for a
// conversion). The caller runs sidecars only once the destination has
// accepted the model and its files.
type produced struct {
	out      []byte
	files    map[string][]byte
	sidecars func() error
}

// writeSidecars writes the migration's report and results, nothing for a conversion.
func (p produced) writeSidecars() error {
	if p.sidecars == nil {
		return nil
	}
	return p.sidecars()
}

// producer writes the input read as from in the to format: convertInput for
// -convert, migrateInput for -migrate.
type producer func(name string, data []byte, from, to convert.Format) (produced, error)

// convertInput runs the conversion -convert asks for. A SysML v1 model was
// refused before anything was read.
func convertInput(name string, data []byte, from, to convert.Format) (produced, error) {
	opts, err := convertOptions(from, to)
	if err != nil {
		return produced{}, err
	}
	out, err := convert.ConvertWith(name, data, from, to, opts)
	if err != nil {
		return produced{}, err
	}
	return produced{out: out}, nil
}

// writeMigrationFiles writes a migration's model and its image files as one
// set: all are staged before any is committed, the model first.
func writeMigrationFiles(path string, out []byte, to convert.Format, replaced bool, dir string, files map[string][]byte) error {
	var staged []*export.Staged
	discard := func() {
		for _, s := range staged {
			s.Discard()
		}
	}
	model, err := export.Stage(path, out)
	if err != nil {
		return err
	}
	staged = append(staged, model)
	names := slices.Sorted(maps.Keys(files))
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(paths[i]), 0o750); err != nil {
			discard()
			return err
		}
		s, err := export.Stage(paths[i], files[name])
		if err != nil {
			discard()
			return err
		}
		staged = append(staged, s)
	}
	if err := model.Commit(); err != nil {
		discard()
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, to, len(out), what)
	for i, name := range names {
		if err := staged[i+1].Commit(); err != nil {
			for _, s := range staged[i+2:] {
				s.Discard()
			}
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s (image file, %d bytes)\n", paths[i], len(files[name]))
	}
	return nil
}

// sessionConverts names the flag that makes -convert write the session's buffer.
func sessionConverts() string {
	if len(modelChecks.records) > 0 {
		return "-record-run converts the recorded session model"
	}
	return "-import converts the imported session model"
}

// recordedConvertMisuse is why a flag cannot share the run -record-run
// converts: what is converted is the session the records join, not a branch
// read or pushed.
func recordedConvertMisuse(input string) error {
	inRef, inputIsURL, err := branchURL(input)
	if err != nil {
		return err
	}
	if inputIsURL {
		return fmt.Errorf("%s; a repository branch is not an input it reads (%s)", sessionConverts(), inRef)
	}
	if outputPath != "" {
		outRef, outputIsURL, err := branchURL(outputPath)
		if err != nil {
			return err
		}
		if outputIsURL {
			return fmt.Errorf("%s; -o cannot push it to a repository branch (%s)", sessionConverts(), outRef)
		}
	}
	if syncState != "" {
		return fmt.Errorf("%s; -sync-state does not apply", sessionConverts())
	}
	return nil
}

// convertRecorded loads the file, makes the runs -record-run names so the
// records join the session's buffer, and converts that text; a load that did
// not analyse or a run that failed converts nothing.
func convertRecorded(input string, to convert.Format) (int, error) {
	if fromFormat != "" && fromFormat != "sysml" {
		if len(modelChecks.records) == 0 {
			return 0, fmt.Errorf("-import writes into SysML notation; -from %s does not apply", fromFormat)
		}
		return 0, fmt.Errorf("-record-run records into SysML notation; -from %s does not apply", fromFormat)
	}
	sess := newSession()
	report, err := sess.LoadPathsReport([]string{input})
	if err != nil {
		return 0, err
	}
	writeLines(os.Stderr, report.Loaded)
	writeLines(os.Stderr, report.Found)
	writeLines(os.Stderr, report.Declared)
	if report.Errors {
		return 0, fmt.Errorf("%s did not analyse cleanly; nothing was converted", input)
	}
	// The objects -instantiate names are materialized first, so a run named on
	// one has it to record.
	for _, name := range modelChecks.instantiate {
		created, err := sess.InstantiateReport(name)
		if err != nil {
			return 0, err
		}
		writeLines(os.Stderr, created.Lines)
		if len(created.FeatureValueErrors) > 0 {
			writeLines(os.Stderr, created.FeatureValueErrors)
			return 0, fmt.Errorf("%s did not materialize cleanly; nothing was converted", name)
		}
	}
	if err := importInto(sess); err != nil {
		return 0, err
	}
	for _, invocation := range modelChecks.records {
		verdict := modelChecks.record(sess, invocation)
		writeLines(os.Stderr, verdict.Lines)
		if verdict.Status != repl.VerdictHolds {
			return 0, fmt.Errorf("%s: the run was not recorded; nothing was converted", invocation)
		}
	}
	opts, err := convertOptions(convert.FormatSysML, to)
	if err != nil {
		return 0, err
	}
	out, tolerated, err := convert.ConvertTolerantWith(repl.SessionOrigin, []byte(sess.Text()), convert.FormatSysML, to, opts)
	if err != nil {
		return 0, err
	}
	if tolerated != nil {
		for _, line := range strings.Split("warning: "+tolerated.Error(), "\n") {
			fmt.Fprintln(os.Stderr, line)
		}
	}
	if outputPath != "" {
		return exitHolds, writeConversion(outputPath, out, to)
	}
	_, err = os.Stdout.Write(out)
	return exitHolds, err
}

// convertOptions are the conversion settings -id asks for, refusing it for a
// direction it does not apply to.
func convertOptions(from, to convert.Format) (convert.Options, error) {
	opts := convert.Options{Warn: func(message string) {
		fmt.Fprintf(os.Stderr, "warning: %s\n", message)
	}}
	if idForm == "" {
		return opts, nil
	}
	if from != convert.FormatSysML || (to != convert.FormatTurtle && to != convert.FormatAPIJSON) {
		return opts, fmt.Errorf("-id applies to -convert ttl or api-json from SysML notation")
	}
	form, ok := export.ParseIDForm(idForm)
	if !ok {
		return opts, fmt.Errorf("-id wants qualified or uuid, not %q", idForm)
	}
	opts.ID = form
	return opts, nil
}

// writeConversion writes converted output to a file and reports it.
func writeConversion(path string, out []byte, to convert.Format) error {
	replaced, err := export.WriteFile(path, out)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes%s)\n", path, to, len(out), what)
	return nil
}

// layoutMisuse reports why -layout would be lost: the export must not name a
// file the run rewrites.
func layoutMisuse(input string) error {
	switch {
	case layoutPath == "":
		return nil
	case outputPath != "" && samePath(layoutPath, outputPath):
		return fmt.Errorf("-layout and -o both name %s; the model would replace the layout export", outputPath)
	case migrationReport != "" && samePath(layoutPath, migrationReport):
		return fmt.Errorf("-layout and -migration-report both name %s; the report would replace the layout export", migrationReport)
	case migrationResults != "" && samePath(layoutPath, migrationResults):
		return fmt.Errorf("-layout and -migration-results both name %s; the results would replace the layout export", migrationResults)
	case input != "-" && samePath(layoutPath, input):
		return fmt.Errorf("-layout names the model being migrated, %s; the migration would replace it", input)
	}
	return nil
}

// migrationResultsMisuse reports why -migration-results would be lost: the
// sidecar must not replace the model or the report.
func migrationResultsMisuse(input string) error {
	switch {
	case migrationResults == "":
		return nil
	case outputPath != "" && samePath(migrationResults, outputPath):
		return fmt.Errorf("-migration-results and -o both name %s; the results would be replaced by the model", outputPath)
	case migrationReport != "" && samePath(migrationResults, migrationReport):
		return fmt.Errorf("-migration-results and -migration-report both name %s; the results would be replaced by the report", migrationReport)
	case input != "-" && samePath(migrationResults, input):
		return fmt.Errorf("-migration-results names the model being migrated, %s; the results would replace it", input)
	}
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

// parseTargetFormat resolves the value of flag (-convert or -migrate),
// explaining the flag when a file name was passed where a format belongs — the
// spelling -convert used to take.
func parseTargetFormat(value, flag string) (convert.Format, error) {
	f, err := convert.ParseFormat(value)
	if err != nil && namesAFile(value) {
		return 0, fmt.Errorf("%w; %s names the format to write, so write `sysml %s %s ttl`", err, flag, value, flag)
	}
	if err == nil && !f.Writable() {
		return 0, &convert.NotWritableError{Format: f, Migrating: flag == "-migrate"}
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
func resolveFormat(flagValue, path string) (convert.Format, error) {
	if flagValue != "" {
		return convert.ParseFormat(flagValue)
	}
	if project.IsStdin(path) {
		return 0, errors.New("standard input carries no file name to take the format from; name it with -from, as `-from sysml`")
	}
	f, err := convert.FormatOfPath(path)
	return f, convert.Advise(err, "pass -from, or "+convert.ExtensionAdvice)
}

// convertModel converts several notation files as one model, each reference
// from one file to an element another declares linked to that element
// (convert.ConvertModel), to Turtle or the API's JSON element form. The
// options of a single conversion that read or write a branch, migrate a v1
// model or write its images do not apply to it.
func convertModel(files []string, to convert.Format) (int, error) {
	if len(modelChecks.records) > 0 || migrationReport != "" || migrationResults != "" || syncState != "" {
		return 0, errors.New("a model of several files converts on its own: -record, -migration-report, -migration-results and -sync-state take one file")
	}
	if outputPath != "" {
		if _, isURL, err := branchURL(outputPath); err != nil {
			return 0, err
		} else if isURL {
			return 0, fmt.Errorf("-o %s: a model of several files is written to a file; a repository branch is pushed from one file", outputPath)
		}
	}
	for _, file := range files {
		if _, isURL, err := branchURL(file); err != nil || isURL {
			return 0, fmt.Errorf("%s: a model of several files converts files, not a repository branch", file)
		}
		from, err := resolveFormat(fromFormat, file)
		if err != nil {
			return 0, err
		}
		if from != convert.FormatSysML {
			return 0, fmt.Errorf("%s: a model of several files converts SysML or KerML notation, not %s", file, from)
		}
		if outputPath != "" && samePath(outputPath, file) {
			return 0, fmt.Errorf("-o names one of the model's files, %s; the %s would replace it", file, to)
		}
	}
	inputs := make([]convert.Input, 0, len(files))
	for _, file := range files {
		name, data, err := project.ReadFile(file)
		if err != nil {
			return 0, err
		}
		inputs = append(inputs, convert.Input{Name: name, Data: data})
	}
	for _, notice := range convert.Notices(convert.FormatSysML, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	opts, err := convertOptions(convert.FormatSysML, to)
	if err != nil {
		return 0, err
	}
	out, err := convert.ConvertModel(inputs, to, opts)
	if err != nil {
		return 0, err
	}
	if outputPath == "" {
		_, err := os.Stdout.Write(out)
		return exitHolds, err
	}
	if err := writeConversion(outputPath, out, to); err != nil {
		return 0, err
	}
	return exitHolds, nil
}
