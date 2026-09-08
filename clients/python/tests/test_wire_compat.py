"""Wire compatibility of the message types that existed before verification.

Adding RPCs and messages to sysml.proto must not move a field of a type already
on the wire: a client built against the older schema has to keep reading what a
newer service writes, and vice versa. These tests pin the field numbers of the
pre-existing types and check that bytes written by one schema parse back
unchanged.
"""

from opensysml.proto import sysml_pb2


# Field numbers as of the schema before the verification RPCs were added. A
# change here is a wire break, not a test to update.
LEGACY_FIELD_NUMBERS = {
    "ParseFileRequest": {"file_path": 1, "content": 2, "content_hash": 3},
    "ParseFileResponse": {"model_hash": 1, "root": 2, "diagnostics": 3},
    "GetSymbolRequest": {"model_hash": 1, "symbol_id": 2},
    "SymbolResponse": {"symbol": 1, "error": 2},
    "DiagnosticsRequest": {"model_hash": 1},
    "DiagnosticsResponse": {"diagnostics": 1},
    "EvaluateRequest": {
        "model_hash": 1,
        "expression": 2,
        "context_symbol_id": 3,
    },
    "EvaluateResponse": {"result": 1, "error": 2, "diagnostics": 3},
    # Field 3 held the pre-0.1.0 `slots` map, removed and reserved before the
    # first release; see test_the_removed_slots_field_stays_reserved.
    "Instance": {"id": 1, "type_symbol_id": 2, "feature_values": 4},
    "FeatureValue": {
        "feature_name": 1,
        "value": 2,
        "values": 3,
        "materialized": 4,
        "error": 5,
    },
    "InstantiateRequest": {"model_hash": 1, "symbol_id": 2},
    "ExecuteActionRequest": {"model_hash": 1, "action_symbol_id": 2, "inputs": 3},
    "ExecuteStateRequest": {
        "model_hash": 1,
        "state_machine_symbol_id": 2,
        "events": 3,
    },
    "Value": {
        "int_value": 1,
        "real_value": 2,
        "bool_value": 3,
        "string_value": 4,
        "instance_id": 5,
        "sequence": 6,
        "null": 7,
    },
    "ValueSequence": {"elements": 1},
    "Diagnostic": {"severity": 1, "message": 2, "span": 3},
    "Span": {
        "file": 1,
        "start_line": 2,
        "start_col": 3,
        "end_line": 4,
        "end_col": 5,
    },
    "ServerInfoResponse": {"version": 1, "capabilities": 2},
}


def test_legacy_field_numbers_are_unchanged():
    for message_name, fields in LEGACY_FIELD_NUMBERS.items():
        descriptor = getattr(sysml_pb2, message_name).DESCRIPTOR
        got = {
            name: descriptor.fields_by_name[name].number
            for name in fields
            if name in descriptor.fields_by_name
        }
        assert got == fields, f"{message_name} field numbers moved"


def test_the_removed_slots_field_stays_reserved():
    """`slots` went away before 0.1.0, and its number must not be reused."""
    descriptor = sysml_pb2.Instance.DESCRIPTOR
    assert "slots" not in descriptor.fields_by_name
    assert not hasattr(sysml_pb2, "SlotValue")
    assert all(field.number != 3 for field in descriptor.fields)


def test_legacy_rpcs_are_still_declared():
    service = sysml_pb2.DESCRIPTOR.services_by_name["SysMLService"]
    names = {method.name for method in service.methods}
    assert {
        "GetServerInfo",
        "ParseFile",
        "GetSymbol",
        "GetDiagnostics",
        "Evaluate",
        "Instantiate",
        "ExecuteAction",
        "ExecuteState",
        "Convert",
    } <= names


