"""Tests for opensysml.fmi_runner: the fmi/1 request/reply protocol over FMPy."""

import io
import json
import os
import sys
import types
from pathlib import Path

import pytest

from opensysml import fmi_runner


def request(**overrides):
    """A well-formed fmi/1 request the tests vary from."""
    req = {
        "protocol": 1,
        "fmu": "/abs/model.fmu",
        "interface": "coSimulation",
        "experiment": {"startTime": 0.0, "stopTime": 3.0, "stepSize": 0.01},
        "start": {"e": 0.7},
        "outputs": ["h", "v", "ok"],
    }
    req.update(overrides)
    return req


def fake_fmpy(rows, variables, error=None, record=None):
    """An fmpy stand-in: variables maps a name to a type or a
    (type, dimensions[, variability[, causality]]) tuple; error fails the call."""
    module = types.ModuleType("fmpy")

    def simulate_fmu(fmu, **kwargs):
        if record is not None:
            record["fmu"] = fmu
            record.update(kwargs)
        if error is not None:
            raise error
        return rows

    def read_model_description(fmu):
        variables_of = []
        for name, spec in variables.items():
            if isinstance(spec, str):
                fmi_type, dimensions, variability, causality = spec, [], "", ""
            else:
                fmi_type, dimensions = spec[0], spec[1]
                variability = spec[2] if len(spec) > 2 else ""
                causality = spec[3] if len(spec) > 3 else ""
            variables_of.append(
                types.SimpleNamespace(
                    name=name, type=fmi_type, dimensions=dimensions,
                    variability=variability, causality=causality,
                )
            )
        return types.SimpleNamespace(modelVariables=variables_of)

    module.simulate_fmu = simulate_fmu
    module.read_model_description = read_model_description
    return module


def test_protocol_mismatch_is_an_error_reply():
    reply = fmi_runner.run(request(protocol=0))
    assert reply["protocol"] == 1
    assert "not served" in reply["error"]
    assert "speaks 1" in reply["error"]


def test_missing_fmpy_names_the_extra(mocker):
    mocker.patch.dict(sys.modules, {"fmpy": None})
    reply = fmi_runner.run(request())
    assert reply["protocol"] == 1
    assert "opensysml[fmi]" in reply["error"]


def test_scheduled_execution_is_refused():
    reply = fmi_runner.run(request(interface="scheduledExecution"))
    assert reply["error"] == "scheduled execution is not served by this runner"


def test_fmpy_failure_is_an_error_reply(mocker):
    mocker.patch.dict(
        sys.modules, {"fmpy": fake_fmpy([], {}, error=RuntimeError("FMU failed to initialize"))}
    )
    reply = fmi_runner.run(request())
    assert reply["protocol"] == 1
    assert "FMU failed to initialize" in reply["error"]


def test_request_maps_to_simulate_fmu_kwargs(mocker):
    record = {}
    rows = [{"time": 3.0, "h": 0.5, "v": -1.0, "ok": True}]
    variables = {"h": "Real", "v": "Real", "ok": "Boolean"}
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, variables, record=record)})
    fmi_runner.run(
        request(
            interface="modelExchange",
            experiment={"startTime": 0.0, "stopTime": 3.0, "tolerance": 1e-4},
        )
    )
    assert record["fmu"] == "/abs/model.fmu"
    assert record["validate"] is False
    assert record["start_time"] == 0.0
    assert record["stop_time"] == 3.0
    assert record["step_size"] is None
    assert record["relative_tolerance"] == 1e-4
    assert record["start_values"] == {"e": 0.7}
    assert record["output"] == ["h", "v", "ok"]
    assert record["fmi_type"] == "ModelExchange"


def test_last_row_and_type_conversion(mocker):
    rows = [
        {"time": 0.0, "h": 1.0, "v": 0.0, "ok": True},
        {"time": 3.0, "h": 0.25, "v": -29.4, "ok": False},
    ]
    variables = {"h": "Float64", "v": "Real", "ok": "Boolean"}
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, variables)})
    reply = fmi_runner.run(request())
    assert reply["protocol"] == 1
    assert reply["time"] == 3.0
    assert reply["outputs"] == {"h": 0.25, "v": -29.4, "ok": False}
    assert isinstance(reply["outputs"]["h"], float)
    assert isinstance(reply["outputs"]["ok"], bool)


def test_no_outputs_requests_none(mocker):
    record = {}
    mocker.patch.dict(
        sys.modules, {"fmpy": fake_fmpy([{"time": 1.0}], {}, record=record)}
    )
    reply = fmi_runner.run(request(outputs=[]))
    assert record["output"] is None
    assert reply["outputs"] == {}


