package flexo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/convert"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

const applyCommitMessage = "OpenSysML sync apply"

// The apply measurement drives the sync against a real project: fixture, then
// revision, then a change made behind its back. Server ids never reach the report.

// applyProject prefixes the project each apply run mints.
const applyProject = "opensysml-sync-apply"

// ApplyRound is one diff-and-apply against the branch.
type ApplyRound struct {
	Name      string
	Base      bool // the diff had the last-seen commit's graph as its base
	Creates   int
	Updates   int
	Deletes   int
	Conflicts int
	Uncarried []string // properties the representation left out of the comparison
	Changes   []string // "kind\tid\tmetaclass" per entry, in diff order
	Refused   string   // the refusal an unconfirmed or conflicted apply got, "" when appliable
	Applied   bool
	Commits   int      // commits the project has after the round
	Advanced  bool     // the sync state moved to the commit the stack named
	Readable  int      // written elements a direct read at that commit returned
	Absent    int      // deleted elements a direct read at that commit refused with 404
	Emptied   int      // deleted elements the read answered with an id and nothing else
	Present   int      // deleted elements the read still returned with content
	ReadBack  []string // "id\tproperty=value" for the properties an update changed
	Rediff    string   // the diff after the apply: counts, or "not run"
}

// ApplyReport is a whole apply measurement.
type ApplyReport struct {
	Fixture  string
	Revision string
	Rounds   []ApplyRound
	Findings []string
}

// Text renders the report as the expectation file, in the same tab-separated
// form as the round-trip report.
func (r *ApplyReport) Text(header string) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n[apply]\nfixture\t%s\nrevision\t%s\n", r.Fixture, r.Revision)
	for i := range r.Rounds {
		r.Rounds[i].write(&b)
	}
	b.WriteString("\n[findings]\n")
	for _, finding := range r.Findings {
		fmt.Fprintf(&b, "%s\n", finding)
	}
	return b.String()
}

func (a *ApplyRound) write(b *strings.Builder) {
	fmt.Fprintf(b, "\n[apply.%s]\nbase\t%s\ndiff\tcreates=%d\tupdates=%d\tdeletes=%d\tconflicts=%d\n",
		a.Name, yesNo(a.Base), a.Creates, a.Updates, a.Deletes, a.Conflicts)
	fmt.Fprintf(b, "uncarried\t%s\n", orNone(strings.Join(a.Uncarried, ",")))
	fmt.Fprintf(b, "refused\t%s\n", orNone(a.Refused))
	fmt.Fprintf(b, "applied\t%s\ncommits\t%d\nstate.advanced\t%s\n", yesNo(a.Applied), a.Commits, yesNo(a.Advanced))
	fmt.Fprintf(b, "elements.readable\t%d\ndeleted.absent\t%d\ndeleted.emptied\t%d\ndeleted.present\t%d\nrediff\t%s\n",
		a.Readable, a.Absent, a.Emptied, a.Present, a.Rediff)
	fmt.Fprintf(b, "\n[apply.%s.changes]\n", a.Name)
	for _, change := range a.Changes {
		fmt.Fprintf(b, "%s\n", change)
	}
	if len(a.ReadBack) > 0 {
		fmt.Fprintf(b, "\n[apply.%s.read-back]\n", a.Name)
		for _, line := range a.ReadBack {
			fmt.Fprintf(b, "%s\n", line)
		}
	}
}

// MeasureApply applies model to a fresh project, then revised on top of it,
// then diffs revised against a change made behind the sync's back.
func MeasureApply(ctx context.Context, c *Client, fixture string, model, revised []byte) (*ApplyReport, error) {
	report := &ApplyReport{Fixture: filepath.Base(fixture), Revision: revisionName(fixture)}
	local, err := convert.SysMLToRDF(report.Fixture, model)
	if err != nil {
		return nil, fmt.Errorf("convert %s to RDF: %w", fixture, err)
	}
	revisedGraph, err := convert.SysMLToRDF(report.Revision, revised)
	if err != nil {
		return nil, fmt.Errorf("convert %s to RDF: %w", report.Revision, err)
	}

	if err := c.EnsureOrg(ctx); err != nil {
		return nil, fmt.Errorf("ensure org %s: %w", c.Config().Org, err)
	}
	project, err := uniqueProjectID(applyProject)
	if err != nil {
		return nil, err
	}
	if _, err := c.CreateProject(ctx, project, applyCommitMessage, BranchID); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	run := &applyRun{
		client: c,
		repo:   c.Repository(project, BranchID),
		state:  &reposync.State{ProjectID: project, Branch: BranchID},
	}

	initial, err := run.round(ctx, "initial", local, false)
	if err != nil {
		return report, err
	}
	report.Rounds = append(report.Rounds, *initial)

	revision, err := run.round(ctx, "revision", revisedGraph, true)
	if err != nil {
		return report, err
	}
	report.Rounds = append(report.Rounds, *revision)

	// Someone else's commit: the renamed element written again under its id
	// with a different name, through the service's own path.
	renamed := renamedID(revision)
	if renamed == "" {
		return report, errors.New("the revision renamed nothing under a retained id, so no conflict can be staged")
	}
	behind, err := json.Marshal(map[string]any{"@type": "Commit", "change": []any{map[string]any{
		"@type":    "DataVersion",
		"identity": map[string]string{"@id": renamed},
		"payload":  map[string]any{"@id": renamed, "@type": "PartDefinition", "declaredName": "RenamedElsewhere"},
	}}})
	if err != nil {
		return report, err
	}
	if _, err := c.PostChanges(ctx, project, BranchID, behind); err != nil {
		return report, fmt.Errorf("stage a repository-side change: %w", err)
	}
	conflict, err := run.round(ctx, "repository-changed", revisedGraph, true)
	if err != nil {
		return report, err
	}
	report.Rounds = append(report.Rounds, *conflict)

	report.Findings = applyFindings(report)
	return report, nil
}

