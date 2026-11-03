package runtime

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

var updateKernelMembersRunning = flag.Bool("update-kernel-members-running", false, "rewrite the running-object Kernel member read goldens")

// runningReceiver is one object a running scenario reads its Kernel members on.
type runningReceiver struct {
	label string
	inst  *Instance
}

// runningPoint is one instant of a scenario's run: the objects read there and
// expressions evaluated on the root object besides the enumerated members.
type runningPoint func(label string, receivers []runningReceiver, exprs ...string)

// runningScenario runs a model to a few instants and reads every Kernel Semantic
// Library member of the objects named at each; drive reports each instant.
type runningScenario struct {
	name  string
	model string
	// expect pins the answer the specification defines where it defines one, by
	// "<point>/<receiver>.<member>"; a measured row differing from it is "wrong".
	expect map[string]string
	drive  func(t *testing.T, ctx *Context, root *Instance, point runningPoint)
}

// runningReadOutcome describes a read for the running golden, naming the
// objects it holds by their scenario labels so the row says which object it is.
func runningReadOutcome(ctx *Context, labels map[int64]string, val Value, err error) string {
	if err != nil {
		return "refused: " + err.Error()
	}
	name := func(id int64) string {
		if l, ok := labels[id]; ok {
			return l
		}
		if inst, ok := ctx.instances[id]; ok {
			return "object of " + symbols.FQNOf(inst.Type)
		}
		return fmt.Sprintf("object #%d", id)
	}
	switch val.Kind {
	case ValInstance:
		return name(val.Instance)
	case ValSequence, ValSet:
		elems := elementsOf(val)
		if len(elems) == 0 {
			return "empty"
		}
		parts := make([]string, 0, len(elems))
		for _, e := range elems {
			if e.Kind == ValInstance {
				parts = append(parts, name(e.Instance))
			} else {
				parts = append(parts, FormatValue(e))
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ValNull:
		return "null"
	case ValUndetermined:
		return "undetermined " + FormatValue(val)
	}
	return fmt.Sprintf("%s %s", val.Kind, FormatValue(val))
}

// runningBucket classifies a running read against an unexecuted object's answer:
// populated, static-only, undetermined, refused, or wrong against a pinned spec value.
func runningBucket(outcome, static, want string, err error) string {
	switch {
	case want != "" && outcome != want:
		return "wrong"
	case err != nil:
		return "refused"
	case strings.HasPrefix(outcome, "undetermined"):
		return "undetermined"
	case outcome == static:
		return "static-only"
	}
	return "populated"
}

// TestKernelLibraryMembersReadableWhileRunning pins every public Kernel Semantic
// Library member at points in seven runs, one golden per scenario (pilot=decl: the pilot cannot run them).
func TestKernelLibraryMembersReadableWhileRunning(t *testing.T) {
	for _, sc := range runningScenarios {
		t.Run(sc.name, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, sc.model)
			static, staticIdx := libraryShapeContext(t, sc.model)
			root := mustInstantiate(t, ctx, oneSymbol(t, idx, sc.rootFQN()))

			var golden bytes.Buffer
			counts := map[string]int{}
			fmt.Fprintf(&golden, "# scenario %s\n# pilot=decl: the OMG pilot evaluator cannot execute a performance; its evidence is declaration-level only\n", sc.name)
			labels := map[int64]string{}
			unexecuted := map[string]*Instance{}
			point := func(label string, receivers []runningReceiver, exprs ...string) {
				for _, r := range receivers {
					labels[r.inst.ID] = r.label
				}
				fmt.Fprintf(&golden, "\n## %s (clock %s)\n", label, FormatValue(ctx.ClockValue()))
				for _, r := range receivers {
					fqn := symbols.FQNOf(r.inst.Type)
					base, ok := unexecuted[fqn]
					if !ok {
						var err error
						base, err = static.materialize(oneSymbol(t, staticIdx, fqn), 0, nil, "")
						if err != nil {
							t.Fatalf("materialize unexecuted %s: %v", fqn, err)
						}
						unexecuted[fqn] = base
					}
					fmt.Fprintf(&golden, "### %s (type %s)\n", r.label, fqn)
					for _, m := range kernelTierMembers(ctx, r.inst.Type) {
						val, err := evalOn(t, ctx, r.inst, "self."+m.Name)
						outVal, outErr := r.inst.MemberValue(ctx, m.Name)
						sVal, sErr := evalOn(t, static, base, "self."+m.Name)
						outcome := runningReadOutcome(ctx, labels, val, err)
						staticOutcome := runningReadOutcome(static, map[int64]string{base.ID: r.label}, sVal, sErr)
						want := sc.expect[label+"/"+r.label+"."+m.Name]
						bucket := runningBucket(outcome, staticOutcome, want, err)
						counts[bucket]++
						fmt.Fprintf(&golden, "%s: %s  [%s pilot=decl]", m.Name, outcome, bucket)
						if want != "" && bucket == "wrong" {
							fmt.Fprintf(&golden, " spec: %s", want)
						}
						if outside := runningReadOutcome(ctx, labels, featureValueOf(outVal), outErr); outside != outcome {
							fmt.Fprintf(&golden, " (outside read: %s)", outside)
						}
						golden.WriteString("\n")
					}
				}
				if len(exprs) > 0 {
					fmt.Fprintf(&golden, "### expressions on %s\n", labels[root.ID])
					for _, e := range exprs {
						val, err := evalOn(t, ctx, root, e)
						outcome := runningReadOutcome(ctx, labels, val, err)
						want := sc.expect[label+"/"+e]
						bucket := runningBucket(outcome, "", want, err)
						counts[bucket]++
						fmt.Fprintf(&golden, "%s: %s  [%s pilot=decl]", e, outcome, bucket)
						if want != "" && bucket == "wrong" {
							fmt.Fprintf(&golden, " spec: %s", want)
						}
						golden.WriteString("\n")
					}
				}
			}
			sc.drive(t, ctx, root, point)

			golden.WriteString("\n## buckets\n")
			keys := make([]string, 0, len(counts))
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&golden, "%s: %d\n", k, counts[k])
			}

			path := filepath.Join("testdata", "kernel_member_reads_running", sc.name+".golden")
			if *updateKernelMembersRunning {
				if err := os.WriteFile(path, golden.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update-kernel-members-running to record it)", err)
			}
			if !bytes.Equal(want, golden.Bytes()) {
				t.Errorf("running Kernel member reads differ from %s; review and rerun with -update-kernel-members-running.\n--- got ---\n%s", path, golden.String())
			}
		})
	}
}

