#!/usr/bin/env python3
"""Stand-in for a simulation: writes the result files the demo imports.

Each file is a shape a script or tool commonly writes: a CSV table with units
in its headers, a JSON report whose names are not the model's, JSON Lines
telemetry and a tab-separated long table. The output is deterministic.
"""

import csv
import json
import os
import sys

ROVERS = {
    "scout": {"serial": "S-001", "dry_mass": 182.4, "cells": 12, "cell_wh": 95.0, "peak_w": 1450.5},
    "hauler": {"serial": "H-002", "dry_mass": 311.0, "cells": 20, "cell_wh": 92.5, "peak_w": 2210.0},
}


def main(out):
    os.makedirs(out, exist_ok=True)

    # Mass properties: one row per rover, the unit in the header.
    with open(os.path.join(out, "mass-properties.csv"), "w", newline="") as f:
        w = csv.writer(f, lineterminator="\n")
        w.writerow(["element", "serial", "mass [kg]", "qualified"])
        for name, r in ROVERS.items():
            serial = r["serial"] if name == "scout" else ""  # hauler's is in the model
            w.writerow([f"RoverDemo::{name}", serial, r["dry_mass"], name == "scout"])

    # Battery rig report: its own field names, mapped by battery-map.json.
    report = {
        "tool": "cellsim 2.1",
        "results": [
            {"unit": f"{name}::battery", "cells": r["cells"],
             "energy": {"value": round(r["cells"] * r["cell_wh"] * 3600), "unit": "J"},
             "note": "25 C, C/5"}
            for name, r in ROVERS.items()
        ],
    }
    with open(os.path.join(out, "battery-test.json"), "w") as f:
        json.dump(report, f, indent=2)
        f.write("\n")

    # Motor telemetry: one JSON object per line, the peak of each run.
    with open(os.path.join(out, "motor-telemetry.jsonl"), "w") as f:
        for name, r in ROVERS.items():
            f.write(json.dumps({"element": f"RoverDemo::{name}::driveMotor", "peakPower [W]": r["peak_w"]}) + "\n")

    # Review overrides: a long table, one value per row.
    with open(os.path.join(out, "overrides.tsv"), "w", newline="") as f:
        w = csv.writer(f, delimiter="\t", lineterminator="\n")
        w.writerow(["element", "feature", "value", "unit"])
        w.writerow(["RoverDemo::hauler", "health", "degraded", ""])


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(__file__), "..", "results"))
