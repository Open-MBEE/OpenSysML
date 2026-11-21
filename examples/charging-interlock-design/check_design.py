#!/usr/bin/env python3
"""Run the native design checks with an already-installed sysml binary."""
import argparse
import json
from pathlib import Path
import re
import subprocess


def expect(condition, message):
    if not condition:
        raise SystemExit(message)


def final_fields(outcome):
    return dict(field.split(" = ", 1) for field in outcome.split("; "))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("sysml", help="OpenSysML CLI (path or command)")
    parser.add_argument("evidence", type=Path, help="new directory for reports and witnesses")
    args = parser.parse_args()
    out = args.evidence.resolve()
    out.mkdir(parents=True, exist_ok=False)
    model = Path(__file__).with_name("station.sysml")
    common = [
        "-no-record-cache", "-instantiate", "ChargingInterlock::rig",
        "-state", "ChargingInterlock::StationController::Interlock ChargingInterlock::rig.controller",
    ]
    prop = "ChargingInterlock::powerGate"

    def run(name, flags, status, path=model, json_out=True, recovery=False):
        scenario, advance = ("recoveryEnvironment", "7") if recovery else ("designEnvironment", "3")
        command = [args.sysml, *common, "-action", f"ChargingInterlock::{scenario}",
                   "-advance", advance, *flags, *(["-json"] if json_out else []), str(path)]
        result = subprocess.run(command, capture_output=True, text=True, timeout=60, check=False)
        report = out / (name + (".json" if json_out else ".txt"))
        report.write_text(result.stdout)
        (out / f"{name}.stderr").write_text(result.stderr)
        expect(result.returncode == status,
               f"{name}: exit {result.returncode}, expected {status}; see {report}")
        return json.loads(result.stdout)["checks"][0] if json_out else result.stdout

    def check(name, depth, status, path=model, extra=(), recovery=False):
        run_result = run(name, [
            "-engine", "check", "-check-property", prop, "-check-depth", str(depth),
            "-check-states", "20000", "-check-timeout", "30s", *extra,
        ], status, path, recovery=recovery)
        return next(result for result in run_result["results"] if result["engine"] == "check")

    safe = check("safe", 100, 0)
    safe_search = safe["check"]
    expect(safe_search["verdict"] == "no violation, exhaustive" and
           not safe_search["boundsHit"] and not safe_search["violations"], "safe search incomplete")

    orders = run("orders", ["-schedule", "explore:runs=100,depth=30", "-trace"], 0)
    expect(orders["exploration"]["complete"], "order exploration incomplete")
    first_senders = set()
    for outcome in orders["outcomes"]:
        values = {value["name"]: value["value"] for value in outcome["values"]}
        expect(values.get("this.energized") == values.get("this.linkLocked") == "true",
               "an order did not reach authorized power")
        choices = [
            match[1] for line in outcome["witness"]
            if (match := re.search(r"\d+@(request|acquire) first of \d+@request, \d+@acquire", line))
        ]
        expect(len(choices) == 1, "missing witness for the first arrival")
        first_senders.add(choices[0])
        visits = next(value for name, value in values.items() if name.endswith("Interlock visits"))
        expect(visits.count("awaitingLock") == (2 if choices[0] == "request" else 1),
               "the controller did not dispatch the witnessed arrival order")
    expect(first_senders == {"request", "acquire"}, "both first-arrival orders not witnessed")

    original = model.read_text()
    connect = "accept VehicleConnected then awaitingLock;"
    expect(original.count(connect) == 1, "connection transition not uniquely identified")
    mutant = out / "early-energy.sysml"
    mutant.write_text(original.replace(connect,
        "accept VehicleConnected do assign energized := true then awaitingLock;"))
    broken = check("mutant", 100, 1, mutant, ("-check-witness", str(out / "witnesses")))
    broken_search = broken["check"]
    violations = broken_search["violations"]
    expect(broken["strength"] == "witnessed" and broken_search["verdict"] == "violation" and
           not broken_search["boundsHit"] and len(violations) == 1 and
           violations[0]["kind"] == "property" and violations[0]["name"] == prop,
           "expected an interpreter-confirmed INT-01 violation")
    expect(sorted(broken_search["outcomes"]) == sorted(safe_search["outcomes"]),
           "negative control must have the same terminal outcomes as the safe design")
    witness = Path(violations[0]["path"])
    replay = run("replay", ["-schedule", f"replay:{witness}", "-trace"], 0, mutant, json_out=False)
    early = replay.find("stmt assign energized")
    entered = replay.find("transition: disconnected -> awaitingLock (event: accept VehicleConnected)")
    locked = replay.find("stmt assign linkLocked")
    expect(0 <= early < entered < locked, "replayed connection checkpoint did not precede locking")

    tight = check("tight", 1, 2)["check"]
    expect(tight["verdict"] == "no violation within bounds" and tight["boundsHit"] == ["depth"]
           and not tight["outcomes"], "the lower bound must cause an incomplete search")
    recovery = check("recovery-safe", 100, 0, recovery=True)["check"]
    expect(recovery["verdict"] == "no violation, exhaustive" and
           not recovery["boundsHit"] and not recovery["violations"], "recovery search incomplete")
    expect(len(recovery["outcomes"]) == 1, "expected one complete recovery outcome")
    samples = final_fields(recovery["outcomes"][0])
    expected_samples = {
        "beforeLossPower": "true", "afterLossPower": "false", "afterLossLocked": "false",
        "afterUnlockedRequestPower": "false", "afterReacquirePower": "false", "afterReacquireLocked": "true",
    }
    for name, value in expected_samples.items():
        expect(samples.get(f"ChargingInterlock::recoveryEnvironment.{name}") == value,
               f"incorrect settled recovery observation: {name}")
    expect(samples.get("this.energized") == samples.get("this.linkLocked") == "true",
           "restart did not return to authorized power")

    shutdown = "                    assign energized := false;\n"
    expect(original.count(shutdown) == 1, "shutdown assignment not uniquely identified")
    stalled = out / "missing-shutdown.sysml"
    stalled.write_text(original.replace(shutdown, ""))
    failed = check("recovery-mutant", 100, 1, stalled,
                   ("-check-witness", str(out / "recovery-witnesses")), recovery=True)
    failed_search = failed["check"]
    failures = failed_search["violations"]
    expect(failed["strength"] == "witnessed" and failed_search["verdict"] == "violation" and
           not failed_search["boundsHit"] and len(failures) == 1 and
           failures[0]["kind"] == "property" and failures[0]["name"] == prop,
           "expected a witnessed INT-01 shutdown violation")
    expect(len(failed_search["outcomes"]) == 1, "expected one complete shutdown-mutant outcome")
    failed_samples = final_fields(failed_search["outcomes"][0])
    expect(failed_samples.get("this.energized") == failed_samples.get("this.linkLocked") == "true",
           "shutdown mutation should still end with authorized power")
    replay = run("recovery-replay", ["-schedule", f"replay:{failures[0]['path']}", "-trace"],
                 0, stalled, json_out=False, recovery=True)
    loss = replay.find("transition: supplying -> inhibited (event: accept LockLost)")
    unlocked = replay.find("transition: inhibited -> inhibited (event: accept ChargeRequested)")
    reacquired = replay.find("transition: inhibited -> armed (event: accept LockDetected)")
    expect(0 <= loss < unlocked < reacquired,
           "replayed loss, blocked request and reacquire sequence did not complete")
    loss_sample = replay[loss:unlocked]
    expect("eval chain energized -> true" in loss_sample and
           "eval chain linkLocked -> false" in loss_sample,
           "replay did not reproduce energized power with lock lost")

    print(f"Safe: {safe_search['states']} states, {safe_search['moves']} moves, depth {safe_search['depth']}")
    print(f"Both orders reach energized + locked; INT-01 mutation replayed at move {violations[0]['depth']}")
    print("Depth 1: incomplete, not safe")
    print(f"Recovery: {recovery['states']} states, {recovery['moves']} moves; "
          f"missing shutdown violated INT-01 at move {failures[0]['depth']}")


if __name__ == "__main__":
    main()