// revisionName is the fixture's revised sibling: <name>_revised<ext>.
func revisionName(fixture string) string {
	base := filepath.Base(fixture)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "_revised" + ext
}

type applyRun struct {
	client *Client
	repo   *Repository
	state  *reposync.State
}

// diff reads the branch — and its last-seen commit when the state names one —
// and computes the change set against local.
func (run *applyRun) diff(ctx context.Context, local *rdf.Graph, confirm bool) (*reposync.ChangeSet, bool, error) {
	opts := reposync.Options{Representation: Representation{}, ConfirmDeletes: confirm}
	if run.state.LastSeenCommit != "" {
		base, err := run.repo.GraphAt(ctx, run.state.LastSeenCommit)
		if err != nil {
			return nil, false, fmt.Errorf("read the branch at the last-seen commit: %w", err)
		}
		opts.Base = base
	}
	remote, err := run.repo.Graph(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("read the branch: %w", err)
	}
	set, err := reposync.Diff(local, remote, opts)
	if err != nil {
		return nil, false, fmt.Errorf("diff: %w", err)
	}
	return set, opts.Base != nil, nil
}

// round diffs local against the branch and applies the result. With deletes
// gated, the unconfirmed apply is attempted first and its refusal recorded.
func (run *applyRun) round(ctx context.Context, name string, local *rdf.Graph, gate bool) (*ApplyRound, error) {
	round := &ApplyRound{Name: name, Rediff: "not run"}

	set, based, err := run.diff(ctx, local, false)
	if err != nil {
		return nil, err
	}
	round.Base = based
	round.Creates, round.Updates, round.Deletes, round.Conflicts = counts(set)
	for _, left := range set.Uncarried {
		round.Uncarried = append(round.Uncarried, label(left.Property))
	}
	for _, change := range set.Changes {
		round.Changes = append(round.Changes, fmt.Sprintf("%s\t%s\t%s", change.Kind, change.ID, orNone(change.Metaclass)))
	}

	before, err := run.client.Commits(ctx, run.repo.project)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}
	if gate && set.Appliable() != nil {
		_, err := reposync.Apply(ctx, run.repo, set, reposync.ApplyOptions{Message: applyCommitMessage})
		var refused *reposync.NotAppliableError
		if !errors.As(err, &refused) {
			return nil, fmt.Errorf("%s: an unappliable set was not refused as one: %v", name, err)
		}
		round.Refused = refused.Error()
		if round.Commits, err = run.commitCount(ctx, len(before)); err != nil {
			return nil, err
		}
		if set, based, err = run.diff(ctx, local, true); err != nil {
			return nil, err
		}
		round.Base = based
	}
	if set.Conflicts() > 0 {
		// Confirmation does not resolve a conflict; the refusal above is the
		// measurement and the branch stays as it was.
		return round, nil
	}

	result, err := reposync.Apply(ctx, run.repo, set, reposync.ApplyOptions{Message: applyCommitMessage})
	if err != nil {
		return nil, fmt.Errorf("%s: apply: %w", name, err)
	}
	round.Applied = len(result.Commits) > 0
	commit := run.state.LastSeenCommit
	if err := run.state.Advance(result); err != nil {
		return nil, fmt.Errorf("%s: advance the state: %w", name, err)
	}
	round.Advanced = run.state.LastSeenCommit != commit
	if round.Commits, err = run.commitCount(ctx, -1); err != nil {
		return nil, err
	}
	if round.Applied && run.state.LastSeenCommit != "" {
		if err := run.readBack(ctx, round, result); err != nil {
			return nil, err
		}
	}

	after, _, err := run.diff(ctx, local, true)
	if err != nil {
		return nil, err
	}
	c, u, d, k := counts(after)
	round.Rediff = fmt.Sprintf("creates=%d\tupdates=%d\tdeletes=%d\tconflicts=%d", c, u, d, k)
	return round, nil
}

