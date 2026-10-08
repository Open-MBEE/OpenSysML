//go:build !sysml_prod && !sysml_nosync

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// convertBranch carries out a conversion one side of which names a repository
// branch; handled is false when neither side does.
func convertBranch(input string, to convert.Format) (status int, handled bool, err error) {
	inputRef, inputIsURL, err := flexo.ParseBranchURL(input)
	if err != nil {
		return 0, true, err
	}
	outputRef := flexo.BranchRef{}
	outputIsURL := false
	if outputPath != "" {
		if outputRef, outputIsURL, err = flexo.ParseBranchURL(outputPath); err != nil {
			return 0, true, err
		}
	}
	switch {
	case inputIsURL && outputIsURL:
		return 0, true, errors.New("a repository branch can be read or pushed in one run, not both; write the branch to a file, or convert a file to the branch")
	case outputIsURL:
		status, err := pushBranch(input, to, outputRef, "-convert", convertInput)
		return status, true, err
	case inputIsURL:
		status, err := readBranch(inputRef, to)
		return status, true, err
	}
	return 0, false, nil
}

// migrateBranch carries out a migration whose -o names a repository branch,
// the place the migration is pushed by produce; a branch as input is refused,
// since it holds a v2 graph, which is converted. handled is false when
// neither side names a branch.
func migrateBranch(input string, to convert.Format, produce producer) (status int, handled bool, err error) {
	inputRef, inputIsURL, err := flexo.ParseBranchURL(input)
	if err != nil {
		return 0, true, err
	}
	if inputIsURL {
		return 0, true, fmt.Errorf("a repository branch holds a SysML v2 graph, which is converted, not migrated; write `sysml %s -convert %s`", inputRef, to)
	}
	if outputPath == "" {
		return 0, false, nil
	}
	outputRef, outputIsURL, err := flexo.ParseBranchURL(outputPath)
	if err != nil {
		return 0, true, err
	}
	if !outputIsURL {
		return 0, false, nil
	}
	status, err = pushBranch(input, to, outputRef, "-migrate", produce)
	return status, true, err
}

// branchURL reports whether path names a repository branch, and the branch as
// a message names it.
func branchURL(path string) (string, bool, error) {
	ref, ok, err := flexo.ParseBranchURL(path)
	return fmt.Sprintf("%s", ref), ok, err
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

// pushBranch replaces a branch's model graph with the input written as Turtle
// by produce — converted under -convert, migrated under -migrate, which verb
// names; a sync state the head moved past refuses the write.
func pushBranch(input string, to convert.Format, ref flexo.BranchRef, verb string, produce producer) (int, error) {
	if to != convert.FormatTurtle {
		return 0, fmt.Errorf("a repository branch holds a graph; %s ttl to push, not %s", strings.TrimPrefix(verb, "-"), to)
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
	if verb == "-migrate" {
		err = requireV1(from, input, to)
	} else {
		err = refuseV1(from, input, to)
	}
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
	if err := migrationResultsMisuse(input); err != nil {
		return 0, err
	}
	if err := layoutMisuse(input); err != nil {
		return 0, err
	}
	made, err := produce(name, data, from, to)
	if err != nil {
		return 0, err
	}
	if len(made.files) > 0 {
		return 0, fmt.Errorf("the migration wrote %d image file(s); a repository branch cannot hold them: -o a local file path is required", len(made.files))
	}
	if err := made.writeSidecars(); err != nil {
		return 0, err
	}
	out := made.out
	head, err := repo.Push(context.Background(), out, "sysml "+verb+" ttl")
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
