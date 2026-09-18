#!/usr/bin/env python3
"""Compare transport latency for the same SysMLService calls.

Measures the four transports of the transport evaluation — gRPC over TCP,
Connect with a protobuf body, Connect with a JSON body, and stdio — against one
service build on one machine, so the numbers differ only by transport.

Two figures are reported, for the reason ``bench_latency.py`` gives: cold start
(spawn to first successful RPC) is paid once and hides in a mean of per-call
times, so it is timed separately, and per-call times are reported as p50/p95/p99
because the tail is what a deadline misses.

Usage:
    python clients/python/scripts/bench_transports.py [--iterations N] [--binary PATH]

Requires ``make build-grpc``. The transports are evaluation prototypes; see
docs/internals/design/transport-evaluation.md.
"""

import argparse
import http.client
import json
import os
import pathlib
import re
import socket
import statistics
import subprocess
import sys
import time

import grpc
from google.protobuf import json_format

from opensysml.proto import sysml_pb2 as pb
from opensysml.proto import sysml_pb2_grpc as pb_grpc

REPO = pathlib.Path(__file__).resolve().parents[3]

# The small model bench_latency.py builds, kept identical so the two scripts'
# numbers are comparable.
PARTS = 20
SMALL_MODEL = "package Bench {\n" + "".join(
    f"    part def P{i} {{ attribute a{i} = {i}; }}\n" for i in range(PARTS)
) + "}\n"

# A published example, for the case where payload size is the difference: its
# Query answer is ~900 elements rather than a handful.
LARGE_MODEL_PATH = (
    REPO / "examples/pilot-corpora/sysml-examples/Vehicle Example"
    / "SysML v2 Spec Annex A SimpleVehicleModel.sysml"
)

# The baseline transport, whose run also measures the payload sizes.
GRPC_TCP = "gRPC over TCP"

# The expression every Evaluate call sends, so the transports are compared on
# the same work.
EVALUATE_EXPRESSION = "2 + 2"

# stdio framing, as internal/stdiorpc writes it.
CONTENT_TYPE_JSON = "application/json"
CONTENT_TYPE_PROTO = "application/proto"


def free_port():
    """Reserve a port by binding and releasing it."""
    with socket.socket() as sock:
        sock.bind(("localhost", 0))
        return sock.getsockname()[1]


class Transport:
    """One way of calling SysMLService, spawning the service it calls."""

    #: Label used in the report.
    name = ""

    def call(self, method, request, response_type):
        raise NotImplementedError

    def reset(self):
        """Recover from a call made before the service was listening."""

    def close(self):
        raise NotImplementedError