def test_array_outputs_are_lists(mocker):
    """An array variable's structured column answers as a list in declaration order."""
    rows = [{"time": 1.0, "y": [2.7, 5.4, 8.1]}]
    variables = {"y": ("Float64", [object()])}
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, variables)})
    reply = fmi_runner.run(request(outputs=["y"]))
    assert reply["outputs"]["y"] == [2.7, 5.4, 8.1]


def test_series_answers_evenly_spaced_samples(mocker):
    """A series request samples the trajectory, not just the last row."""
    rows = [
        {"time": float(t), "h": float(t * t), "ok": t == 4}
        for t in range(5)
    ]
    variables = {"h": "Real", "ok": "Boolean"}
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, variables)})
    reply = fmi_runner.run(
        request(
            outputs=["h"],
            series={"samples": 3, "outputs": ["h", "ok"]},
        )
    )
    assert "error" not in reply
    assert reply["outputs"]["h"] == 16.0
    assert reply["series"]["times"] == [0.0, 2.0, 4.0]
    assert reply["series"]["outputs"] == {"h": [0.0, 4.0, 16.0], "ok": [False, False, True]}


def test_series_only_variables_are_requested_from_fmpy(mocker):
    """A variable sampled only into a trajectory still gets its result column."""
    record = {}
    rows = [{"time": float(t), "v": float(t)} for t in range(3)]
    mocker.patch.dict(
        sys.modules,
        {"fmpy": fake_fmpy(rows, {"v": "Real"}, record=record)},
    )
    reply = fmi_runner.run(
        request(outputs=[], series={"samples": 3, "outputs": ["v"]})
    )
    assert record["output"] == ["v"]
    assert reply["outputs"] == {}
    assert reply["series"]["outputs"]["v"] == [0.0, 1.0, 2.0]


def test_requested_sample_instants_are_honoured(mocker):
    """An interval whose spacing does not divide the experiment is sampled at the requested instants."""
    rows = [{"time": float(t), "h": float(t)} for t in range(4)]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"h": "Real"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 2, "times": [0.0, 2.0], "outputs": ["h"]},
    ))
    assert "error" not in reply
    assert reply["series"]["times"] == [0.0, 2.0]
    assert reply["series"]["outputs"]["h"] == [0.0, 2.0]


def test_requested_instants_interpolate_between_steps(mocker):
    """An instant between recorded steps gets an interpolated value at that instant."""
    rows = [{"time": float(t), "h": float(t)} for t in range(4)]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"h": "Real"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 3, "times": [0.0, 1.5, 3.0], "outputs": ["h"]},
    ))
    assert reply["series"]["times"] == [0.0, 1.5, 3.0]
    assert reply["series"]["outputs"]["h"] == [0.0, 1.5, 3.0]


def test_even_samples_follow_time_not_row_index(mocker):
    """Extra solver-event rows between grid points must not move the sample instants."""
    rows = [
        {"time": 0.0, "h": 0.0},
        {"time": 0.1, "h": 99.0},  # a solver event, not a grid point
        {"time": 0.2, "h": 2.0},
        {"time": 0.5, "h": 5.0},
        {"time": 1.0, "h": 10.0},
    ]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"h": "Real"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 3, "outputs": ["h"]},
        experiment={"startTime": 0.0, "stopTime": 1.0},
    ))
    assert "error" not in reply
    assert reply["series"]["times"] == [0.0, 0.5, 1.0]
    assert reply["series"]["outputs"]["h"] == [0.0, 5.0, 10.0]


def test_requested_instant_past_the_simulated_run_is_refused(mocker):
    """An FMU that stopped early cannot have its last value relabelled at a later instant."""
    rows = [{"time": 0.0, "h": 0.0}, {"time": 1.0, "h": 1.0}, {"time": 2.0, "h": 4.0}]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"h": "Real"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 3, "times": [0.0, 1.5, 3.0], "outputs": ["h"]},
        experiment={"startTime": 0.0, "stopTime": 3.0},
    ))
    assert reply["protocol"] == 1
    assert "error" in reply
    assert "outside the simulated run" in reply["error"]


def test_requested_instant_before_the_simulated_run_is_refused(mocker):
    """An instant before the first recorded row is unobserved too."""
    rows = [{"time": 1.0, "h": 1.0}, {"time": 2.0, "h": 4.0}]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"h": "Real"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 2, "times": [0.5, 1.5], "outputs": ["h"]},
        experiment={"startTime": 0.0, "stopTime": 2.0},
    ))
    assert "error" in reply
    assert "outside the simulated run" in reply["error"]


