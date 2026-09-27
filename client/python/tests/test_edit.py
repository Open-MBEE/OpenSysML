"""Tests for changing a model and writing it back.

Two layers. Against a fake service, the client's own behavior: the capability
gate, the shape of the request, the editor's own refusals, and how a reported
refusal becomes a typed exception. Against the real ``sysml-grpc``, the round
trip itself — load, set a value, apply, save, load the saved file and ask what
the value is now — which is the part a mock cannot tell you anything about.
"""

import os
import subprocess
import time
from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import (
    CAPABILITY_APPLY_EDITS,
    CAPABILITY_AUTHORING,
    CAPABILITY_CONNECTION_AUTHORING,
    CAPABILITY_MEMBER_MODIFIERS,
    CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
    CAPABILITY_SATISFY_AUTHORING,
    CAPABILITY_TRANSITION_AUTHORING,
    CAPABILITY_EDIT_DOCUMENTS,
    CAPABILITY_INLINE_LANGUAGE,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.conversion import Conversion, FORMAT_SYSML
from opensysml.edit import EditedDocument, EditResult, Editor
from opensysml.errors import (
    EditError,
    EditResultError,
    EditTargetError,
    InvalidEditError,
    ModelNotFoundError,
    NoEditsError,
    OverlappingEditsError,
    RenameReferencedError,
    OwnerNotFoundError,
    OwnerNotNamespaceError,
    IllegalMemberKindError,
    MemberNameTakenError,
    DeleteReferencedError,
    OwnerInsideTargetError,
    MoveReferencedError,
    ReferencedElsewhereError,
    Referrer,
)
from opensysml.proto import sysml_pb2, sysml_pb2_grpc

MODEL = """package Demo {
    // The mass of one unit, measured on the bench.
    part def SC {

        attribute unitMass : ISQ::MassValue default = 1000.0[SI::kg];

        // No margin has been agreed yet.
        attribute margin : ISQ::MassValue;

        attribute label : ScalarValues::String = "flight-1";
        attribute active : ScalarValues::Boolean = true;
        attribute total : ISQ::MassValue = unitMass;

        part avionics {
            part board {
                attribute count : ScalarValues::Integer = 2;
            }
        }
    }

    part sc : SC {
        attribute redefines unitMass = 1200.0[SI::kg];
    }
}
"""

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
GRPC_BINARIES = (
    os.path.join(REPO_ROOT, "bin", "sysml-grpc"),
    os.path.join(os.path.expanduser("~"), ".opensysml", "bin", "sysml-grpc"),
)


class FakeService(sysml_pb2_grpc.SysMLServiceServicer):
    """A sysml-grpc whose ApplyEdits records the request and answers as told."""

    def __init__(self, capabilities=(CAPABILITY_APPLY_EDITS,), error="",
                 failure=sysml_pb2.EDIT_FAILURE_UNSPECIFIED, diagnostics=0,
                 referring_elements=(), referrers=(), not_found=False,
                 documents=(), content="edited", legacy=False):
        self._capabilities = list(capabilities)
        self._legacy = legacy
        self._error = error
        self._failure = failure
        self._diagnostics = diagnostics
        self._referring = list(referring_elements)
        self._referrers = list(referrers)
        self._not_found = not_found
        self._documents = list(documents) or [("<content>", content)]
        self._content = content
        self.requests = []

    def GetServerInfo(self, request, context):
        return sysml_pb2.ServerInfoResponse(
            version="fake", capabilities=self._capabilities
        )

    def GetDiagnostics(self, request, context):
        context.abort(grpc.StatusCode.NOT_FOUND, "model not found")

    def ParseFile(self, request, context):
        root = sysml_pb2.SymbolInfo(id="Demo", name="Demo", kind="Package")
        return sysml_pb2.ParseFileResponse(model_hash="fake-hash", root=root)

    def ApplyEdits(self, request, context):
        self.requests.append(request)
        if self._not_found:
            context.abort(
                grpc.StatusCode.NOT_FOUND,
                f"model {request.model_hash} is no longer cached: parse it again "
                f"before editing it",
            )
        if self._error:
            return sysml_pb2.ApplyEditsResponse(
                error=self._error,
                failure=self._failure,
                referring_elements=self._referring,
                referrers=[
                    sysml_pb2.Referrer(name=name, document=document)
                    for name, document in self._referrers
                ],
                diagnostics=[
                    sysml_pb2.Diagnostic(
                        severity="error",
                        message=f"edit error {i}",
                        span=sysml_pb2.Span(file="<content>", start_line=1),
                    )
                    for i in range(self._diagnostics)
                ],
            )
        applied = sysml_pb2.AppliedEdit(
            operation_index=0,
            target="Demo::SC::unitMass",
            offset=7,
            length=3,
            old_text="old",
            new_text="new",
        )
        if self._legacy:
            return sysml_pb2.ApplyEditsResponse(content=self._content, applied=[applied])
        applied.document = self._documents[0][0]
        return sysml_pb2.ApplyEditsResponse(
            content=self._content,
            documents=[
                sysml_pb2.EditedDocument(name=name, content=text)
                for name, text in self._documents
            ],
            applied=[applied],
        )


@pytest.fixture
def fake_service():
    """Start a FakeService on an ephemeral port; yields a (port, service) factory."""
    servers = []

    def start(**kwargs):
        service = FakeService(**kwargs)
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
        sysml_pb2_grpc.add_SysMLServiceServicer_to_server(service, server)
        port = server.add_insecure_port("localhost:0")
        server.start()
        servers.append(server)
        return port, service

    yield start
    for server in servers:
        server.stop(None)


def test_edit_requires_the_capability(fake_service):
    """A service that cannot edit is named, not asked."""
    port, service = fake_service(capabilities=())
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        with pytest.raises(MissingCapabilityError) as excinfo:
            edit.apply()
    assert excinfo.value.capability == CAPABILITY_APPLY_EDITS
    assert service.requests == [], "the request was sent to a service that cannot serve it"


def test_the_request_carries_the_operations_in_order(fake_service):
    """Both operation shapes cross as the caller wrote them."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        result = (
            model.edit()
            .set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
            .rename("Demo::SC::margin", "reserve")
            .apply()
        )

    (request,) = service.requests
    assert request.model_hash == model.hash
    first, second = request.operations
    assert first.WhichOneof("operation") == "set_value"
    assert (first.set_value.target, first.set_value.value) == (
        "Demo::SC::unitMass", "1050.0[SI::kg]",
    )
    assert second.WhichOneof("operation") == "rename"
    assert (second.rename.target, second.rename.new_name) == (
        "Demo::SC::margin", "reserve",
    )
    assert str(result) == "edited"


def test_add_member_and_delete_requests_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        result = (
            model.edit()
            .add_member(
                "Demo::SC", "part", "board", type="Board",
                multiplicity="[1]", value="1", specializes=[]
            )
            .delete("Demo::sc", cascade=False)
            .apply()
        )
    add, delete = service.requests[0].operations
    assert add.WhichOneof("operation") == "add_member"
    assert (
        add.add_member.owner, add.add_member.kind, add.add_member.name,
        add.add_member.type, add.add_member.multiplicity, add.add_member.value,
    ) == ("Demo::SC", "part", "board", "Board", "[1]", "1")
    assert delete.WhichOneof("operation") == "delete"
    assert (delete.delete.target, delete.delete.cascade) == ("Demo::sc", False)
    assert result is not None


def test_calc_and_action_helpers_expand_into_member_edits(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_MEMBER_MODIFIERS,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_calc_def(
                "Demo", "C", inputs=[("x", "ScalarValues::Real")],
                return_type="ScalarValues::Real", return_expression="x * 2",
                abstract=True,
            )
            .add_action(
                "Demo::C", "run", inputs=[("request", "Input")],
                outputs=[("response", "Output")],
            )
            .add_calc("", "BareCalc", inputs=[("value", "Real")])
            .apply()
        )
    calc, parameter, result, action, action_input, action_output, bare_calc, bare_input = (
        operation.add_member for operation in service.requests[0].operations
    )
    assert (calc.kind, calc.name, calc.is_abstract) == ("calc def", "C", True)
    assert (
        parameter.owner,
        parameter.kind,
        parameter.name,
        parameter.type,
        parameter.direction,
    ) == ("Demo::C", "ref", "x", "ScalarValues::Real", "in")
    assert (
        result.owner,
        result.kind,
        result.type,
        result.value,
    ) == ("Demo::C", "return", "ScalarValues::Real", "x * 2")
    assert (action.owner, action.kind, action.name) == (
        "Demo::C", "action", "run"
    )
    assert (action_input.owner, action_input.direction, action_input.name) == (
        "Demo::C::run", "in", "request"
    )
    assert (action_output.owner, action_output.direction, action_output.name) == (
        "Demo::C::run", "out", "response"
    )
    assert (bare_calc.owner, bare_calc.name) == ("", "BareCalc")
    assert (bare_input.owner, bare_input.name) == ("BareCalc", "value")


@pytest.mark.parametrize(
    "method,argument,value",
    [
        ("add_calc", "inputs", (("x", "Real"),)),
        ("add_calc", "inputs", [("x",)]),
        ("add_calc_def", "inputs", [("x", 1)]),
        ("add_action", "outputs", (("result", "Real"),)),
        ("add_action", "outputs", [("result", 1)]),
    ],
)
def test_calc_and_action_helpers_reject_invalid_parameter_shapes(
    method, argument, value
):
    editor = Editor("hash", None)
    with pytest.raises(TypeError, match=argument):
        getattr(editor, method)("Demo", "Declaration", **{argument: value})
    assert len(editor) == 0


@pytest.mark.parametrize("method", ["add_calc_def", "add_calc"])
@pytest.mark.parametrize("return_type", [None, ""])
def test_calc_helpers_require_return_type_for_return_expression(method, return_type):
    editor = Editor("hash", None)
    editor.add_member("Demo", "part", "existing")
    pending = editor.operations

    with pytest.raises(
        ValueError, match="^return_expression requires return_type$"
    ):
        getattr(editor, method)(
            "Demo", "C", return_type=return_type, return_expression="x * 2"
        )

    assert editor.operations == pending


def test_new_authoring_operations_and_member_modifiers_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_MEMBER_MODIFIERS,
            CAPABILITY_SATISFY_AUTHORING,
            CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
            CAPABILITY_TRANSITION_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_member(
                "Demo::SC", "attribute", "input", type="Real",
                abstract=True, redefines=["Demo::SC::old"], default=True, direction="in",
            )
            .add_satisfy("Demo::SC", "Demo::SC::r", by="Demo::SC::t", asserted=True)
            .add_require_constraint("Demo::SC", "true", name="valid")
            .add_assume_constraint("Demo::SC", "true")
            .add_transition(
                "Demo::SC", "a", "b", name="go", trigger="CycleStart",
                guard="ready", effect="action cool",
            )
            .add_entry_transition("Demo::SC", "a")
            .apply()
        )
    member, satisfy, require, assume, transition, entry = service.requests[0].operations
    assert member.WhichOneof("operation") == "add_member"
    assert (
        member.add_member.is_abstract,
        list(member.add_member.redefines),
        member.add_member.is_default,
        member.add_member.direction,
    ) == (True, ["Demo::SC::old"], True, "in")
    assert satisfy.WhichOneof("operation") == "add_satisfy"
    assert (
        satisfy.add_satisfy.owner, satisfy.add_satisfy.requirement,
        satisfy.add_satisfy.satisfying_feature, satisfy.add_satisfy.is_asserted,
        satisfy.add_satisfy.is_negated,
    ) == ("Demo::SC", "Demo::SC::r", "Demo::SC::t", True, False)
    assert require.WhichOneof("operation") == "add_requirement_constraint"
    assert (
        require.add_requirement_constraint.owner,
        require.add_requirement_constraint.kind,
        require.add_requirement_constraint.expression,
        require.add_requirement_constraint.name,
    ) == ("Demo::SC", "require", "true", "valid")
    assert assume.add_requirement_constraint.kind == "assume"
    assert assume.add_requirement_constraint.name == ""
    assert transition.WhichOneof("operation") == "add_transition"
    assert (
        transition.add_transition.owner,
        transition.add_transition.name,
        transition.add_transition.source,
        transition.add_transition.target,
        transition.add_transition.trigger,
        transition.add_transition.guard,
        transition.add_transition.effect,
        transition.add_transition.initial,
    ) == ("Demo::SC", "go", "a", "b", "CycleStart", "ready", "action cool", False)
    assert entry.add_transition.owner == "Demo::SC"
    assert entry.add_transition.target == "a"
    assert entry.add_transition.initial


def test_member_modifier_capability_accumulates_across_operations(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.add_member("Demo::SC", "attribute", "input", abstract=True)
        edit.add_member("Demo::SC", "attribute", "output", direction="")
        with pytest.raises(MissingCapabilityError) as error:
            edit.apply()
    assert error.value.capability == CAPABILITY_MEMBER_MODIFIERS
    assert service.requests == []

def test_transition_requires_authoring_alongside_transition_capability(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_TRANSITION_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.add_transition("Demo::S", "idle", "toasting")
        with pytest.raises(MissingCapabilityError) as error:
            edit.apply()
    assert error.value.capability == CAPABILITY_AUTHORING
    assert service.requests == []


@pytest.mark.parametrize(
    "name,expected",
    [
        (None, "name must be notation text, not NoneType"),
        (3, "name must be notation text, not int"),
    ],
)
def test_add_member_rejects_invalid_names_with_type_message(fake_service, name, expected):
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        with pytest.raises(TypeError) as error:
            edit.add_member("Demo::SC", "attribute", name)
    assert str(error.value) == expected
    assert service.requests == []
    assert len(edit) == 0


def test_add_member_rejects_invalid_direction_with_type_message(fake_service):
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        with pytest.raises(TypeError) as error:
            edit.add_member("Demo::SC", "attribute", "output", direction=3)
    assert str(error.value) == "direction must be notation text, not int"
    assert service.requests == []
    assert len(edit) == 0


@pytest.mark.parametrize(
    "operation,missing",
    [
        (lambda editor: editor.add_member("Demo::SC", "attribute", "x", abstract=True),
         CAPABILITY_MEMBER_MODIFIERS),
        (lambda editor: editor.add_member("Demo::SC", "attribute", "x", default=True),
         CAPABILITY_MEMBER_MODIFIERS),
        (lambda editor: editor.add_member("Demo::SC", "attribute", "x", direction="in"),
         CAPABILITY_MEMBER_MODIFIERS),
        (lambda editor: editor.add_member("Demo::SC", "ref", "x"), CAPABILITY_MEMBER_MODIFIERS),
        (lambda editor: editor.add_member("Demo::SC", "return", "result"),
         CAPABILITY_MEMBER_MODIFIERS),
        (lambda editor: editor.add_satisfy("Demo::SC", "Demo::SC::r"), CAPABILITY_SATISFY_AUTHORING),
        (lambda editor: editor.add_require_constraint("Demo::SC", "true"),
         CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING),
        (lambda editor: editor.add_transition("Demo::SC", "a", "b"),
         CAPABILITY_TRANSITION_AUTHORING),
    ],
)
def test_new_authoring_capabilities_are_preflighted(fake_service, operation, missing):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        editor = operation(conn.load_from_content(MODEL).edit())
        with pytest.raises(MissingCapabilityError) as error:
            editor.apply()
    assert error.value.capability == missing
    assert service.requests == []


def test_add_member_normalizes_reference_strings_and_validates_kind():
    editor = Editor("hash", None)
    editor.add_member(
        "Demo", "part", "wheel", specializes="Vehicle",
        redefines=("Base::wheel",),
    )
    assert editor.operations == [
        (
            "add_member", "Demo", "part", "wheel", "", "", "",
            ["Vehicle"], False, ["Base::wheel"], False, "",
        )
    ]
    with pytest.raises(TypeError, match="kind must be notation text"):
        editor.add_member("Demo", None, "wheel")
    with pytest.raises(TypeError, match="redefines must contain only notation strings"):
        editor.add_member("Demo", "part", "wheel", redefines=["Base::wheel", 1])


def test_add_connection_and_typed_helpers_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_CONNECTION_AUTHORING,
        )
    )

    class Owner:
        id = "Demo::SC"

    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_connection(
                Owner(), "flow", "tank.fuelOut", "engine.fuelIn",
                name="fuelFlow", type="Fuel",
            )
            .add_allocation("Demo::SC", "a", "b", name="alloc1")
            .add_flow("Demo::SC", "c", "d")
            .apply()
        )
    connection, allocation, flow = service.requests[0].operations
    assert connection.WhichOneof("operation") == "add_connection"
    assert (
        connection.add_connection.owner, connection.add_connection.kind,
        connection.add_connection.from_end, connection.add_connection.to_end,
        connection.add_connection.name, connection.add_connection.type,
    ) == (
        "Demo::SC", "flow", "tank.fuelOut", "engine.fuelIn", "fuelFlow", "Fuel",
    )
    assert (
        allocation.add_connection.kind, allocation.add_connection.from_end,
        allocation.add_connection.to_end, allocation.add_connection.name,
    ) == ("allocation", "a", "b", "alloc1")
    assert (
        flow.add_connection.kind, flow.add_connection.from_end,
        flow.add_connection.to_end,
    ) == ("flow", "c", "d")


@pytest.mark.parametrize(
    "args",
    [
        ("Demo::SC", 1, "a", "b"),
        ("Demo::SC", "flow", None, "b"),
        ("Demo::SC", "flow", "a", 2),
        ("Demo::SC", "flow", "a", "b", 1),
        ("Demo::SC", "flow", "a", "b", None, 1),
    ],
)
def test_add_connection_rejects_non_string_fields(fake_service, args):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        with pytest.raises(TypeError):
            edit.add_connection(*args)
    assert service.requests == []
    assert len(edit) == 0


def test_move_request_is_exact(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        model.edit().move("Demo::SC::unitMass", "Demo::sc").move("Demo::sc", "").apply()
    into, to_root = service.requests[0].operations
    assert into.WhichOneof("operation") == "move"
    assert (into.move.target, into.move.owner) == ("Demo::SC::unitMass", "Demo::sc")
    assert (to_root.move.target, to_root.move.owner) == ("Demo::sc", "")


@pytest.mark.parametrize(
    "method,kind",
    [
        ("add_package", "package"), ("add_part_def", "part def"),
        ("add_part", "part"), ("add_attribute_def", "attribute def"),
        ("add_attribute", "attribute"), ("add_item_def", "item def"),
        ("add_item", "item"), ("add_port_def", "port def"),
        ("add_port", "port"), ("add_class", "class"), ("add_struct", "struct"),
        ("add_datatype", "datatype"), ("add_classifier", "classifier"),
        ("add_feature", "feature"), ("add_assoc", "assoc"),
        ("add_behavior", "behavior"), ("add_function", "function"),
        ("add_predicate", "predicate"), ("add_interaction", "interaction"),
        ("add_metaclass", "metaclass"), ("add_calc_def", "calc def"),
        ("add_calc", "calc"),
    ],
)
def test_every_typed_helper_uses_service_kind(fake_service, method, kind):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        getattr(conn.load_from_content(MODEL).edit(), method)("Demo::SC", "New").apply()
    assert service.requests[0].operations[0].add_member.kind == kind


def test_authoring_capability_gates_add_delete_and_move(fake_service):
    port, service = fake_service(capabilities=(CAPABILITY_APPLY_EDITS,))
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        add = model.edit().add_part("Demo::SC", "new")
        delete = model.edit().delete("Demo::sc")
        move = model.edit().move("Demo::sc", "Demo::SC")
        connection = model.edit().add_connection("Demo::SC", "flow", "a", "b")
        with pytest.raises(MissingCapabilityError) as add_error:
            add.apply()
        with pytest.raises(MissingCapabilityError) as delete_error:
            delete.apply()
        with pytest.raises(MissingCapabilityError) as move_error:
            move.apply()
        with pytest.raises(MissingCapabilityError) as connection_error:
            connection.apply()
    assert add_error.value.capability == CAPABILITY_AUTHORING
    assert delete_error.value.capability == CAPABILITY_AUTHORING
    assert move_error.value.capability == CAPABILITY_AUTHORING
    assert connection_error.value.capability == CAPABILITY_AUTHORING
    assert service.requests == []


def test_connection_authoring_capability_gates_add_connection(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        connection = (
            conn.load_from_content(MODEL)
            .edit()
            .add_connection("Demo::SC", "flow", "a", "b")
        )
        with pytest.raises(MissingCapabilityError) as error:
            connection.apply()
    assert error.value.capability == CAPABILITY_CONNECTION_AUTHORING
    assert service.requests == []


@pytest.mark.parametrize(
    "failure,expected",
    [
        (sysml_pb2.EDIT_FAILURE_OWNER_UNKNOWN, OwnerNotFoundError),
        (sysml_pb2.EDIT_FAILURE_OWNER_NOT_NAMESPACE, OwnerNotNamespaceError),
        (sysml_pb2.EDIT_FAILURE_ILLEGAL_KIND, IllegalMemberKindError),
        (sysml_pb2.EDIT_FAILURE_MEMBER_NAME_TAKEN, MemberNameTakenError),
        (sysml_pb2.EDIT_FAILURE_DELETE_REFERENCED, DeleteReferencedError),
        (sysml_pb2.EDIT_FAILURE_OWNER_INSIDE_TARGET, OwnerInsideTargetError),
        (sysml_pb2.EDIT_FAILURE_MOVE_REFERENCED, MoveReferencedError),
    ],
)
def test_every_authoring_failure_is_typed(fake_service, failure, expected):
    port, _ = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING),
        error="refused", failure=failure,
    )
    with Connection(port=port, auto_start=False) as conn:
        add = conn.load_from_content(MODEL).edit().add_part("Demo::SC", "new")
        with pytest.raises(expected):
            add.apply()


def test_inline_language_capability_and_loads(monkeypatch, fake_service):
    import opensysml

    port, _ = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_INLINE_LANGUAGE)
    )
    conn = Connection(port=port, auto_start=False)
    monkeypatch.setattr(opensysml, "_get_default_connection", lambda: conn)
    model = opensysml.loads("namespace N;", language="kerml")
    assert model.root.name == "Demo"


def test_a_symbol_names_its_own_target(fake_service):
    """A Symbol handle names the element the way its id does."""
    port, service = fake_service()

    class FakeSymbol:
        id = "Demo::SC::unitMass"

    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).edit().set_value(FakeSymbol(), "1").apply()

    (request,) = service.requests
    assert request.operations[0].set_value.target == "Demo::SC::unitMass"


def test_a_target_that_names_nothing_is_a_caller_error(fake_service):
    """A target that is neither an id nor a symbol is refused before the call."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        with pytest.raises(TypeError):
            edit.set_value(object(), "1")
        with pytest.raises(TypeError):
            edit.set_value("Demo::SC::unitMass", 1050.0)
        with pytest.raises(TypeError):
            edit.rename("Demo::SC::unitMass", 3)
    assert service.requests == []
    assert len(edit) == 0


def test_the_result_is_a_conversion(fake_service, tmp_path):
    """An edit is written the way a conversion is, and says what it changed."""
    port, _ = fake_service()
    out = tmp_path / "edited.sysml"
    with Connection(port=port, auto_start=False) as conn:
        result = (
            conn.load_from_content(MODEL)
            .edit()
            .set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
            .apply()
        )
    assert isinstance(result, Conversion)
    assert result.save(str(out)) == str(out)
    assert out.read_text() == "edited"
    # write() is the Conversion spelling of the same thing.
    assert result.write(str(out)) == str(out)
    (applied,) = result.applied
    assert (applied.target, applied.old_text, applied.new_text) == (
        "Demo::SC::unitMass", "old", "new",
    )
    assert (applied.offset, applied.length) == (7, 3)


def test_saving_writes_the_service_bytes_verbatim(tmp_path):
    """Line endings are not translated: the file is what the service returned."""
    out = tmp_path / "crlf.sysml"
    content = "package Demo {\r\n\tattribute x = 1;\r\n}\r\n"
    EditResult(
        content=content,
        from_format=FORMAT_SYSML,
        to_format=FORMAT_SYSML,
    ).save(str(out))
    assert out.read_bytes() == content.encode("utf-8")


def test_an_empty_editor_is_not_applied(fake_service):
    """Applying nothing is a mistake, not an empty write."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        assert not edit
        with pytest.raises(NoEditsError):
            edit.apply()
    assert service.requests == []
    assert not edit.applied, "an empty editor was marked as applied"


def test_an_editor_is_applied_once(fake_service):
    """An editor describes an edit of the model it was made from, so it is spent
    after applying: a second apply would edit the pre-edit source again."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        edit.apply()
        assert edit.applied
        with pytest.raises(RuntimeError, match="already been applied"):
            edit.apply()
        with pytest.raises(RuntimeError, match="already been applied"):
            edit.set_value("Demo::SC::margin", "1")
    assert len(service.requests) == 1


@pytest.mark.parametrize(
    "failure,expected",
    [
        (sysml_pb2.EDIT_FAILURE_UNKNOWN_TARGET, EditTargetError),
        (sysml_pb2.EDIT_FAILURE_AMBIGUOUS_TARGET, EditTargetError),
        (sysml_pb2.EDIT_FAILURE_NOT_VALUED, EditTargetError),
        (sysml_pb2.EDIT_FAILURE_NOT_NAMED, EditTargetError),
        (sysml_pb2.EDIT_FAILURE_INVALID_VALUE, InvalidEditError),
        (sysml_pb2.EDIT_FAILURE_INVALID_NAME, InvalidEditError),
        (sysml_pb2.EDIT_FAILURE_RENAME_REFERENCED, RenameReferencedError),
        (sysml_pb2.EDIT_FAILURE_OVERLAPPING_EDITS, OverlappingEditsError),
        (sysml_pb2.EDIT_FAILURE_RESULT_INVALID, EditResultError),
        (sysml_pb2.EDIT_FAILURE_NO_OPERATIONS, NoEditsError),
        # A kind this client has not seen still arrives inside the hierarchy.
        (sysml_pb2.EDIT_FAILURE_UNSPECIFIED, EditError),
    ],
)
def test_a_refusal_becomes_its_own_error(fake_service, failure, expected):
    """Every refusal kind raises the class a caller acts on."""
    port, _ = fake_service(error="refused", failure=failure, diagnostics=2)
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        with pytest.raises(expected) as excinfo:
            edit.apply()
    assert excinfo.value.failure == sysml_pb2.EditFailure.Name(failure)
    assert [d.message for d in excinfo.value.diagnostics] == [
        "edit error 0", "edit error 1",
    ]


def test_a_refusal_kind_newer_than_this_client_stays_an_edit_error(fake_service):
    """proto3 enums are open: an unnamed kind is still an EditError, named by number."""
    unknown = max(sysml_pb2.EditFailure.values()) + 1
    port, _ = fake_service(error="refused for a newer reason", failure=unknown)
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        with pytest.raises(EditError) as excinfo:
            edit.apply()
    assert excinfo.value.failure == f"EDIT_FAILURE_{unknown}"
    assert "newer reason" in str(excinfo.value)


def test_a_refused_rename_names_where_the_references_are(fake_service):
    """The refusal carries the referrers, which is what makes it actionable."""
    port, _ = fake_service(
        error="cannot rename Demo::SC::unitMass: it is referenced",
        failure=sysml_pb2.EDIT_FAILURE_RENAME_REFERENCED,
        referring_elements=("Demo::SC", "Demo::sc"),
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.rename("Demo::SC::unitMass", "unitWeight")
        with pytest.raises(RenameReferencedError) as excinfo:
            edit.apply()
    assert excinfo.value.referring_elements == ["Demo::SC", "Demo::sc"]
    assert excinfo.value.referrers == []


def test_a_refusal_names_each_referrer_with_its_document(fake_service):
    """A referrer in another document of the model arrives with that document."""
    port, _ = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING),
        error="cannot delete Lib::Engine: it is referenced",
        failure=sysml_pb2.EDIT_FAILURE_DELETE_REFERENCED,
        referring_elements=("Car::engine (car.sysml)",),
        referrers=(("Car::engine", "car.sysml"),),
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit().delete("Lib::Engine")
        with pytest.raises(DeleteReferencedError) as excinfo:
            edit.apply()
    assert excinfo.value.referring_elements == ["Car::engine (car.sysml)"]
    assert excinfo.value.referrers == [Referrer("Car::engine", "car.sysml")]


def test_a_referrer_outside_the_model_is_its_own_error(fake_service):
    port, _ = fake_service(
        error="refused", failure=sysml_pb2.EDIT_FAILURE_REFERENCED_ELSEWHERE,
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit().rename("Demo::SC", "Ship")
        with pytest.raises(ReferencedElsewhereError):
            edit.apply()


def test_the_result_lists_the_one_document_it_edited(fake_service):
    """A model of one document answers the same notation twice: content and documents."""
    port, _ = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        result = edit.apply()
    assert result.documents == [EditedDocument(name="<content>", content="edited")]
    assert str(result) == result.documents[0].content == "edited"
    assert [a.document for a in result.applied] == ["<content>"]


def test_a_model_of_several_documents_answers_documents_not_content(fake_service):
    """content is empty for such a model; the rewritten documents carry the notation."""
    port, _ = fake_service(
        content="",
        documents=(("lib.sysml", "package Lib;"), ("car.sysml", "package Car;")),
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit().rename("Lib::Engine", "Motor")
        result = edit.apply()
    assert str(result) == ""
    assert [d.name for d in result.documents] == ["lib.sysml", "car.sysml"]
    assert result.documents[1].content == "package Car;"
    assert result.applied[0].document == "lib.sysml"


def test_a_service_without_edit_documents_answers_content_alone(fake_service):
    """A service lacking the capability edits a model of one document and answers
    content alone: documents stays empty and no applied edit names a document."""
    assert CAPABILITY_EDIT_DOCUMENTS == "edit_documents"
    port, _ = fake_service(legacy=True)
    with Connection(port=port, auto_start=False) as conn:
        assert not conn.server_info().has(CAPABILITY_EDIT_DOCUMENTS)
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        result = edit.apply()
    assert str(result) == "edited"
    assert result.documents == []
    assert [a.document for a in result.applied] == [""]


def test_every_request_accepts_documents(fake_service):
    """The client reads documents, so it says so; the service edits a model of several
    only for a request that does, and refuses one that does not as it always did."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        edit.apply()
    assert [request.accept_documents for request in service.requests] == [True]


def test_an_evicted_model_names_the_eviction(fake_service):
    """A model the service no longer holds is reported as such, as convert does."""
    port, _ = fake_service(not_found=True)
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.set_value("Demo::SC::unitMass", "1050.0[SI::kg]")
        with pytest.raises(ModelNotFoundError) as excinfo:
            edit.apply()
    assert "no longer cached" in str(excinfo.value)
    assert excinfo.value.code == grpc.StatusCode.NOT_FOUND


def test_an_unknown_operation_kind_is_refused(fake_service):
    """The connection's own operation form is checked before anything is sent."""
    port, service = fake_service()
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ValueError, match="malformed delete operation"):
            conn.apply_edits("fake-hash", [("delete", "Demo::SC", "")])
        with pytest.raises(ValueError, match="malformed move operation"):
            conn.apply_edits("fake-hash", [("move", "Demo::SC")])
    assert service.requests == []


@pytest.fixture(scope="module")
def real_service():
    """Run the built sysml-grpc on an ephemeral port, or skip."""
    binary = next((b for b in GRPC_BINARIES if os.access(b, os.X_OK)), None)
    if binary is None:
        pytest.skip(f"no executable sysml-grpc in {GRPC_BINARIES}; run: make build-grpc")

    port = 51153
    process = subprocess.Popen(
        [binary, "-port", str(port)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        deadline = time.time() + 10
        while time.time() < deadline:
            with grpc.insecure_channel(f"localhost:{port}") as channel:
                try:
                    grpc.channel_ready_future(channel).result(timeout=0.5)
                    break
                except grpc.FutureTimeoutError:
                    continue
        else:
            pytest.fail("sysml-grpc did not start")
        yield port
    finally:
        process.terminate()
        process.wait(timeout=10)


@pytest.mark.integration
class TestEditRoundTripAgainstRealService:
    """The round trip itself, through the real edit engine."""

    def test_a_value_is_changed_and_everything_else_is_kept(self, real_service, tmp_path):
        path = tmp_path / "spacecraft.sysml"
        path.write_text(MODEL)
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load(str(path))
            edit = model.edit()
            edit.set_value("Demo::sc::unitMass", "1050.0[SI::kg]")
            result = edit.apply()
            result.save(str(path))

            edited = path.read_text()
            assert "1050.0[SI::kg]" in edited
            # Every byte outside the value's span is the source as it was.
            (applied,) = result.applied
            assert edited[:applied.offset] == MODEL[:applied.offset]
            assert edited[applied.offset + len(applied.new_text):] == (
                MODEL[applied.offset + applied.length:]
            )
            assert "// The mass of one unit, measured on the bench." in edited

            # The saved file is a model, and the new value is what it reports.
            again = conn.load(str(path))
            assert again.ok, [str(d) for d in again.errors]
            value = again.eval("unitMass", subject="Demo::sc")
            assert value.magnitude == pytest.approx(1050.0)
            assert str(value.unit) == "SI::kg"

    def test_authoring_adds_definition_and_part_and_reads_it_back(self, real_service):
        source = "package Demo;\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_part_def("", "Vehicle")
                .add_part("Vehicle", "engine", type="Vehicle")
                .apply()
            )
            edited = str(result)
            again = conn.load_from_content(edited)
            vehicle = again.find("Vehicle")
            assert vehicle is not None
            assert any(part.name == "engine" for part in vehicle.parts())

    def test_add_parameter_precedes_calculation_result(self, real_service):
        source = "calc def C { in x : ScalarValues::Real; x * 2 }\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = model.edit().add_parameter(
                "C", "in", "power", type="ScalarValues::Real"
            ).apply()
            edited = str(result)
            assert edited == (
                "calc def C { in x : ScalarValues::Real; "
                "in ref power : ScalarValues::Real; x * 2 }\n"
            )
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_add_parameter_to_action_definition(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content("action def A;\n")
            result = model.edit().add_parameter(
                "A", "out", "response", type="ScalarValues::Real"
            ).apply()
            edited = str(result)
            assert "out ref response : ScalarValues::Real;" in edited
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_state_transitions_round_trip(self, real_service):
        source = (
            "package P {\n"
            "    attribute def CycleStart;\n"
            "    attribute def CycleEnd;\n"
            "    state def ToastingCycle;\n"
            "}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_state("P::ToastingCycle", "idle")
                .add_state("P::ToastingCycle", "toasting")
                .add_state("P::ToastingCycle::toasting", "heating")
                .add_entry_transition("P::ToastingCycle", "idle")
                .add_transition(
                    "P::ToastingCycle", "idle", "toasting",
                    name="idle_to_toasting", trigger="CycleStart",
                )
                .add_transition(
                    "P::ToastingCycle", "toasting", "idle",
                    name="toasting_to_idle", trigger="CycleEnd",
                )
                .apply()
            )
            edited = str(result)
            assert "entry; then idle;" in edited
            assert (
                "transition idle_to_toasting first idle accept CycleStart then toasting;"
                in edited
            )
            assert (
                "transition toasting_to_idle first toasting accept CycleEnd then idle;"
                in edited
            )
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_calc_helper_adds_inputs_and_bound_result(self, real_service):
        source = "package P {\n    private import ScalarValues::*;\n}\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = model.edit().add_calc_def(
                "P",
                "DeliveredEnergy",
                inputs=[
                    ("power", "ISQ::PowerValue"),
                    ("duration", "ISQ::TimeValue"),
                    ("efficiency", "ScalarValues::Real"),
                ],
                return_type="ISQ::EnergyValue",
                return_expression="power * duration * efficiency",
            ).apply()
            edited = str(result)
            assert "in ref power : ISQ::PowerValue;" in edited
            assert "in ref duration : ISQ::TimeValue;" in edited
            assert "in ref efficiency : ScalarValues::Real;" in edited
            assert (
                "return : ISQ::EnergyValue = power * duration * efficiency;"
                in edited
            )
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_action_helpers_add_nested_parameters_and_succession(self, real_service):
        source = (
            "package BreadHandling {\n"
            "    item def Bread;\n"
            "    item def Toast;\n"
            "}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_action_def(
                    "BreadHandling",
                    "BreadHandling",
                    inputs=[("bread", "Bread")],
                    outputs=[("toast", "Toast")],
                )
                .add_action(
                    "BreadHandling::BreadHandling",
                    "load_bread",
                    inputs=[("bread", "Bread")],
                    outputs=[("loaded", "Bread")],
                )
                .add_action(
                    "BreadHandling::BreadHandling",
                    "eject_toast",
                    inputs=[("loaded", "Bread")],
                    outputs=[("toast", "Toast")],
                )
                .add_succession(
                    "BreadHandling::BreadHandling", "load_bread", "eject_toast"
                )
                .apply()
            )
            edited = str(result)
            assert "action load_bread" in edited
            assert "action eject_toast" in edited
            assert "succession first load_bread then eject_toast;" in edited
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_authoring_adds_an_allocation(self, real_service):
        source = "package Demo { part def System { part a; part b; } }"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = model.edit().add_allocation(
                "Demo::System", "a", "b", name="alloc1"
            ).apply()
        assert "allocation alloc1 allocate a to b;" in str(result)

    def test_a_value_is_added_to_a_feature_that_had_none(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            edit = model.edit()
            edit.set_value("Demo::SC::margin", "50.0[SI::kg]")
            result = edit.apply()

            assert "attribute margin : ISQ::MassValue = 50.0[SI::kg];" in str(result)
            (applied,) = result.applied
            assert applied.length == 0, "an insertion replaced bytes"
            margin = conn.load_from_content(str(result)).eval(
                "margin", subject="Demo::sc"
            )
            assert margin.magnitude == pytest.approx(50.0)

    def test_strings_booleans_expressions_and_nesting(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            result = (
                model.edit()
                .set_value("Demo::SC::label", '"flight-2"')
                .set_value("Demo::SC::active", "false")
                .set_value("Demo::SC::total", "unitMass * 2")
                .set_value("Demo::SC::avionics::board::count", "4")
                .apply()
            )
            edited = str(result)
            assert '= "flight-2"' in edited
            assert "attribute active : ScalarValues::Boolean = false;" in edited
            assert "= unitMass * 2" in edited
            assert "attribute count : ScalarValues::Integer = 4;" in edited
            assert len(result.applied) == 4

            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]
            assert again.eval("label", subject="Demo::sc") == "flight-2"
            assert again.eval("active", subject="Demo::sc") is False

    def test_a_redefining_feature_is_edited_where_it_redefines(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            result = model.edit().set_value("Demo::sc::unitMass", "1300.0[SI::kg]").apply()
            edited = str(result)
            assert "attribute redefines unitMass = 1300.0[SI::kg];" in edited
            # The definition's own value is untouched.
            assert "attribute unitMass : ISQ::MassValue default = 1000.0[SI::kg];" in edited

    def test_an_unreferenced_declaration_is_renamed(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            result = model.edit().rename("Demo::SC::label", "callSign").apply()
            edited = str(result)
            assert "attribute callSign : ScalarValues::String" in edited
            assert conn.load_from_content(edited).find("callSign") is not None

    def test_a_referenced_declaration_is_renamed_with_its_references(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            result = model.edit().rename("Demo::SC::unitMass", "unitWeight").apply()
            edited = str(result)
            assert "attribute unitWeight : ISQ::MassValue default = 1000.0[SI::kg];" in edited
            assert "attribute total : ISQ::MassValue = unitWeight;" in edited
            assert "unitMass" not in edited
            assert conn.load_from_content(edited).find("unitWeight") is not None

    @pytest.mark.parametrize(
        "operation,expected",
        [
            (("set_value", "Demo::SC::nothing", "1"), EditTargetError),
            (("set_value", "Demo::SC", "1"), EditTargetError),
            (("set_value", "Demo::SC::unitMass", "1050.0["), InvalidEditError),
            (("set_value", "Demo::SC::unitMass", "nosuchFeature"), EditResultError),
            (("rename", "Demo::SC::label", "part"), InvalidEditError),
        ],
    )
    def test_refusals_are_typed_and_change_nothing(
        self, real_service, operation, expected
    ):
        kind, target, text = operation
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            edit = model.edit()
            if kind == "set_value":
                edit.set_value(target, text)
            else:
                edit.rename(target, text)
            with pytest.raises(expected) as excinfo:
                edit.apply()
        assert str(excinfo.value), "a refusal carried no message"

    def test_overlapping_edits_are_refused(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(MODEL)
            overlapping = (
                model.edit()
                .set_value("Demo::SC::unitMass", "1.0[SI::kg]")
                .set_value("Demo::SC::unitMass", "2.0[SI::kg]")
            )
            with pytest.raises(OverlappingEditsError):
                overlapping.apply()

    def test_the_service_reports_the_capability(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            assert conn.server_info().has(CAPABILITY_APPLY_EDITS)
