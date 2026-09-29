"""Tests for changing a model and writing it back.

Two layers. Against a fake service, the client's own behavior: the capability
gate, the shape of the request, the editor's own refusals, and how a reported
refusal becomes a typed exception. Against the real ``sysml-grpc``, the round
trip itself — load, set a value, apply, save, load the saved file and ask what
the value is now — which is the part a mock cannot tell you anything about.
"""

import json
import os
import subprocess
import textwrap
import time
from concurrent import futures

import grpc
import pytest

from opensysml.capabilities import (
    CAPABILITY_APPLY_EDITS,
    CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
    CAPABILITY_AUTHORING,
    CAPABILITY_COMMENT_AUTHORING,
    CAPABILITY_CONNECTION_AUTHORING,
    CAPABILITY_IMPLICIT_PARAMETERS,
    CAPABILITY_CONSTRAINT_BODY_AUTHORING,
    CAPABILITY_DOCUMENTATION_AUTHORING,
    CAPABILITY_MEMBER_MODIFIERS,
    CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
    CAPABILITY_SATISFY_AUTHORING,
    CAPABILITY_TRANSITION_AUTHORING,
    CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
    CAPABILITY_METADATA_AUTHORING,
    CAPABILITY_METADATA_PREFIX_AUTHORING,
    CAPABILITY_SEQUENCE_AUTHORING,
    CAPABILITY_STATE_ACTION_AUTHORING,
    CAPABILITY_IMPORT_AUTHORING,
    CAPABILITY_EDIT_DOCUMENTS,
    CAPABILITY_INLINE_LANGUAGE,
    MissingCapabilityError,
)
from opensysml.connection import Connection
from opensysml.conversion import Conversion, FORMAT_SYSML
from opensysml.edit import Body, EditedDocument, EditResult, Editor
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

VERIFICATION_MODEL = """package ToasterDemo {
    private import ISQ::*;
    private import SI::*;
    private import VerificationCases::*;
    part def Toaster { attribute cycleTime : ISQ::DurationValue; }
    requirement def TimelyToast {
        subject toaster : Toaster;
        require constraint { toaster.cycleTime <= 180.0 [SI::s] }
    }
    requirement timely : TimelyToast;
    verification def TimelyToastTest {
        subject toaster : Toaster;
    }
}
"""

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))


def _elements_by_qname(conversion):
    """Describe each api-json element by its @type, qualifiedName and the
    qualified names (or @type, when anonymous) of the elements it relates to,
    so two spellings of one model compare independent of element ids."""
    elements = json.loads(str(conversion))
    by_id = {el["@id"]: el for el in elements if isinstance(el, dict) and "@id" in el}

    def describe(el):
        rels = []
        for key, value in sorted(el.items()):
            refs = value if isinstance(value, list) else [value]
            for ref in refs:
                if isinstance(ref, dict) and "@id" in ref and ref["@id"] in by_id:
                    other = by_id[ref["@id"]]
                    rels.append((key, other.get("qualifiedName") or other.get("@type")))
        return (el.get("@type"), el.get("qualifiedName") or "", tuple(sorted(rels)))

    return sorted(describe(el) for el in elements if isinstance(el, dict))


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
            CAPABILITY_IMPLICIT_PARAMETERS,
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
    ) == ("Demo::C", "", "x", "ScalarValues::Real", "in")
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
            CAPABILITY_IMPORT_AUTHORING,
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
            .add_import(
                "Demo::SC", "ScalarValues::*", visibility="public",
                recursive=True, all=True, filter=["@Safety", "@Approved"],
            )
            .apply()
        )
    (
        member, satisfy, require, assume, transition, entry, declared_import
    ) = service.requests[0].operations
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
    assert declared_import.WhichOneof("operation") == "add_import"
    assert (
        declared_import.add_import.owner,
        declared_import.add_import.visibility,
        declared_import.add_import.target,
        declared_import.add_import.is_recursive,
        declared_import.add_import.is_import_all,
        list(declared_import.add_import.filters),
    ) == ("Demo::SC", "public", "ScalarValues::*", True, True, ["@Safety", "@Approved"])


def test_verify_metadata_objective_and_prefix_operations_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
            CAPABILITY_METADATA_AUTHORING,
            CAPABILITY_METADATA_PREFIX_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_objective("Demo::SC")
            .add_verify("Demo::SC::objective", "Demo::SC::r")
            .add_metadata(
                "Demo::SC", "Demo::VerificationMethod",
                values={"kind": "VerificationMethodKind::test"},
                name="vm", about="Demo::SC",
            )
            .add_metadata(
                "Demo::SC", "Demo::VerificationMethod",
                values=[["kind", "VerificationMethodKind::test"]],
                shorthand=True,
            )
            .add_metadata_prefix("Demo::SC", "Demo::Safety")
            .add_member("Demo::SC", "part", "heater", metadata=["Safety", "Risk"])
            .apply()
        )
    objective, verify, metadata, shorthand, prefix, member = service.requests[0].operations
    assert objective.add_member.kind == "objective"
    assert objective.add_member.name == ""
    assert verify.WhichOneof("operation") == "add_verify"
    assert (verify.add_verify.owner, verify.add_verify.requirement) == (
        "Demo::SC::objective", "Demo::SC::r",
    )
    assert metadata.WhichOneof("operation") == "add_metadata"
    assert (
        metadata.add_metadata.owner,
        metadata.add_metadata.metadata_type,
        metadata.add_metadata.name,
        list(metadata.add_metadata.about),
        metadata.add_metadata.shorthand,
    ) == (
        "Demo::SC", "Demo::VerificationMethod", "vm", ["Demo::SC"], False,
    )
    assert [
        (binding.feature, binding.value)
        for binding in metadata.add_metadata.values
    ] == [("kind", "VerificationMethodKind::test")]
    assert shorthand.add_metadata.shorthand
    assert prefix.WhichOneof("operation") == "add_metadata_prefix"
    assert (prefix.add_metadata_prefix.target, prefix.add_metadata_prefix.metadata_type) == (
        "Demo::SC", "Demo::Safety",
    )
    assert member.add_member.metadata_prefixes == ["Safety", "Risk"]


def test_objective_and_metadata_authoring_tuples_preserve_optional_fields():
    editor = Editor("hash", None)
    editor.add_objective("Demo::Case")
    editor.add_member("Demo::P", "part def", "Heater", metadata="Safety")
    editor.add_metadata(
        "Demo::Case", "M", values={"a": "1"}, about=("Demo::x", "Demo::y"),
        shorthand=True,
    )
    editor.add_metadata_prefix("Demo::P", "M")
    assert editor.operations == [
        ("add_member", "Demo::Case", "objective", "", "", "", "", []),
        (
            "add_member", "Demo::P", "part def", "Heater", "", "", "", [],
            False, [], False, "", ["Safety"],
        ),
        ("add_metadata", "Demo::Case", "M", "", ["Demo::x", "Demo::y"], [("a", "1")], True),
        ("add_metadata_prefix", "Demo::P", "M"),
    ]


@pytest.mark.parametrize("metadata", [42, {"Safety": "ignored"}])
def test_malformed_member_metadata_prefixes_are_rejected(fake_service, metadata):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        editor = conn.load_from_content(MODEL).edit()
        editor._operations.append((
            "add_member", "Demo::SC", "part", "heater", "", "", "", [],
            False, [], False, "", metadata,
        ))
        with pytest.raises(ValueError, match="malformed add_member metadata"):
            editor.apply()
    assert service.requests == []


