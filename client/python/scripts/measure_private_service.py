"""Measure what a private sysml-grpc child costs a script that connects.

Reports the cold start of the first connection, the cost of the connections that
join it, and what a child per connection would have cost instead.

Usage: python3 clients/python/scripts/measure_private_service.py [samples]
"""

import statistics
import sys
import time

from opensysml.connection import Connection, _private_services


def _percentile(values, fraction):
    """The value at a fraction of the sorted samples."""
    ordered = sorted(values)
    index = min(int(fraction * len(ordered)), len(ordered) - 1)
    return ordered[index]


def _report(label, samples):
    """Print milliseconds at p50 and p95 for one measurement."""
    print(
        f"{label:<44} n={len(samples):<4} "
        f"p50={_percentile(samples, 0.5) * 1000:7.2f} ms  "
        f"p95={_percentile(samples, 0.95) * 1000:7.2f} ms  "
        f"mean={statistics.mean(samples) * 1000:7.2f} ms"
    )


def measure_cold_start(samples):
    """One connection with no service running: spawn, bind, report, handshake."""
    timings = []
    for _ in range(samples):
        started = time.perf_counter()
        conn = Connection()
        timings.append(time.perf_counter() - started)
        conn.close()
    return timings


def measure_joining_connection(samples):
    """A connection that joins the child this interpreter already has."""
    held = Connection()
    timings = []
    try:
        for _ in range(samples):
            started = time.perf_counter()
            conn = Connection()
            timings.append(time.perf_counter() - started)
            conn.close()
    finally:
        held.close()
    return timings


def measure_child_per_connection(samples):
    """What one child per connection would cost: the registry is cleared each time."""
    timings = []
    connections = []
    try:
        for _ in range(samples):
            _private_services.clear()
            started = time.perf_counter()
            conn = Connection()
            timings.append(time.perf_counter() - started)
            connections.append(conn)
    finally:
        for conn in connections:
            conn.close()
        _private_services.clear()
    return timings


_MODEL = "package Demo {\n" + "".join(
    f"    part def P{index} {{ attribute a{index} : ScalarValues::Integer; }}\n"
    for index in range(200)
) + "}\n"


def measure_shared_parse_cache(samples):
    """The same model parsed by a second connection, which shares the cache."""
    timings = []
    with Connection() as first:
        first.load_from_content(_MODEL)
        for _ in range(samples):
            with Connection() as second:
                started = time.perf_counter()
                second.load_from_content(_MODEL)
                timings.append(time.perf_counter() - started)
    return timings


def measure_unshared_parse(samples):
    """The same model parsed by a connection whose child has its own cache."""
    timings = []
    for _ in range(samples):
        _private_services.clear()
        with Connection() as conn:
            started = time.perf_counter()
            conn.load_from_content(_MODEL)
            timings.append(time.perf_counter() - started)
    _private_services.clear()
    return timings


def main(argv):
    """Run each measurement and print it."""
    samples = int(argv[1]) if len(argv) > 1 else 30
    _report("cold start, private child", measure_cold_start(samples))
    _report("joining this interpreter's child", measure_joining_connection(samples))
    _report("a child per connection", measure_child_per_connection(samples))
    _report("parse of a model the shared child has", measure_shared_parse_cache(samples))
    _report("parse by a child of its own", measure_unshared_parse(samples))
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv))
