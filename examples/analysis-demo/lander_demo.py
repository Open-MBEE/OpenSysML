#!/usr/bin/env python3
"""Ask the lander model's questions from Python: budget, verdict, choice, timing.

Run it from the repository root, with the `opensysml` client installed and a
`sysml-grpc` service available (see docs/guide/09-clients.md):

    python examples/analysis-demo/lander_demo.py
"""

import pathlib
import sys

import opensysml

MODEL = pathlib.Path(__file__).with_name("lander.sysml")


def main() -> None:
    model = opensysml.load(str(MODEL), strict=True)

    print("== the fuel budget of one descent")
    budget = model.run_analysis("Descent::scoutBudget")
    print(f"fuel left     {budget.outputs['fuelLeft']} kg after {budget.outputs['fuelUsed']} kg burned")
    print(f"reserve held  {budget.satisfied}")

    long_burn = model.run_analysis("Descent::scoutBudget", named_arguments={"burnTime": 80.0})
    print(f"80 s burn     {long_burn.outputs['fuelLeft']} kg left, reserve held {long_burn.satisfied}")
    for verdict in long_burn.verdicts:
        print(f"              {verdict.explain()}")

    hauler = model.run_analysis("Descent::FuelBudget", subject="Landers::hauler")
    print(f"the hauler    {hauler.outputs['fuelLeft']} kg left, reserve held {hauler.satisfied}")

    print("\n== how far the budget stretches")
    table = model.run_sweep("Descent::scoutBudget", {"burnTime": (20.0, 80.0, 20.0)})
    for row in table:
        held = "holds" if row else "fails"
        print(f"burnTime {row.inputs['burnTime']:>5}  fuelLeft {row.outputs['fuelLeft']:>6}  reserve {held}")

    drawn = model.run_sweep("Descent::scoutBudget", {"burnTime": (20.0, 80.0)}, samples=4, seed=7)
    print(f"seed {drawn.seed}: {len(drawn)} draws, "
          f"{sum(1 for row in drawn if row)} within the reserve")

    print("\n== does the scout land softly?")
    check = model.run_analysis("Descent::checkScout")
    for verification in check.verifications:
        print(f"body verdict  {verification.kind}")
    print(f"objective     {'satisfied' if check.satisfied else 'not satisfied'}")

    heavy = model.run_analysis("Descent::TouchdownCheck", subject="Landers::hauler")
    print(f"the hauler    {heavy.verifications[0].kind}")

    print("\n== which lander?")
    lightest = model.run_analysis("Selection::lightest")
    for evaluation in lightest.evaluations:
        mark = " [selected]" if evaluation.selected else ""
        print(f"{evaluation.arguments[0].type_symbol_id:<16} {evaluation.result:>7}{mark}")

    for fuel_cost in (0.0, 0.25, 0.5):
        pick = model.run_analysis("Selection::mostValuable", named_arguments={"fuelCost": fuel_cost})
        print(f"fuel at {fuel_cost:<5} -> {pick.selected[0].arguments[0].type_symbol_id}")

    print("\n== the watch and the flight mode on one clock")
    runs = model.explore_analysis("Timing::watchDescent")
    for outcome in runs:
        print(f"sawBraking = {outcome.outputs['sawBraking']!s:<5} "
              f"({outcome.linearizations} order): {outcome.witness[0]}")


if __name__ == "__main__":
    sys.stdout.reconfigure(line_buffering=True)
    main()
