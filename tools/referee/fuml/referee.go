package fuml

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/report"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// Meaning is what a pass means, and the first sentence of everything the
// referee prints about itself.
const Meaning = "A pass checks that the runtime leaves in an activity's output parameters the values " +
	"the fUML reference implementation did, where the activity has a SysML v2 spelling; it is a " +
	"second opinion on the action executor and never evidence of SysML v2 conformance."

// DefaultBudget is the exploration budget a translated activity is run under.
var DefaultBudget = runtime.DefaultExploreBudget

// Suite is the pinned suite as read: the library and the two test models.
type Suite struct {
	Library   *Library
	Tests     *Model
	Exception *Model
}

// ReadSuite verifies every suite file in dir against the pin and reads the
// library and both models.
func ReadSuite(dir string, pin Pin) (*Suite, error) {
	for _, name := range []string{TestsFile, ExceptionTestsFile, LibraryFile, JarFile} {
		if err := pin.Verify(dir, name); err != nil {
			return nil, err
		}
	}
	lib, err := ReadLibraryFile(filepath.Join(dir, LibraryFile))
	if err != nil {
		return nil, err
	}
	s := &Suite{Library: lib}
	for _, m := range []struct {
		file string
		into **Model
	}{{TestsFile, &s.Tests}, {ExceptionTestsFile, &s.Exception}} {
		if *m.into, err = ReadModelFile(filepath.Join(dir, m.file), lib); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Models lists the two models in report order.
func (s *Suite) Models() []*Model { return []*Model{s.Tests, s.Exception} }

// Problems lists every reader diagnostic over both models.
func (s *Suite) Problems() []string {
	var problems []string
	for _, m := range s.Models() {
		for _, d := range m.Diagnostics {
			problems = append(problems, m.File+": "+d)
		}
	}
	return problems
}

// Activities counts the activities both models declare.
func (s *Suite) Activities() int {
	return len(s.Tests.Activities) + len(s.Exception.Activities)
}

// Provenance identifies the suite and the record a report measured.
type Provenance struct {
	RITag                string `json:"riTag"`
	RICommit             string `json:"riCommit"`
	TestsDigest          string `json:"testsDigest"`
	ExceptionTestsDigest string `json:"exceptionTestsDigest"`
	// LibraryDigest is the downloaded library's, the copy the reader resolves
	// references against; the jar carries its own copy under JarDigest.
	LibraryDigest string `json:"libraryDigest"`
	JarDigest     string `json:"jarDigest"`
	Activities    int    `json:"activities"`
	// Recorded is the date -update stamped; a plain run leaves it empty.
	Recorded string `json:"recorded,omitempty"`
	// Develop is the develop commit whose runtime -update measured; a plain
	// run leaves it empty.
	Develop string `json:"develop,omitempty"`
}

// Provenance is the identity a report of the pinned suite carries.
func (p Pin) Provenance(activities int) Provenance {
	return Provenance{
		RITag: p.Tag, RICommit: p.Commit,
		TestsDigest: p.Tests, ExceptionTestsDigest: p.ExceptionTest, LibraryDigest: p.Library, JarDigest: p.Jar,
		Activities: activities,
	}
}

// Report is one run of the referee over the suite: the buckets' counts, the
// gate CI compares, and one row per activity, for the adjudicator.
type Report struct {
	Meaning    string                `json:"meaning"`
	Provenance Provenance            `json:"provenance"`
	Buckets    map[report.Bucket]int `json:"buckets"`
	Activities []ActivityReport      `json:"activities"`
}

// ActivityReport is one activity's row.
type ActivityReport struct {
	Name   string        `json:"name"`
	Model  string        `json:"model"`
	Class  string        `json:"class"`
	Bucket report.Bucket `json:"bucket"`
	// Reasons say why the activity is in its bucket, one line each; a pass of
	// the activity's own run has none.
	Reasons []string `json:"reasons,omitempty"`
	// Expected and Reached are the implementation's outputs and the distinct
	// output states the runs came to; Runs is how many linearizations were run.
	Expected []string `json:"expected,omitempty"`
	Reached  []string `json:"reached,omitempty"`
	Runs     int      `json:"runs,omitempty"`
	// Status says how the exploration ended: complete, or which budget it hit.
	Status string `json:"status,omitempty"`
	// Fired is the advisory comparison of the actions the implementation fired
	// with those that produced a value here; empty when they agree.
	Fired string `json:"fired,omitempty"`
}

// Options steer a run.
type Options struct {
	// Jobs is how many linearizations of one activity explore at once; the
	// report is the same for any value.
	Jobs int
	// Keep, when set, is a directory each translated model is written to.
	Keep string
	// Filter, when set, keeps only the activities whose name contains it.
	Filter string
	// Budget bounds each activity's exploration.
	Budget runtime.ExploreBudget
}

// Referee files every activity of both models. Only a problem with the
// harness itself, or a suite the reader could not read whole, is an error; an
// activity that cannot be run is a row in its bucket.
func Referee(stop context.Context, s *Suite, x *Expected, prov Provenance, opts Options) (*Report, error) {
	if problems := s.Problems(); len(problems) > 0 {
		return nil, fmt.Errorf("the suite was not read whole, so its counts would not be its own:\n  %s", strings.Join(problems, "\n  "))
	}
	if opts.Jobs < 1 {
		opts.Jobs = 1
	}
	if opts.Budget.Runs == 0 {
		opts.Budget = DefaultBudget
	}
	if opts.Keep != "" {
		if err := os.MkdirAll(opts.Keep, 0o750); err != nil {
			return nil, err
		}
	}
	rf := &refereeing{stop: stop, s: s, x: x, opts: opts, rows: map[*Activity]ActivityReport{}}
	var rows []ActivityReport
	for _, m := range s.Models() {
		for _, a := range m.Activities {
			if opts.Filter != "" && !strings.Contains(a.Name, opts.Filter) {
				continue
			}
			row, err := rf.row(a)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
	}
	counted := &Report{Meaning: Meaning, Provenance: prov, Buckets: make(map[report.Bucket]int, len(report.Buckets)), Activities: rows}
	for _, b := range report.Buckets {
		counted.Buckets[b] = 0
	}
	for _, r := range rows {
		counted.Buckets[r.Bucket]++
	}
	return counted, nil
}

// refereeing is one run over the suite, each activity's row computed once.
type refereeing struct {
	stop context.Context
	s    *Suite
	x    *Expected
	opts Options
	rows map[*Activity]ActivityReport
}

// row files the activity, once.
func (rf *refereeing) row(a *Activity) (ActivityReport, error) {
	if row, ok := rf.rows[a]; ok {
		return row, nil
	}
	row, err := rf.referee(a)
	if err != nil {
		return row, fmt.Errorf("%s: %w", a.Name, err)
	}
	rf.rows[a] = row
	return row, nil
}

// referee files one activity: the classifier's bucket where it fixes one, else
// the run's; a construct the emitter has no rule for yet is not-expressible by
// the emitter, the row naming the construct and keeping the classifier's class.
// A class's owned behavior runs only as the behavior of an object of the class,
// so it is filed by the activities that start one, as the JUnit suite runs it.
func (rf *refereeing) referee(a *Activity) (ActivityReport, error) {
	stop, opts, x := rf.stop, rf.opts, rf.x.Activity(a.Model.File, a.ID)
	c := Classify(a, x)
	row := ActivityReport{Name: a.Name, Model: a.Model.File, Class: c.Class.String()}
	if c.Class == NotExpressible {
		row.Bucket = report.BucketNotExpressible
		row.Reasons = []string{c.Reason()}
		return row, nil
	}
	fixed, _ := c.Class.Bucket()
	if a.Owner != nil {
		return rf.carried(row, a, fixed, c)
	}
	em, err := Emit(a)
	if err != nil {
		var te *TranslateError
		if !errors.As(err, &te) {
			return row, err
		}
		reason := "not yet translated: " + te.Where + ": " + te.Reason
		if fixed != "" {
			return filed(row, fixed, c, reason), nil
		}
		row.Bucket = report.BucketNotExpressible
		row.Reasons = []string{reason}
		return row, nil
	}
	if opts.Keep != "" {
		if err := os.WriteFile(filepath.Join(opts.Keep, em.Name), []byte(em.Text), 0o600); err != nil {
			return row, err
		}
	}
	if problems := Validate(em); len(problems) > 0 {
		return row, fmt.Errorf("translated model does not validate: %s", strings.Join(problems, "; "))
	}
	if x == nil || !x.Executed {
		why := "the implementation's record has no execution of it"
		if x != nil && x.Skipped != "" {
			why += ": " + x.Skipped
		}
		return filed(row, fixed, c, why), nil
	}
	ex, err := Execute(stop, em, x, opts.Budget, opts.Jobs)
	if err != nil {
		return row, err
	}
	row.Expected, row.Reached, row.Runs, row.Status, row.Fired = ex.Expected, ex.Reached, ex.Runs, ex.Status, ex.Fired
	if ex.Passed() {
		if fixed != "" {
			return filed(row, fixed, c, "the run agrees with the implementation"), nil
		}
		row.Bucket = report.BucketPass
		return row, nil
	}
	return filed(row, fixed, c, ex.Reasons()...), nil
}

// carried files a class's owned behavior by the rows of the activities whose
// start runs it: it fails when one fails, else passes when one passes, else
// takes the bucket the starters share. With none, nothing runs it and it fails
// saying so, as an activity the record has no execution of does.
func (rf *refereeing) carried(row ActivityReport, b *Activity, fixed report.Bucket, c Classification) (ActivityReport, error) {
	var reasons []string
	bucket := report.Bucket("")
	for _, s := range starters(b) {
		sr, err := rf.row(s)
		if err != nil {
			return row, err
		}
		reason := fmt.Sprintf("started by %s, which is %s", s.Name, sr.Bucket)
		if len(sr.Reasons) > 0 {
			reason += ": " + strings.Join(sr.Reasons, "; ")
		}
		reasons = append(reasons, reason)
		bucket = carriedBucket(bucket, sr.Bucket)
	}
	if bucket == "" {
		return filed(row, fixed, c, "no activity starts an object of "+b.Owner.Name+", so nothing runs it"), nil
	}
	if fixed != "" {
		return filed(row, fixed, c, reasons...), nil
	}
	row.Bucket, row.Reasons = bucket, reasons
	return row, nil
}

// carriedBucket is the bucket two starters' rows give an owned behavior: fail
// over pass, pass over differs-by-design, that over not-expressible.
func carriedBucket(a, b report.Bucket) report.Bucket {
	rank := map[report.Bucket]int{report.BucketFail: 4, report.BucketPass: 3, report.BucketDiffersByDesign: 2, report.BucketNotExpressible: 1}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// starters lists the activities of the model no class owns whose start action
// runs the owned behavior: the classifier behavior, own or inherited, of the
// type of a start's object pin.
func starters(b *Activity) []*Activity {
	var out []*Activity
	for _, a := range b.Model.Activities {
		if a.Owner != nil {
			continue
		}
		for _, n := range a.AllNodes() {
			if n.Kind != StartObjectBehaviorAction {
				continue
			}
			for _, p := range n.Inputs() {
				if p.Role == "object" && startedBehavior(b.Model, p) == b {
					out = append(out, a)
				}
			}
		}
	}
	return out
}

// filed puts a row in the classifier's fixed bucket with its reason first, or
// in fail with the run's reasons.
func filed(row ActivityReport, fixed report.Bucket, c Classification, reasons ...string) ActivityReport {
	row.Bucket = report.BucketFail
	if fixed != "" {
		row.Bucket = fixed
		reasons = append([]string{c.Reason()}, reasons...)
	}
	row.Reasons = reasons
	return row
}

// Summary renders the counts and the rows of every bucket but pass, one line
// per reason, for a terminal.
func (r *Report) Summary() string {
	var b strings.Builder
	b.WriteString(Meaning)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "fUML reference implementation %s (%s), %d activities\n", r.Provenance.RITag, r.Provenance.RICommit, r.Provenance.Activities)
	for _, bucket := range report.Buckets {
		fmt.Fprintf(&b, "  %-18s %3d\n", bucket, r.Buckets[bucket])
	}
	for _, bucket := range report.Buckets {
		if bucket == report.BucketPass {
			continue
		}
		first := true
		for _, a := range r.Activities {
			if a.Bucket != bucket {
				continue
			}
			if first {
				fmt.Fprintf(&b, "\n%s:\n", bucket)
				first = false
			}
			fmt.Fprintf(&b, "  %s\n", a.Name)
			for _, reason := range a.Reasons {
				fmt.Fprintf(&b, "    %s\n", reason)
			}
		}
	}
	return b.String()
}
