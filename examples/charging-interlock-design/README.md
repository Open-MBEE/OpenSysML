# Design checking a charging interlock

This synthetic model applies requirement INT-01: the controller must never
energize the output unless it recognizes a connector lock. It is not a
hardware safety certification or an IEC, SAE, or ISO interoperability test.

The station model defines `designEnvironment`. It sends a connection
event, lets the vehicle's first request and the sensor's lock report race at
the same simulated instant, then sends one retry. This is a synthetic input
driver, not a vehicle or power-stage simulation. The statically wired ports
are architectural context; the driver sends directly to the controller.

From the repository root, with a locally built `sysml` and Python 3:

```sh
python3 examples/charging-interlock-design/check_design.py \
  ./bin/sysml ./charging-design-evidence
```

The checker and the environment advance together on one clock. A state-only
run without `-advance` would not exercise the stimuli. The script requires:

| Run | Evidence |
| --- | --- |
| Safe design | Exit 0 and no violation or search bound hit through 3 simulated seconds: 28 states, 29 moves, depth 14 in the reference run |
| Non-vacuity | Both request-first and lock-first have their own concrete scheduler witness and observed state sequence; both end with power on and lock recognized |
| Design mutation | A disposable copy energizes on connection before lock. The native checker finds the named INT-01 violation after 3 moves and confirms its witness. The script replays that same file. Both final outcomes are identical to the safe design's |
| Loss and restart | Through 7 simulated seconds, drive to powered, lose lock, request while unlocked, then reacquire and restart. Samples after each settled dispatch confirm power stays off until the fresh request |
| Shutdown mutation | Remove only the power-off assignment. The checker witnesses INT-01 failing after 13 moves. Replay shows energized output with the lock lost, although the scenario still ends powered and locked |
| Tight control | Depth 1 returns exit 2 with the depth bound hit; this does not establish safety |

The script uses the upstream native check and replay commands, without a
recorded implementation or a trace-conformance extension. It writes JSON
reports and a separate interpreter trace.
A move may include a whole state-machine dispatch, so neither a brief
electrical transient nor a value overwritten inside one dispatch is modeled
unless it is represented at an observable checkpoint.

This result covers only the modeled, finite environment through the chosen
horizon, not arbitrary external commands, repetitions of the recovery cycle,
real electrical behavior, or eventual recovery under all environments.

## Check a recorded implementation trace

The design checks above ask what can happen before implementation. The same
station model can also check what was independently observed after
implementation. These two synthetic recordings use only external stimulus
names and power-stage readback. They do not supply controller lock state or
any runtime scheduler choices.

```sh
example=examples/charging-interlock-design
machine='ChargingInterlock::StationController::Interlock ChargingInterlock::rig.controller'
sysml -json -state "$machine" -conform-trace "$example/observed-normal.jsonl" "$example/station.sysml"
sysml -json -state "$machine" -conform-trace "$example/observed-early-output.jsonl" "$example/station.sysml"
```

The first recording matches through row 6 (exit 0). The second reports
`no_match` at row 3 (exit 1): the request's readback shows power before the
lock report. Both finish deenergized. `-check-depth 1` on the first recording
returns `inconclusive` (exit 2); it does not declare a mismatch.

The recorded signal is delivered directly to one controller's untimed state
machine, and observations are checked after it settles. This does not
verify ports, event timing, actual hardware, or unrecorded signals.
See [the CLI reference](../../docs/reference/cli.md#checking-an-external-trace).
