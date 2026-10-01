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
    """An fmpy module stand-in: simulate_fmu returns rows, read_model_description
    returns variables of the named types; error makes the call fail instead."""
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
        for name, t in variables.items():
            fmi_type, dimensions = (t, []) if isinstance(t, str) else t
            variables_of.append(
                types.SimpleNamespace(name=name, type=fmi_type, dimensions=dimensions)
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