def test_structured_fmpy_records_sample_exact_and_interior_instants(mocker):
    """A real numpy result carries named fields, not dicts; exact and interpolated instants both work."""
    np = pytest.importorskip("numpy")
    rows = np.array(
        [(0.0, 0.0, False), (1.0, 2.0, False), (2.0, 4.0, True)],
        dtype=[("time", "f8"), ("h", "f8"), ("ok", "?")],
    )
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"h": "Float64", "ok": "Boolean"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 4, "times": [0.0, 1.0, 1.25, 2.0], "outputs": ["h", "ok"]},
    ))
    assert "error" not in reply
    assert reply["series"]["times"] == [0.0, 1.0, 1.25, 2.0]
    assert reply["series"]["outputs"]["h"] == [0.0, 2.0, 2.5, 4.0]
    assert reply["series"]["outputs"]["ok"] == [False, False, False, True]


def test_structured_array_column_interpolates_elements(mocker):
    """An array column carried by a structured result interpolates element by element."""
    np = pytest.importorskip("numpy")
    rows = np.array(
        [(0.0, [0.0, 1.0]), (2.0, [2.0, 3.0])],
        dtype=[("time", "f8"), ("y", "O")],
    )
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"y": ("Float64", [object()])})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 3, "times": [0.0, 1.0, 2.0], "outputs": ["y"]},
    ))
    assert "error" not in reply
    assert reply["series"]["outputs"]["y"] == [[0.0, 1.0], [1.0, 2.0], [2.0, 3.0]]


def test_discrete_variables_are_not_interpolated(mocker):
    """An Integer between recorded steps stays a recorded value, never an average."""
    rows = [{"time": 0.0, "n": 2}, {"time": 2.0, "n": 4}]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"n": "Integer"})})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 3, "times": [0.0, 1.5, 2.0], "outputs": ["n"]},
    ))
    assert "error" not in reply
    assert reply["series"]["outputs"]["n"] == [2, 4, 4]


def test_declared_discrete_real_holds_a_recorded_value(mocker):
    """A discrete Real holds a recorded value while a continuous Float still blends."""
    rows = [
        {"time": 0.0, "d": 2.0, "c": 0.0},
        {"time": 2.0, "d": 5.0, "c": 6.0},
    ]
    variables = {"d": ("Real", [], "discrete"), "c": "Float64"}
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, variables)})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 3, "times": [0.75, 1.5, 2.0], "outputs": ["d", "c"]},
    ))
    assert "error" not in reply
    assert reply["series"]["outputs"]["d"] == [2.0, 5.0, 5.0]
    assert reply["series"]["outputs"]["c"] == [2.25, 4.5, 6.0]


def test_real_parameter_is_not_interpolated(mocker):
    """A Real parameter without declared variability holds its recorded value."""
    rows = [{"time": 0.0, "p": 1.0}, {"time": 2.0, "p": 3.0}]
    variables = {"p": ("Real", [], "", "parameter")}
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, variables)})
    reply = fmi_runner.run(request(
        outputs=[],
        series={"samples": 2, "times": [0.5, 2.0], "outputs": ["p"]},
    ))
    assert "error" not in reply
    assert reply["series"]["outputs"]["p"] == [1.0, 3.0]


def test_final_event_is_the_last_row_at_stop_time(mocker):
    """A sample at a recorded instant reads the post-event row, as the scalar does."""
    rows = [
        {"time": 0.0, "y": 0.0},
        {"time": 1.0, "y": 3.0},
        {"time": 1.0, "y": 9.0},
    ]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"y": "Real"})})
    reply = fmi_runner.run(request(
        experiment={"startTime": 0.0, "stopTime": 1.0},
        outputs=["y"],
        series={"samples": 3, "times": [0.0, 0.5, 1.0], "outputs": ["y"]},
    ))
    assert "error" not in reply
    assert reply["outputs"]["y"] == 9.0
    assert reply["series"]["outputs"]["y"] == [0.0, 1.5, 9.0]


def test_near_event_sample_within_tolerance_reads_the_last_row(mocker):
    """A request just below a recorded event still reads its post-event row."""
    rows = [
        {"time": 0.0, "y": 0.0},
        {"time": 1.0, "y": 3.0},
        {"time": 1.0, "y": 9.0},
    ]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"y": "Real"})})
    reply = fmi_runner.run(request(
        experiment={"startTime": 0.0, "stopTime": 1.0},
        outputs=[],
        series={"samples": 2, "times": [1.0 - 1e-15, 1.0], "outputs": ["y"]},
    ))
    assert "error" not in reply
    assert reply["series"]["outputs"]["y"] == [9.0, 9.0]