@pytest.mark.parametrize(
    "operation,missing",
    [
        (lambda editor: editor.add_verify("Demo::Case", "Demo::r"),
         CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING),
        (lambda editor: editor.add_objective("Demo::Case"),
         CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING),
        (lambda editor: editor.add_metadata("Demo", "M"),
         CAPABILITY_METADATA_AUTHORING),
        (lambda editor: editor.add_member("Demo", "part", "p", metadata=["M"]),
         CAPABILITY_METADATA_AUTHORING),
        (lambda editor: editor.add_metadata_prefix("Demo::Part", "M"),
         CAPABILITY_METADATA_PREFIX_AUTHORING),
    ],
)
def test_verification_and_metadata_authoring_capabilities_are_preflighted(
    fake_service, operation, missing
):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = operation(conn.load_from_content(MODEL).edit())
        with pytest.raises(MissingCapabilityError) as error:
            edit.apply()
    assert error.value.capability == missing
    assert service.requests == []


def test_metadata_prefix_authoring_requires_authoring(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_METADATA_PREFIX_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.add_metadata_prefix("Demo::Part", "M")
        with pytest.raises(MissingCapabilityError) as error:
            edit.apply()
    assert error.value.capability == CAPABILITY_AUTHORING
    assert service.requests == []

def test_add_import_accepts_a_single_filter_expression():
    editor = Editor("hash", None)
    editor.add_import("Demo", "A::*", filter="@Safety")
    assert len(editor) == 1


@pytest.mark.parametrize(
    "argument,value",
    [
        ("target", 3),
        ("visibility", 3),
        ("recursive", "yes"),
        ("all", "yes"),
        ("filter", 3),
    ],
)
def test_add_import_rejects_invalid_arguments(argument, value):
    editor = Editor("hash", None)
    with pytest.raises(TypeError, match=argument):
        editor.add_import("Demo", "A::*", **{argument: value})
    assert len(editor) == 0


def test_constraint_body_and_state_action_helpers_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_CONSTRAINT_BODY_AUTHORING,
            CAPABILITY_STATE_ACTION_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_constraint_def("Demo::SC", "C", expression="x > 0")
            .add_constraint("Demo::SC", "usage", expression="x > 0")
            .add_assert_constraint("Demo::SC", "positive", expression="x > 0")
            .add_assert_constraint(
                "Demo::SC", "negative", expression="x < 0", negated=True
            )
            .add_assert_constraint("Demo::SC", type="C")
            .add_assert_constraint("Demo::SC", expression="x > 0")
            .add_assert("Demo::SC", "positive")
            .add_assert("Demo::SC", "negative", negated=True)
            .add_calc_def("Demo::SC", "D", expression="x * 2")
            .add_calc("Demo::SC", "c", expression="x * 3")
            .add_exhibit_state("Demo::SC", "shown", type="S")
            .add_exhibit("Demo::SC", "part.state")
            .add_state_action("Demo::SC", "entry", "start", type="A")
            .add_state_action("Demo::SC", "do", "run", type="A")
            .add_state_action("Demo::SC", "exit", "finish", type="A")
            .apply()
        )

    operations = [operation.add_member for operation in service.requests[0].operations]
    assert [
        (
            operation.kind,
            operation.name,
            operation.type,
            operation.body_expression,
        )
        for operation in operations
    ] == [
        ("constraint def", "C", "", "x > 0"),
        ("constraint", "usage", "", "x > 0"),
        ("assert constraint", "positive", "", "x > 0"),
        ("assert not constraint", "negative", "", "x < 0"),
        ("assert constraint", "", "C", ""),
        ("assert constraint", "", "", "x > 0"),
        ("assert", "positive", "", ""),
        ("assert not", "negative", "", ""),
        ("calc def", "D", "", "x * 2"),
        ("calc", "c", "", "x * 3"),
        ("exhibit state", "shown", "S", ""),
        ("exhibit", "part.state", "", ""),
        ("entry action", "start", "A", ""),
        ("do action", "run", "A", ""),
        ("exit action", "finish", "A", ""),
    ]


@pytest.mark.parametrize(
    "operation,missing",
    [
        (
            lambda editor: editor.add_constraint("Demo::SC", "bounded", expression="x > 0"),
            CAPABILITY_CONSTRAINT_BODY_AUTHORING,
        ),
        (
            lambda editor: editor.add_assert_constraint("Demo::SC", "bounded"),
            CAPABILITY_CONSTRAINT_BODY_AUTHORING,
        ),
        (
            lambda editor: editor.add_assert("Demo::SC", "bounded"),
            CAPABILITY_CONSTRAINT_BODY_AUTHORING,
        ),
        (
            lambda editor: editor.add_calc_def("Demo", "Double", expression="x * 2"),
            CAPABILITY_CONSTRAINT_BODY_AUTHORING,
        ),
        (
            lambda editor: editor.add_exhibit_state("Demo::SC", "running", type="S"),
            CAPABILITY_STATE_ACTION_AUTHORING,
        ),
        (
            lambda editor: editor.add_exhibit("Demo::SC", "part.state"),
            CAPABILITY_STATE_ACTION_AUTHORING,
        ),
        (
            lambda editor: editor.add_state_action("Demo::SC", "do", "run"),
            CAPABILITY_STATE_ACTION_AUTHORING,
        ),
    ],
)
def test_constraint_and_state_capabilities_are_preflighted(
    fake_service, operation, missing
):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        editor = operation(conn.load_from_content(MODEL).edit())
        with pytest.raises(MissingCapabilityError) as error:
            editor.apply()
    assert error.value.capability == missing
    assert service.requests == []


def test_new_add_member_expression_and_helper_arguments_are_checked():
    editor = Editor("hash", None)
    with pytest.raises(TypeError, match="expression must be notation text, not int"):
        editor.add_member("Demo", "constraint", "c", expression=1)
    with pytest.raises(TypeError, match="negated must be bool"):
        editor.add_assert_constraint("Demo", "c", negated=1)
    with pytest.raises(TypeError, match="negated must be bool"):
        editor.add_assert("Demo", "c", negated=1)
    with pytest.raises(TypeError, match="ref must be notation text, not int"):
        editor.add_assert("Demo", 1)
    editor.add_calc_def("Demo", "D", return_type="Real", expression="x * 2")
    editor.add_calc("Demo", "c", return_type="Real", expression="x * 2")
    for method in ("add_calc", "add_calc_def"):
        with pytest.raises(
            ValueError,
            match="^expression and return_expression both bind the result; give one$",
        ):
            getattr(editor, method)(
                "Demo", "C", return_type="Real", return_expression="x",
                expression="x * 2",
            )
    with pytest.raises(TypeError, match="kind must be notation text, not int"):
        editor.add_state_action("Demo::S", 1, "a")
    with pytest.raises(ValueError, match="kind must be 'entry', 'do' or 'exit'"):
        editor.add_state_action("Demo::S", "transition", "a")

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


def test_import_requires_authoring_alongside_import_capability(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_IMPORT_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.add_import("Demo", "ScalarValues::*")
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
        (lambda editor: editor.add_first("Demo::A", "start"),
         CAPABILITY_SEQUENCE_AUTHORING),
        (lambda editor: editor.add_then("Demo::A", ref="done"),
         CAPABILITY_SEQUENCE_AUTHORING),
        (lambda editor: editor.add_then("Demo::A", action="b", type="B"),
         CAPABILITY_SEQUENCE_AUTHORING),
        (lambda editor: editor.add_assign("Demo::A", "x", "1"),
         CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING),
        (lambda editor: editor.add_then("Demo::A", ref="done", multiplicity="[1]"),
         CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING),
        (lambda editor: editor.add_member("Demo::SC", "", "x"),
         CAPABILITY_IMPLICIT_PARAMETERS),
        (lambda editor: editor.add_parameter("Demo::SC", "in", "x"),
         CAPABILITY_MEMBER_MODIFIERS),
        (lambda editor: editor.add_part_def("Demo", "Wheel", doc="A wheel."),
         CAPABILITY_DOCUMENTATION_AUTHORING),
        (lambda editor: editor.add_import("Demo::SC", "A::*"),
         CAPABILITY_IMPORT_AUTHORING),
        (lambda editor: editor.add_documentation("Demo::SC", "A spacecraft."),
         CAPABILITY_DOCUMENTATION_AUTHORING),
        (lambda editor: editor.add_comment("Demo", "A note."), CAPABILITY_COMMENT_AUTHORING),
        (lambda editor: editor.add_note("Demo::SC", "A note."), CAPABILITY_COMMENT_AUTHORING),
    ],
)
def test_new_authoring_capabilities_are_preflighted(fake_service, operation, missing):
    capabilities = [CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING]
    if missing == CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING:
        capabilities.append(CAPABILITY_SEQUENCE_AUTHORING)
    port, service = fake_service(capabilities=tuple(capabilities))
    with Connection(port=port, auto_start=False) as conn:
        editor = operation(conn.load_from_content(MODEL).edit())
        with pytest.raises(MissingCapabilityError) as error:
            editor.apply()
    assert error.value.capability == missing
    assert service.requests == []