// featureValueOf is the value a feature read holds, or none for no read.
func featureValueOf(fv *FeatureValue) Value {
	if fv == nil {
		return Value{}
	}
	if fv.Values.Kind != ValInvalid {
		return fv.Values
	}
	return fv.Value
}

// rootFQN names the object each scenario materializes and reads from.
func (sc runningScenario) rootFQN() string { return "Running::" + sc.name + "::Root" }

// receiversOn resolves labelled feature chains on root to the objects they hold.
func receiversOn(t *testing.T, ctx *Context, root *Instance, chains ...string) []runningReceiver {
	t.Helper()
	receivers := []runningReceiver{{label: "root", inst: root}}
	for _, chain := range chains {
		val, err := evalOn(t, ctx, root, chain)
		if err != nil || val.Kind != ValInstance {
			continue // a chain that names no object yet is reported as an expression row
		}
		inst, ok := ctx.Instance(val.Instance)
		if !ok {
			t.Fatalf("%s names object #%d, which the run does not hold", chain, val.Instance)
		}
		receivers = append(receivers, runningReceiver{label: chain, inst: inst})
	}
	return receivers
}

// advance moves the shared clock, failing the test on an error.
func advance(t *testing.T, ctx *Context, seconds float64) {
	t.Helper()
	if _, err := ctx.Advance(seconds); err != nil {
		t.Fatalf("advance %gs: %v", seconds, err)
	}
}

