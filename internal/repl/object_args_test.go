package repl

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// %state <machine> <object>, for the machine the object exhibits, attaches to the
// running machine and says so: its entry action does not run against the object's
// slots a second time.
func TestStateOverTheExhibitedMachineAttachesToIt(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::rover")
	wants(t, run(t, s, "%features Fleet::rover"), "level = 10", `log = "W"`)

	got := run(t, s, "%state Fleet::Rover::modes Fleet::rover")
	wants(t, got, `Debugging state machine "modes" exhibited by object #`, "note:",
		`already exhibits "Fleet::Rover::modes"`, "attaches to that running machine", "`%state Fleet::rover`")
	if strings.Contains(got, "Started state machine executor") {
		t.Errorf("a second performance of the exhibited machine was started:\n%s", got)
	}

	features := run(t, s, "%features Fleet::rover")
	wants(t, features, "level = 10", `log = "W"`)
	for _, twice := range []string{"level = 20", `log = "WW"`} {
		if strings.Contains(features, twice) {
			t.Errorf("the entry action ran twice (%s):\n%s", twice, features)
		}
	}

	// The session drives the object's own machine.
	wants(t, run(t, s, "%advance 5"), "Current state: moving")
	wants(t, run(t, s, "%features Fleet::rover"), `log = "WM"`)
}

// An exhibited usage stating its own body under a definition's type is the
// object's machine of that definition too: naming the definition attaches to it.
func TestStateOverTheDefinitionOfABodyStatingUsageAttachesToIt(t *testing.T) {
	s := loadFixture(t, "testdata/typed_body_machine.sysml")
	run(t, s, "%instantiate Fleet::tank")
	wants(t, run(t, s, "%features Fleet::tank.m"), "level = 10", `log = "W"`)

	got := run(t, s, "%state Fleet::Mission Fleet::tank")
	wants(t, got, `Debugging state machine "m" exhibited by object #`, "note:",
		`already exhibits "Fleet::Mission"`, "attaches to that running machine", "`%state Fleet::tank`")
	if strings.Contains(got, "Started state machine executor") {
		t.Errorf("a second performance of the exhibited machine was started:\n%s", got)
	}
	wants(t, run(t, s, "%state Fleet::Mission"), `Debugging state machine "m" exhibited by object #`, `of "Fleet::tank"`)
	wants(t, run(t, s, "%features Fleet::tank.m"), "level = 10", `log = "W"`)
}

// A machine the object merely performs is still a detached run, as before.
func TestStateOverAPerformedMachineStillStartsIt(t *testing.T) {
	s := loadFixture(t, "testdata/performed_machine.sysml")
	run(t, s, "%instantiate Two::g")

	wants(t, run(t, s, "%state Two::Check Two::g"), "Started state machine executor")
}

// %state addresses a nested part by a feature path from a top-level object and by
// the identity the prompt prints, in both the one- and two-argument forms.
func TestStateAddressesANestedPartByPathAndById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")

	byPath := run(t, s, "%state Fleet::driver.r")
	wants(t, byPath, `Debugging state machine "modes" exhibited by object #`, `of "Fleet::driver.r"`, "Current state: waiting")
	id := objectIDIn(t, byPath)

	wants(t, run(t, s, "%state #"+id), `exhibited by object #`+id+"\n")
	wants(t, run(t, s, "%state Fleet::driver::r"), `exhibited by object #`+id+` of "Fleet::driver.r"`)

	attached := run(t, s, "%state Fleet::Rover::modes Fleet::driver.r")
	wants(t, attached, `exhibited by object #`+id, "note:", "attaches to that running machine", "`%state Fleet::driver.r`")
	wants(t, run(t, s, "%state Fleet::Rover::modes #"+id), `exhibited by object #`+id, "note:", "`%state #"+id+"`")

	wants(t, run(t, s, "%features Fleet::driver.r"), "Instance: Fleet::driver.r (ID: "+id+")", "level = 10", `log = "W"`)
	wants(t, run(t, s, "%features #"+id), "Instance: #"+id+" (ID: "+id+")", "level = 10")
}

// %invoke addresses a nested part by path and by identity, and what the operation
// writes is that object's slot.
func TestInvokeAddressesANestedPartByPathAndById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")
	id := objectIDIn(t, run(t, s, "%features Fleet::driver.r"))

	wants(t, run(t, s, "%invoke Fleet::driver.r bump"), "✓ Invoked bump on object #"+id+` of "Fleet::driver.r"`)
	byID := run(t, s, "%invoke #"+id+" bump")
	wants(t, byID, "✓ Invoked bump on object #"+id)
	if strings.Contains(byID, ` of "`) {
		t.Errorf("an object addressed by id was reported under a name:\n%s", byID)
	}
	wants(t, run(t, s, "%features Fleet::driver.r"), "level = 12")
	wants(t, run(t, s, "%features Fleet::driver"), "level = 12")
}

// A qualified path through a usage (Fleet::driver::r) resolves to the feature's
// declaration (Fleet::Driver::r); with the definition instantiated too, the path
// still denotes the usage's part, on every command taking an object, and is
// reported as the walk it is: Fleet::driver.r.
func TestQualifiedPathDenotesTheUsageTypedNotItsDefinition(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::Driver")
	run(t, s, "%instantiate Fleet::driver")
	defR := objectIDIn(t, run(t, s, "%features Fleet::Driver.r"))
	usageR := objectIDIn(t, run(t, s, "%features Fleet::driver.r"))
	if defR == usageR {
		t.Fatalf("the definition's and the usage's parts are one object #%s", defR)
	}

	wants(t, run(t, s, "%features Fleet::driver::r"), `Instance: Fleet::driver.r (ID: `+usageR+")")
	wants(t, run(t, s, "%features Fleet::Driver::r"), `Instance: Fleet::Driver.r (ID: `+defR+")")

	wants(t, run(t, s, "%invoke Fleet::driver::r bump"), "on object #"+usageR+` of "Fleet::driver.r"`)
	wants(t, run(t, s, "%features Fleet::driver"), "level = 11")
	wants(t, run(t, s, "%features Fleet::Driver"), "level = 10")

	wants(t, run(t, s, "%state Fleet::driver::r"), `exhibited by object #`+usageR+` of "Fleet::driver.r"`)
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::driver::r"), `exhibited by object #`+usageR, "note:")
	wants(t, run(t, s, "%state Fleet::Driver::r"), `exhibited by object #`+defR+` of "Fleet::Driver.r"`)
}

// A member of a multi-valued part is reached by its index in the owner's feature
// (garage.bays[2]) or by its id; the collection itself is no object. Addressed by
// id, the prompt names it by id alone, and a session attached to it by id survives
// an unrelated declaration.
func TestCollectionMemberIsAddressedByIndexOrById(t *testing.T) {
	s := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, s, "%instantiate Depot::garage")
	listing := run(t, s, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	first := objectIDIn(t, bays)
	id := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])
	if first == id {
		t.Fatalf("bays holds one member #%s, want two", id)
	}

	features := run(t, s, "%features #"+id)
	wants(t, features, "Instance: #"+id+" (ID: "+id+")", "level = 10")
	if strings.Contains(features, "bays") {
		t.Errorf("a member of bays was named as the whole collection:\n%s", features)
	}
	wants(t, run(t, s, "%features Depot::garage.bays[2]"), "Instance: Depot::garage.bays[2] (ID: "+id+")", "level = 10")
	wants(t, run(t, s, "%features Depot::garage::bays"), "error:", "bays of Depot::garage holds 2 objects: pick one by index, bays[1] to bays[2]")

	got := run(t, s, "%state #"+id)
	wants(t, got, `Debugging state machine "modes" exhibited by object #`+id+"\n", "Current state: waiting")
	if strings.Contains(got, "bays") {
		t.Errorf("the member's machine was reported under the collection's name:\n%s", got)
	}
	if s.stateExec.selfFQN != "#"+id {
		t.Errorf("the session holds the member as %q, want #%s", s.stateExec.selfFQN, id)
	}

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	if hasNotice(res, "debugging session") {
		t.Errorf("the session over the member's machine was ended:\n%s", strings.Join(res.Notices, "\n"))
	}
	wants(t, run(t, s, "%current"), "waiting")
	wants(t, run(t, s, "%advance 5"), "Current state: moving")

	// The advance moved the clock every machine of the context shares: the other
	// member moved with it, and each session still names its own object.
	wants(t, run(t, s, "%state #"+first), `exhibited by object #`+first, "Current state: moving")
	wants(t, run(t, s, "%state #"+id), `exhibited by object #`+id, "Current state: moving")
}

