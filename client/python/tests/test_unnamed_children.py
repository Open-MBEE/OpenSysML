"""Symbol.children() lists unnamed members and prefix metadata against a real service.

An unnamed member (``bind``, ``connect``, ``part : P;``, ``@M { … }``) is listed
under the positional id the query API reports (``Owner::@3``), GetSymbol
resolves that id, and a ``#M`` prefix is a member of the element it annotates
like the body spelling. Skips without a service on localhost:50051; fails
instead when $OPENSYSML_REQUIRE_SERVICE is set.
"""

import pytest

from opensysml import Connection
from tests.service_gate import fail_if_service_promised, is_server_available

_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)

pytestmark = pytest.mark.skipif(
    not _AVAILABLE,
    reason="sysml-grpc server not running on localhost:50051"
)

UNNAMED_MODEL = """
package UnnamedChildren {
    port def CmdPort { in attribute cmd : ScalarValues::Integer; }
    part def Logic { port cmdIn : CmdPort; port cmdOut : ~CmdPort; }
    part def Controller {
        port cmdIn : CmdPort;
        part logic : Logic;
        part monitor : Logic;
        bind cmdIn = logic.cmdIn;
        binding toMonitor bind cmdIn = monitor.cmdIn;
        bind logic.cmdOut = monitor.cmdIn;
        connect logic.cmdOut to monitor.cmdIn;
        part : Logic;
    }
}
"""

METADATA_MODEL = """
package MetadataAnnotations {
    metadata def FailureMode { attribute severity : ScalarValues::Integer; }
    part def Pump {
        state def PumpStates {
            entry; then idle;
            state idle;
            state jammed { @FailureMode { severity = 3; } }
            #FailureMode state overheated;
            state stalled { metadata fm : FailureMode; }
        }
        action pumpFluid {
            in flowRate : ScalarValues::Real { @FailureMode { severity = 2; } }
            in #FailureMode pressure : ScalarValues::Real;
        }
    }
}
"""


def _load(conn, tmp_path, text):
    path = tmp_path / "model.sysml"
    path.write_text(text)
    model = conn.load(str(path))
    assert not model.diagnostics, model.diagnostics
    return model


def test_children_list_every_member_the_query_reports(tmp_path):
    owner = "UnnamedChildren::Controller"
    with Connection(auto_start=False) as conn:
        model = _load(conn, tmp_path, UNNAMED_MODEL)
        controller = model.get(owner)
        children = [(c.kind, c.id) for c in controller.children()]
        queried = [(e.type, e.id) for e in model.query(scope=[owner]) if e.id != owner]
        assert [c_id for _, c_id in children] == [q_id for _, q_id in queried]
        assert [c_id for kind, c_id in children if kind == "bindingConnectorAsUsage"] == [
            f"{owner}::@3", f"{owner}::toMonitor", f"{owner}::@5"]
        assert [c_id for kind, c_id in children if kind == "connectionUsage"] == [f"{owner}::@6"]
        assert len(controller.parts()) == 3
        assert len(set(controller._pb.child_ids)) == len(controller._pb.child_ids)
        anonymous = model.get(f"{owner}::@7")
        assert anonymous is not None
        assert anonymous.kind == "partUsage"
        assert anonymous.name == ""
        assert conn.get_symbol(model.hash, f"{owner}::") is None


def test_children_list_body_and_prefix_metadata(tmp_path):
    with Connection(auto_start=False) as conn:
        model = _load(conn, tmp_path, METADATA_MODEL)
        expectations = {
            "MetadataAnnotations::Pump::PumpStates::jammed": "@0",
            "MetadataAnnotations::Pump::PumpStates::overheated": "@0",
            "MetadataAnnotations::Pump::PumpStates::stalled": "fm",
            "MetadataAnnotations::Pump::pumpFluid::flowRate": "@0",
            "MetadataAnnotations::Pump::pumpFluid::pressure": "@0",
        }
        for owner, leaf in expectations.items():
            children = model.get(owner).children()
            assert [(c.kind, c.id) for c in children] == [("metadataUsage", f"{owner}::{leaf}")], owner
            assert children[0].metadata.get("type") == "FailureMode"
            queried = [e.id for e in model.query(scope=[owner]) if e.type == "MetadataUsage"]
            assert queried == [f"{owner}::{leaf}"], owner


INHERITED_MODEL = """
package U {
    part def Unit { out item o; in item i; }
    metadata def M;
    part def Holder {
        part x : Unit;
        part y : Unit;
        part : Unit;
        attribute : ScalarValues::Real;
        bind x.o = y.o;
        connect x to y;
        flow from x.o to y.i;
        @M;
    }
    part holder : Holder;
}
"""

UNNAMED_KINDS = ["partUsage", "attributeUsage", "bindingConnectorAsUsage",
                 "connectionUsage", "flowUsage", "metadataUsage"]


def test_unnamed_members_are_inherited_through_typing(tmp_path):
    with Connection(auto_start=False) as conn:
        model = _load(conn, tmp_path, INHERITED_MODEL)
        owned = [c for c in model.get("U::Holder").children() if not c.name]
        assert [c.kind for c in owned] == UNNAMED_KINDS
        assert [b.id for b in model.get("U::Holder").bindings()] == ["U::Holder::@4"]
        for child in owned:
            assert model.get(child.id).kind == child.kind
        holder = model.get("U::holder")
        assert holder.children() == []
        inherited = [m for m in holder.members(inherited=True) if not m.name]
        assert [m.id for m in inherited] == [c.id for c in owned]
        assert all(m.inherited_from == "U::Holder" for m in inherited)


def test_unnamed_library_members_have_positional_ids(tmp_path):
    with Connection(auto_start=False) as conn:
        model = _load(conn, tmp_path, INHERITED_MODEL)
        holder = model.get("U::Holder")
        listed = holder.members(inherited=True, library=True)
        assert listed.withheld_library == 0
        assert len({m.id for m in listed}) == len(listed)
        unnamed = [m for m in listed if not m.name and m.inherited]
        assert unnamed
        for member in unnamed:
            assert member.id.startswith(member.inherited_from + "::@")
            assert model.get(member.id).kind == member.kind
        bindings = [m for m in unnamed if m.kind == "bindingConnectorAsUsage"
                    and m.inherited_from == "Occurrences::Occurrence"]
        assert len(bindings) == 3
