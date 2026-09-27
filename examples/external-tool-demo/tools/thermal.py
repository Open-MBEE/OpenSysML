#!/usr/bin/env python3
"""Steady-state temperature solver for the external-tool demo.

Reads --mass (kg), --power (W) and --ambient (K) from the command line and
prints one CSV record to standard output: the temperature the mass settles
at and the margin left to the 400 K limit.
"""

import argparse
import csv
import sys


def main():
    parser = argparse.ArgumentParser(description="steady-state thermal solver")
    parser.add_argument("--mass", type=float, required=True, help="mass in kg")
    parser.add_argument("--power", type=float, required=True, help="heat load in W")
    parser.add_argument("--ambient", type=float, required=True, help="ambient temperature in K")
    args = parser.parse_args()
    if args.mass <= 0:
        print("mass must be positive", file=sys.stderr)
        return 2
    tmax = args.ambient + 0.8 * args.power / args.mass
    margin = 400.0 - tmax
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["tmax", "tmaxUnit", "margin"])
    writer.writerow([tmax, "K", margin])
    return 0


if __name__ == "__main__":
    sys.exit(main())