def test_parse_file_response_round_trips():
    response = sysml_pb2.ParseFileResponse(
        model_hash="abc123",
        root=sysml_pb2.SymbolInfo(
            id="Demo",
            name="Demo",
            kind="Package",
            child_ids=["Demo::Vehicle"],
            metadata={"declared": "true"},
        ),
        diagnostics=[
            sysml_pb2.Diagnostic(
                severity="error",
                message="expected a namespace member",
                span=sysml_pb2.Span(
                    file="demo.sysml",
                    start_line=3,
                    start_col=2,
                    end_line=3,
                    end_col=11,
                ),
            )
        ],
    )

    again = sysml_pb2.ParseFileResponse()
    again.ParseFromString(response.SerializeToString())
    assert again == response


def test_instance_graph_round_trips():
    response = sysml_pb2.InstantiateResponse(
        instance=sysml_pb2.Instance(
            id=1,
            type_symbol_id="Demo::Vehicle",
            feature_values={
                "mass": sysml_pb2.FeatureValue(
                    feature_name="mass",
                    value=sysml_pb2.Value(real_value=1200.0),
                    materialized=True,
                ),
                "wheels": sysml_pb2.FeatureValue(
                    feature_name="wheels",
                    values=[
                        sysml_pb2.Value(instance_id=2),
                        sysml_pb2.Value(instance_id=3),
                    ],
                    materialized=True,
                ),
                "broken": sysml_pb2.FeatureValue(
                    feature_name="broken",
                    error="feature is unbound",
                ),
            },
        ),
        instances=[
            sysml_pb2.Instance(id=2, type_symbol_id="Demo::Wheel"),
            sysml_pb2.Instance(id=3, type_symbol_id="Demo::Wheel"),
        ],
    )

    again = sysml_pb2.InstantiateResponse()
    again.ParseFromString(response.SerializeToString())
    assert again == response
    assert again.instance.feature_values["mass"].value.real_value == 1200.0


def test_every_value_kind_round_trips():
    for value in (
        sysml_pb2.Value(int_value=-7),
        sysml_pb2.Value(real_value=1.5),
        sysml_pb2.Value(bool_value=True),
        sysml_pb2.Value(string_value="hello"),
        sysml_pb2.Value(instance_id=42),
        sysml_pb2.Value(null=""),
        sysml_pb2.Value(
            sequence=sysml_pb2.ValueSequence(
                elements=[
                    sysml_pb2.Value(int_value=1),
                    sysml_pb2.Value(string_value="two"),
                ]
            )
        ),
    ):
        again = sysml_pb2.Value()
        again.ParseFromString(value.SerializeToString())
        assert again == value
        assert again.WhichOneof("kind") == value.WhichOneof("kind")


def test_execute_responses_round_trip():
    action = sysml_pb2.ExecuteActionResponse(
        outputs={"total": sysml_pb2.Value(int_value=9)},
        error="",
        diagnostics=[sysml_pb2.Diagnostic(severity="warning", message="slow")],
    )
    again = sysml_pb2.ExecuteActionResponse()
    again.ParseFromString(action.SerializeToString())
    assert again == action

    state = sysml_pb2.ExecuteStateResponse(
        states_visited=["Idle", "Running"],
        final_context={"count": sysml_pb2.Value(int_value=2)},
    )
    again_state = sysml_pb2.ExecuteStateResponse()
    again_state.ParseFromString(state.SerializeToString())
    assert again_state == state


def test_unknown_verification_fields_survive_an_older_reader():
    """A verification response is opaque, not corrupting, to an older client.

    An older client parses what it does not know as unknown fields and can write
    them back out byte-for-byte, which is what makes adding messages safe.
    """
    response = sysml_pb2.VerifySatisfactionResponse(
        verdicts=[sysml_pb2.Verdict(kind="satisfy", holds=True, element="satisfy r by p")],
    )
    payload = response.SerializeToString()

    # ServerInfoRequest declares no fields, so it stands in for a client whose
    # schema knows nothing of this message: everything lands in unknown fields.
    older = sysml_pb2.ServerInfoRequest()
    older.ParseFromString(payload)
    assert older.SerializeToString() == payload


