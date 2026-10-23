"""``python -m opensysml.generate`` over a model of several files.

A model split across files is named the way the ``sysml`` command names one —
the files in order, or a directory — and parsed as one model, so a reference
from one file to a definition in another is typed. The fixture under
``tests/fixtures/multifile`` is that: ``top.sysml`` types a part by
``lib.sysml``'s ``Engine``, which values an attribute by ``units.kerml``'s
``Watt``. Its generated module is committed as ``tests/golden/fleet_types.py``;
regenerate it with a running service from the repository root:

    python -m opensysml.generate client/python/tests/fixtures/multifile \\
        -o client/python/tests/golden/fleet_types.py
"""

import hashlib
import os
import subprocess
import sys
from pathlib import Path

import pytest

from opensysml.generate import (
    collect_definitions,
    expand_sources,
    generate_source,
    main as generate_main,
    model_stamp,
    select_packages,
)
from opensysml.typefacts import SymbolFacts
from tests.service_gate import fail_if_service_promised, is_server_available
from tests.test_generate import FakeSymbol

PYTHON_ROOT = Path(__file__).resolve().parents[1]
FIXTURE_DIR = PYTHON_ROOT / "tests" / "fixtures" / "multifile"
FIXTURE_FILES = [FIXTURE_DIR / name for name in ("lib.sysml", "top.sysml", "units.kerml")]
GOLDEN = PYTHON_ROOT / "tests" / "golden" / "fleet_types.py"


def test_the_fixture_is_the_files_the_directory_expands_to():
    assert expand_sources([str(FIXTURE_DIR)]) == [str(path) for path in FIXTURE_FILES]


def test_expand_sources_reads_a_directory_as_the_sysml_command_does(tmp_path):
    """Model files in path order, nested directories walked, hidden ones and
    other file kinds skipped; files are taken as named and never twice."""
    (tmp_path / "b.sysml").write_text("package B;")
    (tmp_path / "a.kerml").write_text("package A;")
    (tmp_path / "notes.md").write_text("not a model")
    (tmp_path / "sub").mkdir()
    (tmp_path / "sub" / "c.sysml").write_text("package C;")
    (tmp_path / ".hidden").mkdir()
    (tmp_path / ".hidden" / "d.sysml").write_text("package D;")
    extra = tmp_path.parent / f"{tmp_path.name}-extra.sysml"
    extra.write_text("package E;")

    files = expand_sources([str(extra), str(tmp_path), str(extra), str(tmp_path / "b.sysml")])

    assert files == [
        str(extra),
        str(tmp_path / "a.kerml"),
        str(tmp_path / "b.sysml"),
        str(tmp_path / "sub" / "c.sysml"),
    ]


def test_expand_sources_orders_a_directory_by_full_path(tmp_path):
    """``a.sysml`` sorts before ``a/inner.sysml``, as under the ``sysml`` command."""
    (tmp_path / "a").mkdir()
    (tmp_path / "a" / "inner.sysml").write_text("package Inner;")
    (tmp_path / "a.sysml").write_text("package A;")
    (tmp_path / "b.sysml").write_text("package B;")

    assert expand_sources([str(tmp_path)]) == [
        str(tmp_path / "a.sysml"),
        str(tmp_path / "a" / "inner.sysml"),
        str(tmp_path / "b.sysml"),
    ]


def test_expand_sources_refuses_a_directory_without_model_files(tmp_path):
    (tmp_path / "models").mkdir()
    (tmp_path / "models" / "a.sysml").write_text("package A;")
    (tmp_path / "empty").mkdir()
    (tmp_path / "empty" / "notes.md").write_text("no model here")

    with pytest.raises(ValueError, match=f"no .sysml or .kerml file under {tmp_path / 'empty'}"):
        expand_sources([str(tmp_path / "empty"), str(tmp_path / "models")])


def test_expand_sources_keeps_a_missing_path_for_the_reader_to_report(tmp_path):
    missing = str(tmp_path / "missing.sysml")
    assert expand_sources([missing]) == [missing]


def test_expand_sources_reads_a_linked_directory_once(tmp_path):
    (tmp_path / "real").mkdir()
    (tmp_path / "real" / "a.sysml").write_text("package A;")
    try:
        os.symlink(tmp_path / "real", tmp_path / "link", target_is_directory=True)
    except (OSError, NotImplementedError):
        pytest.skip("symlinks are not available here")

    assert expand_sources([str(tmp_path)]) == [str(tmp_path / "link" / "a.sysml")]