def test_add_first_and_add_then_serialize_and_validate(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_SEQUENCE_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_first("Demo::A", "start")
            .add_then("Demo::A", action="b", type="B", after="a")
            .add_then("Demo::A", ref="done")
            .apply()
        )
    first, declared, ref = service.requests[0].operations
    assert first.WhichOneof("operation") == "add_sequence"
    assert (
        first.add_sequence.owner, first.add_sequence.keyword,
        first.add_sequence.ref, first.add_sequence.after,
    ) == ("Demo::A", "first", "start", "")
    assert (
        declared.add_sequence.keyword,
        declared.add_sequence.member_kind,
        declared.add_sequence.member_name,
        declared.add_sequence.type,
        declared.add_sequence.after,
    ) == ("then", "action", "b", "B", "a")
    assert (
        ref.add_sequence.keyword, ref.add_sequence.ref,
    ) == ("then", "done")


def test_action_body_statements_serialize_recursively(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_SEQUENCE_AUTHORING,
            CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
        )
    )
    body = Body().add_assign("x", "x + 1").add_send("x", to="self")
    else_body = Body().add_terminate()
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_if("Demo::A", "x < 3", body, else_body, multiplicity="[1]")
            .apply()
        )
    operation = service.requests[0].operations[0].add_sequence
    assert (
        operation.keyword, operation.member_kind, operation.condition,
        operation.multiplicity,
    ) == ("then", "if", "x < 3", "[1]")
    assert [
        (item.keyword, item.member_kind, item.target, item.value)
        for item in operation.body
    ] == [
        ("", "assign", "x", "x + 1"),
        ("then", "send", "self", "x"),
    ]
    assert [(item.keyword, item.member_kind) for item in operation.else_body] == [
        ("", "terminate"),
    ]


def test_action_body_statement_fields_serialize_recursively(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_SEQUENCE_AUTHORING,
            CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
        )
    )
    body = (
        Body()
        .add_accept("message", type="Signal", via="inPort", then=False)
        .add_send("new Signal(value = 7)", to="self", via="outPort", multiplicity="[1]")
        .add_assign("result", "message.value")
        .add_if(
            "result > 0",
            Body().add_assign("result", "result + 1"),
            Body().add_terminate("message"),
        )
        .add_while("result < 3", Body().add_assign("result", "result + 1"),
                   until="result == 3")
        .add_loop(Body().add_terminate(), until="result == 3")
        .add_for("i", "(1, 2)", Body().add_assign("result", "result + i"),
                 type="Integer")
        .add_terminate("message")
    )
    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).edit().add_if(
            "Demo::A", "true", body, multiplicity="[1]"
        ).apply()

    operation = service.requests[0].operations[0].add_sequence
    assert operation.multiplicity == "[1]"
    assert [item.member_kind for item in operation.body] == [
        "accept", "send", "assign", "if", "while", "loop", "for", "terminate",
    ]
    accept, send, assign, conditional, loop, post_loop, iteration, terminate = (
        operation.body
    )
    assert (accept.keyword, accept.parameter, accept.type, accept.via) == (
        "", "message", "Signal", "inPort",
    )
    assert (send.keyword, send.value, send.target, send.via, send.multiplicity) == (
        "then", "new Signal(value = 7)", "self", "outPort", "[1]",
    )
    assert (assign.target, assign.value) == ("result", "message.value")
    assert conditional.condition == "result > 0"
    assert conditional.body[0].member_kind == "assign"
    assert conditional.else_body[0].value == "message"
    assert (loop.condition, loop.until, loop.body[0].value) == (
        "result < 3", "result == 3", "result + 1",
    )
    assert (post_loop.until, post_loop.body[0].member_kind) == (
        "result == 3", "terminate",
    )
    assert (iteration.parameter, iteration.type, iteration.value) == (
        "i", "Integer", "(1, 2)",
    )
    assert iteration.body[0].target == "result"
    assert terminate.value == "message"


def test_add_then_multiplicity_serializes(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_SEQUENCE_AUTHORING,
            CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).edit().add_then(
            "Demo::A", ref="done", multiplicity="[1]"
        ).apply()
    operation = service.requests[0].operations[0].add_sequence
    assert (operation.keyword, operation.ref, operation.multiplicity) == (
        "then", "done", "[1]",
    )


def test_add_then_member_multiplicity_serializes(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_SEQUENCE_AUTHORING,
            CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).edit().add_then(
            "Demo::A", action="b", multiplicity="[0..1]"
        ).apply()
    operation = service.requests[0].operations[0].add_sequence
    assert (
        operation.keyword,
        operation.member_kind,
        operation.member_name,
        operation.multiplicity,
    ) == ("then", "action", "b", "[0..1]")


def test_empty_else_body_is_omitted_and_body_is_retained(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_SEQUENCE_AUTHORING,
            CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).edit().add_if(
            "Demo::A", "true", Body(), Body()
        ).apply()
    operation = service.requests[0].operations[0].add_sequence
    assert operation.body == []
    assert operation.else_body == []


def test_add_then_requires_exactly_one_of_ref_and_action():
    editor = Editor("hash", None)
    with pytest.raises(ValueError, match="exactly one of ref and action"):
        editor.add_then("Demo::A")
    with pytest.raises(ValueError, match="exactly one of ref and action"):
        editor.add_then("Demo::A", ref="done", action="b")


def test_add_then_reference_takes_no_declaration_fields():
    editor = Editor("hash", None)
    with pytest.raises(ValueError, match="no type or kind"):
        editor.add_then("Demo::A", ref="done", type="B")
    with pytest.raises(ValueError, match="no type or kind"):
        editor.add_then("Demo::A", ref="done", kind="merge")
    editor.add_then("Demo::A", ref="done")
    editor.add_then("Demo::A", ref="done", kind="action")
    with pytest.raises(ValueError, match="exactly one of ref and action"):
        editor.add_then("Demo::A", ref="done", action="b")
    with pytest.raises(TypeError, match="ref must be notation text"):
        editor.add_then("Demo::A", ref=3)
    with pytest.raises(TypeError, match="kind must be notation text"):
        editor.add_then("Demo::A", action="b", kind=3)
    with pytest.raises(TypeError, match="ref must be notation text"):
        editor.add_first("Demo::A", None)