def test_enum_literal_is_an_added_value_arm():
    """The literal arm is new field 9, so it displaces no existing value kind."""
    fields = sysml_pb2.Value.DESCRIPTOR.fields_by_name
    assert fields["enum_literal"].number == 9
    assert fields["quantity"].number == 8

    value = sysml_pb2.Value(enum_literal=sysml_pb2.EnumLiteral(
        literal_id="D::Color::red", enumeration_id="D::Color", name="Color::red",
    ))
    payload = value.SerializeToString()

    again = sysml_pb2.Value()
    again.ParseFromString(payload)
    assert again == value

    # A client whose schema predates the arm keeps the bytes intact.
    older = sysml_pb2.ServerInfoRequest()
    older.ParseFromString(payload)
    assert older.SerializeToString() == payload


def test_structured_values_are_added_value_arms():
    """The array, vector and vector quantity arms are new fields 12-14."""
    fields = sysml_pb2.Value.DESCRIPTOR.fields_by_name
    assert fields["complex"].number == 11
    assert fields["array"].number == 12
    assert fields["vector"].number == 13
    assert fields["vector_quantity"].number == 14

    metre = sysml_pb2.Quantity(
        real_magnitude=3.0,
        unit="m",
        unit_term=sysml_pb2.UnitTerm(
            scale_num=1.0, scale_den=1.0,
            factors=[sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0)],
        ),
    )
    for value in (
        sysml_pb2.Value(array=sysml_pb2.Array(
            dimensions=[2, 1],
            elements=[sysml_pb2.Value(int_value=1), sysml_pb2.Value(int_value=2)],
        )),
        sysml_pb2.Value(vector=sysml_pb2.Vector(
            components=[sysml_pb2.Value(real_value=3.0), sysml_pb2.Value(int_value=4)],
        )),
        sysml_pb2.Value(vector_quantity=sysml_pb2.VectorQuantity(components=[metre, metre])),
    ):
        payload = value.SerializeToString()
        again = sysml_pb2.Value()
        again.ParseFromString(payload)
        assert again == value

        # A client whose schema predates the arm keeps the bytes intact.
        older = sysml_pb2.ServerInfoRequest()
        older.ParseFromString(payload)
        assert older.SerializeToString() == payload


def test_a_measurement_reference_is_an_added_value_arm():
    """The bare measurement reference arm is new field 15."""
    fields = sysml_pb2.Value.DESCRIPTOR.fields_by_name
    assert fields["measurement_ref"].number == 15
    ref_fields = sysml_pb2.MeasurementRef.DESCRIPTOR.fields_by_name
    assert {name: f.number for name, f in ref_fields.items()} == {
        "unit": 1, "unit_term": 2, "unit_id": 3,
    }

    value = sysml_pb2.Value(measurement_ref=sysml_pb2.MeasurementRef(
        unit="km",
        unit_term=sysml_pb2.UnitTerm(
            scale_num=1000.0, scale_den=1.0,
            factors=[sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1.0)],
        ),
        unit_id="SI::kilometre",
    ))
    payload = value.SerializeToString()
    again = sysml_pb2.Value()
    again.ParseFromString(payload)
    assert again == value

    # A client whose schema predates the arm keeps the bytes intact.
    older = sysml_pb2.ServerInfoRequest()
    older.ParseFromString(payload)
    assert older.SerializeToString() == payload


def test_a_function_is_an_added_value_arm():
    """The function arm is new field 16."""
    fields = sysml_pb2.Value.DESCRIPTOR.fields_by_name
    assert fields["function"].number == 16
    fn_fields = sysml_pb2.Function.DESCRIPTOR.fields_by_name
    assert {name: f.number for name, f in fn_fields.items()} == {
        "calc_id": 1, "self_id": 2,
    }

    value = sysml_pb2.Value(function=sysml_pb2.Function(
        calc_id="Demo::Scaler::scale", self_id=7,
    ))
    payload = value.SerializeToString()
    again = sysml_pb2.Value()
    again.ParseFromString(payload)
    assert again == value

    # A client whose schema predates the arm keeps the bytes intact.
    older = sysml_pb2.ServerInfoRequest()
    older.ParseFromString(payload)
    assert older.SerializeToString() == payload


