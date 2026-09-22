package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
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
// A SysML v1 model (-from xmi, or a .xmi/.uml/.mdzip file) is migrated to v2 on the
// way in; see writeMigrationReport for where its report goes.
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
	to, err := parseTargetFormat(convertFormat)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, errors.New("no model to convert; name the file to convert, as `sysml model.sysml -convert ttl`")
	}
	if len(files) > 1 {
		return 0, fmt.Errorf("-convert converts one file; unexpected extra argument %q", files[1])
	}
	input := files[0]

	inputRef, inputIsURL, err := flexo.ParseBranchURL(input)
	if err != nil {
		return 0, err
	}
	outputRef := flexo.BranchRef{}
	outputIsURL := false
	if outputPath != "" {
		if outputRef, outputIsURL, err = flexo.ParseBranchURL(outputPath); err != nil {
			return 0, err
		}
	}
	switch {
	case inputIsURL && outputIsURL:
		return 0, errors.New("a repository branch can be read or pushed in one run, not both; write the branch to a file, or convert a file to the branch")
	case outputIsURL:
		return pushBranch(input, to, outputRef)
	case inputIsURL:
		return readBranch(inputRef, to)
	}
	if syncState != "" {
		return 0, fmt.Errorf("-sync-state records a repository branch's head; neither %s nor -o names a branch", input)
	}

	from, err := resolveFormat(fromFormat, input)
	if err != nil {
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
	if migrationReport != "" && from != convert.FormatXMI {
		return 0, fmt.Errorf("-migration-report describes a SysML v1 migration, and %s input is not migrated; pass it with -from xmi or a .xmi/.uml/.mdzip file", from)
	}
	if migrationReport != "" && outputPath != "" && samePath(migrationReport, outputPath) {
		return 0, fmt.Errorf("-migration-report and -o both name %s; the report would be replaced by the model", outputPath)
	}
	if migrationReport != "" && input != "-" && samePath(migrationReport, input) {
		return 0, fmt.Errorf("-migration-report names the model being migrated, %s; the report would replace it", input)
	}
	if err := migrationResultsMisuse(from, input); err != nil {
		return 0, err
	}
	// A v2 model may be rewritten in place; a v1 model would be lost.
	if from == convert.FormatXMI && outputPath != "" && input != "-" && samePath(outputPath, input) {
		return 0, fmt.Errorf("-o names the model being migrated, %s; the v1 model would be replaced by its migration", input)
	}
	out, err := convertInput(name, data, from, to)
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

// convertInput runs the conversion the input format asks for: a SysML v1 model
// is migrated and its report written, anything else converted.
func convertInput(name string, data []byte, from, to convert.Format) ([]byte, error) {
	if from != convert.FormatXMI {
		return convert.Convert(name, data, from, to)
	}
	migrated, err := convert.Migrate(name, data, to)
	if err != nil {
		return nil, err
	}
	if err := writeMigrationReport(migrated.Report); err != nil {
		return nil, err
	}
	if err := writeMigrationResults(migrated.Results); err != nil {
		return nil, err
	}
	return migrated.Output, nil
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

// openBranch resolves a branch URL's repository under the shared bearer token;
// an http(s) form naming another endpoint would split reads and writes across stacks.
func openBranch(ref flexo.BranchRef) (*flexo.Repository, flexo.Config, error) {
	cfg, err := flexo.ConfigFromEnv()
	if err != nil {
		return nil, flexo.Config{}, fmt.Errorf("a repository branch needs its bearer token: %w", err)
	}
	if ref.SysMLV2URL != "" && !flexo.SameEndpoint(ref.SysMLV2URL, cfg.SysMLV2URL) {
		return nil, flexo.Config{}, fmt.Errorf("%s names a SysML v2 endpoint other than the configured %s (%s); point %s and %s at that stack together, or write flexo://%s/%s",
			ref.SysMLV2URL, cfg.SysMLV2URL, flexo.EnvSysMLV2URL, flexo.EnvSysMLV2URL, flexo.EnvLayer1URL, ref.Project, ref.Branch)
	}
	if err := cfg.CheckTransport(); err != nil {
		return nil, flexo.Config{}, err
	}
	return flexo.New(cfg).Repository(ref.Project, ref.Branch), cfg, nil
}

// readBranch converts a repository branch to -convert's format: the branch is
// read as its head commit's RDF graph.
func readBranch(ref flexo.BranchRef, to convert.Format) (int, error) {
	if fromFormat != "" {
		if f, err := convert.ParseFormat(fromFormat); err != nil {
			return 0, err
		} else if f != convert.FormatTurtle {
			return 0, fmt.Errorf("a repository branch is read as its RDF graph; -from %s does not apply", fromFormat)
		}
	}
	for _, notice := range convert.Notices(convert.FormatTurtle, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	if migrationReport != "" {
		return 0, fmt.Errorf("-migration-report describes a SysML v1 migration, and a repository branch is not migrated; pass it with -from xmi or a .xmi/.uml/.mdzip file")
	}
	if migrationResults != "" {
		return 0, fmt.Errorf("-migration-results indexes the result snapshots of a SysML v1 migration, and a repository branch is not migrated; pass it with -from xmi or a .xmi/.uml/.mdzip file")
	}
	repo, cfg, err := openBranch(ref)
	if err != nil {
		return 0, err
	}
	// Resolve and check the state file before anything is written: -o must
	// never replace it, and a state pinned elsewhere refuses first.
	statePath := syncState
	if statePath == "" && outputPath != "" {
		statePath = reposync.StatePath(outputPath)
	}
	if statePath != "" && outputPath != "" && samePath(statePath, outputPath) {
		return 0, fmt.Errorf("-o and -sync-state both name %s; the model would replace the recorded commit", outputPath)
	}
	var state *reposync.State
	if statePath != "" {
		if state, err = reposync.LoadState(statePath); err != nil {
			return 0, err
		}
	}
	scope := reposync.Scope{Org: cfg.Org, ProjectID: ref.Project, Branch: ref.Branch}
	if state != nil {
		if err := state.Check(scope); err != nil {
			return 0, err
		}
	}
	graph, err := repo.Graph(context.Background())
	if err != nil {
		return failRepository(fmt.Errorf("read the repository: %w", err)), nil
	}
	out, err := convert.FromGraph(graph, to)
	if err != nil {
		return 0, err
	}
	if outputPath == "" {
		if _, err := os.Stdout.Write(out); err != nil {
			return 0, err
		}
	} else if err := writeConversion(outputPath, out, to); err != nil {
		return 0, err
	}
	if statePath == "" {
		return exitHolds, nil
	}
	return recordBranchState(repo.Seen(), state, scope, statePath)
}

// pushBranch replaces a branch's model graph with the model converted to
// Turtle; a sync state the head moved past refuses the write.
func pushBranch(input string, to convert.Format, ref flexo.BranchRef) (int, error) {
	if to != convert.FormatTurtle {
		return 0, fmt.Errorf("a repository branch holds a graph; convert to ttl to push, not %s", to)
	}
	statePath := syncState
	if statePath == "" {
		if project.IsStdin(input) {
			return 0, errors.New("a push records the commit it makes beside the model; with the model on stdin, name the state file with -sync-state")
		}
		statePath = reposync.StatePath(input)
	}
	if migrationReport != "" && input != "-" && samePath(migrationReport, input) {
		return 0, fmt.Errorf("-migration-report names the model being migrated, %s; the report would replace it", input)
	}
	if migrationReport != "" && samePath(migrationReport, statePath) {
		return 0, fmt.Errorf("-migration-report and the sync state both name %s; the report would be replaced by the recorded commit", statePath)
	}
	if migrationResults != "" && samePath(migrationResults, statePath) {
		return 0, fmt.Errorf("-migration-results and the sync state both name %s; the results would be replaced by the recorded commit", statePath)
	}
	repo, cfg, err := openBranch(ref)
	if err != nil {
		return 0, err
	}
	scope := reposync.Scope{Org: cfg.Org, ProjectID: ref.Project, Branch: ref.Branch}
	state, err := reposync.LoadState(statePath)
	if err != nil {
		return 0, err
	}
	if state != nil {
		if err := state.Check(scope); err != nil {
			return 0, err
		}
		repo.Resume(state.LastSeenCommit)
	}
	from, err := resolveFormat(fromFormat, input)
	if err != nil {
		return 0, err
	}
	name, data, err := project.ReadFile(input)
	if err != nil {
		return 0, err
	}
	for _, notice := range convert.Notices(from, to) {
		fmt.Fprintf(os.Stderr, "note: %s\n", notice)
	}
	if migrationReport != "" && from != convert.FormatXMI {
		return 0, fmt.Errorf("-migration-report describes a SysML v1 migration, and %s input is not migrated; pass it with -from xmi or a .xmi/.uml/.mdzip file", from)
	}
	if err := migrationResultsMisuse(from, input); err != nil {
		return 0, err
	}
	out, err := convertInput(name, data, from, to)
	if err != nil {
		return 0, err
	}
	head, err := repo.Push(context.Background(), out, "sysml -convert ttl")
	if err != nil {
		var stale *flexo.StaleBranchError
		var unrecorded *flexo.UnrecordedPushError
		var superseded *flexo.SupersededPushError
		switch {
		case errors.As(err, &stale):
			fmt.Fprintf(os.Stderr, "%srefused to push: %v\n", commandPrefix, err)
			return exitFailed, nil
		case errors.As(err, &unrecorded), errors.As(err, &superseded):
			fmt.Fprintf(os.Stderr, "%s%v\n", commandPrefix, err)
			return exitFailed, nil
		}
		return failRepository(fmt.Errorf("push to the repository: %w", err)), nil
	}
	if state == nil {
		state = &reposync.State{}
	}
	state.Org, state.ProjectID, state.Branch, state.LastSeenCommit = cfg.Org, ref.Project, ref.Branch, head
	if err := state.Save(statePath); err != nil {
		return 0, fmt.Errorf("pushed %d bytes of Turtle as commit %s, but could not record it: %w", len(out), head, err)
	}
	fmt.Fprintf(os.Stderr, "pushed %d bytes of Turtle to branch %s of %s; head commit %s recorded in %s\n",
		len(out), ref.Branch, ref.Project, head, statePath)
	return exitHolds, nil
}

// recordBranchState writes the head commit the run stood at to the sync state
// file, over the state already loaded and checked for this scope.
func recordBranchState(head string, state *reposync.State, scope reposync.Scope, statePath string) (int, error) {
	if head == "" {
		return exitHolds, nil
	}
	if state != nil && state.Scope() == scope && state.LastSeenCommit == head {
		return exitHolds, nil
	}
	if state == nil {
		state = &reposync.State{}
	}
	state.Org, state.ProjectID, state.Branch, state.LastSeenCommit = scope.Org, scope.ProjectID, scope.Branch, head
	if err := state.Save(statePath); err != nil {
		return 0, fmt.Errorf("could not record head commit %s: %w", head, err)
	}
	fmt.Fprintf(os.Stderr, "head commit %s recorded in %s\n", head, statePath)
	return exitHolds, nil
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

// migrationResultsMisuse reports why -migration-results writes nothing: a v2 input
// has no tool results to index, and the sidecar must not replace the model or report.
func migrationResultsMisuse(from convert.Format, input string) error {
	switch {
	case migrationResults == "":
		return nil
	case from != convert.FormatXMI:
		return fmt.Errorf("-migration-results indexes the result snapshots of a SysML v1 migration, and %s input is not migrated; pass it with -from xmi or a .xmi/.uml/.mdzip file", from)
	case outputPath != "" && samePath(migrationResults, outputPath):
		return fmt.Errorf("-migration-results and -o both name %s; the results would be replaced by the model", outputPath)
	case migrationReport != "" && samePath(migrationResults, migrationReport):
		return fmt.Errorf("-migration-results and -migration-report both name %s; the results would be replaced by the report", migrationReport)
	case input != "-" && samePath(migrationResults, input):
		return fmt.Errorf("-migration-results names the model being migrated, %s; the results would replace it", input)
	}
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
func parseTargetFormat(value string) (convert.Format, error) {
	f, err := convert.ParseFormat(value)
	if err != nil && namesAFile(value) {
		return 0, fmt.Errorf("%w; -convert names the format to convert to, so write `sysml %s -convert ttl`", err, value)
	}
	if err == nil && !f.Writable() {
		return 0, &convert.NotWritableError{Format: f}
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