// A path that stops short of an object is a typed error naming the segment that
// reached none and why, on every command taking an object.
func TestObjectPathErrorsNameTheSegment(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")

	cases := []struct {
		arg, object, segment, detail string
	}{
		{"Fleet::driver.x", "Fleet::driver", "x", `Fleet::driver has no feature "x" (its features are r, and 13 more the library declares)`},
		{"Fleet::driver.r.level", "Fleet::driver.r", "level", "level of Fleet::driver.r holds a value (10), not an object"},
		{"Fleet::driver.r.nothing", "Fleet::driver.r", "nothing", `Fleet::driver.r has no feature "nothing"`},
	}
	for _, c := range cases {
		_, _, err := s.resolveObject(c.arg)
		var perr *ObjectPathError
		if !errors.As(err, &perr) {
			t.Errorf("%s: got %v, want an ObjectPathError", c.arg, err)
			continue
		}
		if perr.Object != c.object || perr.Segment != c.segment || !strings.Contains(perr.Detail, c.detail) {
			t.Errorf("%s: got %+v, want object %q, segment %q, detail %q", c.arg, perr, c.object, c.segment, c.detail)
		}
		wants(t, run(t, s, "%state "+c.arg), "error: "+c.detail)
		wants(t, run(t, s, "%state Fleet::Rover::modes "+c.arg), "error: "+c.detail)
		wants(t, run(t, s, "%invoke "+c.arg+" bump"), "error: "+c.detail)
		wants(t, run(t, s, "%features "+c.arg), "error: "+c.detail)
	}

	_, _, err := s.resolveObject("#99")
	var nerr *UnknownObjectIDError
	if !errors.As(err, &nerr) || nerr.ID != 99 {
		t.Errorf("#99: got %v, want an UnknownObjectIDError for 99", err)
	}
	wants(t, run(t, s, "%state #99"), "error: no object #99 in this session: nothing materialized has that identity (the objects are #1, #2")
	wants(t, run(t, s, "%invoke #99 bump"), "error: no object #99 in this session: nothing materialized has that identity (the objects are #1, #2")
}

// When only the definition a usage is typed by was materialized, the error says
// so and names the usage to instantiate; the other way round it names the usage
// whose object exists. With nothing related, the plain hint stands.
func TestNotInstantiatedNamesWhatToInstantiate(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")

	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::rover"), `error: no instance of "Fleet::rover" (use %instantiate first)`)

	defOnly := objectIDIn(t, run(t, s, "%instantiate Fleet::Rover"))
	got := run(t, s, "%state Fleet::Rover::modes Fleet::rover")
	wants(t, got, `error: no instance of the usage "Fleet::rover"`,
		`object #`+defOnly+` of "Fleet::Rover" is of its definition "Fleet::Rover", not of the usage`,
		"use %instantiate Fleet::rover to create the usage's object", "or name Fleet::Rover to address it")
	wants(t, run(t, s, "%invoke Fleet::rover bump"), `no instance of the usage "Fleet::rover"`, "%instantiate Fleet::rover")

	_, _, err := s.resolveObject("Fleet::rover")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || !nerr.UsageAsked || nerr.Definition != "Fleet::Rover" || len(nerr.Objects) != 1 || nerr.Objects[0].Label != "Fleet::Rover" {
		t.Errorf("got %v (%+v), want a NotInstantiatedError naming Fleet::Rover's object", err, nerr)
	}

	// A nested object of the definition counts under the path reaching it. A part
	// whose type exhibits a machine is created, and its machine run, with the
	// object holding it, so the error names it as soon as that object exists.
	run(t, s, "%instantiate Fleet::driver")
	if fv := s.instances["Fleet::driver"].FeatureValues["r"]; !fv.Materialized {
		t.Errorf("instantiating Fleet::driver left its machine-exhibiting part unread: %+v", fv)
	}
	wants(t, run(t, s, "%invoke Fleet::rover bump"), `no instance of the usage "Fleet::rover"`)
	wants(t, run(t, s, "%features Fleet::driver.r"), `log = "W"`)
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::rover"), `objects #`+defOnly+` of "Fleet::Rover"`,
		`of "Fleet::driver.r"`, "or name Fleet::Rover or Fleet::driver.r to address one of them")

	// Asking for the definition when only a usage's object exists names the usage.
	fresh := loadFixture(t, "testdata/nested_machine.sysml")
	usage := objectIDIn(t, run(t, fresh, "%instantiate Fleet::rover"))
	wants(t, run(t, fresh, "%invoke Fleet::Rover bump"), `error: no instance of the definition "Fleet::Rover" itself`,
		`object #`+usage+` of "Fleet::rover" is typed by it`, "name Fleet::rover to address it", "%instantiate Fleet::Rover")
}

// A usage that reaches its definition only through the usage it subsets is still
// of that definition, so objects of it are named the same way.
func TestNotInstantiatedFindsTheDefinitionThroughASubsettedUsage(t *testing.T) {
	s := loadFixture(t, "testdata/indirect_usage.sysml")
	wants(t, run(t, s, "%invoke Fleet::scout bump"), `error: no instance of "Fleet::scout" (use %instantiate first)`)

	defOnly := objectIDIn(t, run(t, s, "%instantiate Fleet::Rover"))
	rover := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	wants(t, run(t, s, "%invoke Fleet::scout bump"), `error: no instance of the usage "Fleet::scout"`,
		`objects #`+defOnly+` of "Fleet::Rover", #`+rover+` of "Fleet::rover" are of its definition "Fleet::Rover", not of the usage`,
		"use %instantiate Fleet::scout to create the usage's object", "or name Fleet::Rover or Fleet::rover to address one of them")
	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::scout"), `no instance of the usage "Fleet::scout"`, `of its definition "Fleet::Rover"`)

	_, _, err := s.resolveObject("Fleet::scout")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || !nerr.UsageAsked || nerr.Definition != "Fleet::Rover" || len(nerr.Objects) != 2 {
		t.Errorf("got %v (%+v), want a NotInstantiatedError naming Fleet::Rover's two objects", err, nerr)
	}
}

// A session over a nested part's machine survives an unrelated declaration as one
// over a top-level object does: the object keeps its identity and the session
// follows the restarted machine.
func TestNestedObjectMachineSurvivesAnUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")
	id := objectIDIn(t, run(t, s, "%state Fleet::driver.r"))
	wants(t, run(t, s, "%advance 5"), "Current state: moving")

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	if hasNotice(res, "debugging session") {
		t.Errorf("the session over the nested part's machine was ended:\n%s", strings.Join(res.Notices, "\n"))
	}

	wants(t, run(t, s, "%current"), "waiting")
	wants(t, run(t, s, "%features #"+id), "Instance: #"+id+" (ID: "+id+")", `log = "W"`)
	wants(t, run(t, s, "%advance 5"), "Current state: moving")
}