class ServerTransport(Transport):
    """A transport whose service listens on a port of its own."""

    transport_flag = "grpc"

    def __init__(self, binary):
        self.port = free_port()
        self.process = subprocess.Popen(  # noqa: S603 - a path this script chose
            [
                str(binary),
                "-transport", self.transport_flag,
                "-port", str(self.port),
                "-health-port", str(free_port()),
                "-log-level", "error",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    def close(self):
        self.process.terminate()
        self.process.wait(timeout=10)


class GrpcTransport(ServerTransport):
    """Today's baseline: grpc-go over TCP, called by the generated stub."""

    name = GRPC_TCP

    def __init__(self, binary):
        super().__init__(binary)
        self.channel = None
        self.reset()

    def reset(self):
        # A refused call puts grpc's channel into a reconnect backoff of about a
        # second, which would be timed as the service's startup rather than the
        # client's retry, so each retry gets a channel of its own.
        if self.channel is not None:
            self.channel.close()
        self.channel = grpc.insecure_channel(f"localhost:{self.port}")
        self.stub = pb_grpc.SysMLServiceStub(self.channel)

    def call(self, method, request, response_type):
        return getattr(self.stub, method)(request)

    def close(self):
        self.channel.close()
        super().close()


class ConnectTransport(ServerTransport):
    """Connect protocol over one kept-alive HTTP/1.1 connection."""

    transport_flag = "connect"

    def __init__(self, binary, encoding):
        super().__init__(binary)
        self.encoding = encoding
        self.name = f"Connect, {encoding} body"
        self.content_type = (
            CONTENT_TYPE_PROTO if encoding == "protobuf" else CONTENT_TYPE_JSON
        )
        self.http = http.client.HTTPConnection("localhost", self.port, timeout=120)

    def reset(self):
        self.http.close()

    def call(self, method, request, response_type):
        if self.encoding == "protobuf":
            body = request.SerializeToString()
        else:
            body = json_format.MessageToJson(request).encode()

        self.http.request(
            "POST",
            f"/sysml.SysMLService/{method}",
            body=body,
            headers={"Content-Type": self.content_type},
        )
        answer = self.http.getresponse()
        payload = answer.read()
        if answer.status != 200:
            raise RuntimeError(f"{method}: HTTP {answer.status}: {payload[:200]!r}")

        response = response_type()
        if self.encoding == "protobuf":
            response.ParseFromString(payload)
        else:
            json_format.Parse(payload, response)
        return response

    def close(self):
        self.http.close()
        super().close()


class StdioTransport(Transport):
    """The service as a child process, spoken to over its stdin and stdout."""

    def __init__(self, binary, encoding):
        self.encoding = encoding
        self.name = f"stdio, {encoding} body"
        self.content_type = (
            CONTENT_TYPE_PROTO if encoding == "protobuf" else CONTENT_TYPE_JSON
        )
        self.next_id = 0
        self.process = subprocess.Popen(  # noqa: S603 - a path this script chose
            [str(binary), "-transport", "stdio", "-log-level", "error"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )

    def _send(self, body, headers):
        frame = f"Content-Length: {len(body)}\r\nContent-Type: {self.content_type}\r\n"
        for name, value in headers.items():
            frame += f"{name}: {value}\r\n"
        self.process.stdin.write(frame.encode() + b"\r\n" + body)
        self.process.stdin.flush()

    def _receive(self):
        headers = {}
        while True:
            line = self.process.stdout.readline()
            if not line:
                raise RuntimeError("the service closed its stdout")
            if line in (b"\r\n", b"\n"):
                break
            name, _, value = line.decode().partition(":")
            headers[name.strip().lower()] = value.strip()
        length = int(headers["content-length"])
        return headers, self.process.stdout.read(length)

    def call(self, method, request, response_type):
        self.next_id += 1
        response = response_type()

        if self.encoding == "protobuf":
            self._send(
                request.SerializeToString(),
                {"Sysml-Method": method, "Sysml-Id": str(self.next_id)},
            )
            headers, payload = self._receive()
            if headers.get("sysml-status-code") != "0":
                raise RuntimeError(
                    f"{method}: code {headers.get('sysml-status-code')}: "
                    f"{headers.get('sysml-status-message')}"
                )
            response.ParseFromString(payload)
            return response

        body = json.dumps({
            "jsonrpc": "2.0",
            "id": self.next_id,
            "method": method,
            "params": json.loads(json_format.MessageToJson(request)),
        }).encode()
        self._send(body, {})
        _, payload = self._receive()
        answer = json.loads(payload)
        if "error" in answer:
            raise RuntimeError(f"{method}: {answer['error']}")
        json_format.ParseDict(answer["result"], response)
        return response

    def close(self):
        self.process.stdin.close()
        self.process.wait(timeout=10)


def cold_starts(build, spawns):
    """Time spawn to first successful RPC ``spawns`` times, keeping the last
    transport open for the per-call measurements."""
    samples = []
    transport = None
    for spawn in range(spawns):
        if transport is not None:
            transport.close()
        transport, elapsed = start(build)
        samples.append(elapsed)
        del spawn
    return transport, percentiles(samples)


def start(build):
    """Spawn a transport and time spawn to first successful RPC."""
    start_time = time.perf_counter()
    transport = build()
    deadline = start_time + 60
    while True:
        try:
            transport.call("GetServerInfo", pb.ServerInfoRequest(), pb.ServerInfoResponse)
            break
        except Exception:  # noqa: BLE001 - the service is not up yet
            if time.perf_counter() > deadline:
                transport.close()
                raise
            transport.reset()
            time.sleep(0.002)
    return transport, (time.perf_counter() - start_time) * 1000.0


def percentiles(samples):
    """Report p50/p95/p99 in milliseconds from sorted samples."""
    samples = sorted(samples)

    def pct(p):
        return samples[min(len(samples) - 1, int(len(samples) * p))]

    return statistics.median(samples), pct(0.95), pct(0.99), statistics.stdev(samples)


def measure(call, iterations):
    """Time ``call`` ``iterations`` times after one warm-up."""
    call()
    samples = []
    for _ in range(iterations):
        started = time.perf_counter()
        call()
        samples.append((time.perf_counter() - started) * 1000.0)
    return percentiles(samples)


def operations(transport, model, symbol):
    """The calls measured for one model: a parse, an evaluation, a whole-model
    query and an instantiation."""
    parsed = transport.call(
        "ParseFile",
        pb.ParseFileRequest(content=model),
        pb.ParseFileResponse,
    )
    model_hash = parsed.model_hash
    return [
        ("ParseFile", lambda: transport.call(
            "ParseFile", pb.ParseFileRequest(content=model), pb.ParseFileResponse)),
        ("Evaluate", lambda: transport.call(
            "Evaluate",
            pb.EvaluateRequest(expression=EVALUATE_EXPRESSION, model_hash=model_hash),
            pb.EvaluateResponse)),
        ("Query (whole model)", lambda: transport.call(
            "Query",
            pb.QueryRequest(model_hash=model_hash, query=pb.Query()),
            pb.QueryResponse)),
        ("Instantiate", lambda: transport.call(
            "Instantiate",
            pb.InstantiateRequest(model_hash=model_hash, symbol_id=symbol),
            pb.InstantiateResponse)),
    ]


def payload_sizes(transport, model, symbol):
    """Report the wire bytes of each measured answer, since payload size is what
    a JSON body is expected to lose on."""
    sizes = {}
    parsed = transport.call(
        "ParseFile", pb.ParseFileRequest(content=model), pb.ParseFileResponse)
    for label, request, response in (
        ("ParseFile", pb.ParseFileRequest(content=model), parsed),
        ("Evaluate",
         pb.EvaluateRequest(expression=EVALUATE_EXPRESSION, model_hash=parsed.model_hash),
         transport.call("Evaluate",
                        pb.EvaluateRequest(expression=EVALUATE_EXPRESSION,
                                           model_hash=parsed.model_hash),
                        pb.EvaluateResponse)),
        ("Query (whole model)",
         pb.QueryRequest(model_hash=parsed.model_hash, query=pb.Query()),
         transport.call("Query",
                        pb.QueryRequest(model_hash=parsed.model_hash, query=pb.Query()),
                        pb.QueryResponse)),
        ("Instantiate",
         pb.InstantiateRequest(model_hash=parsed.model_hash, symbol_id=symbol),
         transport.call("Instantiate",
                        pb.InstantiateRequest(model_hash=parsed.model_hash,
                                              symbol_id=symbol),
                        pb.InstantiateResponse)),
    ):
        sizes[label] = (
            len(request.SerializeToString()),
            len(json_format.MessageToJson(request, indent=0).encode()),
            len(response.SerializeToString()),
            len(json_format.MessageToJson(response, indent=0).encode()),
        )
    return sizes


def first_part_def(model):
    """The qualified name of a part def the model declares, for Instantiate."""
    package = re.search(r"^\s*package\s+([\w']+)", model, re.MULTILINE)
    part = re.search(r"^\s*part def\s+([\w']+)", model, re.MULTILINE)
    if not package or not part:
        raise RuntimeError("the model declares no package with a part def")
    return f"{package.group(1)}::{part.group(1)}"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iterations", type=int, default=200)
    parser.add_argument("--spawns", type=int, default=10,
                        help="how many times each transport is cold-started")
    parser.add_argument("--binary", default=str(REPO / "bin/sysml-grpc"))
    parser.add_argument("--json", help="also write the numbers to this path")
    args = parser.parse_args(argv)

    if not os.access(args.binary, os.X_OK):
        print(f"error: {args.binary} is not executable; run make build-grpc",
              file=sys.stderr)
        return 2
    if not LARGE_MODEL_PATH.exists():
        print(f"error: {LARGE_MODEL_PATH} is absent; run "
              "./scripts/download-pilot-corpora.sh", file=sys.stderr)
        return 2

    large_model = LARGE_MODEL_PATH.read_text()
    models = [
        ("small", SMALL_MODEL, first_part_def(SMALL_MODEL)),
        ("large", large_model, first_part_def(large_model)),
    ]
    builders = [
        (GRPC_TCP, lambda: GrpcTransport(args.binary)),
        ("Connect, protobuf body", lambda: ConnectTransport(args.binary, "protobuf")),
        ("Connect, JSON body", lambda: ConnectTransport(args.binary, "json")),
        ("stdio, protobuf body", lambda: StdioTransport(args.binary, "protobuf")),
        ("stdio, JSON body", lambda: StdioTransport(args.binary, "json")),
    ]

    report = {"iterations": args.iterations, "spawns": args.spawns,
              "transports": {}, "payload_bytes": {}}
    for label, build in builders:
        transport, (cold_p50, cold_p95, cold_p99, cold_sd) = cold_starts(
            build, args.spawns)
        entry = {"cold_start_ms": {"p50": cold_p50, "p95": cold_p95,
                                   "p99": cold_p99, "stdev": cold_sd},
                 "models": {}}
        try:
            for size, model, symbol in models:
                measured = {}
                for name, call in operations(transport, model, symbol):
                    p50, p95, p99, stdev = measure(call, args.iterations)
                    measured[name] = {"p50": p50, "p95": p95, "p99": p99,
                                      "stdev": stdev}
                entry["models"][size] = measured
                if label == GRPC_TCP:
                    report["payload_bytes"][size] = payload_sizes(
                        transport, model, symbol)
        finally:
            transport.close()
        report["transports"][label] = entry
        print(f"{label}: cold start p50 {cold_p50:.1f} ms", file=sys.stderr)

    for size, _, _ in models:
        print(f"\n{size} model, {args.iterations} iterations")
        print(f"{'transport':<24}{'operation':<22}"
              f"{'p50 ms':>9}{'p95 ms':>9}{'p99 ms':>9}{'sd ms':>8}")
        for label, entry in report["transports"].items():
            for name, stats in entry["models"][size].items():
                print(f"{label:<24}{name:<22}{stats['p50']:>9.2f}"
                      f"{stats['p95']:>9.2f}{stats['p99']:>9.2f}"
                      f"{stats['stdev']:>8.2f}")

    print(f"\ncold start (spawn to first successful RPC), {args.spawns} spawns")
    print(f"{'transport':<24}{'p50 ms':>9}{'p95 ms':>9}{'p99 ms':>9}{'sd ms':>8}")
    for label, entry in report["transports"].items():
        cold = entry["cold_start_ms"]
        print(f"{label:<24}{cold['p50']:>9.1f}{cold['p95']:>9.1f}"
              f"{cold['p99']:>9.1f}{cold['stdev']:>8.1f}")

    print(f"\n{'model':<8}{'operation':<22}{'req proto':>11}{'req json':>10}"
          f"{'res proto':>11}{'res json':>10}")
    for size, sizes in report["payload_bytes"].items():
        for name, (rq_p, rq_j, rs_p, rs_j) in sizes.items():
            print(f"{size:<8}{name:<22}{rq_p:>11}{rq_j:>10}{rs_p:>11}{rs_j:>10}")

    if args.json:
        pathlib.Path(args.json).write_text(json.dumps(report, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