def test_model_stamp_of_one_file_is_unchanged():
    assert model_stamp("part def A;\n") == hashlib.sha256(b"part def A;\n").hexdigest()
    assert model_stamp("part def A;\r\n") == model_stamp("part def A;\n")


def test_model_stamp_of_several_files_depends_on_their_boundaries_and_order():
    both = model_stamp("package A;", "package B;")
    assert both == model_stamp("package A;\r\n"[:-2], "package B;")
    assert both != model_stamp("package A;package B;")
    assert both != model_stamp("package B;", "package A;")
    assert both != model_stamp("package A;")


def _definitions():
    engine = FakeSymbol(SymbolFacts(id="Lib::Engine", name="Engine", kind="partDef"))
    lib = FakeSymbol(SymbolFacts(id="Lib", name="Lib", kind="package"), [engine])
    car = FakeSymbol(SymbolFacts(id="Top::Car", name="Car", kind="partDef"))
    wheel = FakeSymbol(SymbolFacts(id="Top::Parts::Wheel", name="Wheel", kind="partDef"))
    parts = FakeSymbol(SymbolFacts(id="Top::Parts", name="Parts", kind="package"), [wheel])
    top = FakeSymbol(SymbolFacts(id="Top", name="Top", kind="package"), [car, parts])
    tops = FakeSymbol(SymbolFacts(id="Tops::Hat", name="Hat", kind="partDef"))
    return collect_definitions(lib, top, FakeSymbol(SymbolFacts(id="Tops", name="Tops", kind="package"), [tops]))


def test_select_packages_keeps_a_package_and_what_it_nests():
    definitions = _definitions()
    assert [d.id for d in select_packages(definitions, ())] == [
        "Lib::Engine", "Top::Car", "Top::Parts::Wheel", "Tops::Hat"
    ]
    assert [d.id for d in select_packages(definitions, ["Top"])] == [
        "Top::Car", "Top::Parts::Wheel"
    ]
    assert [d.id for d in select_packages(definitions, ["Top::Parts", "Lib"])] == [
        "Lib::Engine", "Top::Parts::Wheel"
    ]


@pytest.mark.parametrize("packages", [["Top", "Top::Parts"], ["Top::Parts", "Top"], ["Top", "Top"]])
def test_select_packages_overlapping_selections_count_each_once(packages):
    assert [d.id for d in select_packages(_definitions(), packages)] == [
        "Top::Car", "Top::Parts::Wheel"
    ]


def test_select_packages_refuses_a_package_that_owns_nothing():
    with pytest.raises(ValueError, match="owned by package 'Nope'"):
        select_packages(_definitions(), ["Top", "Nope"])


def test_a_missing_file_among_several_is_refused_before_the_service_is_contacted(
    tmp_path, capsys, monkeypatch
):
    import opensysml

    def no_connection(*args, **kwargs):
        raise AssertionError("the service was contacted")

    monkeypatch.setattr(opensysml, "connect", no_connection)
    missing = tmp_path / "missing.sysml"

    assert generate_main([str(FIXTURE_FILES[0]), str(missing), "-o", str(tmp_path / "out.py")]) == 2
    assert f"error: cannot read {missing}:" in capsys.readouterr().err
    assert not (tmp_path / "out.py").exists()


def test_a_directory_without_model_files_is_refused_even_beside_a_model(
    tmp_path, capsys, monkeypatch
):
    import opensysml

    monkeypatch.setattr(opensysml, "connect", lambda *a, **k: pytest.fail("connected"))
    (tmp_path / "notes.md").write_text("no model here")

    assert generate_main([str(FIXTURE_DIR), str(tmp_path)]) == 2
    assert capsys.readouterr().err == f"error: no .sysml or .kerml file under {tmp_path}\n"


def test_no_source_at_all_is_a_usage_error(capsys):
    with pytest.raises(SystemExit) as excinfo:
        generate_main([])
    assert excinfo.value.code == 2
    assert "SOURCE" in capsys.readouterr().err