// A session attached to the second of two exhibited machines follows that
// machine through the restart an unrelated declaration causes, not the first.
func TestSecondExhibitedMachineSurvivesAnUnrelatedDeclaration(t *testing.T) {
	s := loadFixture(t, "testdata/two_machines.sysml")
	run(t, s, "%instantiate Pair::gauge")
	wants(t, run(t, s, "%state Pair::Gauge::link Pair::gauge"), `Debugging state machine "link"`, "Current state: idle")

	res := s.Submit("package Other { part def Unrelated; }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("unrelated declaration has diagnostics: %v", res.Diagnostics)
	}
	if hasNotice(res, "debugging session") {
		t.Errorf("the session over the second machine was ended:\n%s", strings.Join(res.Notices, "\n"))
	}

	current := run(t, s, "%current")
	wants(t, current, "idle")
	if strings.Contains(current, "off") {
		t.Errorf("the session moved to the first exhibited machine:\n%s", current)
	}
	wants(t, run(t, s, "%advance 3"), "Current state: busy")
	wants(t, run(t, s, "%features Pair::gauge"), "power", "off", "link", "busy")
}

// A definition an object exhibits as the body of two usages names no one
// running machine, so %state over it refuses and names the usages that would,
// rather than attaching to whichever was declared first. Naming a usage attaches.
func TestStateOverASharedDefinitionNamesTheUsages(t *testing.T) {
	s := loadFixture(t, "testdata/shared_machine.sysml")
	run(t, s, "%instantiate Shared::lamp")

	_, err := s.startStateMachine("Shared::Blink", []string{"Shared::lamp"})
	var aerr *AmbiguousMachineError
	if !errors.As(err, &aerr) {
		t.Fatalf("got %v, want an AmbiguousMachineError", err)
	}
	if aerr.Machine != "Shared::Blink" || strings.Join(aerr.Usages, ",") != "Shared::Lamp::front,Shared::Lamp::rear" {
		t.Errorf("got %+v, want both exhibited usages of Shared::Blink", aerr)
	}
	if s.stateExec != nil {
		t.Errorf("a machine was attached to despite the ambiguity: %q", s.stateExec.machine)
	}

	got := run(t, s, "%state Shared::Blink Shared::lamp")
	wants(t, got, "error:", `object #1 of "Shared::lamp" exhibits "Shared::Blink" as 2 machines`,
		"name the exhibited usage instead", "Shared::Lamp::front or Shared::Lamp::rear")
	for _, attached := range []string{"Debugging state machine", "Started state machine executor"} {
		if strings.Contains(got, attached) {
			t.Errorf("a machine was selected despite the ambiguity:\n%s", got)
		}
	}

	wants(t, run(t, s, "%state Shared::Lamp::rear Shared::lamp"), `Debugging state machine "rear"`, "note:", "Current state: dark")
	// Both usages wait on the object's one clock, so the advance lights both.
	wants(t, run(t, s, "%advance 2"), "Current state: lit")
	wants(t, run(t, s, "%features Shared::lamp"), "front: exhibited state machine, current state lit", "rear: exhibited state machine, current state lit")
}

// A command's expressions are parsed before its object is reached: a malformed
// one names a part not yet materialized without creating it or using up an id.
func TestMalformedExpressionsMaterializeNothing(t *testing.T) {
	for _, tc := range []struct{ line, reject string }{
		{"%eval in car.fl : radius *", "car.fl"},
		{"%invoke car.fl spin turns=1 +", "car.fl"},
		{"%invoke car.fl spin 1", "car.fl"},
		{"%send Spin(turns=1 +) to car.fl", "car.fl"},
		{"%send Spin(1) to car.fl", "car.fl"},
		{"%send Spin(turns=1, turns=2) to car.fl", "car.fl"},
	} {
		t.Run(tc.line, func(t *testing.T) {
			s := loadSource(t, `package Demo {
				private import ScalarValues::*;
				item def Spin { attribute turns : Integer; }
				part def Wheel { attribute radius = 0.3; }
				part def Car { part fl : Wheel; }
				part car : Car;
			}`)
			wants(t, run(t, s, "%instantiate Demo::car"), "ID: 1")
			held := s.rtCtx.InstanceIDs()

			got := run(t, s, tc.line)
			wants(t, got, "error:")
			rejects(t, got, tc.reject)
			if now := s.rtCtx.InstanceIDs(); len(now) != len(held) {
				t.Fatalf("the rejected command materialized objects: %v were held, now %v", held, now)
			}
			wants(t, run(t, s, "%eval in car.fl : radius * 2"), "= 0.6", "(on Demo::car.fl ID: 2)")
		})
	}
}

// A segment whose feature value the runtime could not materialize is not a missing
// feature: the path error carries the runtime's reason and its typed cause, and
// the failure reaches the session status as any other command's would.
func TestObjectPathErrorKeepsTheMaterializationFailure(t *testing.T) {
	s := loadFixture(t, "testdata/shared_machine.sysml")
	run(t, s, "%instantiate Shared::lamp")

	_, _, err := s.resolveObject("Shared::lamp.spare")
	var perr *ObjectPathError
	if !errors.As(err, &perr) {
		t.Fatalf("got %v, want an ObjectPathError", err)
	}
	if perr.Object != "Shared::lamp" || perr.Segment != "spare" || strings.Contains(perr.Detail, "has no feature") {
		t.Errorf("got %+v, want the segment spare reported as failing to materialize", perr)
	}
	wants(t, perr.Detail, `spare of Shared::lamp could not be materialized`, "multiplicity violation")
	if !errors.Is(err, runtime.ErrFeatureValueMaterialization) || !errors.Is(err, runtime.ErrMultiplicityViolation) {
		t.Errorf("%v does not carry the runtime's materialization failure", err)
	}

	want := `error: spare of Shared::lamp could not be materialized`
	wants(t, run(t, s, "%state Shared::lamp.spare"), want, "multiplicity violation")
	wants(t, run(t, s, "%invoke Shared::lamp.spare flip"), want, "multiplicity violation")
	if got := s.MaterializationFailures(); len(got) != 2 || !errors.Is(got[0], runtime.ErrMultiplicityViolation) {
		t.Errorf("materialization failures = %v, want the multiplicity violation from each command", got)
	}

	if v := s.RunStateMachine("Shared::lamp.spare"); v.Status != VerdictUnresolved || !strings.Contains(strings.Join(v.Lines, "\n"), "multiplicity violation") {
		t.Errorf("non-interactive run: got %+v, want an unresolved verdict naming the multiplicity violation", v)
	}
}

// Members of a multi-valued part are among the objects a not-instantiated error
// names, each by its index in the collection, and a feature they all carry is a
// question between them. Members exhibiting a machine exist with their holder.
func TestNotInstantiatedNamesCollectionMembersByIndex(t *testing.T) {
	s := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, s, "%instantiate Depot::garage")
	if fv := s.instances["Depot::garage"].FeatureValues["bays"]; !fv.Materialized {
		t.Fatalf("instantiating Depot::garage left its machine-exhibiting members unread: %+v", fv)
	}

	listing := run(t, s, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	first := objectIDIn(t, bays)
	second := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])

	got := run(t, s, "%state Depot::Rover::modes Depot::Rover")
	wants(t, got, `error: no instance of the definition "Depot::Rover" itself`,
		"objects #"+first+` of "Depot::garage.bays[1]", #`+second+` of "Depot::garage.bays[2]" are typed by it`,
		"name Depot::garage.bays[1] or Depot::garage.bays[2] to address one of them",
		"%instantiate Depot::Rover")
	_, _, err := s.resolveObject("Depot::Rover")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || nerr.UsageAsked || len(nerr.Objects) != 2 {
		t.Fatalf("got %v (%+v), want a NotInstantiatedError naming both members", err, nerr)
	}
	for i, o := range nerr.Objects {
		if want := fmt.Sprintf("Depot::garage.bays[%d]", i+1); o.Label != want {
			t.Errorf("member #%d is named %q, want %s", o.ID, o.Label, want)
		}
	}
	if s.stateExec != nil {
		t.Error("a machine was attached to despite the error")
	}

	wants(t, run(t, s, "%eval Depot::Rover::level"), "error:", "carried by more than one object of this session (Depot::garage.bays[1], Depot::garage.bays[2])")
}

