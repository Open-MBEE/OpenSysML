package hygiene

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// workflowEntry is one job of a CircleCI workflow: the job it runs, the name it
// runs under, what it requires, and whether it halts on a verified tree.
type workflowEntry struct {
	Job              string
	Name             string
	Requires         []string
	SkipWhenVerified bool
}

func workflowEntries(t *testing.T, w workflow) map[string]workflowEntry {
	t.Helper()
	entries := map[string]workflowEntry{}
	for _, node := range w.Jobs {
		var entry workflowEntry
		switch node.Kind {
		case yaml.ScalarNode:
			entry.Job, entry.Name = node.Value, node.Value
		case yaml.MappingNode:
			entry.Job = node.Content[0].Value
			var body struct {
				Name             string   `yaml:"name"`
				Requires         []string `yaml:"requires"`
				SkipWhenVerified bool     `yaml:"skip_when_verified"`
			}
			if err := node.Content[1].Decode(&body); err != nil {
				t.Fatalf("decode workflow job %s: %v", entry.Job, err)
			}
			entry.Name, entry.Requires, entry.SkipWhenVerified = body.Name, body.Requires, body.SkipWhenVerified
			if entry.Name == "" {
				entry.Name = entry.Job
			}
		default:
			t.Fatalf("workflow job of kind %v", node.Kind)
		}
		entries[entry.Name] = entry
	}
	return entries
}

// upstream names every job the entry requires, transitively.
func upstream(t *testing.T, entries map[string]workflowEntry, name string) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	var walk func(string)
	walk = func(name string) {
		entry, ok := entries[name]
		if !ok {
			t.Fatalf("%q is required but is not a job of the workflow", name)
		}
		for _, req := range entry.Requires {
			if !seen[req] {
				seen[req] = true
				walk(req)
			}
		}
	}
	walk(name)
	return seen
}

// haltPlan is what a job does around halt-if-verified: whether it is reached (always, or under
// skip_when_verified when the workflow sets it), and what is attached and persisted around it.
type haltPlan struct {
	Halts               bool
	AttachedBeforeHalt  bool
	PersistedAfterHalt  []string
	PersistedBeforeHalt []string
}

func planHalt(t *testing.T, steps []yaml.Node, skipWhenVerified bool) haltPlan {
	t.Helper()
	var plan haltPlan
	attached := false
	var walk func(steps []yaml.Node, conditional bool)
	walk = func(steps []yaml.Node, conditional bool) {
		for _, step := range steps {
			if step.Kind == yaml.ScalarNode {
				if step.Value == "halt-if-verified" && (!conditional || skipWhenVerified) {
					plan.Halts = true
					plan.AttachedBeforeHalt = attached
				}
				continue
			}
			if step.Kind != yaml.MappingNode || len(step.Content) < 2 {
				continue
			}
			key, value := step.Content[0].Value, step.Content[1]
			switch key {
			case "attach_workspace":
				attached = true
			case "persist_to_workspace":
				var persist struct {
					Paths []string `yaml:"paths"`
				}
				if err := value.Decode(&persist); err != nil {
					t.Fatalf("decode persist_to_workspace: %v", err)
				}
				if plan.Halts {
					plan.PersistedAfterHalt = append(plan.PersistedAfterHalt, persist.Paths...)
				} else {
					plan.PersistedBeforeHalt = append(plan.PersistedBeforeHalt, persist.Paths...)
				}
			case "when", "unless":
				var block struct {
					Condition string      `yaml:"condition"`
					Steps     []yaml.Node `yaml:"steps"`
				}
				if err := value.Decode(&block); err != nil {
					t.Fatalf("decode %s block: %v", key, err)
				}
				onParameter := strings.Contains(block.Condition, "parameters.skip_when_verified")
				walk(block.Steps, conditional || (key == "when" && onParameter))
			}
		}
	}
	walk(steps, false)
	return plan
}