def test_close_distinct_events_are_not_grouped(mocker):
    """A request just below one event must not read a later event inside the tolerance."""
    event, later = 1e-9, 1.05e-9
    rows = [
        {"time": 0.0, "y": 0.0},
        {"time": event, "y": 1.0},
        {"time": event, "y": 4.0},
        {"time": later, "y": 9.0},
    ]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {"y": "Real"})})
    reply = fmi_runner.run(request(
        experiment={"startTime": 0.0, "stopTime": later},
        outputs=[],
        series={"samples": 2, "times": [event - 1e-15, later], "outputs": ["y"]},
    ))
    assert "error" not in reply
    assert reply["series"]["outputs"]["y"] == [4.0, 9.0]


def test_bad_series_requests_are_error_replies(mocker):
    """A sample count that is not an integer of at least two is refused."""
    rows = [{"time": 0.0}]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {})})
    for samples in (1, 0, 2.5, None, fmi_runner.MAX_SERIES_SAMPLES + 1):
        reply = fmi_runner.run(request(series={"samples": samples, "outputs": []}))
        assert "error" in reply
        assert "series samples" in reply["error"]
    for times in ([0.0], [2.0, 0.0], [0.0, float("nan")]):
        reply = fmi_runner.run(request(
            series={"samples": 2, "times": times, "outputs": []}
        ))
        assert "error" in reply
        assert "series times" in reply["error"]


def test_main_writes_the_reply_and_exits_zero(mocker, capsys):
    rows = [{"time": 0.0}]
    mocker.patch.dict(sys.modules, {"fmpy": fake_fmpy(rows, {})})
    mocker.patch("sys.stdin", io.StringIO(json.dumps(request(outputs=[]))))
    assert fmi_runner.main() == 0
    reply = json.loads(capsys.readouterr().out)
    assert reply["protocol"] == 1
    assert reply["time"] == 0.0


def test_main_replies_an_error_on_non_json(mocker, capsys):
    mocker.patch("sys.stdin", io.StringIO("not json"))
    assert fmi_runner.main() == 2
    reply = json.loads(capsys.readouterr().out)
    assert reply["protocol"] == 1
    assert "not a JSON object" in reply["error"]


BOUNCING_BALL = (
    Path(__file__).resolve().parents[3] / "examples" / "reference-fmus" / "2.0" / "BouncingBall.fmu"
)


def has_fmpy():
    try:
        import fmpy  # noqa: F401

        return True
    except ImportError:
        return False


def test_bouncing_ball_runs_for_real():
    """Simulates the Reference-FMUs BouncingBall when the FMU and fmpy are present."""
    missing = []
    if not BOUNCING_BALL.exists():
        missing.append(f"{BOUNCING_BALL} (run ./scripts/download-reference-fmus.sh)")
    if not has_fmpy():
        missing.append("fmpy (pip install opensysml[fmi])")
    if missing:
        if os.environ.get("OPENSYSML_REQUIRE_REFERENCE_FMUS") == "1":
            pytest.fail("the Reference-FMUs gate is required but missing: " + "; ".join(missing))
        pytest.skip("Reference-FMUs not downloaded: " + "; ".join(missing))
    reply = fmi_runner.run(request(fmu=str(BOUNCING_BALL), outputs=["h", "v"]))
    assert reply["protocol"] == 1
    assert "error" not in reply
    assert reply["time"] == 3.0
    assert 0.0 <= reply["outputs"]["h"] <= 1.0


STATE_SPACE = (
    Path(__file__).resolve().parents[3] / "examples" / "reference-fmus" / "3.0" / "StateSpace.fmu"
)


def test_state_space_answers_arrays_for_real():
    """A 1-D array input starts as a list and its output answers as a list."""
    if not STATE_SPACE.exists() or not has_fmpy():
        if os.environ.get("OPENSYSML_REQUIRE_REFERENCE_FMUS") == "1":
            pytest.fail(f"the Reference-FMUs gate is required but missing: {STATE_SPACE}")
        pytest.skip("StateSpace.fmu not downloaded")
    reply = fmi_runner.run(
        request(
            fmu=str(STATE_SPACE),
            experiment={"startTime": 0.0, "stopTime": 1.0},
            start={"u": [1.0, 2.0, 3.0]},
            outputs=["y"],
        )
    )
    assert "error" not in reply
    y = reply["outputs"]["y"]
    assert isinstance(y, list)
    assert len(y) == 3
    assert all(isinstance(e, float) for e in y)
