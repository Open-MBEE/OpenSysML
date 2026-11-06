#!/usr/bin/env python3
"""Tests for dotnet-gates.py and its Make wiring, with stand-in dotnet hosts and reports."""

import contextlib
import glob
import importlib.util
import io
import json
import os
import stat
import subprocess
import sys
import tempfile
import textwrap
import unittest
import xml.etree.ElementTree as ET
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("dotnet_gates", os.path.join(HERE, "dotnet-gates.py"))
gates = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gates)

RUNTIMES_10 = (
    "Microsoft.AspNetCore.App 10.0.13 [/x/shared/Microsoft.AspNetCore.App]\n"
    "Microsoft.NETCore.App 10.0.13 [/x/shared/Microsoft.NETCore.App]\n"
)
RUNTIMES_9 = "Microsoft.NETCore.App 9.0.10 [/x/shared/Microsoft.NETCore.App]\n"
IDENTITY = "OpenMBEE.OpenSysML 0.10.1 (.NETCoreApp,Version=v10.0)"
METAMODEL_IDENTITY = "OpenMBEE.OpenSysML.Metamodel 0.10.1 (.NETCoreApp,Version=v10.0)"
METAMODEL_LINES = (
    METAMODEL_IDENTITY,
    "87 elements, 1 root namespace(s), metamodel 20250201",
    "part def Base owns mass",
    "part def Vehicle owns mass2, wheel, p, cargo, a, c, s",
    "part wheel : (none)",
    "part v : Vehicle",
)


def report(classes, valid=10, covered=5, sources=()):
    rows = "".join(f'<class name="C{i}" filename="{f}" />' for i, f in enumerate(classes))
    listed = "".join(f"<source>{s}</source>" for s in sources)
    return (
        f'<coverage lines-covered="{covered}" lines-valid="{valid}"><sources>{listed}</sources>'
        f'<packages><package name="P"><classes>{rows}</classes></package></packages></coverage>'
    )


def python_hosts(test):
    """Launch registered Python stand-ins explicitly, including on Windows."""
    registered = set()
    popen = gates.subprocess.Popen

    def launch(argv, *args, **kwargs):
        if argv and argv[0] in registered:
            argv = [sys.executable, *argv]
        return popen(argv, *args, **kwargs)

    patch = mock.patch.object(gates.subprocess, "Popen", side_effect=launch)
    patch.start()
    test.addCleanup(patch.stop)
    return registered


class GatesTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.python_hosts = python_hosts(self)
        self.global_json = os.path.join(self.tmp.name, "global.json")
        with open(self.global_json, "w", encoding="utf-8") as f:
            json.dump({"sdk": {"version": "10.0.402"}}, f)

    def host(self, version, runtimes, version_exit=0):
        """A stand-in dotnet printing a version and a runtime list."""
        path = os.path.join(self.tmp.name, "dotnet")
        with open(path, "w", encoding="utf-8") as f:
            f.write(textwrap.dedent(f"""\
                #!{sys.executable}
                import sys
                if sys.argv[1] == "--version":
                    print({version!r}); sys.exit({version_exit})
                sys.stdout.write({runtimes!r})
                """))
        os.chmod(path, os.stat(path).st_mode | stat.S_IEXEC)
        self.python_hosts.add(path)
        return path

    def write(self, name, text):
        path = os.path.join(self.tmp.name, name)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def main_on_legacy_console(self, argv):
        buffer = io.BytesIO()
        with io.TextIOWrapper(buffer, encoding="cp1252", errors="strict", newline="\n") as stdout:
            with contextlib.redirect_stdout(stdout):
                result = gates.main(argv)
            stdout.flush()
            return result, buffer.getvalue().decode("cp1252")

    def test_main_preflight_can_report_success_on_a_legacy_windows_console(self):
        with mock.patch.object(gates, "preflight", return_value=("10.0.402", ["10.0.13"])):
            result, stdout = self.main_on_legacy_console(["preflight"])
        self.assertEqual(result, 0)
        self.assertEqual(stdout, "OK .NET SDK 10.0.402, runtime 10.0.13\n")

    def test_main_coverage_can_report_success_on_a_legacy_windows_console(self):
        path = self.write("run/coverage.cobertura.1.xml", report(["/_/client/dotnet/src/Lib/A.cs"]))
        result, stdout = self.main_on_legacy_console(["coverage", os.path.dirname(path)])
        self.assertEqual(result, 0)
        self.assertEqual(stdout, f"OK {os.path.normpath(path)}: 5 of 10 lines covered\n")

    def test_main_sample_can_report_success_on_a_legacy_windows_console(self):
        for served in (None, "sysml-grpc v1.0.0, 3 capabilities"):
            with self.subTest(served=served), mock.patch.object(gates, "sample", return_value=served):
                result, stdout = self.main_on_legacy_console(["sample", "--expect", IDENTITY])
                self.assertEqual(result, 0)
                expected = f"OK {served}\n" if served else (
                    "- no sysml-grpc configured ($OPENSYSML_GRPC_BINARY); skipped the service run\n"
                )
                self.assertEqual(stdout, f"OK {IDENTITY}\n" + expected)

    def test_preflight_accepts_the_pinned_sdk_with_a_net10_runtime(self):
        sdk, runtimes = gates.preflight(self.host("10.0.402", RUNTIMES_10), self.global_json)
        self.assertEqual(sdk, "10.0.402")
        self.assertEqual(runtimes, ["10.0.13"])

    def test_preflight_accepts_a_later_patch_in_the_pinned_band(self):
        sdk, _ = gates.preflight(self.host("10.0.403", RUNTIMES_10), self.global_json)
        self.assertEqual(sdk, "10.0.403")

    def test_preflight_rejects_an_sdk_outside_the_pinned_band_or_below_its_patch(self):
        for sdk in ("10.0.401", "10.0.300", "10.0.500", "10.1.402", "11.0.402"):
            with self.subTest(sdk=sdk), self.assertRaisesRegex(
                gates.GateError,
                rf"SDK '{sdk}', but global.json pins '10.0.402'; install SDK 10.0.402 or a later 10.0.4xx patch",
            ):
                gates.preflight(self.host(sdk, RUNTIMES_10), self.global_json)

    def test_preflight_rejects_a_prerelease_sdk(self):
        with self.assertRaisesRegex(gates.GateError, "'10.0.403-rc.1' is not a release .NET SDK version"):
            gates.preflight(self.host("10.0.403-rc.1", RUNTIMES_10), self.global_json)

    def test_preflight_rejects_a_pin_that_is_not_a_release_sdk_version(self):
        with open(self.global_json, "w", encoding="utf-8") as f:
            json.dump({"sdk": {"version": "10.0"}}, f)
        with self.assertRaisesRegex(gates.GateError, "'10.0' is not a release .NET SDK version"):
            gates.preflight(self.host("10.0.402", RUNTIMES_10), self.global_json)

    def test_preflight_rejects_a_host_that_cannot_select_the_pinned_sdk(self):
        with self.assertRaisesRegex(gates.GateError, "exited 155"):
            gates.preflight(self.host("", RUNTIMES_10, version_exit=155), self.global_json)

    def test_preflight_rejects_an_absent_net10_runtime(self):
        with self.assertRaisesRegex(gates.GateError, "no .NET 10 runtime"):
            gates.preflight(self.host("10.0.402", RUNTIMES_9), self.global_json)

    def test_preflight_rejects_a_missing_host(self):
        with self.assertRaisesRegex(gates.GateError, "cannot run"):
            gates.preflight(os.path.join(self.tmp.name, "absent"), self.global_json)

    def test_preflight_rejects_a_global_json_without_a_version(self):
        with open(self.global_json, "w", encoding="utf-8") as f:
            json.dump({"test": {}}, f)
        with self.assertRaisesRegex(gates.GateError, "pins no sdk.version"):
            gates.preflight(self.host("10.0.402", RUNTIMES_10), self.global_json)

    def test_the_real_global_json_pins_an_sdk(self):
        self.assertRegex(gates.pinned_sdk(), r"^10\.0\.\d+$")
        with open(gates.GLOBAL_JSON, encoding="utf-8") as f:
            self.assertEqual(json.load(f)["sdk"].get("rollForward"), "latestPatch")

    def test_coverage_accepts_handwritten_library_code(self):
        self.write("run/coverage.cobertura.1.xml", report(["/_/client/dotnet/src/Lib/A.cs"]))
        [(_, covered, valid)] = gates.coverage(os.path.join(self.tmp.name, "run"), ["Lib/Wire/Sysml.cs"])
        self.assertEqual((covered, valid), (5, 10))

    def test_coverage_accepts_windows_paths(self):
        self.write("run/coverage.cobertura.1.xml", report([r"C:\r\client\dotnet\src\Lib\A.cs"]))
        gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_resolves_filenames_against_the_sources(self):
        self.write("run/coverage.cobertura.1.xml", report(
            ["A.cs", "Inner/B.cs"], sources=["/_/client/dotnet/src/Lib/Internal/"]))
        gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_resolves_windows_sources(self):
        self.write("run/coverage.cobertura.1.xml", report(["A.cs"], sources=["C:\\r\\client\\dotnet\\src\\Lib\\"]))
        gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_a_relative_filename_outside_the_library_sources(self):
        self.write("run/coverage.cobertura.1.xml", report(["ATests.cs"], sources=["/r/client/dotnet/test/T/"]))
        with self.assertRaisesRegex(gates.GateError, "measures ATests.cs, outside client/dotnet/src/"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_relative_generated_code(self):
        self.write("run/coverage.cobertura.1.xml", report(
            ["Internal/A.cs", "Wire/Sysml.cs"], sources=["/r/client/dotnet/src/Lib/"]))
        with self.assertRaisesRegex(gates.GateError, "measures generated Wire/Sysml.cs"):
            gates.coverage(os.path.join(self.tmp.name, "run"), ["Lib/Wire/Sysml.cs"])

    def test_coverage_rejects_a_run_without_a_report(self):
        os.makedirs(os.path.join(self.tmp.name, "run"))
        with self.assertRaisesRegex(gates.GateError, "holds no Cobertura report"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_an_empty_report(self):
        self.write("run/coverage.cobertura.1.xml", report([], valid=0, covered=0))
        with self.assertRaisesRegex(gates.GateError, "measures no code"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_a_report_with_no_valid_lines(self):
        self.write("run/coverage.cobertura.1.xml", report(["/r/client/dotnet/src/A.cs"], valid=0))
        with self.assertRaisesRegex(gates.GateError, "measures no code"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_generated_code(self):
        self.write("run/coverage.cobertura.1.xml", report([
            "/r/client/dotnet/src/Lib/A.cs", "/r/client/dotnet/src/Lib/Wire/Sysml.cs",
        ]))
        with self.assertRaisesRegex(gates.GateError, "measures generated .*Wire/Sysml.cs"):
            gates.coverage(os.path.join(self.tmp.name, "run"), ["Lib/Wire/Sysml.cs"])

    def test_coverage_rejects_code_outside_the_library_sources(self):
        self.write("run/coverage.cobertura.1.xml", report(["/r/client/dotnet/test/T/ATests.cs"]))
        with self.assertRaisesRegex(gates.GateError, "outside client/dotnet/src/"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_a_report_that_is_not_cobertura(self):
        self.write("run/coverage.cobertura.1.xml", "<testsuites />")
        with self.assertRaisesRegex(gates.GateError, "not a Cobertura report"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_rejects_a_report_that_is_not_xml(self):
        self.write("run/coverage.cobertura.1.xml", "<coverage")
        with self.assertRaisesRegex(gates.GateError, "is not XML"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def test_coverage_checks_every_report(self):
        self.write("run/a/coverage.cobertura.1.xml", report(["/r/client/dotnet/src/A.cs"]))
        self.write("run/b/coverage.cobertura.2.xml", report([], valid=0))
        with self.assertRaisesRegex(gates.GateError, "b.coverage.cobertura.2.xml measures no code"):
            gates.coverage(os.path.join(self.tmp.name, "run"))

    def executable(self, name, body):
        path = self.write(name, f"#!{sys.executable}\n" + textwrap.dedent(body))
        os.chmod(path, os.stat(path).st_mode | stat.S_IEXEC)
        self.python_hosts.add(path)
        return path

    def sample_host(self, service_line="sysml-grpc v1.0.0, 3 capabilities", identity=IDENTITY, model_line=gates.SAMPLE_MODEL_LINE,
                    private_model_line=gates.SAMPLE_MODEL_LINE):
        """A stand-in dotnet whose sample prints its identity and, with a service or a binary named, a service line and a model line."""
        seen = os.path.join(self.tmp.name, "seen-service")
        started = os.path.join(self.tmp.name, "seen-binary")
        return self.executable("sample-dotnet", f"""\
            import os
            print({identity!r})
            service = os.environ.get("OPENSYSML_SERVICE")
            binary = os.environ.get("OPENSYSML_GRPC_BINARY")
            if service and binary:
                print("both named")
            elif service:
                open({seen!r}, "w").write(service)
                if {service_line!r}:
                    print({service_line!r})
                if {model_line!r}:
                    print({model_line!r})
            elif binary:
                open({started!r}, "w").write(binary)
                print({service_line!r})
                if {private_model_line!r}:
                    print({private_model_line!r})
            """), seen

    def service_binary(self, address="127.0.0.1:4321", version="v1.0.0"):
        """A stand-in sysml-grpc that reports an address and exits at end of file on stdin."""
        stopped = os.path.join(self.tmp.name, "service-stopped")
        return self.executable("sysml-grpc", f"""\
            import sys
            if sys.argv[1:] == ["-version"]:
                print("sysml-grpc version {version}"); sys.exit(0)
            print({address!r}, flush=True)
            sys.stdin.read()
            open({stopped!r}, "w").write(" ".join(sys.argv[1:]))
            """), stopped

    def test_sample_without_a_service_prints_its_identity_alone(self):
        dotnet, seen = self.sample_host()
        self.assertIsNone(gates.sample(IDENTITY, dotnet, environ={"OPENSYSML_SERVICE": "elsewhere:1"}))
        self.assertFalse(os.path.exists(seen))

    def test_sample_without_a_binary_fails_when_a_service_is_required(self):
        dotnet, _ = self.sample_host()
        with self.assertRaisesRegex(gates.GateError, "OPENSYSML_REQUIRE_SERVICE is set"):
            gates.sample(IDENTITY, dotnet, environ={"OPENSYSML_REQUIRE_SERVICE": "1"})

    def test_sample_rejects_another_identity(self):
        dotnet, _ = self.sample_host(identity="OpenMBEE.OpenSysML 0.0.0 (.NETCoreApp,Version=v8.0)")
        with self.assertRaisesRegex(gates.GateError, "with no service named"):
            gates.sample(IDENTITY, dotnet, environ={})

    def test_sample_connects_to_the_service_it_starts_and_stops_it(self):
        dotnet, seen = self.sample_host()
        binary, stopped = self.service_binary()
        self.assertEqual(gates.sample(IDENTITY, dotnet, binary, environ={}), "sysml-grpc v1.0.0, 3 capabilities")
        with open(seen, encoding="utf-8") as f:
            self.assertEqual(f.read(), "127.0.0.1:4321")
        with open(stopped, encoding="utf-8") as f:
            self.assertEqual(f.read(), "-port 0 -health-port 0 -report-address -exit-with-parent")

    def test_sample_takes_the_binary_from_the_environment(self):
        dotnet, _ = self.sample_host()
        binary, _ = self.service_binary()
        self.assertIsNotNone(gates.sample(IDENTITY, dotnet, environ={"OPENSYSML_GRPC_BINARY": binary}))

    def test_sample_starts_the_binary_privately_with_no_service_named(self):
        dotnet, _ = self.sample_host()
        binary, _ = self.service_binary()
        gates.sample(IDENTITY, dotnet, binary, environ={"OPENSYSML_SERVICE": "elsewhere:1"})
        with open(os.path.join(self.tmp.name, "seen-binary"), encoding="utf-8") as f:
            self.assertEqual(f.read(), binary)

    def test_sample_rejects_a_private_run_that_does_not_reach_the_model(self):
        dotnet, _ = self.sample_host(private_model_line="")
        binary, stopped = self.service_binary()
        with self.assertRaisesRegex(gates.GateError, "starting sysml-grpc v1.0.0 privately, the sample printed"):
            gates.sample(IDENTITY, dotnet, binary, environ={})
        self.assertTrue(os.path.exists(stopped))

    def test_sample_rejects_a_service_line_of_another_version(self):
        dotnet, _ = self.sample_host(service_line="sysml-grpc v2.0.0, 3 capabilities")
        binary, stopped = self.service_binary()
        with self.assertRaisesRegex(gates.GateError, "connected to sysml-grpc v1.0.0"):
            gates.sample(IDENTITY, dotnet, binary, environ={})
        self.assertTrue(os.path.exists(stopped))

    def test_sample_rejects_no_capabilities_or_no_service_line(self):
        for line in ("sysml-grpc v1.0.0, 0 capabilities", ""):
            dotnet, _ = self.sample_host(service_line=line)
            binary, _ = self.service_binary()
            with self.assertRaisesRegex(gates.GateError, "the sample printed"):
                gates.sample(IDENTITY, dotnet, binary, environ={})

    def test_sample_rejects_another_model_line_or_none(self):
        for line in ("Demo::sedan (partUsage): mass = 1500.0", ""):
            dotnet, _ = self.sample_host(model_line=line)
            binary, _ = self.service_binary()
            with self.assertRaisesRegex(gates.GateError, "Demo::sedan \\(partUsage\\): mass = 1800.0"):
                gates.sample(IDENTITY, dotnet, binary, environ={})

    def test_sample_rejects_a_service_that_reports_no_address(self):
        dotnet, _ = self.sample_host()
        binary, stopped = self.service_binary(address="")
        with self.assertRaisesRegex(gates.GateError, "reported no address"):
            gates.sample(IDENTITY, dotnet, binary, environ={})
        self.assertTrue(os.path.exists(stopped))

    def metamodel_host(self, lines=METAMODEL_LINES, exit_code=0):
        """A stand-in dotnet whose metamodel sample records how it was run, then prints lines and exits."""
        seen = os.path.join(self.tmp.name, "seen-metamodel")
        return self.executable("metamodel-dotnet", f"""\
            import json, os, sys
            json.dump({{"argv": sys.argv[1:], "service": os.environ.get("OPENSYSML_SERVICE"),
                       "binary": os.environ.get("OPENSYSML_GRPC_BINARY")}}, open({seen!r}, "w"))
            for line in {list(lines)!r}:
                print(line)
            sys.exit({exit_code})
            """), seen

    def test_metamodel_sample_reads_the_fixture_with_no_service_named(self):
        dotnet, seen = self.metamodel_host()
        environ = {"OPENSYSML_SERVICE": "elsewhere:1", "OPENSYSML_GRPC_BINARY": "/bin/sysml-grpc"}
        self.assertEqual(gates.metamodel_sample(METAMODEL_IDENTITY, dotnet, environ=environ), list(METAMODEL_LINES))
        with open(seen, encoding="utf-8") as f:
            run = json.load(f)
        fixture = os.path.join(gates.REPO_ROOT, gates.METAMODEL_SAMPLE_INPUT)
        self.assertEqual(run["argv"], ["run", "--project", gates.METAMODEL_SAMPLE_PROJECT, "-c", "Release", "--no-build", "--", fixture])
        self.assertTrue(os.path.isfile(fixture))
        self.assertEqual((run["service"], run["binary"]), (None, None))

    def test_metamodel_sample_accepts_another_metamodel_release(self):
        lines = (METAMODEL_LINES[0], "87 elements, 1 root namespace(s), metamodel 20990101", *METAMODEL_LINES[2:])
        dotnet, _ = self.metamodel_host(lines)
        self.assertEqual(gates.metamodel_sample(METAMODEL_IDENTITY, dotnet, environ={}), list(lines))

    def test_metamodel_sample_rejects_other_output(self):
        for lines in (
            (IDENTITY, *METAMODEL_LINES[1:]),
            METAMODEL_LINES[:-1],
            (*METAMODEL_LINES, "part extra : (none)"),
            (*METAMODEL_LINES[:-1], "part v : (type not exported)"),
            (METAMODEL_LINES[0], "87 elements, 1 root namespace(s), metamodel 2025", *METAMODEL_LINES[2:]),
        ):
            with self.subTest(lines=lines):
                dotnet, _ = self.metamodel_host(lines)
                with self.assertRaisesRegex(gates.GateError, "the sample printed"):
                    gates.metamodel_sample(METAMODEL_IDENTITY, dotnet, environ={})

    def test_metamodel_sample_rejects_a_failing_run(self):
        dotnet, _ = self.metamodel_host(exit_code=2)
        with self.assertRaisesRegex(gates.GateError, "exited 2"):
            gates.metamodel_sample(METAMODEL_IDENTITY, dotnet, environ={})

    def test_main_metamodel_sample_can_report_success_on_a_legacy_windows_console(self):
        with mock.patch.object(gates, "metamodel_sample", return_value=list(METAMODEL_LINES)):
            result, stdout = self.main_on_legacy_console(["metamodel-sample", "--expect", METAMODEL_IDENTITY])
        self.assertEqual(result, 0)
        self.assertEqual(stdout, f"OK {METAMODEL_IDENTITY}: {METAMODEL_LINES[1]}\n")

    def test_sample_rejects_a_binary_that_is_not_a_file(self):
        dotnet, _ = self.sample_host()
        with self.assertRaisesRegex(gates.GateError, "is not a file"):
            gates.sample(IDENTITY, dotnet, os.path.join(self.tmp.name, "absent"), environ={})

    def test_main_reports_a_failed_gate(self):
        os.makedirs(os.path.join(self.tmp.name, "run"))
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            self.assertEqual(gates.main(["coverage", os.path.join(self.tmp.name, "run")]), 1)
        self.assertIn("holds no Cobertura report", stderr.getvalue())


def result(scenario, rpc, outcome="pass", status="OK", **extra):
    """One result of a conformance report, its members in the reference runner's order."""
    entry = {"id": scenario, "outcome": outcome, "rpc": rpc}
    if "reason" in extra:
        entry["reason"] = extra["reason"]
    entry["status"] = status
    if extra.get("without_capability"):
        entry["without_capability"] = True
    if extra.get("missing_capabilities"):
        entry["missing_capabilities"] = extra["missing_capabilities"]
    entry["duration_ms"] = extra.get("duration_ms", 1.5)
    return entry


def totals(results):
    counted = {"total": len(results), "passed": 0, "failed": 0, "skipped": 0, "errored": 0, "without_capability": 0}
    for entry in results:
        counted[{"pass": "passed", "fail": "failed", "skip": "skipped", "error": "errored"}[entry["outcome"]]] += 1
        counted["without_capability"] += 1 if entry.get("without_capability") else 0
    return counted


def conformance_report(service, runs):
    """A report of runs: [(configuration, unavailable, [(protocol, capabilities, results)])]."""
    configurations = []
    for name, unavailable, protocols in runs:
        configuration = {"name": name}
        if unavailable:
            configuration["unavailable_capabilities"] = unavailable
        rendered = []
        for protocol, capabilities, results in protocols:
            rendered.append({"protocol": protocol, "service": service, "capabilities": capabilities, **totals(results), "results": results})
        configuration.update(totals([r for _, _, results in protocols for r in results]))
        configuration["protocols"] = rendered
        configurations.append(configuration)
    every = [r for _, _, protocols in runs for _, _, results in protocols for r in results]
    return {"service": service, **totals(every), "configurations": configurations}


def conformance_log(runs, responses):
    """A runner's -v output of runs, printing responses[(configuration, protocol, id)] before its line."""
    lines = []
    for name, _, protocols in runs:
        lines += ["", f"Configuration {name}"]
        for protocol, _, results in protocols:
            for entry in results:
                response = responses.get((name, protocol, entry["id"]))
                if response is not None:
                    rendered = json.dumps(response, indent=2, sort_keys=True).split("\n")
                    lines += ["       " + line for line in rendered]
                mark = {"pass": "PASS", "fail": "FAIL", "skip": "SKIP", "error": "ERR "}[entry["outcome"]]
                lines.append(f"{mark} {entry['id']:<46} {entry['status']}")
                if "reason" in entry:
                    lines.append(f"       {entry['reason']}")
            counted = totals(results)
            lines += ["", f"[{protocol}] {counted['total']} scenarios: {counted['passed']} passed, {counted['failed']} failed, "
                      f"{counted['skipped']} skipped, {counted['errored']} in error"]
    return "\n".join(lines) + "\n"


def conformance_junit(report):
    suites = []
    dispositions = {"pass": "", "fail": "<failure/>", "skip": "<skipped/>", "error": "<error/>"}
    for configuration in report["configurations"]:
        for protocol in configuration["protocols"]:
            cases = "".join(
                f'<testcase name="{r["id"]}" classname="x" time="0">{dispositions[r["outcome"]]}</testcase>'
                for r in protocol["results"]
            )
            suites.append(
                f'<testsuite name="{configuration["name"]}/{protocol["protocol"]}" tests="{protocol["total"]}" '
                f'failures="{protocol["failed"]}" errors="{protocol["errored"]}" skipped="{protocol["skipped"]}">{cases}</testsuite>'
            )
    return (
        f'<?xml version="1.0" encoding="UTF-8"?>\n<testsuites name="conformance" tests="{report["total"]}" '
        f'failures="{report["failed"]}" errors="{report["errored"]}" skipped="{report["skipped"]}">{"".join(suites)}</testsuites>\n'
    )


SKIPS = {"uncovered_rpcs": ["Query"], "unexpressible": {"parse/no_source": "the public API always names a source"}}


class ConformanceCheckTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def runs(self, **change):
        """A default and a withholding configuration over one protocol, as both runners judged them."""
        default = [
            result("server_info/caps", "GetServerInfo"),
            result("evaluate/real", "Evaluate"),
            result("parse/no_source", "ParseFile", "skip", "-", reason="the public API always names a source"),
            result("query/oslc", "Query", "skip", "-", reason="the public API does not cover Query"),
        ]
        withheld = [
            result("parse/strict", "ParseFile", status="UNIMPLEMENTED", reason="fallback", without_capability=True,
                   missing_capabilities=["strict_conformance"]),
        ]
        runs = [("default", [], [("grpc", ["b", "a"], default)]), ("without-strict", ["strict_conformance"], [("grpc", ["b"], withheld)])]
        responses = {
            ("default", "grpc", "server_info/caps"): {"capabilities": ["b", "a"], "version": "${version}"},
            ("default", "grpc", "evaluate/real"): {"result": {"real_value": 1500.5}},
        }
        for key, value in change.items():
            if key == "responses":
                responses = value
            elif key == "runs":
                runs = value
        return runs, responses

    def write(self, name, text):
        path = os.path.join(self.tmp.name, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def check(self, ours=None, theirs=None, report=None, junit=None, skips=SKIPS):
        ours = ours or self.runs()
        theirs = theirs or self.runs()
        our_report = report or conformance_report("dotnet", ours[0])
        return gates.conformance_check(
            self.write("dotnet.json", json.dumps(our_report)),
            self.write("dotnet.log", conformance_log(*ours)),
            self.write("dotnet.xml", junit or conformance_junit(our_report)),
            self.write("reference.json", json.dumps(conformance_report("reference", theirs[0]))),
            self.write("reference.log", conformance_log(*theirs)),
            self.write("skips.json", json.dumps(skips)),
        )

    def reference_answering(self, scenario, response, **fields):
        """The reference run, with one default-configuration scenario answered otherwise."""
        runs, responses = self.runs()
        default = runs[0][2][0][2]
        default[[r["id"] for r in default].index(scenario)] = result(scenario, fields.pop("rpc", "ParseFile"), **fields)
        responses = dict(responses)
        if response is not None:
            responses[("default", "grpc", scenario)] = response
        return runs, responses

    def test_agreeing_runs_pass_and_exclude_only_the_reviewed_skips(self):
        self.assertEqual(self.check(), (3, 2))

    def test_timing_and_what_answered_are_not_compared(self):
        runs, responses = self.runs()
        runs[0][2][0][2][1]["duration_ms"] = 99.0
        self.assertEqual(self.check(theirs=(runs, responses)), (3, 2))

    def sweeping(self, response, rpc="RunSweep"):
        """Runs whose default configuration also answers one scenario of rpc with response."""
        runs, responses = self.runs()
        runs[0][2][0][2].append(result("run_sweep/rows", rpc))
        responses = dict(responses)
        responses[("default", "grpc", "run_sweep/rows")] = response
        return runs, responses

    def test_the_wall_time_a_sweep_row_reports_is_not_compared(self):
        row = {"inputs": [{"name": "b", "value": {"real_value": 0.0}}], "elapsed_micros": "220"}
        ours = self.sweeping({"rows": [row, dict(row, elapsed_micros="17")]})
        theirs = self.sweeping({"rows": [dict(row, elapsed_micros="371"), {"inputs": row["inputs"]}]})
        self.assertEqual(self.check(ours=ours, theirs=theirs), (4, 2))

    def test_every_other_field_of_a_sweep_row_is_compared(self):
        row = {"inputs": [{"name": "b", "value": {"real_value": 0.0}}], "elapsed_micros": "220"}
        ours = self.sweeping({"rows": [dict(row, inputs=[{"name": "b", "value": {"real_value": 2.0}}])]})
        with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response rows.0.inputs.0.value.real_value: 2.0, the reference 0.0"):
            self.check(ours=ours, theirs=self.sweeping({"rows": [row]}))

    def test_the_wall_time_a_row_of_runs_or_of_a_recording_reports_is_not_compared(self):
        row = {"inputs": [{"name": "run", "value": {"int_value": 1}}], "elapsed_micros": "220"}
        ours = self.sweeping({"rows": [row]}, rpc="RunRuns")
        theirs = self.sweeping({"rows": [dict(row, elapsed_micros="9")]}, rpc="RunRuns")
        self.assertEqual(self.check(ours=ours, theirs=theirs), (4, 2))
        for part in ("sweep", "sample"):
            ours = self.sweeping({part: {"rows": [row]}, "package": "Records"}, rpc="RecordRun")
            theirs = self.sweeping({part: {"rows": [dict(row, elapsed_micros="9")]}, "package": "Records"}, rpc="RecordRun")
            self.assertEqual(self.check(ours=ours, theirs=theirs), (4, 2))

    def test_the_wall_time_a_row_of_a_comparison_reports_is_not_compared_and_the_rest_of_it_is(self):
        row = {"inputs": [{"name": "run", "value": {"int_value": 1}}], "elapsed_micros": "220"}
        ours = self.sweeping({"configurations": [{"name": "C", "rows": [row]}, {"name": "D"}]}, rpc="CompareResults")
        theirs = self.sweeping({"configurations": [{"name": "C", "rows": [dict(row, elapsed_micros="9")]}, {"name": "D"}]}, rpc="CompareResults")
        self.assertEqual(self.check(ours=ours, theirs=theirs), (4, 2))
        ours = self.sweeping({"configurations": [{"name": "C", "rows": [dict(row, inputs=[])]}]}, rpc="CompareResults")
        with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response configurations.0.rows.0.inputs: "):
            self.check(ours=ours, theirs=self.sweeping({"configurations": [{"name": "C", "rows": [row]}]}, rpc="CompareResults"))

    def recording(self, at, model_hash="9f2c", extra=""):
        source = f'package Records {{\n    @AnalysisRecords::RecordedRun {{\n        runAt = "{at}";\n    }}\n{extra}}}\n'
        return {"source": source, "documents": [{"name": "Records.sysml", "content": source}], "content": source, "model_hash": model_hash}

    def test_the_time_a_recording_states_and_the_hash_following_from_it_are_not_compared(self):
        ours = self.sweeping(self.recording("2026-10-23T20:34:15Z", "9f2c"), rpc="RecordRun")
        theirs = self.sweeping(self.recording("2026-10-23T20:34:16Z", "41aa"), rpc="RecordRun")
        self.assertEqual(self.check(ours=ours, theirs=theirs), (4, 2))

    def test_the_rest_of_a_recordings_text_and_the_hashs_presence_are_compared(self):
        theirs = self.sweeping(self.recording("2026-10-23T20:34:16Z"), rpc="RecordRun")
        ours = self.sweeping(self.recording("2026-10-23T20:34:15Z", extra="    part extra;\n"), rpc="RecordRun")
        with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response content: "):
            self.check(ours=ours, theirs=theirs)
        for model_hash in ("", "${model_hash}"):
            ours = self.sweeping(self.recording("2026-10-23T20:34:16Z", model_hash), rpc="RecordRun")
            with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response model_hash: "):
                self.check(ours=ours, theirs=theirs)
        ours = self.sweeping(dict(self.recording("2026-10-23T20:34:16Z"), source='runAt = "2026-10-23";'), rpc="RecordRun")
        with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response source: "):
            self.check(ours=ours, theirs=theirs)

    def test_a_recorded_time_is_compared_outside_a_recording(self):
        ours = self.sweeping({"source": 'runAt = "2026-10-23T20:34:15Z";'}, rpc="Evaluate")
        theirs = self.sweeping({"source": 'runAt = "2026-10-23T20:34:16Z";'}, rpc="Evaluate")
        with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response source: "):
            self.check(ours=ours, theirs=theirs)

    def test_a_field_named_for_a_wall_time_is_compared_outside_a_sweep(self):
        ours = self.sweeping({"rows": [{"elapsed_micros": "1"}]}, rpc="Evaluate")
        theirs = self.sweeping({"rows": [{"elapsed_micros": "2"}]}, rpc="Evaluate")
        with self.assertRaisesRegex(gates.GateError, r"run_sweep/rows: response rows.0.elapsed_micros: '1', the reference '2'"):
            self.check(ours=ours, theirs=theirs)

    def test_capabilities_compare_as_a_set(self):
        runs, responses = self.runs()
        responses = dict(responses)
        responses[("default", "grpc", "server_info/caps")] = {"capabilities": ["a", "b"], "version": "${version}"}
        self.assertEqual(self.check(ours=(runs, responses)), (3, 2))
        responses[("default", "grpc", "server_info/caps")] = {"capabilities": ["a"], "version": "${version}"}
        with self.assertRaisesRegex(gates.GateError, r"server_info/caps: response capabilities: 1 entries, the reference 2"):
            self.check(ours=(runs, responses))

    def test_a_different_response_fails(self):
        runs, responses = self.runs()
        responses = dict(responses)
        responses[("default", "grpc", "evaluate/real")] = {"result": {"real_value": 1500.0}}
        with self.assertRaisesRegex(gates.GateError, r"evaluate/real: response result.real_value: 1500.0, the reference 1500.5"):
            self.check(ours=(runs, responses))

    def test_a_response_only_one_run_printed_fails(self):
        runs, responses = self.runs()
        responses = {key: value for key, value in responses.items() if key[2] != "evaluate/real"}
        with self.assertRaisesRegex(gates.GateError, r"evaluate/real: response : None, the reference"):
            self.check(ours=(runs, responses))

    def test_a_different_outcome_or_status_fails(self):
        with self.assertRaisesRegex(gates.GateError, r"evaluate/real: outcome 'pass', the reference 'fail'"):
            self.check(theirs=self.reference_answering("evaluate/real", {"result": {"real_value": 1500.5}}, rpc="Evaluate", outcome="fail"))
        with self.assertRaisesRegex(gates.GateError, r"evaluate/real: status 'OK', the reference 'NOT_FOUND'"):
            self.check(theirs=self.reference_answering("evaluate/real", None, rpc="Evaluate", status="NOT_FOUND"))

    def test_a_different_capability_verdict_fails(self):
        runs, responses = self.runs()
        runs[1][2][0][2][0]["missing_capabilities"] = ["oslc_query"]
        with self.assertRaisesRegex(gates.GateError, r"parse/strict: missing_capabilities"):
            self.check(theirs=(runs, responses))

    def test_an_unlisted_skip_is_compared_not_excluded(self):
        runs, responses = self.runs()
        runs[0][2][0][2][1] = result("evaluate/real", "Evaluate", "skip", "-", reason="something else")
        with self.assertRaisesRegex(gates.GateError, r"evaluate/real: outcome 'skip', the reference 'pass'"):
            self.check(ours=(runs, {k: v for k, v in responses.items() if k[2] != "evaluate/real"}))

    def test_a_listed_skip_for_another_reason_is_compared(self):
        skips = {"uncovered_rpcs": ["Query"], "unexpressible": {"parse/no_source": "another reason"}}
        with self.assertRaisesRegex(gates.GateError, r"parse/no_source: status '-', the reference 'INVALID_ARGUMENT'"):
            self.check(theirs=self.reference_answering("parse/no_source", {"error": "x"}, status="INVALID_ARGUMENT", outcome="skip", reason="r"), skips=skips)

    def test_a_report_whose_totals_do_not_add_up_fails(self):
        report = conformance_report("dotnet", self.runs()[0])
        report["passed"] += 1
        with self.assertRaisesRegex(gates.GateError, r"passed is \d+, but its parts add up to"):
            self.check(report=report)
        report = conformance_report("dotnet", self.runs()[0])
        report["configurations"][0]["protocols"][0]["skipped"] = 0
        with self.assertRaisesRegex(gates.GateError, r"protocol 0: skipped is 0, but its parts add up to 2"):
            self.check(report=report)

    def test_a_report_of_another_shape_fails(self):
        report = conformance_report("dotnet", self.runs()[0])
        report["configurations"][0]["protocols"][0]["results"][0]["observed"] = {}
        with self.assertRaisesRegex(gates.GateError, r"unknown member 'observed'"):
            self.check(report=report)
        report = conformance_report("dotnet", self.runs()[0])
        report = {"total": report["total"], **report}
        with self.assertRaisesRegex(gates.GateError, r"orders its members"):
            self.check(report=report)
        report = conformance_report("dotnet", self.runs()[0])
        del report["configurations"][1]["protocols"][0]["results"][0]["status"]
        with self.assertRaisesRegex(gates.GateError, r"has no 'status'"):
            self.check(report=report)

    def test_a_junit_report_disagreeing_with_the_json_fails(self):
        report = conformance_report("dotnet", self.runs()[0])
        junit = conformance_junit(report)
        with self.assertRaisesRegex(gates.GateError, r"skipped='0', but the report counts 2"):
            self.check(junit=junit.replace('skipped="2"', 'skipped="0"', 1))
        with self.assertRaisesRegex(gates.GateError, r"cases are not the report's scenarios"):
            self.check(junit=junit.replace("evaluate/real", "evaluate/other"))
        start = junit.index('<testsuite name="without-strict/grpc"')
        end = junit.index("</testsuite>", start) + len("</testsuite>")
        with self.assertRaisesRegex(gates.GateError, r"has 1 suites, but the report runs 2"):
            self.check(junit=junit[:start] + junit[end:])

    def test_junit_case_outcomes_agree_for_all_results(self):
        results = [result(f"evaluate/{outcome}", "Evaluate", outcome=outcome) for outcome in ("pass", "fail", "skip", "error")]
        report = conformance_report("dotnet", [("default", [], [("grpc", [], results)])])

        gates.check_conformance_junit(self.write("outcomes.xml", conformance_junit(report)), report)

    def test_junit_missing_wrong_or_duplicate_outcomes_fail(self):
        expected = {"pass": [], "fail": ["failure"], "skip": ["skipped"], "error": ["error"]}
        corruptions = [
            [], ["failure"], ["skipped"], ["error"],
            ["failure", "failure"], ["skipped", "skipped"], ["error", "error"],
            ["failure", "error"], ["skipped", "failure"], ["error", "skipped"],
        ]
        for outcome, want in expected.items():
            report = conformance_report("dotnet", [("default", [], [("grpc", [], [result("evaluate/a", "Evaluate", outcome=outcome)])])])
            for elements in corruptions:
                if elements == want:
                    continue
                with self.subTest(outcome=outcome, elements=elements):
                    root = ET.fromstring(conformance_junit(report))
                    case = root.find("./testsuite/testcase")
                    for child in list(case):
                        case.remove(child)
                    for tag in elements:
                        ET.SubElement(case, tag)
                    with self.assertRaisesRegex(gates.GateError, r"case evaluate/a: outcome elements .* want"):
                        gates.check_conformance_junit(self.write("corrupt.xml", ET.tostring(root, encoding="unicode")), report)

    def test_junit_swapped_outcomes_fail_with_unchanged_totals(self):
        report = conformance_report("dotnet", self.runs()[0])
        root = ET.fromstring(conformance_junit(report))
        passed, _, skipped, _ = root.find("testsuite").findall("testcase")
        passed.append(skipped.find("skipped"))
        skipped.remove(skipped.find("skipped"))

        with self.assertRaisesRegex(gates.GateError, r"case server_info/caps: outcome elements .* want"):
            self.check(junit=ET.tostring(root, encoding="unicode"))

    def test_a_junit_report_with_all_skips_removed_fails(self):
        report = conformance_report("dotnet", self.runs()[0])
        junit = conformance_junit(report).replace("<skipped/>", "")

        with self.assertRaisesRegex(gates.GateError, r"case parse/no_source: outcome elements .* want"):
            self.check(junit=junit)

    def test_different_configurations_fail(self):
        runs, responses = self.runs()
        with self.assertRaisesRegex(gates.GateError, r"the runs test configurations"):
            self.check(theirs=(runs[:1], responses))

    def test_a_log_without_a_scenario_fails(self):
        runs, responses = self.runs()
        log = conformance_log(runs, responses).replace("PASS evaluate/real", "PASS evaluate/elsewhere")
        report = conformance_report("dotnet", runs)
        with self.assertRaisesRegex(gates.GateError, r"evaluate/real: a log has no line for it"):
            gates.conformance_check(
                self.write("d.json", json.dumps(report)),
                self.write("d.log", log),
                self.write("d.xml", conformance_junit(report)),
                self.write("r.json", json.dumps(conformance_report("reference", runs))),
                self.write("r.log", conformance_log(runs, responses)),
                self.write("s.json", json.dumps(SKIPS)),
            )

    def test_main_reports_the_check(self):
        runs, responses = self.runs()
        report = conformance_report("dotnet", runs)
        argv = [
            "conformance-check",
            "--report", self.write("d.json", json.dumps(report)),
            "--log", self.write("d.log", conformance_log(runs, responses)),
            "--junit", self.write("d.xml", conformance_junit(report)),
            "--reference-report", self.write("r.json", json.dumps(conformance_report("reference", runs))),
            "--reference-log", self.write("r.log", conformance_log(runs, responses)),
            "--expected-skips", self.write("s.json", json.dumps(SKIPS)),
        ]
        stdout = io.StringIO()
        with contextlib.redirect_stdout(stdout):
            self.assertEqual(gates.main(argv), 0)
        self.assertEqual(stdout.getvalue(), "OK 3 results agree with the reference runner; 2 reviewed skips excluded\n")


class PromoteApiTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = self.tmp.name

    def project(self, name, shipped, unshipped):
        directory = os.path.join(self.root, name)
        os.makedirs(directory)
        for file, text in ((gates.API_SHIPPED, shipped), (gates.API_UNSHIPPED, unshipped)):
            with open(os.path.join(directory, file), "w", encoding="utf-8") as f:
                f.write(text)
        return directory

    @staticmethod
    def read(directory, file):
        with open(os.path.join(directory, file), encoding="utf-8") as f:
            return f.read()

    def test_promotes_the_unshipped_entries_in_ordinal_order_and_leaves_the_header(self):
        lib = self.project("Lib", "#nullable enable\nN.B.get -> int\n", "#nullable enable\nN.C() -> void\nN.A\n\nN.a\n")

        [(project, added, removed)] = gates.promote_api(self.root)

        self.assertTrue(project.endswith("Lib"))
        self.assertEqual((added, removed), (3, 0))
        self.assertEqual(self.read(lib, gates.API_SHIPPED), "#nullable enable\nN.A\nN.B.get -> int\nN.C() -> void\nN.a\n")
        self.assertEqual(self.read(lib, gates.API_UNSHIPPED), "#nullable enable\n")
        self.assertEqual(gates.promote_api(self.root, check=True), [(project, 0, 0)])

    def test_a_removal_deletes_the_shipped_entry_it_names(self):
        lib = self.project("Lib", "#nullable enable\nN.A\nN.B\n", "#nullable enable\n*REMOVED*N.A\nN.A(int x) -> void\n")

        self.assertEqual(gates.promote_api(self.root)[0][1:], (1, 1))

        self.assertEqual(self.read(lib, gates.API_SHIPPED), "#nullable enable\nN.A(int x) -> void\nN.B\n")

    def test_a_baseline_without_the_nullable_header_stays_without_it(self):
        lib = self.project("Lib", "", "N.A\n")

        gates.promote_api(self.root)

        self.assertEqual(self.read(lib, gates.API_SHIPPED), "N.A\n")
        self.assertEqual(self.read(lib, gates.API_UNSHIPPED), "")

    def test_check_fails_while_anything_is_unshipped_and_changes_nothing(self):
        self.project("A", "#nullable enable\nN.A\n", "#nullable enable\n")
        b = self.project("B", "#nullable enable\nN.B\n", "#nullable enable\nN.C\n*REMOVED*N.B\n")

        with self.assertRaisesRegex(gates.GateError, r"not promoted to shipped:\n  .*B: 1 unshipped and 1 removed entries\n.*promote-api") as raised:
            gates.promote_api(self.root, check=True)

        self.assertNotRegex(str(raised.exception), r"A: ")
        self.assertEqual(self.read(b, gates.API_UNSHIPPED), "#nullable enable\nN.C\n*REMOVED*N.B\n")
        self.assertEqual(self.read(b, gates.API_SHIPPED), "#nullable enable\nN.B\n")

    def test_refuses_an_inconsistent_baseline_before_writing_any_project(self):
        cases = [
            ("#nullable enable\n", "#nullable enable\nN.A\nN.A\n", r"'N.A' is unshipped twice"),
            ("#nullable enable\nN.A\n", "#nullable enable\nN.A\n", r"'N.A' is unshipped but already shipped"),
            ("#nullable enable\n", "#nullable enable\n*REMOVED*N.A\n", r"'N.A' is removed but was never shipped"),
            ("#nullable enable\n", "N.A\n", r"disagree on '#nullable enable'"),
            ("#nullable enable\n", "#nullable enable\nN.A\n#nullable enable\n", r"is a directive the baseline does not place there"),
        ]
        for shipped, unshipped, message in cases:
            with self.subTest(message=message):
                self.tmp.cleanup()
                os.makedirs(self.root)
                first = self.project("A", "#nullable enable\n", "#nullable enable\nN.Z\n")
                self.project("B", shipped, unshipped)

                with self.assertRaisesRegex(gates.GateError, message):
                    gates.promote_api(self.root)

                self.assertEqual(self.read(first, gates.API_UNSHIPPED), "#nullable enable\nN.Z\n")
                self.assertEqual(self.read(first, gates.API_SHIPPED), "#nullable enable\n")

    def test_refuses_a_root_without_a_baseline_and_a_missing_shipped_file(self):
        with self.assertRaisesRegex(gates.GateError, r"no project under .* has a PublicAPI.Unshipped.txt"):
            gates.promote_api(self.root)
        lib = self.project("Lib", "", "")
        os.remove(os.path.join(lib, gates.API_SHIPPED))
        with self.assertRaisesRegex(gates.GateError, r"cannot read .*PublicAPI.Shipped.txt"):
            gates.promote_api(self.root)

    def test_the_real_baselines_promote_to_a_shipped_set_that_keeps_every_entry(self):
        before = {}
        for unshipped in glob.glob(os.path.join(gates.API_ROOT, "*", gates.API_UNSHIPPED)):
            name = os.path.basename(os.path.dirname(unshipped))
            shipped = os.path.join(os.path.dirname(unshipped), gates.API_SHIPPED)
            self.project(name, self.read(os.path.dirname(shipped), gates.API_SHIPPED), self.read(os.path.dirname(unshipped), gates.API_UNSHIPPED))
            entries = [line for line in (self.read(os.path.dirname(shipped), gates.API_SHIPPED) + self.read(os.path.dirname(unshipped), gates.API_UNSHIPPED)).splitlines() if line.strip() and not line.startswith("#")]
            removed = {line[len(gates.API_REMOVED):] for line in entries if line.startswith(gates.API_REMOVED)}
            before[name] = {line for line in entries if not line.startswith(gates.API_REMOVED)} - removed
        self.assertIn("OpenMBEE.OpenSysML", before)

        gates.promote_api(self.root)

        for name, entries in before.items():
            shipped = self.read(os.path.join(self.root, name), gates.API_SHIPPED).splitlines()
            self.assertEqual(shipped[0], gates.API_NULLABLE)
            self.assertEqual(set(shipped[1:]), entries)
            self.assertEqual(shipped[1:], sorted(entries))
        gates.promote_api(self.root, check=True)

    def test_main_reports_each_project(self):
        self.project("Lib", "#nullable enable\n", "#nullable enable\nN.A\n")
        stdout, stderr = io.StringIO(), io.StringIO()
        with mock.patch.object(gates, "API_ROOT", self.root), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            self.assertEqual(gates.main(["promote-api", "--check"]), 1)
            self.assertEqual(gates.main(["promote-api"]), 0)
            self.assertEqual(gates.main(["promote-api", "--check"]), 0)
        self.assertRegex(stderr.getvalue(), r"\Aerror: the public API is not promoted to shipped:\n  \S*Lib: 1 unshipped and 0 removed entries\n")
        self.assertRegex(stdout.getvalue(), r"\AOK \S*Lib: shipped 1 entries, removed 0\nOK \S*Lib: nothing unshipped\n\Z")


class GoldensTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.python_hosts = python_hosts(self)
        self.calls = os.path.join(self.tmp.name, "calls.json")
        self.manifest = os.path.join(self.tmp.name, "goldens.json")
        self.write_manifest([
            {"sources": ["a.sysml"], "namespace": "A.Model", "output": "out/ATypes.cs"},
            {"sources": ["lib", "top.sysml"], "namespace": "B", "output": "out/BTypes.cs"},
        ])

    def write_manifest(self, entries):
        with open(self.manifest, "w", encoding="utf-8") as f:
            json.dump(entries, f)

    def host(self, fail_for=None):
        """A stand-in dotnet recording each command line, failing for an output it is told to."""
        path = os.path.join(self.tmp.name, "dotnet")
        with open(path, "w", encoding="utf-8") as f:
            f.write(textwrap.dedent(f"""\
                #!{sys.executable}
                import json, os, sys
                calls = json.load(open({self.calls!r})) if os.path.exists({self.calls!r}) else []
                calls.append(sys.argv[1:])
                json.dump(calls, open({self.calls!r}, "w"))
                if {fail_for!r} in sys.argv:
                    print("error: out of date", file=sys.stderr); sys.exit(1)
                """))
        os.chmod(path, os.stat(path).st_mode | stat.S_IEXEC)
        self.python_hosts.add(path)
        return path

    def recorded(self):
        with open(self.calls, encoding="utf-8") as f:
            return json.load(f)

    def test_runs_the_built_generator_with_each_goldens_arguments(self):
        done = gates.goldens(self.host(), manifest=self.manifest)

        self.assertEqual(done, ["out/ATypes.cs", "out/BTypes.cs"])
        run = ["run", "--project", gates.GENERATE_PROJECT, "-c", "Release", "--no-build", "--"]
        self.assertEqual(self.recorded(), [
            run + ["a.sysml", "-o", "out/ATypes.cs", "--namespace", "A.Model"],
            run + ["lib", "top.sysml", "-o", "out/BTypes.cs", "--namespace", "B"],
        ])

    def test_check_asks_the_generator_to_check(self):
        gates.goldens(self.host(), check=True, manifest=self.manifest)

        self.assertTrue(all(call[-1] == "--check" for call in self.recorded()))

    def test_a_golden_the_generator_refuses_fails_naming_it(self):
        with self.assertRaisesRegex(gates.GateError, r"exited 1 for out/BTypes.cs:\nerror: out of date"):
            gates.goldens(self.host(fail_for="out/BTypes.cs"), check=True, manifest=self.manifest)

    def test_a_malformed_manifest_is_refused(self):
        for entries, message in [
            ([], "names no golden"),
            ([{"sources": ["a"], "namespace": "A"}], "exactly sources, namespace and output"),
            ([{"sources": [], "namespace": "A", "output": "o"}], "sources is not a list of paths"),
            ([{"sources": ["a"], "namespace": "", "output": "o"}], "names no namespace or output"),
        ]:
            self.write_manifest(entries)
            with self.subTest(message=message), self.assertRaisesRegex(gates.GateError, message):
                gates.goldens(self.host(), manifest=self.manifest)

    def test_the_committed_manifest_names_goldens_that_exist(self):
        for sources, _, output in gates.read_goldens():
            self.assertTrue(os.path.isfile(os.path.join(gates.REPO_ROOT, output)), output)
            for source in sources:
                self.assertTrue(os.path.exists(os.path.join(gates.REPO_ROOT, source)), source)


class MakefileTest(unittest.TestCase):
    def test_test_and_coverage_require_the_schema_check_with_makes_buf_command(self):
        environment = dict(os.environ)
        for variable in ("BUF", "MAKEFLAGS", "MFLAGS", "MAKEOVERRIDES"):
            environment.pop(variable, None)
        environment["OPENSYSML_REQUIRE_BUF"] = "0"
        for target in ("dotnet-test", "dotnet-coverage"):
            for buf in (None, "buf-schema-probe --debug"):
                with self.subTest(target=target, buf=buf):
                    command = ["make", "--no-print-directory", "-n", target, "DOTNET=dotnet-schema-probe"]
                    if buf is not None:
                        command.append("BUF=" + buf)
                    result = subprocess.run(
                        command, cwd=gates.REPO_ROOT, capture_output=True, text=True, timeout=30,
                        env=environment,
                    )
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    invocations = [
                        line for line in result.stdout.splitlines()
                        if "dotnet-schema-probe test --solution " in line
                    ]
                    self.assertEqual(len(invocations), 1, result.stdout)
                    expected = buf or "go run github.com/bufbuild/buf/cmd/buf@v1.57.2"
                    self.assertTrue(
                        invocations[0].startswith(f'BUF="{expected}" OPENSYSML_REQUIRE_BUF=1 '),
                        invocations[0],
                    )


if __name__ == "__main__":
    unittest.main()
