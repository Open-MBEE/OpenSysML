"""Tests for ``Symbol.annotating_metadata``: the metadata usages annotating a
symbol, the ``about`` ones written elsewhere included."""

from unittest.mock import Mock

import pytest

from opensysml import Connection
from opensysml.capabilities import (
    CAPABILITY_SYMBOL_ANNOTATING_METADATA,
    MissingCapabilityError,
    ServerInfo,
)
from opensysml.errors import UnresolvedChildError
from opensysml.proto import sysml_pb2
from opensysml.symbol import Symbol

MODEL_SOURCE = '''
package D10 {
    metadata def FailureMode {
        attribute severity : ScalarValues::Integer;
    }
    #FailureMode part def PumpP;
    part def PumpB { @FailureMode { severity = 3; } }
    part def PumpA;
    metadata pumpA_fm : FailureMode about PumpA { severity = 2; }
    part def Pump { part motorA; }
    metadata motorA_fm : FailureMode about Pump::motorA;
}
'''


def _pump(client, capabilities=frozenset({CAPABILITY_SYMBOL_ANNOTATING_METADATA}),
          usage_ids=("D10::pumpA_fm",)):
    """Build `part def PumpA` as the service reports it: no child, annotated by
    the usages named."""
    client.server_info.return_value = ServerInfo(
        version="v1.0.0", capabilities=capabilities, answered=True, origin="sysml-grpc",
    )
    symbols = {
        "D10::pumpA_fm": sysml_pb2.SymbolInfo(
            id="D10::pumpA_fm", name="pumpA_fm", kind="metadataUsage",
        ),
    }
    client.get_symbol.side_effect = lambda _hash, symbol_id: symbols.get(symbol_id)
    info = sysml_pb2.SymbolInfo(
        id="D10::PumpA", name="PumpA", kind="partDef", annotating_metadata_ids=list(usage_ids),
    )
    return Symbol(info, client, "model_abc123")


class TestAnnotatingMetadata:
    def test_lists_the_usages_the_service_names(self):
        pump = _pump(Mock())

        usages = pump.annotating_metadata()

        assert [(u.id, u.kind) for u in usages] == [("D10::pumpA_fm", "metadataUsage")]
        assert pump.children() == []

    def test_is_fetched_once(self):
        pump = _pump(Mock())

        pump.annotating_metadata()
        pump.annotating_metadata()

        assert pump._client.get_symbol.call_count == 1

    def test_needs_the_capability(self):
        """A service without the capability reports none, which would read as
        an unannotated element."""
        pump = _pump(Mock(), capabilities=frozenset({"symbol_members"}), usage_ids=())

        with pytest.raises(MissingCapabilityError) as excinfo:
            pump.annotating_metadata()

        assert CAPABILITY_SYMBOL_ANNOTATING_METADATA in str(excinfo.value)

    def test_an_unresolved_usage_is_an_error(self):
        pump = _pump(Mock(), usage_ids=("D10::pumpA_fm", "D10::gone"))

        with pytest.raises(UnresolvedChildError) as excinfo:
            pump.annotating_metadata()

        assert "D10::gone" in str(excinfo.value)

    def test_without_a_client_there_is_none(self):
        assert Symbol(sysml_pb2.SymbolInfo(id="D10::PumpA"), None, "h").annotating_metadata() == []


@pytest.mark.integration
class TestAnnotatingMetadataIntegration:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def _annotating(self, fqn):
        return [usage.id for usage in self.model.get(fqn).annotating_metadata()]

    def test_every_form_names_its_usage(self):
        assert self._annotating("D10::PumpP") == ["D10::PumpP::@0"]
        assert self._annotating("D10::PumpB") == ["D10::PumpB::@0"]
        assert self._annotating("D10::PumpA") == ["D10::pumpA_fm"]
        assert self._annotating("D10::Pump::motorA") == ["D10::motorA_fm"]
        assert self._annotating("D10::pumpA_fm") == []
        assert self._annotating("D10") == []

    def test_an_about_usage_is_not_a_child_of_what_it_annotates(self):
        pump = self.model.get("D10::PumpA")

        assert pump.children() == []
        assert "D10::pumpA_fm" in [child.id for child in self.model.get("D10").children()]

    def test_the_usage_reads_its_type_and_value(self):
        (usage,) = self.model.get("D10::PumpA").annotating_metadata()

        assert usage.kind == "metadataUsage"
        assert usage.type_facts.resolved_id == "D10::FailureMode"
        assert [(a.name, a.value) for a in usage.attribute_facts()] == [("severity", 2)]

    def test_the_body_form_reads_its_value(self):
        (usage,) = self.model.get("D10::PumpB").annotating_metadata()

        assert [(a.name, a.value) for a in usage.attribute_facts()] == [("severity", 3)]
