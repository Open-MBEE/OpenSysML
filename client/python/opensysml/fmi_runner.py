"""The reference FMI runner: one fmi/1 request on standard input, one reply on standard output.

The runner is what `OPENSYSML_FMI_RUNNER` names — the `tool:fmi` engine starts one
per evaluation and speaks the protocol of docs/reference/fmi.md: a request object
carries the FMU's path, the interface to run, the experiment, the start values and
the outputs to read; the reply is `{"protocol": 1, "time": …, "outputs": {…}}` or
`{"protocol": 1, "error": "…"}`. FMPy does the simulation; it is an optional
dependency, installed by `pip install opensysml[fmi]`.
"""

import json
import sys
from typing import Any

PROTOCOL = 1

# The tool:fmi request's interface name, as the fmi/1 protocol spells it, to the
# fmpy.simulate_fmu fmi_type argument.
FMI_TYPES = {
    "coSimulation": "CoSimulation",
    "modelExchange": "ModelExchange",
}


def _error(message: str) -> dict[str, Any]:
    return {"protocol": PROTOCOL, "error": message}


def _variable_types(fmpy: Any, fmu: str) -> dict[str, str]:
    """The declared type of each model variable, to convert the result's scalars."""
    description = fmpy.read_model_description(fmu)
    return {v.name: v.type for v in description.modelVariables}


def _convert(value: Any, fmi_type: str) -> Any:
    """A numpy scalar or string from the result, as the JSON type its FMI type reads."""
    if hasattr(value, "item"):
        value = value.item()
    if fmi_type == "Boolean":
        return bool(value)
    if fmi_type.startswith("Int") or fmi_type.startswith("UInt") or fmi_type in (
        "Integer",
        "Enumeration",
    ):
        return int(value)
    if fmi_type.startswith("Float") or fmi_type == "Real":
        return float(value)
    if fmi_type == "String":
        return str(value)
    return value


def run(request: dict[str, Any]) -> dict[str, Any]:
    """Answer one request object with its reply object."""
    protocol = request.get("protocol")
    if protocol != PROTOCOL:
        return _error(f"protocol {protocol!r} is not served; the runner speaks 1")
    interface = request.get("interface") or ""
    fmi_type = FMI_TYPES.get(interface)
    if interface == "scheduledExecution":
        return _error("scheduled execution is not served by this runner")
    if fmi_type is None:
        return _error(f"interface {interface!r} is not served by this runner")
    try:
        import fmpy
    except ImportError:
        return _error(
            "fmpy is not installed; the reference runner needs `pip install opensysml[fmi]`"
        )
    experiment = request.get("experiment") or {}
    outputs = request.get("outputs") or []
    fmu = request.get("fmu") or ""
    try:
        result = fmpy.simulate_fmu(
            fmu,
            validate=False,
            start_time=experiment.get("startTime"),
            stop_time=experiment.get("stopTime"),
            step_size=experiment.get("stepSize") or None,
            relative_tolerance=experiment.get("tolerance") or None,
            start_values=request.get("start") or {},
            output=outputs or None,
            fmi_type=fmi_type,
        )
    except Exception as e:  # noqa: BLE001 — every fmpy failure is a reply, not a crash
        return _error(str(e))
    types = _variable_types(fmpy, fmu)
    last = result[-1]
    reply: dict[str, Any] = {
        "protocol": PROTOCOL,
        "time": float(last["time"]),
        "outputs": {},
    }
    for name in outputs:
        reply["outputs"][name] = _convert(last[name], types.get(name, ""))
    return reply


def main() -> int:
    """Read one JSON object from stdin and write one JSON object to stdout."""
    sys.tracebacklimit = 0
    try:
        request = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError) as e:
        print(json.dumps(_error(f"the request is not a JSON object: {e}")))
        return 2
    if not isinstance(request, dict):
        print(json.dumps(_error("the request is not a JSON object")))
        return 2
    try:
        reply = run(request)
    except Exception as e:  # noqa: BLE001 — a reply, never a traceback, on stdout
        print(f"{sys.argv[0]}: {e}", file=sys.stderr)
        reply = _error(str(e))
    print(json.dumps(reply))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