def test_sequence_requires_authoring_alongside_sequence_capability(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_SEQUENCE_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        edit = conn.load_from_content(MODEL).edit()
        edit.add_first("Demo::A", "start")
        with pytest.raises(MissingCapabilityError) as error:
            edit.apply()
    assert error.value.capability == CAPABILITY_AUTHORING
    assert service.requests == []

def test_documentation_requests_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_MEMBER_MODIFIERS,
            CAPABILITY_IMPLICIT_PARAMETERS,
            CAPABILITY_DOCUMENTATION_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        (
            model.edit()
            .add_part_def("Demo", "Wheel", doc="A wheel.\nRound.")
            .add_parameter("Demo::SC", "in", "power", type="Real", doc="Power in.")
            .add_member("Demo::SC", "attribute", "plain")
            .add_documentation("Demo::SC", "A spacecraft.")
            .add_documentation(
                "Demo::SC::unitMass", "Bench mass.",
                name="Mass", locale="en_US", replace=True,
            )
            .apply()
        )
    part_def, parameter, plain, documented, replaced = service.requests[0].operations
    assert part_def.WhichOneof("operation") == "add_member"
    assert (part_def.add_member.kind, part_def.add_member.name, part_def.add_member.doc) == (
        "part def", "Wheel", "A wheel.\nRound.",
    )
    assert (
        parameter.add_member.kind, parameter.add_member.direction, parameter.add_member.doc,
    ) == ("", "in", "Power in.")
    assert plain.add_member.doc == ""
    assert documented.WhichOneof("operation") == "add_documentation"
    doc = documented.add_documentation
    assert (doc.target, doc.body, doc.name, doc.locale, doc.replace) == (
        "Demo::SC", "A spacecraft.", "", "", False,
    )
    doc = replaced.add_documentation
    assert (doc.target, doc.body, doc.name, doc.locale, doc.replace) == (
        "Demo::SC::unitMass", "Bench mass.", "Mass", "en_US", True,
    )


def test_add_member_carries_documentation_and_body_expression(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_CONSTRAINT_BODY_AUTHORING,
            CAPABILITY_DOCUMENTATION_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        (
            conn.load_from_content(MODEL)
            .edit()
            .add_member(
                "Demo::SC", "constraint", "bounded",
                expression="true", doc="Checks true.",
            )
            .add_member("Demo", "part def", "Wheel", doc="A wheel.")
            .apply()
        )
    bounded, documented = service.requests[0].operations
    assert (
        bounded.add_member.body_expression,
        bounded.add_member.doc,
    ) == ("true", "Checks true.")
    assert (
        documented.add_member.body_expression,
        documented.add_member.doc,
    ) == ("", "A wheel.")


@pytest.mark.parametrize("method", [
    "add_part_def", "add_part", "add_attribute", "add_calc_def", "add_calc",
    "add_action_def", "add_action", "add_perform_action", "add_state_def",
    "add_state", "add_constraint_def", "add_constraint", "add_requirement_def",
    "add_requirement", "add_item_def", "add_port_def",
])
def test_every_member_helper_carries_documentation(method):
    editor = Editor("hash", None)
    getattr(editor, method)("Demo", "x", doc="Text.")
    (operation,) = [op for op in editor.operations if op[3] == "x"]
    assert operation[0] == "add_member"
    assert operation[-1] == "Text."


def test_parameter_return_and_perform_helpers_carry_documentation():
    editor = Editor("hash", None)
    editor.add_parameter("Demo::A", "in", "x", doc="In.")
    editor.add_return("Demo::C", "r", doc="Out.")
    editor.add_perform("Demo::P", "Demo::A", doc="Done.")
    assert [op[-1] for op in editor.operations] == ["In.", "Out.", "Done."]


@pytest.mark.parametrize(
    "call,error",
    [
        (lambda e: e.add_member("Demo", "part", "x", doc=3), "doc must be text, not int"),
        (lambda e: e.add_documentation("Demo", None), "body must be text, not NoneType"),
        (lambda e: e.add_documentation("Demo", "b", name=3), "name must be text, not int"),
        (lambda e: e.add_documentation("Demo", "b", locale=3), "locale must be text, not int"),
        (lambda e: e.add_documentation("Demo", "b", replace="yes"),
         "replace must be a bool, not str"),
    ],
)
def test_documentation_arguments_are_type_checked(call, error):
    editor = Editor("hash", None)
    with pytest.raises(TypeError) as excinfo:
        call(editor)
    assert str(excinfo.value) == error
    assert len(editor) == 0


def test_comment_and_note_requests_are_exact(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        model = conn.load_from_content(MODEL)
        (
            model.edit()
            .add_comment("", "Top.")
            .add_comment(
                "Demo", " Two\nlines ", name="Why", about=["Demo::SC", "Demo"], locale="en",
            )
            .add_note("Demo::SC", "DimensionOneValue")
            .apply()
        )
    top, full, note = service.requests[0].operations
    assert top.WhichOneof("operation") == "add_comment"
    comment = top.add_comment
    assert (comment.owner, comment.body, comment.name, list(comment.about), comment.locale) == (
        "", "Top.", "", [], "",
    )
    comment = full.add_comment
    assert (comment.owner, comment.body, comment.name, list(comment.about), comment.locale) == (
        "Demo", " Two\nlines ", "Why", ["Demo::SC", "Demo"], "en",
    )
    assert note.WhichOneof("operation") == "add_note"
    assert (note.add_note.target, note.add_note.text) == ("Demo::SC", "DimensionOneValue")


@pytest.mark.parametrize(
    "call,error",
    [
        (lambda e: e.add_comment("Demo", None), "body must be text, not NoneType"),
        (lambda e: e.add_comment("Demo", "b", name=3), "name must be text, not int"),
        (lambda e: e.add_comment("Demo", "b", locale=3), "locale must be text, not int"),
        (lambda e: e.add_comment("Demo", "b", about="Demo::SC"),
         "about must be a sequence of names or symbols, not one name"),
        (lambda e: e.add_comment("Demo", "b", about=[3]),
         "target must be a symbol id (FQN) or a Symbol, not int"),
        (lambda e: e.add_note("Demo::SC", None), "text must be text, not NoneType"),
    ],
)
def test_comment_and_note_arguments_are_type_checked(call, error):
    editor = Editor("hash", None)
    with pytest.raises(TypeError) as excinfo:
        call(editor)
    assert str(excinfo.value) == error
    assert len(editor) == 0


@pytest.mark.parametrize("text", ["one\ntwo", "one\rtwo", "trailing\n"])
def test_a_note_of_several_lines_is_refused(text):
    editor = Editor("hash", None)
    with pytest.raises(ValueError, match="a note is one line"):
        editor.add_note("Demo::SC", text)
    assert len(editor) == 0


def test_malformed_comment_and_note_operations_are_refused(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ValueError, match="malformed add_comment operation"):
            conn.apply_edits("fake-hash", [("add_comment", "Demo", "b")])
        with pytest.raises(ValueError, match="malformed add_comment operation"):
            conn.apply_edits("fake-hash", [("add_comment", "Demo", "b", "", "Demo::SC", "")])
        with pytest.raises(ValueError, match="malformed add_comment operation"):
            conn.apply_edits(
                "fake-hash",
                [("add_comment", "Demo", "b", "", (name for name in ["Demo::SC"]), "")],
            )
        with pytest.raises(ValueError, match="malformed add_note operation"):
            conn.apply_edits("fake-hash", [("add_note", "Demo::SC")])
        with pytest.raises(ValueError, match="malformed add_note operation"):
            conn.apply_edits("fake-hash", [("add_note", "Demo::SC", 3)])
    assert service.requests == []


def test_malformed_documentation_operations_are_refused(fake_service):
    port, service = fake_service(
        capabilities=(
            CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING, CAPABILITY_DOCUMENTATION_AUTHORING,
        )
    )
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ValueError, match="malformed add_documentation operation"):
            conn.apply_edits("fake-hash", [("add_documentation", "Demo", "b")])
        with pytest.raises(ValueError, match="malformed add_documentation operation"):
            conn.apply_edits("fake-hash", [("add_documentation", "Demo", "b", "", "", "no")])
        with pytest.raises(ValueError, match="doc must be text"):
            conn.apply_edits("fake-hash", [(
                "add_member", "Demo", "part", "x", "", "", "", [],
                False, [], False, "", "", 3,
            )])
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
        ("add_calc", "calc"), ("add_perform_action", "perform action"),
    ],
)
def test_every_typed_helper_uses_service_kind(fake_service, method, kind):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        getattr(conn.load_from_content(MODEL).edit(), method)("Demo::SC", "New").apply()
    assert service.requests[0].operations[0].add_member.kind == kind


