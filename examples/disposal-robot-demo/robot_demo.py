#!/usr/bin/env python3
"""Drive the robot model from Python: what it is, what it does, what holds.

Run it from the repository root, with the `opensysml` client installed and a
`sysml-grpc` service available (see docs/guide/09-clients.md):

    python examples/disposal-robot-demo/robot_demo.py
"""

import pathlib
import sys

import opensysml

MODEL = pathlib.Path(__file__).with_name("robot.sysml")


def main() -> None:
    model = opensysml.load(str(MODEL), strict=True)

    print("== the robot as an object")
    robot = model.instantiate("Robot::fielded")
    print(f"mass          {robot.mass} kg")
    print(f"drive power   {robot.mobility.drivePower} W at {robot.mobility.speed} m/s")
    print(f"wheels        {len(robot.mobility.wheels)}")
    print(f"battery       {robot.battery.charge} of {robot.battery.capacity} Wh")

    print("\n== planning the approach")
    metres = model.calc("RobotBehavior::Approach", arguments=[120.0, 90.0]).value[0]
    energy = model.calc(
        "RobotBehavior::energyForDrive",
        arguments=[metres, robot.mobility.speed, robot.mobility.drivePower],
    ).value[0]
    print(f"{metres} m costs {energy:.1f} Wh of the {robot.battery.charge} Wh aboard")

    print("\n== running the call-out")
    results = model.execute_action("RobotBehavior::DisposalRun")
    print(f"frames held   {results['framesHeld']}")
    print(f"arm frames    {results['armFrames']} (the retrieval branch ran through)")
    print(f"streamed      {results['streamed']}")
    print(f"charge left   {results['chargeLeft']} Wh")

    print("\n== running the mode machine")
    run = model.execute_state("RobotBehavior::Modes")
    print(f"states        {' -> '.join(run['states_visited'])}")
    written = run["final_context"]
    print("wrote         " + ", ".join(f"{k}={written[k]}" for k in sorted(written)))

    print("\n== checking the call-out budget")
    verdicts = model.verify_satisfaction("RobotSolver::plans")
    for verdict in verdicts:
        verdict.raise_for_error()
        print(verdict)
    print(f"both hold     {all(verdicts)}")

    heavy = model.verify_constraint(
        "Robot::Platform::withinMassBudget", subject="Robot::heavyMockup"
    )
    print(f"heavy mockup  {'within' if heavy else 'over'} the mass budget")


if __name__ == "__main__":
    # Each section prints as it runs, so a piped or logged run reads along
    # rather than arriving at once when the script ends.
    sys.stdout.reconfigure(line_buffering=True)
    main()
