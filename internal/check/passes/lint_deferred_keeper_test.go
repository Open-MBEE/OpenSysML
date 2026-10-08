package passes

import "testing"

// deferringMachine is a state machine whose state busy defers Ping and keeps it
// by the standard encoding, the keeping accept written as accept; other adds to
// the do action's root and annotation to the state's body.
func deferringMachine(annotation, accept, other string) string {
	return `package P {
	item def Ping; item def Go;
	state def Machine {
		entry; then busy;
		state busy {
			` + annotation + `
			item deferred : Ping[*] ordered;
			do action buffer {
				first start then receive;
				` + accept + `
				then action keep { assign deferred := SequenceFunctions::including(deferred, receive.kept); }
				then receive;
				` + other + `
			}
			exit action flush {
				for kept in deferred { send kept to self; }
				then action clear { assign deferred := (); }
			}
		}
		state ready;
		transition first busy accept Go then ready;
	}
}`
}

const (
	deferredEventPing = `@MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }`
	unmarkedKeeper    = `action receive accept kept : Ping;`
	markedKeeper      = `#MigrationMetadata::DeferredKeeper action receive accept kept : Ping;`
)

func TestDeferredKeeperUnmarkedLint(t *testing.T) {
	want := []string{
		"accept of deferred signal Ping is not marked #MigrationMetadata::DeferredKeeper",
		"ordinary accept, not the keeping loop; re-migrate the model or mark it",
	}
	cases := []struct {
		name  string
		src   string
		wants [][]string
	}{
		{"unmarked loop under a deferring state", deferringMachine(deferredEventPing, unmarkedKeeper, ""), [][]string{want}},
		{"annotation imported by name", `package P {
	private import MigrationMetadata::DeferredEvent;
	` + deferringMachine(`@DeferredEvent { ref :>> signal : Ping; }`, unmarkedKeeper, "")[len("package P {"):], [][]string{want}},
		{"annotation through an alias", `package P {
	alias Kept for MigrationMetadata::DeferredEvent;
	` + deferringMachine(`@Kept { ref :>> signal : Ping; }`, unmarkedKeeper, "")[len("package P {"):], [][]string{want}},
		{"marked loop", deferringMachine(deferredEventPing, markedKeeper, ""), nil},
		{"unmarked accept under a state deferring nothing", deferringMachine("", unmarkedKeeper, ""), nil},
		{"homonymous annotation of the model", `package P {
	metadata def DeferredEvent { ref signal : Base::Anything; }
	` + deferringMachine(`@DeferredEvent { ref :>> signal : Ping; }`, unmarkedKeeper, "")[len("package P {"):], nil},
		{"unmarked accept of another signal", deferringMachine(deferredEventPing, markedKeeper, `action other accept go : Go;`), nil},
		{"unmarked accept nested beside the marked loop", deferringMachine(deferredEventPing, markedKeeper,
			`action work { action take accept p : Ping; }`), nil},
		{"ordinary accept beside the marked loop", deferringMachine(deferredEventPing, markedKeeper,
			`action take accept p : Ping;`), nil},
		{"two unmarked accepts, neither the keeper", deferringMachine(deferredEventPing, unmarkedKeeper,
			`action take accept p : Ping;`), [][]string{want, want}},
		{"signal deferred twice, reported once", deferringMachine(
			deferredEventPing+"\n\t\t\t"+deferredEventPing, unmarkedKeeper, ""), [][]string{want}},
		{"second deferred signal kept by an unmarked loop", deferringMachine(
			deferredEventPing+"\n\t\t\t@MigrationMetadata::DeferredEvent { ref :>> signal : Go; }",
			markedKeeper, `action keepGo accept go : Go;`), [][]string{{"deferred signal Go is not marked"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantLints(t, tc.src, CodeDeferredKeeperUnmarked, tc.wants...)
		})
	}
}