// timeExprs are the Clocks/Time function and association evaluations over the
// named occurrences, read on the root object.
func timeExprs(a, b string) []string {
	return []string{
		"Time::TimeOf(" + a + ")",
		"Time::DurationOf(" + a + ")",
		"Clocks::TimeOf(" + a + ", Clocks::universalClock)",
		"Clocks::DurationOf(" + a + ", Clocks::universalClock)",
		a + ".localClock.currentTime",
		"Clocks::universalClock.currentTime",
		"Occurrences::HappensBefore(" + a + ", " + b + ")",
		"Occurrences::HappensDuring(" + a + ", " + b + ")",
		"Occurrences::Without(" + a + ", " + b + ")",
		"Occurrences::HappensWhile(" + a + ", " + b + ")",
		"Occurrences::HappensLink(" + a + ", " + b + ")",
		a + ".successors->SequenceFunctions::includes(" + b + ")",
		b + ".predecessors->SequenceFunctions::includes(" + a + ")",
		a + ".timeEnclosedOccurrences->SequenceFunctions::includes(" + b + ")",
	}
}

var runningScenarios = []runningScenario{
	{
		name: "nested_action_completed",
		model: `package Running { package nested_action_completed {
			private import ScalarValues::*;
			part def Root {
				attribute n : Integer = 0;
				perform action act {
					first start;
					then action s1 assign n := n + 1;
					then action s2 assign n := n + 1;
					then done;
				}
			}
		} }`,
		expect: map[string]string{
			"after completion/act.done": "object of Running::nested_action_completed::Root::act",
		},
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			point("after completion", receiversOn(t, ctx, root, "act"),
				append([]string{"n", "act.s1", "act.s2", "act.s1.startShot", "act.s2.predecessors", "act.start", "act.done"}, timeExprs("act", "self")...)...)
		},
	},
	{
		name: "clock_wait",
		model: `package Running { package clock_wait {
			private import ScalarValues::*;
			part def Root {
				attribute n : Integer = 0;
				perform action act {
					first start;
					then action w accept after 60.0 [SI::s];
					then action m assign n := n + 1;
					then done;
				}
			}
		} }`,
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			exprs := append([]string{"n", "act.w", "act.m", "act.start", "act.done"}, timeExprs("act", "self")...)
			point("waiting at accept after 60 s", receiversOn(t, ctx, root, "act"), exprs...)
			advance(t, ctx, 30)
			point("still waiting at 30 s", receiversOn(t, ctx, root, "act"), exprs...)
			advance(t, ctx, 30)
			point("after the wait", receiversOn(t, ctx, root, "act"), exprs...)
		},
	},
	{
		name: "succession_chain",
		model: `package Running { package succession_chain {
			private import ScalarValues::*;
			part def Root {
				attribute n : Integer = 0;
				perform action a assign n := n + 1;
				perform action b { first start; then action w accept after 5.0 [SI::s]; then done; }
				perform action c assign n := n + 100;
				first a then b;
				first b then c;
			}
		} }`,
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			exprs := append([]string{"n", "a", "b", "c", "b.predecessors", "c.predecessors", "a.successors", "b.immediateSuccessors"}, timeExprs("a", "b")...)
			point("a fired, b waiting, c not started", receiversOn(t, ctx, root, "a", "b", "c"), exprs...)
			advance(t, ctx, 5)
			point("all fired", receiversOn(t, ctx, root, "a", "b", "c"), exprs...)
		},
	},
	{
		name: "state_machine",
		model: `package Running { package state_machine {
			private import ScalarValues::*;
			attribute def Go;
			part def Root {
				attribute n : Integer = 0;
				exhibit state sm {
					entry action en assign n := n + 1000;
					then s1;
					state s1 {
						entry assign n := n + 1;
						do action d assign n := n + 5;
						exit assign n := n + 2;
					}
					transition t12 first s1 accept Go then s2;
					state s2 { entry assign n := n + 20; }
					transition t23 first s2 accept after 3.0 [SI::s] then s3;
					state s3;
				}
			}
		} }`,
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			exprs := append([]string{"n", "sm.s1", "sm.s2", "sm.en", "sm.start", "sm.done", "sm.t12"}, timeExprs("sm", "self")...)
			point("in s1 after entry and do", receiversOn(t, ctx, root, "sm"), exprs...)
			ctx.PostMessage(Message{SignalType: "Go", Object: root.ID})
			if err := ctx.drainObjectBehaviors(); err != nil {
				t.Fatal(err)
			}
			point("in s2 after s1 exit", receiversOn(t, ctx, root, "sm"), exprs...)
			advance(t, ctx, 3)
			point("in s3 after the timed transition", receiversOn(t, ctx, root, "sm"), exprs...)
		},
	},
	{
		name: "transfer",
		model: `package Running { package transfer {
			private import ScalarValues::*;
			attribute def Ping;
			port def PP { out item p : Ping; }
			part def Sender {
				port o : PP;
				perform action snd {
					first start;
					then action w accept after 2.0 [SI::s];
					then action s send new Ping() via o;
					then done;
				}
			}
			part def Recv {
				port i : ~PP;
				attribute got : Integer = 0;
				perform action rcv {
					first start;
					then action r accept Ping via i;
					then action g assign got := got + 1;
					then done;
				}
			}
			part def Root {
				part x : Sender;
				part y : Recv;
				connect x.o to y.i;
			}
		} }`,
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			exprs := append([]string{"y.got", "x.snd.s", "y.rcv.r", "x.o.outgoingTransfers", "y.i.incomingTransfers", "x.snd.outgoingTransfers->SequenceFunctions::size()"}, timeExprs("x.snd", "y.rcv")...)
			chains := []string{"x", "y", "x.o", "y.i", "x.snd", "y.rcv"}
			point("before the send", receiversOn(t, ctx, root, chains...), exprs...)
			advance(t, ctx, 5)
			point("after the transfer", receiversOn(t, ctx, root, chains...), exprs...)
		},
	},
	{
		name: "dynamic_creation",
		model: `package Running { package dynamic_creation {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			private import SequenceFunctions::*;
			part def Car { attribute n : Integer; }
			part def Root {
				part cars : Car[0..*];
				perform action build {
					first start;
					then action make1 assign cars := addNew(cars, new Car(1));
					then action w accept after 1.0 [SI::s];
					then action make2 assign cars := addNew(cars, new Car(2));
					then done;
				}
			}
		} }`,
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			exprs := append([]string{"size(cars)", "cars#(1).suboccurrences", "isDuring(cars#(1))", "self.suboccurrences"}, timeExprs("build", "cars#(1)")...)
			point("one object created, waiting", receiversOn(t, ctx, root, "build", "cars#(1)"), exprs...)
			advance(t, ctx, 1)
			point("after completion with two objects", receiversOn(t, ctx, root, "build", "cars#(1)", "cars#(2)"), exprs...)
		},
	},
	{
		name: "loop",
		model: `package Running { package loop {
			private import ScalarValues::*;
			part def Root {
				attribute k : Integer = 0;
				perform action loopy {
					first start;
					then loop action body {
						first start;
						then action w accept after 1.0 [SI::s];
						then action inc assign k := k + 1;
						then done;
					} until k >= 3;
					then done;
				}
			}
		} }`,
		drive: func(t *testing.T, ctx *Context, root *Instance, point runningPoint) {
			exprs := append([]string{"k", "loopy.body", "loopy.body.w", "loopy.start", "loopy.done"}, timeExprs("loopy", "self")...)
			point("first iteration waiting", receiversOn(t, ctx, root, "loopy"), exprs...)
			advance(t, ctx, 1.5)
			point("second iteration waiting", receiversOn(t, ctx, root, "loopy"), exprs...)
			advance(t, ctx, 10)
			point("after three iterations", receiversOn(t, ctx, root, "loopy"), exprs...)
		},
	},
}