// A member of a multi-valued part that a single-valued part of the same owner
// also holds is reached by that part's path, not its index — whichever of the
// owner's feature values is read first — and named so by every surface.
func TestCollectionMemberHeldAloneElsewhereKeepsItsPath(t *testing.T) {
	s := loadFixture(t, "testdata/shared_member.sysml")
	run(t, s, "%instantiate Depot::garage")
	listing := run(t, s, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	front := objectIDIn(t, bays)
	other := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])
	wants(t, listing, "front = Instance(ID: "+front+")", "lead = Instance(ID: "+front+")")

	got := run(t, s, "%state Depot::Rover::modes Depot::Rover")
	wants(t, got, `objects #`+front+` of "Depot::garage.front", #`+other+` of "Depot::garage.bays[2]" are typed by it`,
		"name Depot::garage.front or Depot::garage.bays[2] to address one of them")
	_, _, err := s.resolveObject("Depot::Rover")
	var nerr *NotInstantiatedError
	if !errors.As(err, &nerr) || len(nerr.Objects) != 2 || nerr.Objects[0].Label != "Depot::garage.front" || nerr.Objects[1].Label != "Depot::garage.bays[2]" {
		t.Errorf("got %v (%+v), want the shared member under front's path and the other by index", err, nerr)
	}

	wants(t, run(t, s, "%eval Depot::Rover::level"), "carried by more than one object of this session (Depot::garage.bays[2], Depot::garage.front)")
	wants(t, run(t, s, "%state Depot::garage.bays[1]"), `exhibited by object #`+front+` of "Depot::garage.bays[1]"`)
	wants(t, run(t, s, "%state #"+front), `exhibited by object #`+front+"\n")
	wants(t, run(t, s, "%features #"+front), "Instance: #"+front+" (ID: "+front+")")
	wants(t, run(t, s, "%state #"+other), `exhibited by object #`+other+"\n")
}

// After a second %instantiate of a name, the object it displaced stays
// addressable by identity on every command: the session still holds it, lists
// it, and says so in the notice. The current object is addressed by the name.
func TestSupersededObjectStaysAddressableById(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	first := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	wants(t, run(t, s, "%features #"+first), "Instance: #"+first+" (ID: "+first+")")

	again := run(t, s, "%instantiate Fleet::rover")
	second := objectIDIn(t, again)
	if second == first {
		t.Fatalf("the second %%instantiate reused object #%s", first)
	}
	wants(t, again, "note: Fleet::rover now denotes this object; object #"+first+
		" is displaced from that name, with 1 behavior of its own and stays reachable as #"+first)
	rejects(t, again, "debugging session")

	inst, label, err := s.resolveObject("#" + first)
	if err != nil || inst == nil || strconv.FormatInt(inst.ID, 10) != first || label != "#"+first {
		t.Errorf("#%s: got %v, %q, %v; want the displaced object under its id", first, inst, label, err)
	}
	wants(t, run(t, s, "%instances"),
		"Fleet::rover (ID: "+second+")",
		"#"+first+" (ID: "+first+", displaced from Fleet::rover)")
	wants(t, run(t, s, "%features #"+first), "Instance: #"+first+" (ID: "+first+")", "level = 10")
	wants(t, run(t, s, "%invoke #"+first+" bump"), "✓ Invoked bump on object #"+first)
	wants(t, run(t, s, "%features #"+first), "level = 11")
	wants(t, run(t, s, "%state #"+first), `exhibited by object #`+first+"\n", "Current state: waiting")
	wants(t, run(t, s, "%state Fleet::Rover::modes #"+first), `exhibited by object #`+first+"\n")

	// An id the runtime never issued is still no object, and says so.
	never := strconv.FormatInt(inst.ID+1000, 10)
	wants(t, run(t, s, "%features #"+never), "error: no object #"+never+" in this session: nothing materialized has that identity (the objects are #"+first+", ")

	wants(t, run(t, s, "%features #"+second), "Instance: #"+second+" (ID: "+second+")", "level = 10")
	wants(t, run(t, s, "%invoke #"+second+" bump"), "✓")
	wants(t, run(t, s, "%features Fleet::rover"), "level = 11")
	wants(t, run(t, s, "%state #"+second), `exhibited by object #`+second+"\n")
}

// The objects a current root's materialized features hold stay addressable by
// identity, members of a multi-valued part included; the check reads no feature
// that has not been materialized already.
func TestHeldObjectsStayAddressableById(t *testing.T) {
	lazy := loadFixture(t, "testdata/nested_part.sysml")
	run(t, lazy, "%instantiate Nested::Car")
	car := lazy.instances["Nested::Car"]
	if fv := car.FeatureValues["engine"]; fv != nil && fv.Materialized {
		t.Fatal("engine was materialized by %instantiate alone")
	}
	if _, _, err := lazy.resolveObject("#99"); err == nil {
		t.Error("#99 reached an object")
	}
	if fv := car.FeatureValues["engine"]; fv != nil && fv.Materialized {
		t.Error("looking up an identity materialized engine")
	}

	s := loadFixture(t, "testdata/nested_machine.sysml")
	run(t, s, "%instantiate Fleet::driver")
	if fv := s.instances["Fleet::driver"].FeatureValues["r"]; !fv.Materialized {
		t.Fatalf("instantiating Fleet::driver left its machine-exhibiting part unread: %+v", fv)
	}
	if _, _, err := s.resolveObject("#99"); err == nil {
		t.Error("#99 reached an object")
	}

	nested := objectIDIn(t, run(t, s, "%state Fleet::driver.r"))
	wants(t, run(t, s, "%features #"+nested), "Instance: #"+nested+" (ID: "+nested+")")

	c := loadFixture(t, "testdata/collection_machine.sysml")
	run(t, c, "%instantiate Depot::garage")
	listing := run(t, c, "%features Depot::garage")
	bays := listing[strings.Index(listing, "bays = [Instance(ID: "):]
	bays = bays[:strings.Index(bays, "]")]
	member := objectIDIn(t, bays[strings.LastIndex(bays, "Instance(ID: "):])
	wants(t, run(t, c, "%features #"+member), "Instance: #"+member+" (ID: "+member+")")
	wants(t, run(t, c, "%state #"+member), `exhibited by object #`+member, "Current state: waiting")
}

