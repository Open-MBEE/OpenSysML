module OpenSysML

using HTTP
using JSON
import Base: convert, get

export Connection, Model, Diagnostic, Instance, InstanceRef, Quantity,
       EnumLiteral, Unset, Infinity, FunctionRef, Metaobject, Undetermined,
       Unit, UnitFactor, MeasurementRef, ArrayValue, VectorValue, VectorQuantity,
       TensorQuantity, in_unit, to, same_value, value_capabilities,
       rank, unit, magnitudes, nested, exponents, reduction, commensurable,
       OpenSysMLError, ConnectError, ServiceError, TransportError, DiagnosticError,
       ExecutionError, EditError, ExecutionFailure, WrongKindError, ModelError,
       ServiceCallError, ModelNotFoundError, ModelFileNotFoundError, InvalidRequestError,
       ServiceTimeoutError, UnsupportedOperationError, ServiceUnavailableError,
       ConversionError, MissingCapabilityError, StaleServiceError, SymbolNotFoundError,
       QueryError, DocumentQueryError, UnsupportedValueError, FeatureValueError,
       TypeMismatchError, InstanceTypeError, IncommensurableUnitsError,
       QueryElement, build_query, as_dict,
       TypeFacts, Multiplicity, Specialization, AttributeFacts, SymbolFacts, SymbolInfo,
       Bound, Standing, EngineInfo, VerificationVerdict, WitnessAssignment, Verdict,
       Validation, CalcResult, CaseEvaluation, AnalysisResult, SweepRow, SweepTable,
       holds, valid, violated, undecided, selected, failures, is_collection, is_optional, facts, children,
       attributes, attribute_facts, parts, get_attr, get_symbol, find, list_engines,
       ServerInfo, has, describe, mismatch_reason, upgrade_remedy, require_capability,
       connect, external, private, call, server_info, has_capability,
       parse_file, parse_source, parse_sources, diagnostics, symbol,
       SourceDocument, errors,
       evaluate, instantiate, execute_action, execute_state, query, roots, root,
       documents, isok, raise_for_errors, get_feature, features, get, haskey,
       verify_constraint, verify_requirement, verify_satisfaction, satisfied,
       validate_instance, calc, run_analysis, run_sweep, explore_analysis,
       explore_action, explore_state, Outcome, Exploration, failed, status,
       raise_for_error, raise_for_incomplete, Conversion, convert,
       convert_file, convert_source, convert_model, to_sysml, to_turtle,
       to_api_json, save, format_of_path,
       ElementRef, ObjectRef, DocumentVerdict, DocumentState, DocumentEvent,
       DocumentRow, DocumentQueryResult, build_document_bindings,
       run_document_query, render_document,
       decode_value, encode_value, resolve_binary

include("errors.jl")
include("values.jl")
include("capabilities.jl")
include("connection.jl")
include("model.jl")
include("query.jl")
include("typefacts.jl")
include("symbol.jl")
include("engines.jl")
include("verdict.jl")
include("verification.jl")
include("document.jl")
include("conversion.jl")
include("exploration.jl")

for capability in (
    :CAPABILITY_TYPE_FACTS, :CAPABILITY_CONVERT, :CAPABILITY_VERIFICATION,
    :CAPABILITY_VERIFICATION_QUESTIONS, :CAPABILITY_QUERY, :CAPABILITY_DOCUMENT_QUERY,
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
