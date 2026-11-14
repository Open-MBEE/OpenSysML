"""Symbol.children() lists a connector's ends against a real service.

A connection, interface or binding usage has its two ends as children of kind
``connectorEnd``, a named end under its qualified name and an unnamed one under
its element id, each referencing the feature it attaches; a feature chain is
one ``references`` entry per chaining feature. Skips without a service on
localhost:50051; fails instead when $OPENSYSML_REQUIRE_SERVICE is set.
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

MODEL = """
package Ends {
    port def P;
    part def Unit { port p : P; }
    interface def Iface { end port a : P; end port b : P; }
    part def Vehicle {
        port q : P;
        port r : P;
        part x : Unit;
        part y : Unit;
        connection plain connect q to r;
        connection named connect (a ::> x.p, b ::> y.p);
        interface wired : Iface connect q to r;
        binding bound bind x.p = y.p;
        bind q = r;
    }
}
"""

V = "Ends::Vehicle"
PLAIN = [[f"{V}::q"], [f"{V}::r"]]
CHAINED = [[f"{V}::x", "Ends::Unit::p"], [f"{V}::y", "Ends::Unit::p"]]

CASES = [
    (f"{V}::plain", ["Ends__Vehicle__plain_pend0", "Ends__Vehicle__plain_pend1"], PLAIN),
    (f"{V}::named", [f"{V}::named::a", f"{V}::named::b"], CHAINED),
    (f"{V}::wired", ["Ends__Vehicle__wired_pend0", "Ends__Vehicle__wired_pend1"], PLAIN),
    (f"{V}::bound", ["Ends__Vehicle__bound_pend0", "Ends__Vehicle__bound_pend1"], CHAINED),
    (f"{V}::@8", ["Ends__Vehicle___408_pend0", "Ends__Vehicle___408_pend1"], PLAIN),
]


@pytest.fixture(scope="module")
def model(tmp_path_factory):
    path = tmp_path_factory.mktemp("ends") / "model.sysml"
    path.write_text(MODEL)
    with Connection(auto_start=False) as conn:
        loaded = conn.load(str(path))
        assert not loaded.diagnostics, loaded.diagnostics
        yield loaded


@pytest.mark.parametrize("connector, ids, targets", CASES)
def test_connector_lists_its_ends_and_what_they_reference(model, connector, ids, targets):
    ends = model.get(connector).children()
    assert [(e.kind, e.id) for e in ends] == [("connectorEnd", i) for i in ids]
    assert [
        [s.target_id for s in e.specializations if s.kind == "references"] for e in ends
    ] == targets
    for end in ends:
        assert model.get(end.id).id == end.id


def test_owner_children_do_not_list_the_ends(model):
    assert not [c for c in model.get(V).children() if c.kind == "connectorEnd"]
