module OpenSysML

using HTTP
using JSON
using SHA

export Connection, Model, Diagnostic, Instance, InstanceRef, Quantity,
       EnumLiteral, Unset, Infinity, FunctionRef, Metaobject, Undetermined,
       Unit, UnitFactor, MeasurementRef, ArrayValue, VectorValue, VectorQuantity,
       TensorQuantity, in_unit, to_unit, same_value, value_capabilities,
       OpenSysMLError, ConnectError, ServiceError, TransportError, DiagnosticError,
       ExecutionError, EditError, EditResultError, EditTargetError, InvalidEditError,
       NoEditsError, OverlappingEditsError, RenameReferencedError, OwnerNotFoundError,
       OwnerNotNamespaceError, IllegalMemberKindError, MemberNameTakenError,
       DeleteReferencedError, OwnerInsideTargetError, MoveReferencedError,
       ReferencedElsewhereError, EditFailureError, Referrer,
       ExecutionFailure, WrongKindError, ModelError,
       ServiceCallError, ModelNotFoundError, ModelFileNotFoundError, InvalidRequestError,
       ServiceTimeoutError, UnsupportedOperationError, ServiceUnavailableError,
       ConversionError, MissingCapabilityError, StaleServiceError, SymbolNotFoundError,
       ChecksumMismatchError, UnpinnedReleaseError,
       QueryError, DocumentQueryError, UnsupportedValueError, FeatureValueError,
       TypeMismatchError, InstanceTypeError, IncommensurableUnitsError,
       QueryElement, build_query, as_dict,
       TypeFacts, Multiplicity, Specialization, AttributeFacts, SymbolFacts, SymbolInfo,
       Bound, Standing, EngineInfo, VerificationVerdict, WitnessAssignment, Verdict,
       Validation, CalcResult, CaseEvaluation, AnalysisResult, SweepRow, SweepTable,
       holds, valid, violated, undecided, selected, failures, is_collection, is_optional, facts, children,
       attributes, attribute_facts, parts, get_attr, get_symbol, find, list_engines,
       ServerInfo, mismatch_reason, upgrade_remedy, require_capability,
       connect, external, private, call, server_info, has_capability,
       parse_file, parse_source, parse_sources, diagnostics, symbol,
       SourceDocument, errors,
       evaluate, instantiate, execute_action, execute_state, query, roots, root,
       documents, isok, raise_for_errors, get_feature, features,
       verify_constraint, verify_requirement, verify_satisfaction, satisfied,
       validate_instance, calc, run_analysis, run_sweep, explore_analysis,
       explore_action, explore_state, Outcome, Exploration, failed,
       raise_for_error, raise_for_incomplete, Conversion,
       convert_file, convert_source, convert_model, to_sysml, to_turtle,
       to_api_json, save, format_of_path,
       ElementRef, ObjectRef, DocumentVerdict, DocumentState, DocumentEvent,
       DocumentRow, DocumentQueryResult, build_document_bindings,
       run_document_query, render_document, Editor, Body, operations, applied, AppliedEdit,
       EditedDocument, EditResult, edit, apply, apply_edits,
       decode_value, encode_value, resolve_binary, ensure_binary, download_binary,
       TypedObject, from_instance, unchecked, as_bool, as_int, as_float, as_complex,
       as_str, as_quantity, as_enum_literal, as_object, as_typed, feature_value,
       optional_feature_value, list_feature_value, generate_source, generate_main

include("errors.jl")
include("binary.jl")
include("values.jl")
include("capabilities.jl")
include("connection.jl")
include("model.jl")
include("typed.jl")
include("query.jl")
include("typefacts.jl")
include("symbol.jl")
include("generate.jl")
include("engines.jl")
include("verdict.jl")
include("verification.jl")
include("document.jl")
include("conversion.jl")
include("authoring.jl")
include("exploration.jl")