def test_help_describes_several_sources_and_package_selection(capsys):
    with pytest.raises(SystemExit) as excinfo:
        generate_main(["--help"])
    assert excinfo.value.code == 0
    out = capsys.readouterr().out
    assert "SOURCE [SOURCE ...]" in out
    assert "directory" in out
    assert "--package NAME" in out


def _run_cli(*args):
    env = dict(os.environ)
    env["OPENSYSML_SERVICE"] = "localhost:50051"
    return subprocess.run(
        [sys.executable, "-m", "opensysml.generate", *args],
        cwd=PYTHON_ROOT,
        capture_output=True,
        text=True,
        env=env,
    )


_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)


@pytest.mark.integration
@pytest.mark.skipif(not _AVAILABLE, reason="sysml-grpc server not running on localhost:50051")
class TestSeveralFilesAgainstRealService:
    def test_the_file_list_generates_the_golden_with_the_cross_file_reference_typed(self, tmp_path):
        output = tmp_path / "fleet_types.py"
        result = _run_cli(*(str(path) for path in FIXTURE_FILES), "-o", str(output))
        assert result.returncode == 0, result.stderr
        source = output.read_text()
        assert source == GOLDEN.read_text(), (
            "tests/golden/fleet_types.py is out of date; regenerate it as this module's docstring says"
        )
        assert "class Engine(" in source
        assert 'def engine(self) -> Engine:' in source
        assert '_t.feature_value(self, "engine", _t.as_typed(Engine))' in source
        assert 'sysml_id = "FleetUnits::Watt"' in source
        stamp = model_stamp(*(path.read_text() for path in FIXTURE_FILES))
        assert f'SYSML_MODEL_HASH = "sha256:{stamp}"' in source

    def test_the_directory_form_generates_the_same_module(self, tmp_path):
        output = tmp_path / "fleet_types.py"
        result = _run_cli(str(FIXTURE_DIR), "-o", str(output))
        assert result.returncode == 0, result.stderr
        assert output.read_text() == GOLDEN.read_text()
        assert _run_cli(str(FIXTURE_DIR), "--check", "-o", str(output)).returncode == 0

    def test_one_file_alone_leaves_the_cross_file_reference_unresolved(self, tmp_path):
        result = _run_cli(str(FIXTURE_DIR / "top.sysml"), "-o", str(tmp_path / "out.py"))
        assert result.returncode == 1
        assert "error: unresolved reference: FleetLib::Engine" in result.stderr

    def test_a_package_selects_its_definitions(self, tmp_path):
        output = tmp_path / "top_types.py"
        result = _run_cli(str(FIXTURE_DIR), "--package", "FleetTop", "-o", str(output))
        assert result.returncode == 0, result.stderr
        source = output.read_text()
        assert "class Car(" in source
        assert "class Engine(" not in source
        assert "FleetLib::Engine has no generated class" in source

        result = _run_cli(str(FIXTURE_DIR), "--package", "Nope", "-o", str(output))
        assert result.returncode == 1
        assert result.stderr == "error: no definition in the model is owned by package 'Nope'\n"

    def test_check_names_every_source_package_and_service_option_to_regenerate_with(self, tmp_path):
        output = tmp_path / "top_types.py"
        sources = [str(path) for path in FIXTURE_FILES]
        result = _run_cli(
            *sources, "--package", "FleetTop", "--port", "50051", "--check", "-o", str(output)
        )
        assert result.returncode == 1
        assert result.stderr == (
            f"error: {output} does not exist; regenerate with: python -m opensysml.generate "
            f"{' '.join(sources)} --package FleetTop --port 50051 -o {output}\n"
        )

    def test_a_diagnostic_of_several_files_names_its_file(self, tmp_path):
        broken = tmp_path / "broken.sysml"
        broken.write_text("package Broken { part def Wreck { part wheel : Missing; } }")
        result = _run_cli(str(FIXTURE_DIR), str(broken), "-o", str(tmp_path / "out.py"))
        assert result.returncode == 1
        assert f"error: {broken}: " in result.stderr

    def test_generate_source_in_process_matches_the_cli(self):
        from opensysml import Connection

        with Connection(auto_start=False) as conn:
            model = conn.parse_sources(FIXTURE_FILES)
            assert model.ok, model.diagnostics
            texts = [path.read_text() for path in FIXTURE_FILES]
            assert generate_source(model, texts) == GOLDEN.read_text()
            assert "class Engine(" not in generate_source(model, texts, ["FleetTop"])
