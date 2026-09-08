from opensysml.proto import sysml_pb2
from opensysml.diagnostic import Diagnostic


def test_diagnostic_properties():
    pb_diag = sysml_pb2.Diagnostic(
        severity="error",
        message="Undefined symbol 'Foo'",
        span=sysml_pb2.Span(
            file="test.sysml",
            start_line=10,
            start_col=5,
            end_line=10,
            end_col=8,
        ),
    )
    
    diag = Diagnostic(pb_diag)
    
    assert diag.severity == "error"
    assert diag.message == "Undefined symbol 'Foo'"
    assert diag.file == "test.sysml"
    assert diag.start_line == 10
    assert diag.start_column == 5
    assert diag.end_line == 10
    assert diag.end_column == 8


def test_diagnostic_str():
    pb_diag = sysml_pb2.Diagnostic(
        severity="warning",
        message="Unused import",
        span=sysml_pb2.Span(
            file="model.sysml",
            start_line=5,
            start_col=1,
            end_line=5,
            end_col=20,
        ),
    )
    
    diag = Diagnostic(pb_diag)
    result = str(diag)
    
    assert "model.sysml:5:1" in result
    assert "warning" in result.lower()
    assert "Unused import" in result


def test_diagnostic_span_property():
    pb_span = sysml_pb2.Span(
        file="test.sysml",
        start_line=1,
        start_col=2,
        end_line=3,
        end_col=4,
    )
    pb_diag = sysml_pb2.Diagnostic(
        severity="error",
        message="Test",
        span=pb_span,
    )
    
    diag = Diagnostic(pb_diag)
    
    # span property returns the protobuf Span object
    assert diag.span == pb_span
    assert diag.span.file == "test.sysml"


def test_diagnostic_code():
    coded = Diagnostic(sysml_pb2.Diagnostic(
        severity="info",
        message="choice point: 2 steppable tokens",
        code="choice-point",
    ))
    uncoded = Diagnostic(sysml_pb2.Diagnostic(severity="error", message="expected '}'"))

    assert coded.code == "choice-point"
    assert "code='choice-point'" in repr(coded)
    assert uncoded.code == ""
