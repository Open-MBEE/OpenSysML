"""The reference FMI runner: one fmi/1 request on standard input, one reply on standard output.

The runner is what `OPENSYSML_FMI_RUNNER` names — the `tool:fmi` engine starts one
per evaluation and speaks the protocol of docs/reference/fmi.md: a request object
carries the FMU's path, the interface to run, the experiment, the start values and
the outputs to read; a `series` member asks for a sampled trajectory. The reply is
`{"protocol": 1, "time": …, "outputs": {…}}`, with a `series` member of sample
times and values when asked, or `{"protocol": 1, "error": "…"}`. FMPy does the
simulation; it is an optional dependency, installed by `pip install opensysml[fmi]`.
"""

import json
import sys
from typing import Any

PROTOCOL = 1
MAX_SERIES_SAMPLES = 10000

# The tool:fmi request's interface name, as the fmi/1 protocol spells it, to the
# fmpy.simulate_fmu fmi_type argument.
FMI_TYPES = {
    "coSimulation": "CoSimulation",
    "modelExchange": "ModelExchange",
}


def _error(message: str) -> dict[str, Any]:
    return {"protocol": PROTOCOL, "error": message}


def _variable_types(fmpy: Any, fmu: str) -> dict[str, tuple[str, bool]]:
    """Each model variable's declared type and whether it is an array variable."""
    description = fmpy.read_model_description(fmu)
    return {
        v.name: (v.type, bool(getattr(v, "dimensions", None)))
        for v in description.modelVariables
    }


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
    series = request.get("series")
    series_outputs = list(dict.fromkeys((series or {}).get("outputs") or []))
    # A variable sampled only into a trajectory still needs an fmpy column.
    requested = list(dict.fromkeys(outputs + series_outputs))
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
            output=requested or None,
            fmi_type=fmi_type,
        )
    except Exception as e:  # noqa: BLE001  # every fmpy failure is a reply, not a crash
        return _error(str(e))
    if series is not None:
        problem = _series_problem(series)
        if problem:
            return _error(problem)
    types = _variable_types(fmpy, fmu)
    last = result[-1]
    reply: dict[str, Any] = {
        "protocol": PROTOCOL,
        "time": float(last["time"]),
        "outputs": {},
    }
    for name in outputs:
        fmi_type, is_array = types.get(name, ("", False))
        column = last[name]
        if is_array:
            # fmpy answers an array variable as one structured column whose
            # each entry is the element list, in declaration order.
            reply["outputs"][name] = [_convert(v, fmi_type) for v in column]
        else:
            reply["outputs"][name] = _convert(column, fmi_type)
    if series is not None:
        reply["series"] = _series(result, series, series_outputs, types)
    return reply


def _series_problem(series: dict[str, Any]) -> str:
    """A bad series request, or the empty string when it can be answered."""
    if not isinstance(series, dict):
        return "series must be an object"
    samples = series.get("samples")
    if not isinstance(samples, int) or isinstance(samples, bool) or samples < 2:
        return f"series samples must be an integer of at least 2, got {samples!r}"
    if samples > MAX_SERIES_SAMPLES:
        return f"series samples exceed the limit of {MAX_SERIES_SAMPLES}"
    if not isinstance(series.get("outputs"), list):
        return "series outputs must be a list"
    return ""


def _series(result: Any, series: dict[str, Any], names: list[str],
            types: dict[str, tuple[str, bool]]) -> dict[str, Any]:
    """Sample the simulation result at evenly spaced recorded instants."""
    samples = series["samples"]
    sampled = _sample_rows(result, samples)
    answer: dict[str, Any] = {
        "times": [float(row["time"]) for row in sampled],
        "outputs": {},
    }
    for name in names:
        fmi_type, is_array = types.get(name, ("", False))
        if is_array:
            answer["outputs"][name] = [
                [_convert(v, fmi_type) for v in row[name]] for row in sampled
            ]
        else:
            answer["outputs"][name] = [_convert(row[name], fmi_type) for row in sampled]
    return answer


def _sample_rows(result: Any, samples: int) -> list[Any]:
    """The recorded rows nearest to each of `samples` evenly spaced instants."""
    rows = len(result)
    return [result[min(round(i * (rows - 1) / (samples - 1)), rows - 1)]
            for i in range(samples)]


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
    except Exception as e:  # noqa: BLE001  # a reply, never a traceback, on stdout
        print(f"{sys.argv[0]}: {e}", file=sys.stderr)
        reply = _error(str(e))
    print(json.dumps(reply))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