// A debugging session over the object a second %instantiate displaced keeps
// running: the notice says it now follows the object by id, and the next
// debugging command drives that object, not the one the name now denotes. One
// over the object a different name denotes is untouched.
func TestSupersededObjectKeepsItsDebuggingSession(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	first := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	wants(t, run(t, s, "%state Fleet::rover"), `Debugging state machine "modes"`, "Current state: waiting")

	again := run(t, s, "%instantiate Fleet::rover")
	second := objectIDIn(t, again)
	wants(t, again, `note: state debugging session for "Fleet::rover" keeps running over the object Fleet::rover named, now #`+first)
	rejects(t, again, "ended")
	if s.stateExec == nil {
		t.Fatal("the session over the displaced object ended")
	}
	if got := s.stateExec.selfFQN; got != "#"+first {
		t.Fatalf("debugger label after displacement = %q, want #%s", got, first)
	}
	wants(t, run(t, s, "%advance 5"), "Current state: moving")
	// Both objects run on the one clock the advance moved.
	wants(t, run(t, s, "%features #"+first), `log = "WM"`)
	wants(t, run(t, s, "%features #"+second), `log = "WM"`)

	// A debugger over a nested performing object follows it through its root's
	// id when the root's name is given away; one over another name is untouched.
	driver := objectIDIn(t, run(t, s, "%instantiate Fleet::driver"))
	nested := objectIDIn(t, run(t, s, "%state Fleet::driver.r"))
	wants(t, run(t, s, "%action Fleet::Rover::bump Fleet::driver.r"), "Started action executor")
	kept := run(t, s, "%instantiate Fleet::rover")
	rejects(t, kept, "debugging session")
	if s.actionExec == nil || s.actionExec.selfFQN != "Fleet::driver.r" {
		t.Fatal("instantiating an unrelated name touched the action session")
	}
	followed := run(t, s, "%instantiate Fleet::driver")
	wants(t, followed,
		`note: action debugging session for "Fleet::Rover::bump" keeps running over the object Fleet::driver.r named, now #`+driver+".r",
		`note: state debugging session for "Fleet::driver.r" keeps running over the object Fleet::driver.r named, now #`+driver+".r")
	if s.actionExec == nil || s.actionExec.selfFQN != "#"+driver+".r" {
		t.Fatalf("action session after displacement = %+v, want one over #%s.r", s.actionExec, driver)
	}
	if inst, _, err := s.resolveObject("#" + driver + ".r"); err != nil || strconv.FormatInt(inst.ID, 10) != nested {
		t.Fatalf("#%s.r resolves to %v, %v; want object #%s", driver, inst, err, nested)
	}
	wants(t, run(t, s, "%continue"), "✓ Action completed")
	wants(t, run(t, s, "%features #"+nested), "level = 11")
	wants(t, run(t, s, "%features Fleet::driver.r"), "level = 10")
}

// Every object the session holds is addressable by id, however many there are:
// the bound on a subject search does not apply to what is already materialized.
// A debugging session over a member beyond that bound survives an unrelated
// second %instantiate.
func TestEveryHeldObjectIsAddressableById(t *testing.T) {
	s := loadFixture(t, "testdata/large_collection.sysml")
	run(t, s, "%instantiate Farm::field")
	rows := collectionIDs(t, run(t, s, "%features Farm::field"), "rows")
	if len(rows) != 3 {
		t.Fatalf("rows holds %d members, want 3", len(rows))
	}
	held := 2 + len(rows) // field, spare and the rows
	var last string
	for _, row := range rows {
		cells := collectionIDs(t, run(t, s, "%features #"+row), "cells")
		held += len(cells)
		last = cells[len(cells)-1]
	}
	if held <= carrierLimit {
		t.Fatalf("the session holds %d objects, want more than %d", held, carrierLimit)
	}

	wants(t, run(t, s, "%features #"+last), "Instance: #"+last+" (ID: "+last+")", "count = 0")
	wants(t, run(t, s, "%features Farm::Cell"), `no instance of the definition "Farm::Cell" itself: objects #`,
		fmt.Sprintf(" … (%d in all) are typed by it", 3*1000))
	wants(t, run(t, s, "%invoke #"+last+" tick"), "✓ Invoked tick on object #"+last)
	wants(t, run(t, s, "%features #"+last), "count = 1")

	wants(t, run(t, s, "%action Farm::Cell::tick #"+last), "Started action executor")
	run(t, s, "%instantiate Farm::spare")
	again := run(t, s, "%instantiate Farm::spare")
	wants(t, again, "is displaced from that name")
	rejects(t, again, "debugging session")
	if s.actionExec == nil {
		t.Fatal("a second %instantiate of another name ended the session over a held member")
	}
	wants(t, run(t, s, "%continue"), "✓ Action completed")
	wants(t, run(t, s, "%features #"+last), "count = 2")
}

// An object evaluation created in passing — %eval in a usage nothing was
// instantiated under — is no id the REPL answers to: it is absent from
// %instances, so it is absent from lookups, the ids an error lists and completion.
func TestEvaluationOnlyObjectsAreNotAddressable(t *testing.T) {
	s := loadFixture(t, "testdata/nested_part.sysml")
	wants(t, run(t, s, "%eval in Nested::Car : engine.power"), "= 300.0")
	scratch := s.rtCtx.InstanceIDs()
	if len(scratch) == 0 {
		t.Fatal("the evaluation created no object to keep out of reach")
	}
	wants(t, run(t, s, "%instances"), "(no instances created)")
	for _, id := range scratch {
		wants(t, run(t, s, fmt.Sprintf("%%features #%d", id)),
			fmt.Sprintf("error: no object #%d in this session: nothing materialized has that identity (no objects have been created)", id))
	}
	if got := s.Complete("%features #", len("%features #")); len(got.Candidates) != 0 {
		t.Errorf("completion offered %v, want no ids", got.Candidates)
	}

	created := objectIDIn(t, run(t, s, "%instantiate Nested::Car"))
	listing := run(t, s, "%features #"+created)
	wants(t, listing, "Instance: #"+created+" (ID: "+created+")")
	engine := objectIDIn(t, listing[strings.Index(listing, "engine = "):])
	held := "#" + created + ", #" + engine
	for _, id := range scratch {
		wants(t, run(t, s, fmt.Sprintf("%%features #%d", id)),
			fmt.Sprintf("error: no object #%d in this session: nothing materialized has that identity (the objects are %s)", id, held))
	}
	if got := s.Complete("%features #", len("%features #")); strings.Join(got.Candidates, ", ") != held {
		t.Errorf("completion offered %v, want %s", got.Candidates, held)
	}
}

// An anonymous connector is held by its owner though no feature names it: once
// %features has materialized it, the id it printed reaches it and completes, and
// asking after ids materializes none that were not shown.
func TestMaterializedAnonymousConnectorIsAddressableByID(t *testing.T) {
	s := loadSource(t, `package Demo {
		port def P;
		part def A { port p : P; }
		part def B { port q : P; }
		part def Sys { part a : A; part b : B; connect a.p to b.q; }
	}`)
	wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 1")
	before := s.Complete("%features #", len("%features #")).Candidates
	if len(s.rtCtx.InstanceIDs()) != len(before) {
		t.Fatalf("completion offered %v of %v: the connector was materialized to be listed", before, s.rtCtx.InstanceIDs())
	}

	conn := objectIDIn(t, connectorLine(t, run(t, s, "%features Demo::Sys")))
	if slices.Contains(before, "#"+conn) {
		t.Fatalf("completion offered the connector %s before %%features materialized it", conn)
	}
	wants(t, run(t, s, "%features #"+conn), "Instance: #"+conn+" (ID: "+conn+")")
	after := s.Complete("%features #", len("%features #")).Candidates
	if !slices.Contains(after, "#"+conn) {
		t.Errorf("completion offered %v, want the connector #%s among them", after, conn)
	}
	wants(t, run(t, s, "%features #99"), "the objects are "+strings.Join(after, ", ")+")")
}

