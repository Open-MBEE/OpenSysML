#!/usr/bin/env python3
"""Measure opensysml call latency against a running sysml-grpc.

Reports p50/p95/p99 per operation, which is what an analytics loop has to budget
for: a mean hides the tail, and the tail is what a deadline meets or misses.

Usage:
    python clients/python/scripts/bench_latency.py [--host H] [--port P] [--iterations N]

The service must be reachable (``make build-grpc && bin/sysml-grpc``). Numbers
are per-call round trips over a warm channel, so they include protobuf
encode/decode and loopback transport but not connection setup, which is paid
once per Connection and is reported separately.
"""

import argparse
import statistics
import sys
import time

from opensysml import Connection

# A model of a few dozen elements: large enough that parsing is not measuring an
# empty file, small enough to stay a per-call cost rather than a document load.
PARTS = 20
MODEL = "package Bench {\n" + "".join(
    f"    part def P{i} {{ attribute a{i} = {i}; }}\n" for i in range(PARTS)
) + "}\n"


def measure(label, call, iterations, results):
    """Time ``call`` ``iterations`` times, after one warm-up, and record it."""
    call()
    samples = []
    for _ in range(iterations):
        start = time.perf_counter()
        call()
        samples.append((time.perf_counter() - start) * 1000.0)
    samples.sort()

    def pct(p):
        return samples[min(len(samples) - 1, int(len(samples) * p))]

    results.append((label, statistics.median(samples), pct(0.95), pct(0.99)))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default="localhost")
    # No default: an unwritten port lets --host carry its own (default 50051).
    parser.add_argument("--port", type=int)
    parser.add_argument("--iterations", type=int, default=200)
    args = parser.parse_args(argv)

    start = time.perf_counter()
    try:
        conn = Connection(host=args.host, port=args.port, auto_start=False)
    except ValueError as exc:
        # A misread --host/--port is the caller's mistake, not a crash.
        print(f"error: {exc}", file=sys.stderr)
        return 2
    model = conn.load_from_content(MODEL)
    setup_ms = (time.perf_counter() - start) * 1000.0

    results = []
    with conn:
        measure("server_info (cached client-side)", conn.server_info, args.iterations, results)
        measure("load_from_content (cache hit)",
                lambda: conn.load_from_content(MODEL), args.iterations, results)
        misses = iter(range(args.iterations + 1))
        measure("load_from_content (cache miss)",
                lambda: conn.load_from_content(f"{MODEL}// {next(misses)}\n"),
                args.iterations, results)
        # Distinct sources evict, so re-seat the model the runtime calls name.
        model = conn.load_from_content(MODEL)
        measure("eval 2 + 2", lambda: conn.eval("2 + 2", model.hash), args.iterations, results)
        measure("convert sysml -> sysml",
                lambda: conn.convert("sysml", content=MODEL, from_format="sysml"),
                args.iterations, results)
        measure("convert sysml -> ttl",
                lambda: conn.convert("ttl", content=MODEL, from_format="sysml"),
                args.iterations, results)
        turtle = str(conn.convert("ttl", content=MODEL, from_format="sysml"))
        measure("convert ttl -> sysml",
                lambda: conn.convert("sysml", content=turtle, from_format="ttl"),
                args.iterations, results)

    print(f"model: {PARTS} part defs, {len(MODEL)} bytes; "
          f"{args.iterations} iterations; connect + first parse: {setup_ms:.1f} ms")
    print(f"{'operation':<34}{'p50 ms':>9}{'p95 ms':>9}{'p99 ms':>9}")
    for label, p50, p95, p99 in results:
        print(f"{label:<34}{p50:>9.2f}{p95:>9.2f}{p99:>9.2f}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
