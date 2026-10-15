"""The typed preview of an analysis dry run."""

from opensysml import ToolDryRun
from opensysml.proto import sysml_pb2


def test_a_dry_run_preview_is_read_field_by_field():
    proto = sysml_pb2.ToolDryRun(
        action="Demo::compute",
        tool="octave", version="8.4",
        manifest="/tools/octave.json",
        executable="/usr/bin/octave",
        protocol="stdio",
        args=["--eval", "run"],
        env=["HOME=/tmp"],
        cwd="/tmp/run",
        stdin="data",
        stdin_data="x = 1",
        input_file=sysml_pb2.ToolDryRunFile(name="in.json", format="json", data="{}"),
        output_dir=True,
        inputs=[sysml_pb2.ToolDryRunInput(variable="x", parameter="p", value="250.0", unit="m")],
        outputs=[sysml_pb2.ToolDryRunOutput(variable="y", parameter="q")],
        reply_source="stdout",
        lines=["would run /usr/bin/octave"],
    )

    preview = ToolDryRun(proto)

    assert (preview.action, preview.tool, preview.version) == ("Demo::compute", "octave", "8.4")
    assert preview.args == ["--eval", "run"]
    assert preview.env == ["HOME=/tmp"]
    assert preview.input_file == {"name": "in.json", "format": "json", "data": "{}"}
    assert preview.output_dir is True
    assert preview.inputs == [{"variable": "x", "parameter": "p", "value": "250.0", "unit": "m"}]
    assert preview.outputs == [{"variable": "y", "parameter": "q"}]
    assert preview.reply_source == "stdout"
    assert preview.lines == ["would run /usr/bin/octave"]
    assert "Demo::compute" in repr(preview)


def test_a_preview_without_an_input_file_reads_none():
    preview = ToolDryRun(sysml_pb2.ToolDryRun(action="Demo::compute"))
    assert preview.input_file is None
    assert preview.inputs == []