// A carry-over sets a connector's object aside to attach its ends again against
// the declarations as they are now, but the id %features printed for it still
// reaches it — anonymous or named — and still completes, without an owner being
// asked after first; completion builds nothing to say so.
func TestShownConnectorsStayAddressableByIDAcrossCarryOver(t *testing.T) {
	s := loadSource(t, `package Demo {
		port def P;
		part def A { port p : P; }
		part def B { port q : P; }
		part def Sys { part a : A; part b : B; connect a.p to b.q; connection c connect a.p to b.q; }
	}`)
	wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 1")
	listing := run(t, s, "%features Demo::Sys")
	anon := objectIDIn(t, connectorLine(t, listing))
	named := objectIDIn(t, featureLine(t, listing, "c"))
	wants(t, run(t, s, "%features #"+anon), "Instance: #"+anon+" (ID: "+anon+")")
	wants(t, run(t, s, "%features #"+named), "Instance: #"+named+" (ID: "+named+")")
	shown := s.Complete("%features #", len("%features #")).Candidates

	if res := s.Submit("part def Widget;"); len(res.Notices) != 0 {
		t.Fatalf("the declaration reported %v", res.Notices)
	}
	held := len(s.rtCtx.InstanceIDs())
	if got := s.Complete("%features #", len("%features #")).Candidates; !slices.Equal(got, shown) {
		t.Errorf("after the declaration completion offers %v, want %v as before", got, shown)
	}
	if len(s.rtCtx.InstanceIDs()) != held {
		t.Errorf("completion materialized objects: %v were held, now %v", held, s.rtCtx.InstanceIDs())
	}
	wants(t, run(t, s, "%features #99"), "the objects are "+strings.Join(shown, ", ")+")")
	if len(s.rtCtx.InstanceIDs()) != held {
		t.Errorf("an unknown id materialized objects: %v were held, now %v", held, s.rtCtx.InstanceIDs())
	}

	wants(t, run(t, s, "%features #"+anon), "Instance: #"+anon+" (ID: "+anon+")", "source = ")
	wants(t, run(t, s, "%features #"+named), "Instance: #"+named+" (ID: "+named+")", "source = ")
	wants(t, run(t, s, "%eval in #"+anon+" : source"), "Instance(ID: ")
	if got := s.Complete("%features #", len("%features #")).Candidates; !slices.Equal(got, shown) {
		t.Errorf("after reaching the connectors again completion offers %v, want %v", got, shown)
	}
	if got := connectorLine(t, run(t, s, "%features Demo::Sys")); objectIDIn(t, got) != anon {
		t.Errorf("the owner lists its connector as %q, want #%s", got, anon)
	}
	// Nothing else took an identity along the way.
	next := fmt.Sprintf("ID: %d", slices.Max(s.rtCtx.InstanceIDs())+1)
	wants(t, run(t, s, "%instantiate Demo::A"), next)
}

// Reaching one connector set aside by id materializes that one alone: its
// siblings stay set aside, still offered and reachable by their ids, and the
// owner's listing shows them all under their ids once they are.
func TestReachingOneKeptConnectorLeavesItsSiblingsSetAside(t *testing.T) {
	s := loadSource(t, `package Demo {
		port def P;
		part def A { port p : P; }
		part def B { port q : P; }
		part def Sys { part a : A; part b : B; connect a.p to b.q; connect b.q to a.p; }
	}`)
	wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 1")
	listing := run(t, s, "%features Demo::Sys")
	conns := connectorLines(t, listing)
	if len(conns) != 2 {
		t.Fatalf("the listing shows %d anonymous connectors, want 2:\n%s", len(conns), listing)
	}
	first, second := objectIDIn(t, conns[0]), objectIDIn(t, conns[1])
	shown := s.Complete("%features #", len("%features #")).Candidates

	if res := s.Submit("part def Widget;"); len(res.Notices) != 0 {
		t.Fatalf("the declaration reported %v", res.Notices)
	}
	held := len(s.rtCtx.InstanceIDs())
	wants(t, run(t, s, "%features #"+second), "Instance: #"+second+" (ID: "+second+")", "source = ")
	if got := len(s.rtCtx.InstanceIDs()); got != held+1 {
		t.Errorf("reaching #%s materialized %d objects, want it alone", second, got-held)
	}
	if _, found := s.rtCtx.Instance(mustID(t, first)); found {
		t.Errorf("reaching #%s materialized its sibling #%s", second, first)
	}
	if got := s.Complete("%features #", len("%features #")).Candidates; !slices.Equal(got, shown) {
		t.Errorf("completion offers %v, want %v: the sibling set aside among them", got, shown)
	}
	wants(t, run(t, s, "%features #99"), "the objects are "+strings.Join(shown, ", ")+")")

	wants(t, run(t, s, "%features #"+first), "Instance: #"+first+" (ID: "+first+")", "source = ")
	if got := connectorLines(t, run(t, s, "%features Demo::Sys")); len(got) != 2 ||
		objectIDIn(t, got[0]) != first || objectIDIn(t, got[1]) != second {
		t.Errorf("the owner lists its connectors as %q, want #%s then #%s", got, first, second)
	}
	next := fmt.Sprintf("ID: %d", slices.Max(s.rtCtx.InstanceIDs())+1)
	wants(t, run(t, s, "%instantiate Demo::A"), next)
}

// The id %features printed for an anonymous connector keeps naming the connector
// of the declaration it was materialized from when the owner is declared again
// with its connectors reordered or one inserted before them; a connector whose
// declaration is gone is gone with it, its id unknown rather than another's.
func TestKeptConnectorIDsFollowTheirDeclarations(t *testing.T) {
	const (
		first  = "connect a.p to b.q;"
		second = "connect a.r to b.s;"
	)
	sys := func(connects string) string {
		return `package Demo {
		port def P;
		part def A { port p : P; port r : P; }
		part def B { port q : P; port s : P; }
		part def Sys { part a : A; part b : B; ` + connects + ` }
	}`
	}
	// ends reads the ports a connector's listing shows its ends holding.
	ends := func(listing string) string {
		return objectIDIn(t, featureLine(t, listing, "source")) + "-" + objectIDIn(t, featureLine(t, listing, "target"))
	}
	cases := []struct {
		name, connects string
		kept           bool // whether the second connector's declaration is still there
	}{
		{"inserted before", "connect a.p to b.s; " + first + " " + second, true},
		{"reordered", second + " " + first, true},
		{"removed", first, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := loadSource(t, sys(first+" "+second))
			wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 1")
			conns := connectorLines(t, run(t, s, "%features Demo::Sys"))
			if len(conns) != 2 {
				t.Fatalf("the listing shows %d anonymous connectors, want 2", len(conns))
			}
			a, b := objectIDIn(t, conns[0]), objectIDIn(t, conns[1])
			aEnds, bEnds := ends(run(t, s, "%features #"+a)), ends(run(t, s, "%features #"+b))

			if res := s.Submit(sys(tc.connects)); len(res.Notices) != 1 {
				t.Fatalf("the declaration reported %v", res.Notices)
			}
			wants(t, run(t, s, "%instances"), "Demo::Sys (ID: 1)")
			if got := ends(run(t, s, "%features #"+a)); got != aEnds {
				t.Errorf("#%s connects %s after the declaration, want %s as before", a, got, aEnds)
			}
			got := run(t, s, "%features #"+b)
			if !tc.kept {
				wants(t, got, "error: no object #"+b+" in this session")
				rejects(t, got, "Instance: #"+b)
				return
			}
			if ends(got) != bEnds {
				t.Errorf("#%s connects %s after the declaration, want %s as before", b, ends(got), bEnds)
			}
			listed := connectorLines(t, run(t, s, "%features Demo::Sys"))
			if len(listed) != strings.Count(tc.connects, "connect") {
				t.Errorf("the owner lists %d connectors, want one per declaration: %q", len(listed), listed)
			}
		})
	}
}

