"""The reference FMI runner: one fmi/1 request on standard input, one reply on standard output.

The runner is what `OPENSYSML_FMI_RUNNER` names — the `tool:fmi` engine starts one
per evaluation and speaks the protocol of docs/reference/fmi.md: a request object
carries the FMU's path, the interface to run, the experiment, the start values and
the outputs to read; a `series` member asks for a sampled trajectory. The reply is
`{"protocol": 1, "time": …, "outputs": {…}}`, with a `series` member of sample
times and values when asked, or `{"protocol": 1, "error": "…"}`. FMPy does the
simulation; it is an optional dependency, installed by `pip install opensysml[fmi]`.
"""

import bisect
import json
import math
import sys
from typing import Any, NamedTuple

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


class SeriesRangeError(ValueError):
    """A requested instant lies outside the instants the simulation recorded."""

    def __init__(self, time: float, first: float, last: float) -> None:
        super().__init__(f"sample instant {time} lies outside the simulated run {first} to {last}")


class _Variable(NamedTuple):
    """The declared facts a variable brings to sampling."""

    fmi_type: str = ""
    is_array: bool = False
    continuous: bool = False


def _time_close(a: float, b: float) -> bool:
    return abs(a - b) <= 64 * max(math.ulp(a), math.ulp(b), 1e-12)


def _row_window(recorded: list[float], time: float) -> tuple[int, int]:
    """The last row at a recorded instant, else the two rows bracketing it."""
    end = bisect.bisect_right(recorded, time)
    last = end - 1
    if last >= 0 and _time_close(recorded[last], time):
        return last, last
    first = bisect.bisect_left(recorded, time)
    if first < len(recorded) and _time_close(recorded[first], time):
        # A request just below an event still reads the row recorded after it.
        while first + 1 < len(recorded) and _time_close(recorded[first + 1], time):
            first += 1
        return first, first
    return max(0, first - 1), min(len(recorded) - 1, first)


def _variable_types(fmpy: Any, fmu: str) -> dict[str, _Variable]:
    """Each variable's type, shape and whether its value varies between recorded steps."""
    description = fmpy.read_model_description(fmu)
    variables = {}
    for v in description.modelVariables:
        fmi_type = str(getattr(v, "type", "") or "")
        numeric = fmi_type == "Real" or fmi_type.startswith("Float")
        variability = str(getattr(v, "variability", "") or "").lower()
        causality = str(getattr(v, "causality", "") or "").lower()
        if variability:
            continuous = numeric and variability in ("continuous", "dependent")
        else:
            continuous = numeric and causality not in ("parameter", "calculatedparameter")
        variables[v.name] = _Variable(
            fmi_type, bool(getattr(v, "dimensions", None)), continuous
        )
    return variables


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
        variable = types.get(name, _Variable())
        column = last[name]
        if variable.is_array:
            # fmpy answers an array variable as one structured column whose
            # each entry is the element list, in declaration order.
            reply["outputs"][name] = [_convert(v, variable.fmi_type) for v in column]
        else:
            reply["outputs"][name] = _convert(column, variable.fmi_type)
    if series is not None:
        try:
            reply["series"] = _series(result, series, series_outputs, types)
        except SeriesRangeError as e:
            return _error(str(e))
    return reply


def _fields(result: Any) -> list[str]:
    """An fmpy result's column names, from its structured dtype or its dict rows."""
    names = getattr(getattr(result, "dtype", None), "names", None)
    if names:
        return list(names)
    return list(result[0].keys()) if len(result) else []


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
    times = series.get("times")
    if times is not None:
        if not isinstance(times, list) or len(times) != samples:
            return "series times must list one instant per sample"
        previous = None
        for value in times:
            if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
                return "series times must be finite numbers"
            if previous is not None and value < previous:
                return "series times must be ordered"
            previous = value
    return ""


def _series(result: Any, series: dict[str, Any], names: list[str],
            types: dict[str, _Variable]) -> dict[str, Any]:
    """Sample the result at requested instants, or evenly spaced ones."""
    samples = series["samples"]
    wanted = series.get("times")
    if wanted is None:
        wanted = _even_times(result, samples)
    times = [float(value) for value in wanted]
    sampled = _interpolate(result, times, types)
    answer: dict[str, Any] = {"times": times, "outputs": {}}
    for name in names:
        variable = types.get(name, _Variable())
        if variable.is_array:
            answer["outputs"][name] = [
                [_convert(v, variable.fmi_type) for v in row[name]] for row in sampled
            ]
        else:
            answer["outputs"][name] = [
                _convert(row[name], variable.fmi_type) for row in sampled
            ]
    return answer


def _even_times(result: Any, samples: int) -> list[float]:
    """`samples` instants spread over recorded time, never over recorded row index."""
    first, last = float(result[0]["time"]), float(result[-1]["time"])
    return [first + i * (last - first) / (samples - 1) for i in range(samples)]


def _interpolate_value(before: Any, after: Any, fraction: float,
                       variable: _Variable) -> Any:
    """One field between two recorded steps: continuous numbers blend, the rest hold."""
    if not variable.continuous:
        return before if fraction < 0.5 else after
    if variable.is_array:
        return [v + (w - v) * fraction for v, w in zip(before, after)]
    return before + (after - before) * fraction


def _interpolate(result: Any, times: list[float],
                 types: dict[str, _Variable]) -> list[Any]:
    """Rows at the requested instants; an unobserved instant is refused, not fabricated."""
    recorded = [float(row["time"]) for row in result]
    fields = _fields(result)
    first, last = recorded[0], recorded[-1]
    sampled = []
    for time in times:
        if ((time < first and not _time_close(time, first))
                or (time > last and not _time_close(time, last))):
            raise SeriesRangeError(time, first, last)
        low, high = _row_window(recorded, time)
        if low == high:
            sampled.append(result[low])
            continue
        before, after = result[low], result[high]
        fraction = (time - recorded[low]) / (recorded[high] - recorded[low])
        row = {"time": time}
        for name in fields:
            if name == "time":
                continue
            row[name] = _interpolate_value(
                before[name], after[name], fraction, types.get(name, _Variable())
            )
        sampled.append(row)
    return sampled


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
