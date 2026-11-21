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
        "-action", "ChargingInterlock::designEnvironment", "-advance", "3",
    ]
    prop = "ChargingInterlock::powerGate"

    def run(name, flags, status, path=model, json_out=True):
        command = [args.sysml, *common, *flags, *(["-json"] if json_out else []), str(path)]
        result = subprocess.run(command, capture_output=True, text=True, timeout=60, check=False)
        report = out / (name + (".json" if json_out else ".txt"))
        report.write_text(result.stdout)
        (out / f"{name}.stderr").write_text(result.stderr)
        expect(result.returncode == status,
               f"{name}: exit {result.returncode}, expected {status}; see {report}")
        return json.loads(result.stdout)["checks"][0] if json_out else result.stdout

    def check(name, depth, status, path=model, extra=()):
        run_result = run(name, [
            "-engine", "check", "-check-property", prop, "-check-depth", str(depth),
            "-check-states", "20000", "-check-timeout", "30s", *extra,
        ], status, path)
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
    print(f"Safe: {safe_search['states']} states, {safe_search['moves']} moves, depth {safe_search['depth']}")
    print(f"Both orders reach energized + locked; INT-01 mutation replayed at move {violations[0]['depth']}")
    print("Depth 1: incomplete, not safe")


if __name__ == "__main__":
    main()