def test_apply_edits_is_an_added_rpc():
    """The edit RPC is new, so it displaces nothing a client already calls."""
    service = sysml_pb2.DESCRIPTOR.services_by_name["SysMLService"]
    methods = {method.name: method for method in service.methods}
    assert "ApplyEdits" in methods
    assert methods["ApplyEdits"].input_type.name == "ApplyEditsRequest"
    assert methods["ApplyEdits"].output_type.name == "ApplyEditsResponse"


def test_edit_messages_pin_their_field_numbers():
    """The edit messages' own numbering, pinned from the release that added it."""
    expected = {
        "ApplyEditsRequest": {"model_hash": 1, "operations": 2},
        "EditOperation": {
            "set_value": 1, "rename": 2, "add_member": 3, "delete": 4
        },
        "AddMemberEdit": {
            "owner": 1, "kind": 2, "name": 3, "type": 4,
            "multiplicity": 5, "value": 6, "specializes": 7,
        },
        "DeleteEdit": {"target": 1, "cascade": 2},
        "SetValueEdit": {"target": 1, "value": 2},
        "RenameEdit": {"target": 1, "new_name": 2},
        "ApplyEditsResponse": {
            "content": 1,
            "applied": 2,
            "error": 3,
            "failure": 4,
            "diagnostics": 5,
            "referring_elements": 6,
        },
        "AppliedEdit": {
            "operation_index": 1,
            "target": 2,
            "offset": 3,
            "length": 4,
            "old_text": 5,
            "new_text": 6,
        },
    }
    for message_name, fields in expected.items():
        descriptor = getattr(sysml_pb2, message_name).DESCRIPTOR
        got = {
            name: descriptor.fields_by_name[name].number
            for name in descriptor.fields_by_name
        }
        assert got == fields, f"{message_name} field numbers moved"


def test_edit_failure_kinds_keep_their_values():
    """A refusal kind is read by number, so a value is never reassigned."""
    assert {
        value.name: value.number
        for value in sysml_pb2.EditFailure.DESCRIPTOR.values
    } == {
        "EDIT_FAILURE_UNSPECIFIED": 0,
        "EDIT_FAILURE_NO_OPERATIONS": 1,
        "EDIT_FAILURE_UNKNOWN_TARGET": 2,
        "EDIT_FAILURE_AMBIGUOUS_TARGET": 3,
        "EDIT_FAILURE_NOT_VALUED": 4,
        "EDIT_FAILURE_INVALID_VALUE": 5,
        "EDIT_FAILURE_INVALID_NAME": 6,
        "EDIT_FAILURE_NOT_NAMED": 7,
        "EDIT_FAILURE_RENAME_REFERENCED": 8,
        "EDIT_FAILURE_OVERLAPPING_EDITS": 9,
        "EDIT_FAILURE_RESULT_INVALID": 10,
        "EDIT_FAILURE_OWNER_UNKNOWN": 11,
        "EDIT_FAILURE_OWNER_NOT_NAMESPACE": 12,
        "EDIT_FAILURE_ILLEGAL_KIND": 13,
        "EDIT_FAILURE_MEMBER_NAME_TAKEN": 14,
        "EDIT_FAILURE_DELETE_REFERENCED": 15,
    }


def test_an_edit_response_survives_an_older_reader():
    """An older client parses an edit response as unknown fields, intact."""
    response = sysml_pb2.ApplyEditsResponse(
        content="package Demo { part def SC; }\n",
        applied=[sysml_pb2.AppliedEdit(
            operation_index=0, target="Demo::SC::unitMass", offset=42, length=14,
            old_text="1000.0[SI::kg]", new_text="1050.0[SI::kg]",
        )],
    )
    payload = response.SerializeToString()

    again = sysml_pb2.ApplyEditsResponse()
    again.ParseFromString(payload)
    assert again == response

    older = sysml_pb2.ServerInfoRequest()
    older.ParseFromString(payload)
    assert older.SerializeToString() == payload
