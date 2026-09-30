"""The service reports what the parser warns about, as the command line does.

A reserved keyword written where the grammar admits only a name is an error
on ``sysml -validate`` (code ``reserved-keyword-name``); a model loaded over
gRPC must report the same, so ``Model.ok`` and ``strict=True`` judge it as the
command line would.
"""

import pytest

from opensysml.connection import Connection
from opensysml.errors import ModelError
from tests.service_gate import fail_if_service_promised, is_server_available

KEYWORD_AS_NAME = "package P { part def X; part def M { part filter : X; } }"
QUOTED_KEYWORD_NAME = "package P { part def X; part def M { part 'filter' : X; } }"

_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)


@pytest.mark.integration
@pytest.mark.skipif(not _AVAILABLE, reason="sysml-grpc server not running on localhost:50051")
class TestReservedKeywordNameAgainstRealService:
    @pytest.mark.parametrize("strict_conformance", [False, True])
    def test_a_keyword_written_as_a_name_is_an_error(self, strict_conformance):
        with Connection(auto_start=False) as conn:
            model = conn.load_from_content(
                KEYWORD_AS_NAME, strict_conformance=strict_conformance
            )
            assert not model.ok
            (diagnostic,) = model.diagnostics
            assert diagnostic.severity == "error"
            assert diagnostic.code == "reserved-keyword-name"
            assert "reserved keyword" in diagnostic.message

    def test_strict_raises_for_a_keyword_written_as_a_name(self):
        with Connection(auto_start=False) as conn:
            with pytest.raises(ModelError) as raised:
                conn.load_from_content(KEYWORD_AS_NAME, strict=True)
            assert not raised.value.model.ok
            assert any("reserved keyword" in d.message for d in raised.value.diagnostics)

    def test_the_quoted_name_is_clean(self):
        with Connection(auto_start=False) as conn:
            model = conn.load_from_content(QUOTED_KEYWORD_NAME, strict=True)
            assert model.ok
            assert model.diagnostics == []