for operation in (
    :set_value, :rename, :add_member, :add_objective, :add_verify, :add_metadata,
    :add_metadata_prefix, :add_documentation, :add_comment, :add_note, :add_satisfy,
    :add_requirement_constraint, :add_transition, :add_entry_transition, :add_first,
    :add_then, :add_accept, :add_send, :add_assign, :add_if, :add_while, :add_loop,
    :add_for, :add_terminate, :add_guarded_then, :add_else, :add_import,
    :add_require_constraint, :add_assume_constraint, :add_connection, :add_allocation,
    :add_flow, :add_succession, :delete, :move, :add_package, :add_part_def, :add_part,
    :add_attribute_def, :add_attribute, :add_item_def, :add_item, :add_port_def,
    :add_port, :add_class, :add_struct, :add_datatype, :add_classifier, :add_feature,
    :add_assoc, :add_behavior, :add_function, :add_predicate, :add_interaction,
    :add_metaclass, :add_calc_def, :add_calc, :add_parameter, :add_return,
    :add_action_def, :add_action, :add_perform_action, :add_perform, :add_exhibit_state,
    :add_exhibit, :add_state_action, :add_state_def, :add_state, :add_constraint_def,
    :add_constraint, :add_assert_constraint, :add_assert, :add_requirement_def,
    :add_requirement,
)
    @eval export $operation
end

for capability in (
    :CAPABILITY_TYPE_FACTS, :CAPABILITY_CONVERT, :CAPABILITY_VERIFICATION,
    :CAPABILITY_VERIFICATION_QUESTIONS, :CAPABILITY_QUERY, :CAPABILITY_OSLC_QUERY,
    :CAPABILITY_DOCUMENT_QUERY,
    :CAPABILITY_RENDER_DOCUMENT, :CAPABILITY_RENDER_DOCUMENT_HTML, :CAPABILITY_ENUM_VALUES,
    :CAPABILITY_EVALUATE_SUBJECT, :CAPABILITY_SYMBOL_ATTRIBUTES, :CAPABILITY_UNSET_VALUE,
    :CAPABILITY_FEATURE_VALUES, :CAPABILITY_APPLY_EDITS, :CAPABILITY_AUTHORING,
    :CAPABILITY_CONNECTION_AUTHORING, :CAPABILITY_SATISFY_AUTHORING,
    :CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING, :CAPABILITY_TRANSITION_AUTHORING,
    :CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING, :CAPABILITY_METADATA_AUTHORING,
    :CAPABILITY_METADATA_PREFIX_AUTHORING, :CAPABILITY_SEQUENCE_AUTHORING,
    :CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING, :CAPABILITY_IMPORT_AUTHORING,
    :CAPABILITY_DOCUMENTATION_AUTHORING, :CAPABILITY_COMMENT_AUTHORING,
    :CAPABILITY_MEMBER_MODIFIERS, :CAPABILITY_IMPLICIT_PARAMETERS,
    :CAPABILITY_CONSTRAINT_BODY_AUTHORING, :CAPABILITY_STATE_ACTION_AUTHORING,
    :CAPABILITY_EDIT_DOCUMENTS, :CAPABILITY_PARSE_SOURCES, :CAPABILITY_INLINE_LANGUAGE,
    :CAPABILITY_STRICT_CONFORMANCE, :CAPABILITY_COMPLEX_VALUES, :CAPABILITY_STRUCTURED_VALUES,
    :CAPABILITY_MEASUREMENT_REFS, :CAPABILITY_FUNCTION_VALUES, :CAPABILITY_SET_VALUES,
    :CAPABILITY_TENSOR_VALUES, :CAPABILITY_METAOBJECT_VALUES,
    :CAPABILITY_VERIFICATION_VERDICTS, :CAPABILITY_INFINITY_VALUE,
    :CAPABILITY_DIAGNOSTIC_CODES, :CAPABILITY_SCHEDULE, :CAPABILITY_CASE_EVALUATIONS,
    :CAPABILITY_SCHEDULE_EXPLORE, :CAPABILITY_PERFORMER, :CAPABILITY_FINAL_TIME,
    :CAPABILITY_ENGINES, :CAPABILITY_UNDETERMINED_VALUE
)
    @eval export $capability
end

end
