package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/flexo"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// runSyncDiff reports the identity-keyed change set against a graph file or a
// live endpoint on stdout, and never writes to a repository.
func runSyncDiff(files []string) int {
	return runSync(files, syncDiffWith, false)
}

// runSyncApply writes the change set to the live repository as commits and
// records the last one in the sync state; refusals and failures exit 1.
func runSyncApply(files []string) int {
	return runSync(files, syncApplyTo, true)
}

// isEndpoint tells a live SysML v2 API base URL from a graph file path.
func isEndpoint(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

// syncRepository is a graph file or a live project branch; only the live one
// has history and takes writes.
type syncRepository struct {
	graph func(context.Context) (*rdf.Graph, error)
	live  *flexo.Repository
}

func runSync(files []string, target string, apply bool) int {
	mode := "-sync-diff"
	if apply {
		mode = "-sync-apply"
	}
	if len(files) == 0 {
		return fail(fmt.Errorf("no model to sync; name the file, as `sysml model.sysml %s %s`", mode, target))
	}
	if len(files) > 1 {
		return fail(fmt.Errorf("%s syncs one model; unexpected extra argument %q", mode, files[1]))
	}
	if syncAnnotate != "" && !syncMintIDs {
		return fail(errors.New("-sync-annotate writes minted ids back into the notation, so it needs -sync-mint-ids"))
	}
	if apply && syncMintIDs && syncAnnotate == "" {
		return fail(errors.New("-sync-apply with -sync-mint-ids gives the repository ids the model does not declare; write them back with -sync-annotate"))
	}
	model := files[0]
	if apply && !isEndpoint(target) {
		return fail(fmt.Errorf("-sync-apply writes to a live repository; %q is not an http(s) endpoint", target))
	}
	statePath, err := syncStatePath(model, apply)
	if err != nil {
		return fail(err)
	}

	local, err := loadSyncGraph(model)
	if err != nil {
		return fail(err)
	}
	scope, err := reposync.GraphScope(local)
	if err != nil {
		return fail(err)
	}
	state, err := loadSyncState(statePath, scope)
	if err != nil {
		return fail(err)
	}
	ctx := context.Background()
	repository, opts, err := openSyncRepository(target, scope, state)
	if err != nil {
		return fail(err)
	}
	opts.ConfirmDeletes, opts.MintIDs = syncConfirmDeletes, syncMintIDs

	switch {
	case syncBase != "":
		if opts.Base, err = loadSyncGraph(syncBase); err != nil {
			return fail(err)
		}
	case repository.live != nil && state != nil && state.LastSeenCommit != "":
		if opts.Base, err = repository.live.GraphAt(ctx, state.LastSeenCommit); err != nil {
			return failRepository(fmt.Errorf("read the repository at last-seen commit %s: %w", state.LastSeenCommit, err))
		}
	}
	remote, err := repository.graph(ctx)
	if err != nil {
		if repository.live != nil {
			return failRepository(fmt.Errorf("read the repository: %w", err))
		}
		return fail(err)
	}

	set, err := reposync.Diff(local, remote, opts)
	if err != nil {
		return fail(err)
	}
	fmt.Print(set.Text())
	if state != nil && state.LastSeenCommit != "" && opts.Base == nil {
		fmt.Fprintf(os.Stderr, "note: the sync state last saw commit %s; pass its graph with -sync-base to surface repository changes since then as conflicts\n", state.LastSeenCommit)
	}

	var annotated *annotation
	if syncAnnotate != "" {
		if annotated, err = prepareAnnotation(local, model, set, syncAnnotate); err != nil {
			return fail(err)
		}
	}

	if !apply {
		if annotated != nil {
			for _, change := range annotated.unnamed {
				fmt.Fprintf(os.Stderr, "note: %s (%s) has no name to write an annotation against; its minted id %s stays in the repository only\n", change.ID, change.Metaclass, change.MintedID)
			}
			if len(annotated.clauses) == 0 {
				return fail(fmt.Errorf("nothing to annotate: no minted element has a name to write an annotation against, so %s would repeat the model unchanged", annotated.path))
			}
			if err := annotated.write(); err != nil {
				return fail(err)
			}
		}
		if err := set.Appliable(); err != nil {
			fmt.Fprintf(os.Stderr, "note: applying would be refused: %v\n", err)
		}
		if set.Conflicts() > 0 {
			return exitFailed
		}
		return exitHolds
	}
	return applySync(ctx, repository.live, set, state, scope, statePath, annotated)
}

// applySync writes an appliable change set and records its last commit; the
// state only moves past a complete apply, and the annotated model is written
// only once the repository holds the ids it declares.
func applySync(ctx context.Context, repo *flexo.Repository, set *reposync.ChangeSet, state *reposync.State, scope reposync.Scope, statePath string, annotated *annotation) int {
	if err := set.Appliable(); err != nil {
		fmt.Fprintf(os.Stderr, "%srefused to apply: %v\n", commandPrefix, err)
		return exitFailed
	}
	if annotated != nil && len(annotated.unnamed) > 0 {
		fmt.Fprintf(os.Stderr, "%srefused to apply: %v\n", commandPrefix, &UnannotatableError{Changes: annotated.unnamed})
		return exitFailed
	}
	result, err := reposync.Apply(ctx, repo, set, reposync.ApplyOptions{Message: "sysml -sync-apply"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sapply failed: %v\n", commandPrefix, err)
		if result != nil {
			reportFates(os.Stderr, result)
		}
		return exitFailed
	}
	if state == nil {
		state = &reposync.State{Org: scope.Org, ProjectID: scope.ProjectID, Branch: scope.Branch}
	}
	if len(result.Applied) == 0 {
		// An agreeing repository still fixes the baseline: the head just read is
		// what later repository changes are conflicts against.
		if head := repo.Seen(); head != state.LastSeenCommit {
			state.LastSeenCommit = head
			if err := state.Save(statePath); err != nil {
				return fail(fmt.Errorf("nothing to apply, but could not record head commit %s: %w", head, err))
			}
			fmt.Fprintf(os.Stderr, "nothing applied: the repository already agrees with the model; head commit %s recorded in %s\n", head, statePath)
			return exitHolds
		}
		fmt.Fprintln(os.Stderr, "nothing applied: the repository already agrees with the model")
		return exitHolds
	}
	if err := state.Advance(result); err != nil {
		return fail(err)
	}
	if err := state.Save(statePath); err != nil {
		return fail(fmt.Errorf("applied %d change(s) as commit %s, but could not record it: %w", len(result.Applied), result.LastCommit(), err))
	}
	fmt.Fprintf(os.Stderr, "applied %d change(s) in %d commit(s); last-seen commit %s recorded in %s\n",
		len(result.Applied), len(result.Commits), result.LastCommit(), statePath)
	if annotated != nil {
		if err := annotated.write(); err != nil {
			fmt.Fprintf(os.Stderr, "%scommit %s holds the minted ids, but the annotated model could not be written: %v\ndeclare them by hand so the next sync finds these elements again:\n", commandPrefix, result.LastCommit(), err)
			for _, clause := range annotated.clauses {
				fmt.Fprintf(os.Stderr, "  %s\n", clause)
			}
			return exitFailed
		}
	}
	return exitHolds
}

// UnannotatableError is an apply that would mint ids the notation cannot keep:
// the elements have no name for an about-form annotation to address.
type UnannotatableError struct {
	Changes []reposync.Change
}

func (e *UnannotatableError) Error() string {
	var ids []string
	for _, change := range e.Changes {
		ids = append(ids, fmt.Sprintf("%s (%s)", change.ID, change.Metaclass))
	}
	return fmt.Sprintf("%d minted id(s) could not be written back because the elements have no name: %s; name them, declare their ids, or apply without -sync-mint-ids",
		len(e.Changes), strings.Join(ids, ", "))
}

// failRepository reports a live repository operation that failed: status 1,
// as a failed apply, not the misuse status an unusable run gets.
func failRepository(err error) int {
	fmt.Fprintf(os.Stderr, "%s%v\n", commandPrefix, err)
	return exitFailed
}

// reportFates lists what an interrupted apply did and did not write.
func reportFates(w io.Writer, result *reposync.Result) {
	for _, fate := range []struct {
		label   string
		changes []reposync.Change
	}{{"applied", result.Applied}, {"failed", result.Failed}, {"never sent", result.Pending}} {
		for _, change := range fate.changes {
			fmt.Fprintf(w, "  %-10s %-8s %s\n", fate.label, change.Kind, change.ID)
		}
	}
	if commit := result.LastCommit(); commit != "" {
		fmt.Fprintf(w, "the last commit that landed is %s; the sync state was not advanced\n", commit)
	}
}

// openSyncRepository opens a graph file as-is, or the model's project branch on
// a live stack under the representation its commit path stores.
func openSyncRepository(target string, scope reposync.Scope, state *reposync.State) (syncRepository, reposync.Options, error) {
	if !isEndpoint(target) {
		return syncRepository{graph: func(context.Context) (*rdf.Graph, error) { return loadSyncGraph(target) }}, reposync.Options{}, nil
	}
	cfg, err := flexo.ConfigFromEnv()
	if err != nil {
		return syncRepository{}, reposync.Options{}, fmt.Errorf("a live repository needs its bearer token: %w", err)
	}
	cfg.SysMLV2URL = strings.TrimRight(target, "/")
	if err := cfg.CheckTransport(); err != nil {
		return syncRepository{}, reposync.Options{}, err
	}
	if scope.ProjectID == "" {
		return syncRepository{}, reposync.Options{}, errors.New("the model names no project; annotate it with @IdentityMetadata::ProjectRef { projectId = \"...\"; branch = \"...\"; }")
	}
	if scope.Org != "" && scope.Org != cfg.Org {
		return syncRepository{}, reposync.Options{}, fmt.Errorf("the model is scoped to org %q but the stack is configured for org %q (%s)", scope.Org, cfg.Org, flexo.EnvOrg)
	}
	branch := scope.Branch
	if branch == "" && state != nil {
		branch = state.Branch
	}
	if branch == "" {
		return syncRepository{}, reposync.Options{}, errors.New("a live sync reads one branch; name it in the ProjectRef annotation (branch = \"...\") or the sync state")
	}
	live := flexo.New(cfg).Repository(scope.ProjectID, branch)
	return syncRepository{graph: live.Graph, live: live}, reposync.Options{Representation: flexo.Representation{}}, nil
}

// syncStatePath is -sync-state or <model>.sync.json beside the model; a model
// on stdin has neither, which an apply cannot do without.
func syncStatePath(model string, apply bool) (string, error) {
	if syncState != "" {
		return syncState, nil
	}
	if project.IsStdin(model) {
		if apply {
			return "", errors.New("-sync-apply records the last-seen commit beside the model; with the model on stdin, name the state file with -sync-state")
		}
		return "", nil
	}
	return reposync.StatePath(model), nil
}

// loadSyncGraph reads a graph for the sync: Turtle as-is, the API's element
// form parsed back to its triples, notation converted through the
// identity-carrying RDF mapping.
func loadSyncGraph(path string) (*rdf.Graph, error) {
	format, err := convert.FormatOfPath(path)
	if err != nil {
		return nil, convert.Advise(err, convert.ExtensionAdvice)
	}
	name, data, err := project.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch format {
	case convert.FormatTurtle:
		return rdf.ParseTurtle(data)
	case convert.FormatAPIJSON:
		return export.ReadAPIJSON(data)
	case convert.FormatXMI:
		return nil, fmt.Errorf("%s is SysML v1 XMI, which a sync cannot read; migrate it first with `sysml %s -convert sysml -o model.sysml`", path, path)
	}
	return convert.SysMLToRDF(name, data)
}

// loadSyncState reads the sync state file, if any, and refuses one pinning
// another project or branch.
func loadSyncState(path string, scope reposync.Scope) (*reposync.State, error) {
	if path == "" {
		return nil, nil
	}
	state, err := reposync.LoadState(path)
	if err != nil || state == nil {
		return nil, err
	}
	if err := state.Check(scope); err != nil {
		return nil, err
	}
	return state, nil
}

// annotation is the model's original text with an about-form ElementId per
// minted id appended, analyzed and ready to write; unnamed elements, which no
// about clause can address, are listed rather than annotated.
type annotation struct {
	path     string
	notation []byte
	clauses  []string
	unnamed  []reposync.Change
}

// prepareAnnotation builds the annotated notation and checks that it analyzes,
// so the write-back is known good before anything reaches a repository.
func prepareAnnotation(local *rdf.Graph, model string, set *reposync.ChangeSet, path string) (*annotation, error) {
	minted := mintedChanges(set)
	if len(minted) == 0 {
		return nil, fmt.Errorf("nothing to annotate: no id was minted, so %s would repeat the model unchanged", path)
	}
	if project.IsStdin(model) {
		return nil, errors.New("-sync-annotate keeps the model's text, so the model must be a file, not stdin")
	}
	name, data, err := project.ReadFile(model)
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	out.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		out.WriteByte('\n')
	}
	a := &annotation{path: path}
	for _, change := range minted {
		target := annotationPath(local, change.Subject)
		if target == "" {
			// A minted relationship element belongs to its related elements
			// directly — no membership names it, so notation can never address
			// it; only a member element that lacks a name needs reporting.
			if !local.HasProperty(rdf.IRI(change.Subject), rdf.SysML+"owningMembership") {
				continue
			}
			a.unnamed = append(a.unnamed, change)
			continue
		}
		clause := identity.ElementIdAbout(target, change.MintedID)
		out.WriteString(clause + "\n")
		a.clauses = append(a.clauses, clause)
	}
	a.notation = []byte(out.String())
	if _, err := convert.SysMLToRDF(name, a.notation); err != nil {
		return nil, fmt.Errorf("the annotated notation does not analyze: %w", err)
	}
	return a, nil
}

// write puts the annotated notation at its path — the explicit opt-in write-back.
func (a *annotation) write() error {
	replaced, err := export.WriteFile(a.path, a.notation)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s with %d minted id(s) annotated (%d bytes%s)\n", a.path, len(a.clauses), len(a.notation), what)
	return nil
}

// annotationPath walks a subject's ownership chain, naming each step by its
// declared name or else its short name; empty when no name reaches the element.
func annotationPath(g *rdf.Graph, subject string) string {
	var segments []string
	seen := make(map[string]bool)
	for current := subject; current != "" && !seen[current]; {
		seen[current] = true
		term := rdf.IRI(current)
		name, ok := g.Lexical(term, rdf.SysML+"declaredName")
		if !ok {
			if name, ok = g.Lexical(term, rdf.SysML+"declaredShortName"); !ok {
				return ""
			}
		}
		segments = append([]string{source.NameText(name)}, segments...)
		owner, ok := g.Object(term, rdf.SysML+"owningNamespace")
		if !ok {
			break
		}
		current = owner.Value
	}
	return strings.Join(segments, "::")
}

// mintedChanges lists the creates that minted an id, in change-set order.
func mintedChanges(set *reposync.ChangeSet) []reposync.Change {
	var minted []reposync.Change
	for _, change := range set.Changes {
		if change.MintedID != "" {
			minted = append(minted, change)
		}
	}
	return minted
}
