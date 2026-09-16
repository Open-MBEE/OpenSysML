package repl

import (
	"testing"
)

const spacecraftShowcase = "../../examples/runtime-showcase/spacecraft-comms.sysml"

// spacecraftSession instantiates the mission assembly under a schedule and
// attaches the debugger to the spacecraft's running state machine.
func spacecraftSession(t *testing.T, schedule string) *Session {
	t.Helper()
	s := loadFixture(t, spacecraftShowcase)
	wants(t, run(t, s, "%schedule "+schedule), "schedule: "+schedule)
	wants(t, run(t, s, "%instantiate SpacecraftComms::mission"), "✓ Created instance of SpacecraftComms::mission")
	wants(t, run(t, s, "%state SpacecraftComms::mission.spacecraftVehicle"),
		`Debugging state machine "modes"`, "Current state: waitingGSPing | notRecharging", "Time: 0.0")
	return s
}

// The showcase's checkpoints under the default schedule: ping at t=30, interrupted
// at t=79, resumed at t=120, every frame counted at the station by t=241.
func TestSpacecraftShowcaseTransmitsEveryFrame(t *testing.T) {
	s := spacecraftSession(t, "reverse")

	wants(t, run(t, s, "%advance 40"), "Advanced to 40.0", "Current state: transmitting | notRecharging", "Last event at: 30.0")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 80")

	wants(t, run(t, s, "%advance 39"), "Advanced to 79.0", "Current state: lowPower | recharging")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 40")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : data"), "= 52224")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 49")

	wants(t, run(t, s, "%advance 41"), "Advanced to 120.0", "Current state: transmitting | recharging")

	wants(t, run(t, s, "%advance 180"), "Advanced to 300.0", "Current state: transmitted | notRecharging", "Last event at: 241.0", "Remaining events: 0")
	wants(t, run(t, s, "%current"), "Execution state: Suspended", "waiting on change condition: notRecharging")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : data"), "= 0")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 100")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 100")
}

// At t=79 the drain, send and charge timers fall due together; whether the 50th
// frame gets out before lowPower is the schedule's choice, the end is not.
func TestSpacecraftShowcaseFrameCountAtLowPowerIsScheduleDependent(t *testing.T) {
	cases := []struct {
		schedule string
		frames   string
		battery  string
	}{
		{"reverse", "= 49", "= 41"},
		{"declared", "= 49", "= 41"},
		{"seed:7", "= 50", "= 39"},
		{"seed:42", "= 50", "= 39"},
	}
	for _, tc := range cases {
		t.Run(tc.schedule, func(t *testing.T) {
			s := spacecraftSession(t, tc.schedule)
			wants(t, run(t, s, "%advance 80"), "Advanced to 80.0", "Current state: lowPower | recharging")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), tc.frames)
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), tc.battery)

			wants(t, run(t, s, "%advance 220"), "Advanced to 300.0", "Current state: transmitted | notRecharging", "Last event at: 241.0")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 100")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 100")
		})
	}
}