// TestVerifiedTreeSkipsOnlyWhatNothingDownstreamReads: every job halting on verify-tree's verdict
// attaches it downstream of the check, and no halt loses an output another job of the workflow reads.
func TestVerifiedTreeSkipsOnlyWhatNothingDownstreamReads(t *testing.T) {
	config := loadCircleConfig(t)

	verify, ok := config.Jobs["verify-tree"]
	if !ok {
		t.Fatal("no verify-tree job")
	}
	verifySteps := runSteps(t, verify.Steps)
	if stepIndex(verifySteps, "python3 scripts/ci-verified-tree.py") < 0 {
		t.Error("verify-tree does not run scripts/ci-verified-tree.py")
	}
	if stepIndex(verifySteps, "--rehearsal") < 0 || !strings.Contains(string(readRepoFile(t, ".circleci/config.yml")), "RELEASE_REHEARSAL: << pipeline.parameters.release_rehearsal >>") {
		t.Error("verify-tree does not pass --rehearsal from the release_rehearsal parameter, so a rehearsal could skip the suite it exists to run")
	}
	requireAll(t, "verify-tree persists", persistedPaths(t, verify.Steps), "verified-tree.txt")

	halt, ok := config.Commands["halt-if-verified"]
	if !ok {
		t.Fatal("no halt-if-verified command")
	}
	haltSteps := runSteps(t, halt.Steps)
	if stepIndex(haltSteps, "verified-tree.txt") < 0 || stepIndex(haltSteps, "circleci-agent step halt") < 0 {
		t.Error("halt-if-verified does not halt on verified-tree.txt")
	}

	for _, name := range []string{"build-test", "release"} {
		entries := workflowEntries(t, config.workflow(t, name))
		check, ok := entries["Verified tree check"]
		if !ok || check.Job != "verify-tree" {
			t.Errorf("%s has no 'Verified tree check' entry running verify-tree", name)
			continue
		}
		halting := map[string]haltPlan{}
		for entryName, entry := range entries {
			job, ok := config.Jobs[entry.Job]
			if !ok {
				t.Fatalf("%s: no %s job", name, entry.Job)
			}
			plan := planHalt(t, job.Steps, entry.SkipWhenVerified)
			if entry.SkipWhenVerified && !plan.Halts {
				t.Errorf("%s: %s sets skip_when_verified but %s never reaches halt-if-verified under it", name, entryName, entry.Job)
			}
			if !plan.Halts {
				continue
			}
			halting[entryName] = plan
			if !plan.AttachedBeforeHalt {
				t.Errorf("%s: %s halts on the verdict without attaching the workspace that carries it first", name, entryName)
			}
			if !upstream(t, entries, entryName)["Verified tree check"] {
				t.Errorf("%s: %s halts on the verdict but does not run after 'Verified tree check'", name, entryName)
			}
		}

		// What a halted job never writes must not be read in the same workflow: no
		// job that does not halt itself may name one of those paths in a command.
		for halted, plan := range halting {
			for reader, entry := range entries {
				if _, halts := halting[reader]; halts {
					continue
				}
				for _, step := range runSteps(t, config.Jobs[entry.Job].Steps) {
					for _, path := range plan.PersistedAfterHalt {
						if pathPattern(path).MatchString(step.Command) {
							t.Errorf("%s: %s halts on a verified tree before persisting %q, which %s reads (%q)", name, halted, path, reader, step.Name)
						}
					}
				}
			}
		}
	}

	// build-test keeps every scan input: only outputs persisted before the halt
	// (go-static's landing bundle) may belong to a halting job there.
	buildTest := workflowEntries(t, config.workflow(t, "build-test"))
	for entryName, entry := range buildTest {
		plan := planHalt(t, config.Jobs[entry.Job].Steps, entry.SkipWhenVerified)
		if entry.SkipWhenVerified {
			t.Errorf("build-test: %s sets skip_when_verified, but the scan reads every report build-test produces", entryName)
		}
		if plan.Halts && len(plan.PersistedAfterHalt) > 0 {
			t.Errorf("build-test: %s persists %q after halt-if-verified; the scan needs it", entryName, plan.PersistedAfterHalt)
		}
	}
	static := planHalt(t, config.Jobs["go-static"].Steps, false)
	if !static.Halts {
		t.Error("go-static does not halt on a verified tree")
	}
	requireAll(t, "go-static persists before it halts", static.PersistedBeforeHalt, "docs/assets/landing")

	// On a verified tag the release starts from the binaries: everything else
	// 'Build release binaries' waits for halts.
	release := workflowEntries(t, config.workflow(t, "release"))
	for required := range upstream(t, release, "Build release binaries") {
		entry := release[required]
		if required == "Go binaries" || required == "Verified tree check" {
			continue
		}
		if !planHalt(t, config.Jobs[entry.Job].Steps, entry.SkipWhenVerified).Halts {
			t.Errorf("release: 'Build release binaries' waits for %s, which does not halt on a verified tree", required)
		}
	}
	if _, scans := release["SonarCloud scan"]; scans {
		t.Error("release runs the scan, so its coverage jobs can no longer skip on a verified tree")
	}
}

// pathPattern matches a persisted path, with its shell globs, inside a command.
func pathPattern(path string) *regexp.Regexp {
	var pattern strings.Builder
	for _, part := range strings.Split(path, "*") {
		if pattern.Len() > 0 {
			pattern.WriteString(`[^\s/]*`)
		}
		pattern.WriteString(regexp.QuoteMeta(part))
	}
	return regexp.MustCompile(`(^|[\s"'=/])` + pattern.String() + `($|[\s"'])`)
}
