package repl

import (
	"testing"
)

const spacecraftShowcase = "../../../examples/runtime-showcase/spacecraft-comms.sysml"

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
// at t=80, resumed at t=122, every frame counted at the station by t=241.
func TestSpacecraftShowcaseTransmitsEveryFrame(t *testing.T) {
	s := spacecraftSession(t, "reverse")

	wants(t, run(t, s, "%advance 40"), "Advanced to 40.0", "Current state: transmitting | notRecharging", "Last event at: 30.0")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 80")

	wants(t, run(t, s, "%advance 39"), "Advanced to 79.0", "Current state: transmitting | recharging")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 40")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 49")

	wants(t, run(t, s, "%advance 1"), "Advanced to 80.0", "Current state: lowPower | recharging", "Last event at: 80.0")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 39")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : data"), "= 51200")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 50")

	wants(t, run(t, s, "%advance 41"), "Advanced to 121.0", "Current state: lowPower | recharging")
	wants(t, run(t, s, "%advance 1"), "Advanced to 122.0", "Current state: transmitting | recharging")

	wants(t, run(t, s, "%advance 178"), "Advanced to 300.0", "Current state: transmitted | notRecharging", "Last event at: 241.0", "Remaining events: 0")
	wants(t, run(t, s, "%current"), "Execution state: Suspended", "waiting on change condition: notRecharging")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : data"), "= 0")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 100")
	wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 100")
}

// At t=79 the drain, send and charge timers fall due together; the drain's
// branch reads the battery in the round after, once the charge is in whichever
// order the schedule picked, so every schedule passes the same checkpoints.
func TestSpacecraftShowcaseLowPowerCheckpointsUnderEverySchedule(t *testing.T) {
	for _, schedule := range []string{"reverse", "declared", "seed:7", "seed:42"} {
		t.Run(schedule, func(t *testing.T) {
			s := spacecraftSession(t, schedule)
			wants(t, run(t, s, "%advance 79"), "Advanced to 79.0", "Current state: transmitting | recharging")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 49")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 40")

			wants(t, run(t, s, "%advance 1"), "Advanced to 80.0", "Current state: lowPower | recharging")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 50")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 39")

			wants(t, run(t, s, "%advance 220"), "Advanced to 300.0", "Current state: transmitted | notRecharging", "Last event at: 241.0")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.groundStation : framesReceived"), "= 100")
			wants(t, run(t, s, "%eval in SpacecraftComms::mission.spacecraftVehicle : battery"), "= 100")
		})
	}
}