// commitCount lists the project's commits; want >= 0 asserts the count held.
func (run *applyRun) commitCount(ctx context.Context, want int) (int, error) {
	commits, err := run.client.Commits(ctx, run.repo.project)
	if err != nil {
		return 0, fmt.Errorf("list commits: %w", err)
	}
	if want >= 0 && len(commits) != want {
		return len(commits), fmt.Errorf("a refused apply changed the branch: %d commit(s) became %d", want, len(commits))
	}
	return len(commits), nil
}

// readBack reads each written element at the recorded commit, and each deleted
// one, and records what an update's changed properties now read as.
func (run *applyRun) readBack(ctx context.Context, round *ApplyRound, result *reposync.Result) error {
	commit := run.state.LastSeenCommit
	for _, change := range result.Applied {
		element, err := run.client.ElementByID(ctx, run.repo.project, commit, change.ID)
		switch {
		case change.Kind == reposync.KindDelete && Status(err) == http.StatusNotFound:
			round.Absent++
			continue
		case change.Kind == reposync.KindDelete && err == nil && emptied(element):
			round.Emptied++
			continue
		case change.Kind == reposync.KindDelete && err == nil:
			round.Present++
			continue
		case err != nil && Status(err) != 0:
			continue
		case err != nil:
			return fmt.Errorf("read %s back: %w", change.ID, err)
		}
		round.Readable++
		if change.Kind != reposync.KindUpdate {
			continue
		}
		for _, delta := range change.Deltas {
			round.ReadBack = append(round.ReadBack,
				fmt.Sprintf("%s\t%s=%s", change.ID, delta.Property, compactJSON(element[delta.Property])))
		}
	}
	sort.Strings(round.ReadBack)
	return nil
}

// emptied reports an element the service answered with its id and a null type
// and nothing else: how a deleted id reads on the deployed service.
func emptied(element Element) bool {
	for name, raw := range element {
		if name != "@id" && (name != "@type" || string(raw) != "null") {
			return false
		}
	}
	return true
}

// compactJSON renders a delivered value for the report, or "absent".
func compactJSON(raw json.RawMessage) string {
	if raw == nil {
		return "absent"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "unreadable"
	}
	out, err := json.Marshal(value)
	if err != nil {
		return "unreadable"
	}
	return string(out)
}

func counts(set *reposync.ChangeSet) (creates, updates, deletes, conflicts int) {
	for _, change := range set.Changes {
		switch change.Kind {
		case reposync.KindCreate:
			creates++
		case reposync.KindUpdate:
			updates++
		case reposync.KindDelete:
			deletes++
		case reposync.KindConflict:
			conflicts++
		}
	}
	return
}

// renamedID is the declared id the revision round updated with a new
// declaredName, the element a repository-side change is staged on.
func renamedID(round *ApplyRound) string {
	for _, line := range round.ReadBack {
		id, rest, _ := strings.Cut(line, "\t")
		if strings.HasPrefix(rest, "declaredName=") {
			return id
		}
	}
	return ""
}

// applyFindings names what each round showed, in a fixed order.
func applyFindings(report *ApplyReport) []string {
	var found []string
	add := func(format string, args ...any) { found = append(found, fmt.Sprintf(format, args...)) }
	for _, round := range report.Rounds {
		switch {
		case round.Conflicts > 0:
			add("%s: %d conflict(s) surfaced from the last-seen commit and the apply was refused; the branch kept its %d commit(s)",
				round.Name, round.Conflicts, round.Commits)
		case round.Applied:
			add("%s: %d create(s), %d update(s), %d delete(s) reached the stack as one commit; %d written element(s) read back at it; rediff %s",
				round.Name, round.Creates, round.Updates, round.Deletes, round.Readable,
				strings.ReplaceAll(round.Rediff, "\t", " "))
			if round.Deletes > 0 {
				add("%s: of %d deleted element(s), %d read as 404, %d as an id with a null type and no properties, %d still with content",
					round.Name, round.Deletes, round.Absent, round.Emptied, round.Present)
			}
		default:
			add("%s: nothing to apply", round.Name)
		}
		if round.Refused != "" {
			add("%s: the first apply was refused before any write: %s", round.Name, round.Refused)
		}
		for _, line := range round.ReadBack {
			add("%s: %s reads back under its retained id", round.Name, strings.ReplaceAll(line, "\t", " "))
		}
	}
	return found
}