// A connector attached whole is kept when an older object's behavior fails
// answering it, at first and when a carry-over materializes it again: the
// failure is reported as the older object's, and the connector — its write with
// it — is there for the next command, materialized once.
func TestConnectorAnsweredByAFailingBehaviorIsKept(t *testing.T) {
	s := loadSource(t, `package Demo {
		private import ScalarValues::*;
		item def Ping;
		port def P;
		part def Listener {
			attribute heard : Integer = 0;
			exhibit state listening {
				entry; then waiting;
				state waiting;
				transition first waiting accept Ping if 10 / (heard - heard) > 1 then noted;
				state noted;
			}
		}
		part good : Listener;
		part def A { port p : P; }
		part def B { port q : P; }
		connection def Fine {
			end : P; end : P;
			exhibit state life {
				entry; then poke;
				state poke { entry action bump { assign good.heard := good.heard + 10; } }
				transition first poke then ping;
				state ping { entry send new Ping() to good; }
			}
		}
		part def Sys { part a : A; part b : B; connection fine : Fine connect a.p to b.q; }
	}`)
	const failure = "exhibited state machine listening of object #1: process event: state waiting: eval guard of transition waiting -> noted: division by zero"
	wants(t, run(t, s, "%instantiate Demo::good"), "ID: 1")
	wants(t, run(t, s, "%instantiate Demo::Sys"), "ID: 3")
	wants(t, run(t, s, "%features Demo::Sys"), "fine: <error: "+failure+">")
	listing := run(t, s, "%features Sys.fine")
	wants(t, listing, "Instance: Demo::Sys.fine (ID: ", "source = Instance(ID: ", "life: exhibited state machine, current state ping")
	conn := objectIDIn(t, listing)
	wants(t, run(t, s, "%eval in good : heard"), "= 10")
	wants(t, run(t, s, "%instances"), "Demo::Sys (ID: 3)", "Demo::good (ID: 1)")

	if res := s.Submit("part def Widget;"); len(res.Notices) != 1 {
		t.Fatalf("the declaration reported %v", res.Notices)
	}
	wants(t, run(t, s, "%eval in good : heard"), "= 0")
	got := run(t, s, "%features #"+conn)
	wants(t, got, "error: #"+conn+" is materialized again, but an older object's behavior failed: "+failure)
	rejects(t, got, "no object #"+conn)
	wants(t, run(t, s, "%eval in good : heard"), "= 10")
	wants(t, run(t, s, "%features #"+conn), "Instance: #"+conn+" (ID: "+conn+")", "source = Instance(ID: ", "current state ping")
	wants(t, run(t, s, "%eval in good : heard"), "= 10")
	if got := objectIDIn(t, featureLine(t, run(t, s, "%features Demo::Sys"), "fine")); got != conn {
		t.Errorf("the owner holds connector #%s, want the kept #%s", got, conn)
	}
}

// connectorLines returns the lines of a %features listing that hold the object's
// anonymous connectors, in the order listed.
func connectorLines(t *testing.T, listing string) []string {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(listing, "\n") {
		if strings.Contains(line, "(anonymous connector)") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

// mustID parses the digits of an object identity a listing showed.
func mustID(t *testing.T, digits string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		t.Fatalf("%q is no object identity: %v", digits, err)
	}
	return id
}

// featureLine returns the line of a %features listing that shows feature's value.
func featureLine(t *testing.T, listing, feature string) string {
	t.Helper()
	for _, line := range strings.Split(listing, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), feature+" = ") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("no feature %s in:\n%s", feature, listing)
	return ""
}

// %state <machine>, for a machine one held object exhibits, drives that object's
// running performance: the do action its timer re-runs writes the object's own
// slots, the ones %features shows under the identity %instances lists.
func TestStateOverAMachineNamedAloneDrivesItsOneExhibitor(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_timer.sysml")
	id := objectIDIn(t, run(t, s, "%instantiate TA::Sys"))

	got := run(t, s, "%state lp")
	wants(t, got, `Debugging state machine "lp" exhibited by object #`+id+` of "TA::Sys"`, "Current state: run")
	rejects(t, got, "Started state machine executor")

	wants(t, run(t, s, "%advance 2.5 [s]"), "Advanced to 2.5 (2 event(s) processed)")
	wants(t, run(t, s, "%instances"), "TA::Sys (ID: "+id+")")
	wants(t, run(t, s, "%features #"+id), "x = 0.6000000000000001", "n = 3")

	// The qualified name of the machine attaches the same way.
	wants(t, run(t, s, "%state TA::Sys::lp"), `exhibited by object #`+id+` of "TA::Sys"`)
}

// %state <machine> <object>, for the machine the object exhibits, attaches to the
// running performance: its do action is not started a second time on the object,
// and the events the session dispatches write the object's slots.
func TestStateOverTheExhibitedMachineAndObjectDoesNotDoubleRun(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_timer.sysml")
	id := objectIDIn(t, run(t, s, "%instantiate TA::Sys"))
	wants(t, run(t, s, "%features #"+id), "x = 0.2", "n = 1")

	got := run(t, s, "%state lp #"+id)
	wants(t, got, `exhibited by object #`+id, "note:", "attaches to that running machine")
	rejects(t, got, "Started state machine executor")
	wants(t, run(t, s, "%features #"+id), "x = 0.2", "n = 1")
	wants(t, run(t, s, "%events"), "1 events")

	wants(t, run(t, s, "%advance 2.5 [s]"), "2 event(s) processed")
	wants(t, run(t, s, "%features #"+id), "x = 0.6000000000000001", "n = 3")
}