def test_add_perform_names_the_performed_action_by_reference(fake_service):
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        conn.load_from_content(MODEL).edit().add_perform(
            "Demo::SC", "t.heat"
        ).apply()
    operation = service.requests[0].operations[0]
    assert operation.WhichOneof("operation") == "add_member"
    assert operation.add_member.kind == "perform"
    assert operation.add_member.name == "t.heat"


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
    port, service = fake_service(
        capabilities=(CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING)
    )
    with Connection(port=port, auto_start=False) as conn:
        with pytest.raises(ValueError, match="malformed delete operation"):
            conn.apply_edits("fake-hash", [("delete", "Demo::SC", "")])
        with pytest.raises(ValueError, match="malformed move operation"):
            conn.apply_edits("fake-hash", [("move", "Demo::SC")])
        for fields in (("add_member",), ("add_member",) * 9, ("add_member",) * 16):
            with pytest.raises(
                ValueError, match="expected 8, 12, 13, 14 or 15 fields"
            ):
                conn.apply_edits("fake-hash", [fields])
        for expression in (1, 0, None):
            with pytest.raises(ValueError, match="expression must be notation text"):
                conn.apply_edits(
                    "fake-hash",
                    [(
                        "add_member", "P", "constraint", "c", "", "", "", [],
                        False, [], False, "", [], expression,
                    )],
                )
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

    def test_constraint_assert_exhibit_and_state_action_forms(self, real_service):
        source = """package P {
    private import ScalarValues::*;
    action def A;
    constraint def ConstraintType;
    attribute x : Real = 1.0;
    state def S { state active; }
    part def Base { state cycle : S; }
    part def Host :> Base;
}
"""
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_constraint_def("P", "Positive", expression="x > 0")
                .add_constraint(
                    "P", "Bounded", value="true", expression="x > 0"
                )
                .add_assert_constraint("P", "checked", expression="true")
                .add_assert_constraint("P", type="ConstraintType")
                .add_assert_constraint(
                    "P", "notChecked", expression="false", negated=True
                )
                .add_exhibit_state("P::Host", "shown", type="S")
                .add_exhibit("P::Host", "cycle")
                .add_member("P::Host::shown", "state", "nested")
                .add_state_action("P::S::active", "entry", "onEntry", type="A")
                .add_state_action("P::S::active", "do", "work", type="A")
                .add_member(
                    "P::S::active::work",
                    "attribute",
                    "input",
                    type="ScalarValues::Real",
                    direction="in",
                )
                .add_state_action("P::S::active", "exit", "onExit", type="A")
                .apply()
            )
            edited = str(result)
            assert "constraint def Positive { x > 0 }" in edited
            assert "constraint Bounded = true { x > 0 }" in edited
            assert "assert constraint checked { true }" in edited
            assert "assert constraint : ConstraintType;" in edited
            assert "assert not constraint notChecked { false }" in edited
            assert "exhibit state shown : S {" in edited
            assert "exhibit cycle;" in edited
            assert "do action work : A {" in edited
            assert "in attribute input : ScalarValues::Real;" in edited
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_toaster_constraint_exhibit_and_state_action_authoring_matches_target(
        self, real_service
    ):
        start = """package ToasterDemo {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;
    action def GenerateHeat { in energyIn : ISQ::EnergyValue[0..*]; }
    action def ApplyHeat {
        in energy : ISQ::EnergyValue[0..*];
        out delivered : ISQ::EnergyValue;
        out loss : ISQ::EnergyValue;
    }
    state def Cycle { entry; then idle; state idle; state heating; }
    part def ToastingSystem;
    part heatGenCheck { attribute efficiency : Real; attribute power : ISQ::PowerValue; }
    attribute heatGenCheckDuration : ISQ::DurationValue;
}
"""
        expression = textwrap.dedent("""\
            (heatGenCheck.efficiency >= 0.0 and heatGenCheck.efficiency <= 1.0
             and heatGenCheck.power >= 0.0 [SI::W] and heatGenCheckDuration >= 0.0 [SI::s])
            implies (heatGenCheck.power * heatGenCheckDuration * heatGenCheck.efficiency)
                    <= heatGenCheck.power * heatGenCheckDuration
        """).rstrip()
        target = """package ToasterDemo {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::*;
    action def GenerateHeat { in energyIn : ISQ::EnergyValue[0..*]; }
    action def ApplyHeat {
        in energy : ISQ::EnergyValue[0..*];
        out delivered : ISQ::EnergyValue;
        out loss : ISQ::EnergyValue;
        assert constraint balance {
            delivered >= 0.0 [SI::J] and loss >= 0.0 [SI::J] and delivered + loss <= energy
        }
    }
    state def Cycle { entry; then idle; state idle; state heating {
            do action generateHeat : GenerateHeat;
        } }
    part def ToastingSystem {
        exhibit state cycle : Cycle;
    }
    part heatGenCheck { attribute efficiency : Real; attribute power : ISQ::PowerValue; }
    attribute heatGenCheckDuration : ISQ::DurationValue;
    assert constraint deliveredEnergyBoundedBySupply {
        (heatGenCheck.efficiency >= 0.0 and heatGenCheck.efficiency <= 1.0
         and heatGenCheck.power >= 0.0 [SI::W] and heatGenCheckDuration >= 0.0 [SI::s])
        implies (heatGenCheck.power * heatGenCheckDuration * heatGenCheck.efficiency)
                <= heatGenCheck.power * heatGenCheckDuration
    }
}
"""
        with Connection(port=real_service, auto_start=False) as conn:
            source_model = conn.load_from_content(start)
            edited = (
                source_model.edit()
                .add_assert_constraint(
                    "ToasterDemo::ApplyHeat",
                    "balance",
                    expression=(
                        "delivered >= 0.0 [SI::J] and loss >= 0.0 [SI::J] "
                        "and delivered + loss <= energy"
                    ),
                )
                .add_exhibit_state(
                    "ToasterDemo::ToastingSystem", "cycle", type="Cycle"
                )
                .add_state_action(
                    "ToasterDemo::Cycle::heating",
                    "do",
                    "generateHeat",
                    type="GenerateHeat",
                )
                .add_assert_constraint(
                    "ToasterDemo",
                    "deliveredEnergyBoundedBySupply",
                    expression=expression,
                )
                .apply()
            )
            edited_text = str(edited)
            assert (
                "assert constraint deliveredEnergyBoundedBySupply {\n"
                "        (heatGenCheck.efficiency >= 0.0 and "
                "heatGenCheck.efficiency <= 1.0\n"
                "         and heatGenCheck.power >= 0.0 [SI::W] and "
                "heatGenCheckDuration >= 0.0 [SI::s])\n"
                "        implies (heatGenCheck.power * heatGenCheckDuration * "
                "heatGenCheck.efficiency)\n"
                "                <= heatGenCheck.power * heatGenCheckDuration\n"
                "    }"
            ) in edited_text
            edited_model = conn.load_from_content(edited_text)
            target_model = conn.load_from_content(target)
            assert edited_model.ok, [str(d) for d in edited_model.errors]
            assert target_model.ok, [str(d) for d in target_model.errors]

            def authoring_elements(model):
                data = json.loads(str(model.to_api_json()))
                elements = data if isinstance(data, list) else data.get("elements", data)
                return {
                    (element.get("@type"), element.get("qualifiedName"))
                    for element in elements
                    if element.get("@type") in {
                        "AssertConstraintUsage",
                        "ExhibitStateUsage",
                        "StateSubactionMembership",
                        "PerformActionUsage",
                    }
                }

            assert authoring_elements(edited_model) == authoring_elements(target_model)

    def test_constraint_verification_and_state_execution_match_parsed_target(
        self, real_service
    ):
        source = """package P {
    private import ScalarValues::*;
    attribute x : Real = 3.0;
    action def Work;
    state def Machine {
        entry; then active;
        state active;
    }
}
"""
        target = """package P {
    private import ScalarValues::*;
    attribute x : Real = 3.0;
    action def Work;
    state def Machine {
        entry; then active;
        state active {
            do action work : Work;
        }
    }
    assert constraint holding { x == 3.0 }
    assert not constraint negated { x < 0.0 }
    assert constraint violated { x < 0.0 }
}
"""
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            edited = (
                model.edit()
                .add_state_action("P::Machine::active", "do", "work", type="Work")
                .add_assert_constraint("P", "holding", expression="x == 3.0")
                .add_assert_constraint(
                    "P", "negated", expression="x < 0.0", negated=True
                )
                .add_assert_constraint("P", "violated", expression="x < 0.0")
                .apply()
            )
            edited_model = conn.load_from_content(str(edited))
            target_model = conn.load_from_content(target)
            assert edited_model.ok, [str(d) for d in edited_model.errors]
            assert target_model.ok, [str(d) for d in target_model.errors]

            for symbol, expected in (
                ("P::holding", True),
                ("P::negated", True),
                ("P::violated", False),
            ):
                edited_verdict = edited_model.verify_constraint(symbol)
                target_verdict = target_model.verify_constraint(symbol)
                assert edited_verdict.holds is expected
                assert edited_verdict.holds == target_verdict.holds
                assert edited_verdict.condition == target_verdict.condition

            edited_run = edited_model.execute_state("P::Machine")
            target_run = target_model.execute_state("P::Machine")
            assert edited_run == target_run

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
                "in power : ScalarValues::Real; x * 2 }\n"
            )
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_reference_assertions_and_calculation_result_expression(self, real_service):
        source = """package P {
    private import ScalarValues::*;
    constraint c;
}
"""
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_assert("P", "c")
                .add_assert("P", "c", negated=True)
                .add_calc_def(
                    "P", "D", inputs=[("x", "ScalarValues::Real")],
                    expression="x * 2",
                )
                .apply()
            )
            edited = str(result)
            assert "assert c;" in edited
            assert "assert not c;" in edited
            assert (
                "calc def D { in x : ScalarValues::Real; x * 2 }"
                in edited
            )
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]
            assert again.calc("P::D", arguments=[3]).value == 6

    def test_add_parameter_to_action_definition(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content("action def A;\n")
            result = model.edit().add_parameter(
                "A", "out", "response", type="ScalarValues::Real"
            ).apply()
            edited = str(result)
            assert "out response : ScalarValues::Real;" in edited
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_editor_spelling_matches_written_equivalents(self, real_service):
        """Editor-built `specializes` and directed parameters yield the same
        elements as the directly written `:>` and `in x : T` spellings."""
        with Connection(port=real_service, auto_start=False) as conn:
            built = conn.load_from_content(
                "package P {\n    part def A;\n    action def Do;\n}\n"
            )
            result = (
                built.edit()
                .add_part_def("P", "B", specializes=["P::A"])
                .add_parameter("P::Do", "in", "x", type="ScalarValues::Real")
                .apply()
            )
            edited = conn.load_from_content(str(result))
            assert edited.ok, [str(d) for d in edited.errors]
            written = conn.load_from_content(
                "package P {\n"
                "    part def A;\n"
                "    part def B :> A;\n"
                "    action def Do {\n"
                "        in x : ScalarValues::Real;\n"
                "    }\n"
                "}\n"
            )
            assert _elements_by_qname(edited.to_api_json()) == \
                _elements_by_qname(written.to_api_json())

    def test_add_parameter_explicit_ref_kind_writes_ref(self, real_service):
        source = "calc def C;\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = model.edit().add_parameter(
                "C", "in", "power", type="ScalarValues::Real", kind="ref"
            ).apply()
            edited = str(result)
            assert "in ref power : ScalarValues::Real;" in edited
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

    @pytest.mark.parametrize(
        "shorthand,metadata_line",
        [
            (
                False,
                "metadata VerificationMethod { kind = VerificationMethodKind::test; }",
            ),
            (
                True,
                "@VerificationMethod { kind = VerificationMethodKind::test; }",
            ),
        ],
    )
    def test_verification_objective_and_metadata_reproduce_api_results(
        self, real_service, shorthand, metadata_line
    ):
        expected = VERIFICATION_MODEL.replace(
            "        subject toaster : Toaster;\n    }\n}",
            "        subject toaster : Toaster;\n"
            "        objective {\n"
            "            verify timely;\n"
            "        }\n"
            f"        {metadata_line}\n"
            "    }\n}",
        )

        def api_signature(model):
            elements = json.loads(str(model.to_api_json()))
            if isinstance(elements, dict):
                elements = elements.get("elements", [elements])
            signature = [
                (element.get("@type"), element.get("qualifiedName"))
                for element in elements
            ]
            return sorted(
                signature,
                key=lambda item: (item[0] or "", item[1] or ""),
            )

        def metadata_query(model):
            return model.query(
                select=["@type", "qualifiedName"],
                where={
                    "@type": "PrimitiveConstraint",
                    "operator": "=",
                    "property": "@type",
                    "value": ["MetadataUsage"],
                },
            )

        def verification_signature(model):
            verdict = model.verify_requirement("ToasterDemo::timely")
            return [
                (entry.case_id, entry.kind, entry.detail, entry.subcase, entry.requirement_id)
                for entry in verdict.verifications
            ]

        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(VERIFICATION_MODEL)
            result = (
                model.edit()
                .add_verify("ToasterDemo::TimelyToastTest", "timely")
                .add_metadata(
                    "ToasterDemo::TimelyToastTest",
                    "VerificationMethod",
                    values={"kind": "VerificationMethodKind::test"},
                    shorthand=shorthand,
                )
                .apply()
            )
            authored_text = str(result)
            assert authored_text == expected
            authored = conn.load_from_content(authored_text)
            direct = conn.load_from_content(expected)
            assert authored.ok, [str(d) for d in authored.errors]
            assert direct.ok, [str(d) for d in direct.errors]
            assert api_signature(authored) == api_signature(direct)
            assert [
                element.as_dict() for element in metadata_query(authored)
            ] == [
                element.as_dict() for element in metadata_query(direct)
            ]
            authored_verdicts = verification_signature(authored)
            direct_verdicts = verification_signature(direct)
            assert authored_verdicts == direct_verdicts

    def test_metadata_prefix_is_written_on_a_new_part_definition(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content("metadata def Safety;\npackage P;\n")
            result = model.edit().add_part_def("P", "Heater", metadata=["Safety"]).apply()
            edited = str(result)
            assert edited == (
                "metadata def Safety;\n"
                "package P {\n"
                "    #Safety part def Heater;\n"
                "}\n"
            )
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_metadata_prefix_is_added_to_an_existing_declaration(self, real_service):
        source = "metadata def Safety;\npart def Vehicle;\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = model.edit().add_metadata_prefix("Vehicle", "Safety").apply()
            edited = str(result)
            assert edited == "metadata def Safety;\n#Safety part def Vehicle;\n"
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_import_declarations_round_trip_every_form(self, real_service):
        source = (
            "package Q {\n"
            "    metadata def Safety;\n"
            "    metadata def Approved;\n"
            "}\n"
            "package P {\n"
            "    part x;\n"
            "}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_import("P", "ScalarValues::*")
                .add_import("P", "ISQ::MassValue", visibility="public")
                .add_import("P", "ISQ::MassValue", visibility="protected")
                .add_import("P", "ISQ::*", recursive=True)
                .add_import("P", "Q::*", all=True)
                .add_import("P", "Q::*", filter=["@Q::Safety", "@Q::Approved"])
                .add_import("P", "$::Q::*")
                .apply()
            )
            edited = str(result)
            for line in (
                "private import ScalarValues::*;",
                "public import ISQ::MassValue;",
                "protected import ISQ::MassValue;",
                "private import ISQ::*::**;",
                "private import all Q::*;",
                "private import Q::*[@Q::Safety][@Q::Approved];",
                "private import $::Q::*;",
            ):
                assert "    " + line in edited
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_an_import_at_the_document_root_is_private(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content("package P {\n    part x;\n}\n")
            result = model.edit().add_import("", "ScalarValues::*").apply()
            assert str(result).startswith("private import ScalarValues::*;\n")

    def test_import_refusals_map_to_typed_errors(self, real_service):
        source = "package P {\n    private import ScalarValues::*;\n    part x;\n}\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            with pytest.raises(OwnerNotFoundError):
                model.edit().add_import("P::nope", "ScalarValues::*").apply()
            with pytest.raises(EditResultError):
                model.edit().add_import("P", "Nope::*").apply()
            with pytest.raises(MemberNameTakenError):
                model.edit().add_import("P", "ScalarValues::*").apply()

    def test_imports_author_typed_members_that_resolve_downstream(self, real_service):
        source = "package ToasterDemo { }\n"
        target = (
            "package ToasterDemo {\n"
            "    private import ScalarValues::*;\n"
            "    private import SI::*;\n"
            "    private import ISQ::*;\n"
            "    private import MeasurementReferences::*;\n"
            "    attribute efficiency : DimensionOneValue;\n"
            "}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            result = (
                conn.load_from_content(source)
                .edit()
                .add_import("ToasterDemo", "ScalarValues::*")
                .add_import("ToasterDemo", "SI::*")
                .add_import("ToasterDemo", "ISQ::*")
                .add_import("ToasterDemo", "MeasurementReferences::*")
                .add_member(
                    "ToasterDemo", "attribute", "efficiency",
                    type="DimensionOneValue",
                )
                .apply()
            )
            edited = str(result)
            assert edited == target
            model = conn.load_from_content(edited)
            assert model.ok, [str(d) for d in model.errors]
            efficiency = model.get("ToasterDemo::efficiency")
            assert (
                efficiency.type_facts.resolved_id
                == "MeasurementReferences::DimensionOneValue"
            )

            def element_pairs(conversion):
                return sorted(
                    (element.get("@type"), element.get("qualifiedName"))
                    for element in json.loads(str(conversion))
                )

            expected = conn.load_from_content(target)
            assert expected.ok, [str(d) for d in expected.errors]
            assert element_pairs(model.to_api_json()) == element_pairs(
                expected.to_api_json()
            )

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
            assert "in power : ISQ::PowerValue;" in edited
            assert "in duration : ISQ::TimeValue;" in edited
            assert "in efficiency : ScalarValues::Real;" in edited
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

    def test_authoring_adds_perform_action_usages(self, real_service):
        source = (
            "package Demo {\n"
            "    action def ToastBread;\n"
            "    part def Toaster {\n"
            "        action heat : ToastBread;\n"
            "    }\n"
            "    part def Kitchen {\n"
            "        part t : Toaster;\n"
            "    }\n"
            "}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            declared = model.edit().add_perform_action(
                "Demo::Kitchen", "toast", type="ToastBread"
            ).apply()
            result = (
                conn.load_from_content(str(declared))
                .edit()
                .add_perform("Demo::Kitchen", "t.heat")
                .apply()
            )
            edited = str(result)
            assert "perform action toast : ToastBread;" in edited
            assert "perform t.heat;" in edited
            again = conn.load_from_content(edited)
            assert again.ok, [str(d) for d in again.errors]

    def test_add_member_accepts_the_perform_action_kind(self, real_service):
        """``perform action`` is a member kind, not an illegal kind."""
        source = "package Demo { action def ToastBread; part def Kitchen; }"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = model.edit().add_member(
                "Demo::Kitchen", "perform action", "heat", type="ToastBread"
            ).apply()
            assert "perform action heat : ToastBread;" in str(result)

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

    @pytest.mark.filterwarnings("ignore::opensysml.conversion.ExperimentalFeatureWarning")
    def test_the_toaster_tutorial_documentation_is_built_by_the_editor(self, real_service):
        start = (
            "package ToasterDemo {\n"
            "    item def Bread;\n"
            "    item def Toast;\n"
            "}\n"
        )
        target = (
            "package ToasterDemo {\n"
            "    item def Bread {\n"
            "        doc /* A slice of bread, before toasting.*/\n"
            "    }\n"
            "    item def Toast;\n"
            "    action def ToastBread {\n"
            "        doc /* Transform bread into toast acceptable to its user.*/\n"
            "        in bread : Bread;\n"
            "        out toast : Toast;\n"
            "    }\n"
            "    action def ApplyHeat {\n"
            "        in duration : ISQ::DurationValue[0..*] {\n"
            "            doc /* Signal from a control function: how long to apply heat.\n"
            "             * No control function is modeled in this chapter, so this input\n"
            "             * is declared and typed but not yet connected to a value.*/\n"
            "        }\n"
            "    }\n"
            "}\n"
        )
        heat = (
            "Signal from a control function: how long to apply heat.\n"
            "No control function is modeled in this chapter, so this input\n"
            "is declared and typed but not yet connected to a value."
        )
        with Connection(port=real_service, auto_start=False) as conn:
            result = (
                conn.load_from_content(start).edit()
                .add_documentation("ToasterDemo::Bread", "A slice of bread, before toasting.")
                .add_action_def(
                    "ToasterDemo", "ToastBread",
                    inputs=[("bread", "Bread")], outputs=[("toast", "Toast")],
                    doc="Transform bread into toast acceptable to its user.",
                )
                .add_action_def("ToasterDemo", "ApplyHeat")
                .add_parameter(
                    "ToasterDemo::ApplyHeat", "in", "duration",
                    type="ISQ::DurationValue", multiplicity="[0..*]", doc=heat,
                )
                .apply()
            )
            edited = str(result)
            assert edited == (
                "package ToasterDemo {\n"
                "    item def Bread {\n"
                "        doc /* A slice of bread, before toasting.*/\n"
                "    }\n"
                "    item def Toast;\n"
                "    action def ToastBread {\n"
                "        doc /* Transform bread into toast acceptable to its user.*/\n"
                "        in bread : Bread;\n"
                "        out toast : Toast;\n"
                "    }\n"
                "    action def ApplyHeat {\n"
                "        in duration : ISQ::DurationValue [0..*] {\n"
                "            doc /* Signal from a control function: how long to apply heat.\n"
                "             * No control function is modeled in this chapter, so this input\n"
                "             * is declared and typed but not yet connected to a value.*/\n"
                "        }\n"
                "    }\n"
                "}\n"
            )
            built = conn.load_from_content(edited)
            expected = conn.load_from_content(target)
            assert built.ok, [str(d) for d in built.errors]
            assert expected.ok, [str(d) for d in expected.errors]

            def construct(model):
                return sorted(
                    (e["@type"], e.get("qualifiedName"), e.get("body"))
                    for e in json.loads(str(model.to_api_json()))
                    if e.get("qualifiedName", "").startswith("ToasterDemo")
                    and e["@type"] in (
                        "Documentation", "ItemDefinition", "ActionDefinition",
                        "ReferenceUsage",
                    )
                )

            documentation = [e for e in construct(built) if e[0] == "Documentation"]
            assert [qn for _, qn, _ in documentation] == [
                "ToasterDemo::ApplyHeat::duration::@0",
                "ToasterDemo::Bread::@0",
                "ToasterDemo::ToastBread::@0",
            ]
            assert construct(built) == construct(expected)

            def documentation_text(model):
                return {
                    e.get("qualifiedName"): e.get("documentation")
                    for e in model.query(select=["qualifiedName", "documentation"])
                    if e.get("documentation") is not None
                }

            assert documentation_text(built) == documentation_text(expected) == {
                "ToasterDemo::Bread": "A slice of bread, before toasting.",
                "ToasterDemo::ToastBread":
                    "Transform bread into toast acceptable to its user.",
                "ToasterDemo::ApplyHeat::duration": heat,
            }

    def test_documentation_on_a_body_goes_first_and_is_refused_twice(self, real_service):
        source = "package P {\n    part def V {\n        attribute m;\n    }\n}\n"
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            edited = str(model.edit().add_documentation("P::V", "A vehicle.").apply())
            assert edited == (
                "package P {\n    part def V {\n"
                "        doc /* A vehicle.*/\n"
                "        attribute m;\n    }\n}\n"
            )
            documented = conn.load_from_content(edited)
            with pytest.raises(MemberNameTakenError):
                documented.edit().add_documentation("P::V", "Again.").apply()
            replaced = str(
                documented.edit().add_documentation("P::V", "A car.", replace=True).apply()
            )
            assert "doc /* A car.*/" in replaced and "A vehicle." not in replaced
            with pytest.raises(InvalidEditError):
                model.edit().add_documentation("P::V", "ends */ early").apply()
            with pytest.raises(EditTargetError):
                model.edit().add_documentation("P::W", "Nothing.").apply()

    @pytest.mark.filterwarnings("ignore::opensysml.conversion.ExperimentalFeatureWarning")
    @pytest.mark.parametrize("body", [
        "One line.", "", "  ", " leading", "trailing ", "a \n  indented\n\ttabbed\t",
        "\nopens blank", "ends with a break\n", "* bullet\n* bullet",
    ])
    def test_comment_and_documentation_bodies_read_back_exactly(self, real_service, body):
        source = "package P {\n    part def V;\n}\n"
        with Connection(port=real_service, auto_start=False) as conn:
            edited = str(
                conn.load_from_content(source).edit()
                .add_comment("P", body, name="C", about=["P::V"], locale="en")
                .add_documentation("P::V", body)
                .apply()
            )
            model = conn.load_from_content(edited)
            assert model.ok, [str(d) for d in model.errors]
            bodies = {
                e["@type"]: e.get("body")
                for e in json.loads(str(model.to_api_json()))
                if e["@type"] in ("Comment", "Documentation")
            }
            assert bodies == {"Comment": body, "Documentation": body}
            comment = [
                e for e in json.loads(str(model.to_api_json())) if e["@type"] == "Comment"
            ][0]
            assert (comment.get("declaredName"), comment.get("locale")) == ("C", "en")

    def test_comments_go_at_the_top_level_and_in_a_body(self, real_service):
        source = "package P {\n    part def V;\n    part def W;\n}\n"
        with Connection(port=real_service, auto_start=False) as conn:
            edited = str(
                conn.load_from_content(source).edit()
                .add_comment("", "File note.")
                .add_comment("P", "Both.", about=["V", "P::W"])
                .add_comment("P::W", "Inside.\nTwo lines.")
                .apply()
            )
            assert edited == (
                "package P {\n    part def V;\n    part def W {\n"
                "        comment /* Inside.\n         * Two lines.*/\n    }\n"
                "    comment about V, P::W /* Both.*/\n}\ncomment /* File note.*/\n"
            )
            assert conn.load_from_content(edited).ok
            with pytest.raises(EditResultError):
                conn.load_from_content(source).edit().add_comment(
                    "P", "Dangling.", about=["Nowhere"],
                ).apply()
            with pytest.raises(InvalidEditError):
                conn.load_from_content(source).edit().add_comment("P", "ends */ early").apply()

    def test_a_note_survives_reparsing_and_later_edits(self, real_service):
        source = (
            "package P {\n    attribute def A;\n"
            "    part def V {\n        attribute m : A;\n    }\n}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            noted = str(
                conn.load_from_content(source).edit()
                .add_note("P::V::m", "DimensionOneValue").apply()
            )
            assert "        // DimensionOneValue\n        attribute m : A;\n" in noted
            model = conn.load_from_content(noted)
            assert model.ok
            later = str(
                model.edit()
                .rename("P::V::m", "mass")
                .add_attribute("P::V", "extra", type="A")
                .add_documentation("P::V", "A vehicle.")
                .apply()
            )
            assert "        // DimensionOneValue\n        attribute mass : A;\n" in later
            moved = str(conn.load_from_content(later).edit().move("P::V::mass", "P").apply())
            assert "    // DimensionOneValue\n    attribute mass : A;\n" in moved
            elements = json.loads(str(conn.load_from_content(later).to_api_json()))
            assert not any(
                "DimensionOneValue" in json.dumps(value)
                for e in elements for key, value in e.items() if not key.startswith("sysx:")
            )
            with pytest.raises(EditTargetError):
                model.edit().add_note("P::Nowhere", "text").apply()

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

    def test_sequence_authoring_builds_the_toaster_actions(self, real_service):
        """Editor-only `first`/`then` ops build the same action bodies as the
        directly written notation, execute the same, and element-match it."""
        source = (
            "package ToasterDemo {\n"
            "    private import ISQ::*;\n"
            "    item def Bread;\n"
            "    item def Toast;\n"
            "    action def GenerateHeat { in energyIn : ISQ::EnergyValue[0..*]; }\n"
            "    action def ApplyHeat {\n"
            "        in bread : Bread;\n"
            "        in energy : ISQ::EnergyValue[0..*];\n"
            "        out toast : Toast;\n"
            "    }\n"
            "    action def ToastBread {\n"
            "        in bread : Bread;\n"
            "        out toast : Toast;\n"
            "    }\n"
            "}\n"
        )
        target = (
            "package ToasterDemo {\n"
            "    private import ISQ::*;\n"
            "    item def Bread;\n"
            "    item def Toast;\n"
            "    action def GenerateHeat { in energyIn : ISQ::EnergyValue[0..*]; }\n"
            "    action def ApplyHeat {\n"
            "        in bread : Bread;\n"
            "        in energy : ISQ::EnergyValue[0..*];\n"
            "        out toast : Toast;\n"
            "        first start;\n"
            "        then action generateHeat : GenerateHeat {\n"
            "            in energyIn = ApplyHeat::energy;\n"
            "        }\n"
            "        then done;\n"
            "    }\n"
            "    action def ToastBread {\n"
            "        in bread : Bread;\n"
            "        out toast : Toast;\n"
            "        first start;\n"
            "        then action applyHeat : ApplyHeat {\n"
            "            in bread = ToastBread::bread;\n"
            "        }\n"
            "        then done;\n"
            "    }\n"
            "}\n"
        )
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(source)
            result = (
                model.edit()
                .add_first("ToasterDemo::ApplyHeat", "start")
                .add_then(
                    "ToasterDemo::ApplyHeat",
                    action="generateHeat", type="GenerateHeat",
                )
                .add_parameter(
                    "ToasterDemo::ApplyHeat::generateHeat",
                    "in", "energyIn", value="ApplyHeat::energy",
                )
                .add_then("ToasterDemo::ApplyHeat", ref="done")
                .add_first("ToasterDemo::ToastBread", "start")
                .add_then(
                    "ToasterDemo::ToastBread",
                    action="applyHeat", type="ApplyHeat",
                )
                .add_parameter(
                    "ToasterDemo::ToastBread::applyHeat",
                    "in", "bread", value="ToastBread::bread",
                )
                .add_then("ToasterDemo::ToastBread", ref="done")
                .apply()
            )
            edited = str(result)
            assert edited == target
            built = conn.load_from_content(edited)
            assert built.ok, [str(d) for d in built.errors]
            written = conn.load_from_content(target)
            assert _elements_by_qname(built.to_api_json()) == \
                _elements_by_qname(written.to_api_json())
            assert built.execute_action("ToasterDemo::ToastBread") == \
                written.execute_action("ToasterDemo::ToastBread")

    def test_sequence_refusals_are_typed(self, real_service):
        with Connection(port=real_service, auto_start=False) as conn:
            model = conn.load_from_content(
                "action def A {\n    action a;\n}\npart def P;\n"
            )
            with pytest.raises(IllegalMemberKindError):
                model.edit().add_then("P", ref="done").apply()
            with pytest.raises(EditTargetError):
                model.edit().add_then("A", ref="nosuch").apply()
            with pytest.raises(IllegalMemberKindError):
                model.edit().add_then("A", action="b", kind="part").apply()