// A machine a type exhibits that no held object runs is refused rather than
// performed detached from any object, and the refusal names both forms that
// address an object.
func TestStateOverAMachineNoObjectExhibitsRefuses(t *testing.T) {
	s := loadFixture(t, "testdata/exhibited_timer.sysml")

	_, err := s.startStateMachine("lp", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	if eerr.Machine != "lp" || strings.Join(eerr.Types, ",") != "TA::Sys" || len(eerr.Objects) != 0 {
		t.Errorf("got %+v, want lp of TA::Sys exhibited by no object", eerr)
	}
	if s.stateExec != nil {
		t.Errorf("a detached performance was started: %q", s.stateExec.fqn)
	}

	got := run(t, s, "%state TA::Sys::lp")
	wants(t, got, "error:", `no object of this session exhibits "TA::Sys::lp"`, `object of "TA::Sys"`,
		"%instantiate", "%state <object>", "%state TA::Sys::lp <object>")
	rejects(t, got, "Started state machine executor", "Debugging state machine")

	// Once an object exhibits it, the same command attaches.
	id := objectIDIn(t, run(t, s, "%instantiate TA::Sys"))
	wants(t, run(t, s, "%state TA::Sys::lp"), `exhibited by object #`+id)
}

// A machine several held objects exhibit is refused, naming every one of them,
// rather than attached to whichever was found first; naming an object attaches.
func TestStateOverAMachineSeveralObjectsExhibitRefuses(t *testing.T) {
	s := loadFixture(t, "testdata/nested_machine.sysml")
	rover := objectIDIn(t, run(t, s, "%instantiate Fleet::rover"))
	run(t, s, "%instantiate Fleet::driver")
	nested := objectIDIn(t, run(t, s, "%features Fleet::driver.r"))

	_, err := s.startStateMachine("Fleet::Rover::modes", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	want := []RelatedObject{{ID: mustID(t, rover), Label: "Fleet::rover"}, {ID: mustID(t, nested), Label: "Fleet::driver.r"}}
	if len(eerr.Objects) != len(want) || eerr.Objects[0] != want[0] || eerr.Objects[1] != want[1] {
		t.Errorf("got objects %+v, want %+v", eerr.Objects, want)
	}
	if s.stateExec != nil {
		t.Errorf("a machine was attached to despite the ambiguity: %q", s.stateExec.selfFQN)
	}

	got := run(t, s, "%state Fleet::Rover::modes")
	wants(t, got, "error:", `2 objects of this session exhibit "Fleet::Rover::modes"`,
		`#`+nested+` of "Fleet::driver.r"`, `#`+rover+` of "Fleet::rover"`, "%state <object>", "%state Fleet::Rover::modes <object>")
	rejects(t, got, "Started state machine executor", "Debugging state machine")

	wants(t, run(t, s, "%state Fleet::Rover::modes Fleet::driver.r"), `exhibited by object #`+nested, "note:")
	wants(t, run(t, s, "%state #"+rover), `exhibited by object #`+rover+"\n")
}

// A definition a type exhibits through usages typed by it is refused named alone
// before any object is held, naming that type rather than running detached; once
// one held object exhibits it as two usages, it is as ambiguous named alone as it
// is with the object named.
func TestStateOverASharedDefinitionNamedAlone(t *testing.T) {
	s := loadFixture(t, "testdata/shared_machine.sysml")

	_, err := s.startStateMachine("Shared::Blink", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	if eerr.Machine != "Shared::Blink" || strings.Join(eerr.Types, ",") != "Shared::Lamp" || len(eerr.Objects) != 0 {
		t.Errorf("got %+v, want Shared::Blink of Shared::Lamp exhibited by no object", eerr)
	}
	if s.stateExec != nil {
		t.Errorf("a detached performance was started: %q", s.stateExec.fqn)
	}
	got := run(t, s, "%state Shared::Blink")
	wants(t, got, "error:", `no object of this session exhibits "Shared::Blink"`, `object of "Shared::Lamp"`, "%state Shared::Blink <object>")
	rejects(t, got, "Started state machine executor")

	run(t, s, "%instantiate Shared::lamp")
	_, err = s.startStateMachine("Shared::Blink", nil)
	var aerr *AmbiguousMachineError
	if !errors.As(err, &aerr) {
		t.Fatalf("got %v, want an AmbiguousMachineError", err)
	}
	if strings.Join(aerr.Usages, ",") != "Shared::Lamp::front,Shared::Lamp::rear" {
		t.Errorf("got %+v, want both exhibited usages of Shared::Blink", aerr)
	}
	wants(t, run(t, s, "%state Shared::Blink"), "error:", `exhibits "Shared::Blink" as 2 machines`, "Shared::Lamp::front or Shared::Lamp::rear")

	wants(t, run(t, s, "%state Shared::Lamp::rear"), `Debugging state machine "rear" exhibited by object #`, "Current state: dark")
	wants(t, run(t, s, "%advance 2"), "Current state: lit")
	wants(t, run(t, s, "%features Shared::lamp"), "front: exhibited state machine, current state lit", "rear: exhibited state machine, current state lit")
}

// A definition no type exhibits still runs detached when named alone, since no
// object's performance of it exists to attach to.
func TestStateOverAnUnexhibitedDefinitionRunsDetached(t *testing.T) {
	s := loadFixture(t, "testdata/performed_machine.sysml")
	wants(t, run(t, s, "%state Two::Check"), "Started state machine executor", "Current state: checking")
	wants(t, run(t, s, "%advance 5"), "Current state: checked")
}

// Every binding on the way to a machine's body addresses it: the state usage an
// exhibited usage references, and the definition typing that. Named alone, each
// refuses before an object exists and attaches to that object's running machine
// afterwards, never running detached.
func TestStateOverAMachineReachedThroughABindingChain(t *testing.T) {
	s := loadFixture(t, "testdata/named_usage_machine.sysml")

	for _, name := range []string{"Relay::Beacon::spare", "Relay::Blink"} {
		_, err := s.startStateMachine(name, nil)
		var eerr *ExhibitorsError
		if !errors.As(err, &eerr) {
			t.Fatalf("%s: got %v, want an ExhibitorsError", name, err)
		}
		if eerr.Machine != name || strings.Join(eerr.Types, ",") != "Relay::Beacon" || len(eerr.Objects) != 0 {
			t.Errorf("got %+v, want %s of Relay::Beacon exhibited by no object", eerr, name)
		}
		if s.stateExec != nil {
			t.Fatalf("%s: a detached performance was started: %q", name, s.stateExec.fqn)
		}
		got := run(t, s, "%state "+name)
		wants(t, got, "error:", `object of "Relay::Beacon"`, "%state "+name+" <object>")
		rejects(t, got, "Started state machine executor")
	}

	run(t, s, "%instantiate Relay::beacon")
	wants(t, run(t, s, "%features Relay::beacon"), "ticks = 1")
	for _, name := range []string{"Relay::Beacon::spare", "Relay::Blink", "Relay::Beacon::spare Relay::beacon"} {
		got := run(t, s, "%state "+name)
		wants(t, got, `Debugging state machine "active" exhibited by object #`, "Current state: dark")
		rejects(t, got, "Started state machine executor")
		wants(t, run(t, s, "%features Relay::beacon"), "ticks = 1")
	}
	wants(t, run(t, s, "%advance 4"), "Current state: dark")
	wants(t, run(t, s, "%features Relay::beacon"), "ticks = 2")
}

// collectionIDs reads the identities a %features listing shows feature holding.
func collectionIDs(t *testing.T, listing, feature string) []string {
	t.Helper()
	at := strings.Index(listing, feature+" = [")
	if at < 0 {
		t.Fatalf("%s holds no collection:\n%s", feature, listing)
	}
	members := listing[at+len(feature)+4:]
	members = members[:strings.Index(members, "]")]
	var ids []string
	for _, m := range strings.Split(members, ",") {
		ids = append(ids, objectIDIn(t, m))
	}
	return ids
}

// A type exhibiting the same definition through several usages, one of them
// unnamed, is named once in the refusal; the search stops at its first match.
func TestRefusalNamesAnExhibitingTypeOnce(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package Twice {
		state def Modes { entry; then idle; state idle; }
		part def Sys {
			exhibit state a : Modes;
			exhibit state : Modes;
			exhibit state b : Modes;
		}
	}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}

	_, err := s.startStateMachine("Twice::Modes", nil)
	var eerr *ExhibitorsError
	if !errors.As(err, &eerr) {
		t.Fatalf("got %v, want an ExhibitorsError", err)
	}
	if got := strings.Join(eerr.Types, ","); got != "Twice::Sys" {
		t.Errorf("got types %q, want Twice::Sys once", got)
	}
}

// The exhibitor index a refusal is answered from follows the declarations: a
// submission adding an exhibiting type is named by the next refusal.
func TestRefusalFollowsNewExhibitingTypes(t *testing.T) {
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(`package M {
		state def Modes { entry; then idle; state idle; }
		part def A { exhibit state a : Modes; }
	}`).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	types := func() string {
		_, err := s.startStateMachine("M::Modes", nil)
		var eerr *ExhibitorsError
		if !errors.As(err, &eerr) {
			t.Fatalf("got %v, want an ExhibitorsError", err)
		}
		return strings.Join(eerr.Types, ",")
	}
	if got := types(); got != "M::A" {
		t.Fatalf("got types %q, want M::A", got)
	}
	if errs := errorDiagnostics(s.Submit(`package N { part def B { exhibit state b : M::Modes; } }`).Diagnostics); len(errs) > 0 {
		t.Fatalf("second submission has errors: %v", errs)
	}
	if got := types(); got != "M::A,N::B" {
		t.Errorf("got types %q after a submission, want M::A,N::B", got)
	}
}
