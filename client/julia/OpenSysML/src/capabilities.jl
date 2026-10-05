const CAPABILITY_TYPE_FACTS = "type_facts"
const CAPABILITY_CONVERT = "convert"
const CAPABILITY_MIGRATE = "migrate"
const CAPABILITY_VERIFICATION = "verification"
const CAPABILITY_VERIFICATION_QUESTIONS = "verification_questions"
const CAPABILITY_QUERY = "query"
const CAPABILITY_OSLC_QUERY = "oslc_query"
const CAPABILITY_DOCUMENT_QUERY = "document_query"
const CAPABILITY_RENDER_DOCUMENT = "render_document"
const CAPABILITY_RENDER_DOCUMENT_HTML = "render_document_html"
const CAPABILITY_ENUM_VALUES = "enum_values"
const CAPABILITY_EVALUATE_SUBJECT = "evaluate_subject"
const CAPABILITY_SYMBOL_ATTRIBUTES = "symbol_attributes"
const CAPABILITY_UNSET_VALUE = "unset_value"
const CAPABILITY_FEATURE_VALUES = "feature_values"
const CAPABILITY_APPLY_EDITS = "apply_edits"
const CAPABILITY_AUTHORING = "authoring"
const CAPABILITY_CONNECTION_AUTHORING = "connection_authoring"
const CAPABILITY_SATISFY_AUTHORING = "satisfy_authoring"
const CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING = "requirement_constraint_authoring"
const CAPABILITY_TRANSITION_AUTHORING = "transition_authoring"
const CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING = "verification_objective_authoring"
const CAPABILITY_METADATA_AUTHORING = "metadata_authoring"
const CAPABILITY_METADATA_PREFIX_AUTHORING = "metadata_prefix_authoring"
const CAPABILITY_SEQUENCE_AUTHORING = "sequence_authoring"
const CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING = "action_body_statement_authoring"
const CAPABILITY_IMPORT_AUTHORING = "import_authoring"
const CAPABILITY_DOCUMENTATION_AUTHORING = "documentation_authoring"
const CAPABILITY_COMMENT_AUTHORING = "comment_authoring"
const CAPABILITY_MEMBER_MODIFIERS = "member_modifiers"
const CAPABILITY_IMPLICIT_PARAMETERS = "implicit_parameters"
const CAPABILITY_CONSTRAINT_BODY_AUTHORING = "constraint_body_authoring"
const CAPABILITY_STATE_ACTION_AUTHORING = "state_action_authoring"
const CAPABILITY_EDIT_DOCUMENTS = "edit_documents"
const CAPABILITY_PARSE_SOURCES = "parse_sources"
const CAPABILITY_INLINE_LANGUAGE = "inline_language"
const CAPABILITY_STRICT_CONFORMANCE = "strict_conformance"
const CAPABILITY_COMPLEX_VALUES = "complex_values"
const CAPABILITY_STRUCTURED_VALUES = "structured_values"
const CAPABILITY_MEASUREMENT_REFS = "measurement_refs"
const CAPABILITY_FUNCTION_VALUES = "function_values"
const CAPABILITY_SET_VALUES = "set_values"
const CAPABILITY_TENSOR_VALUES = "tensor_values"
const CAPABILITY_METAOBJECT_VALUES = "metaobject_values"
const CAPABILITY_VERIFICATION_VERDICTS = "verification_verdicts"
const CAPABILITY_INFINITY_VALUE = "infinity_value"
const CAPABILITY_DIAGNOSTIC_CODES = "diagnostic_codes"
const CAPABILITY_SCHEDULE = "schedule"
const CAPABILITY_CASE_EVALUATIONS = "case_evaluations"
const CAPABILITY_SCHEDULE_EXPLORE = "schedule_explore"
const CAPABILITY_PERFORMER = "performer"
const CAPABILITY_STATE_TRACE = "state_trace"
const CAPABILITY_FINAL_TIME = "final_time"
const CAPABILITY_ENGINES = "engines"
const CAPABILITY_UNDETERMINED_VALUE = "undetermined_value"
const CAPABILITY_BIG_INT_VALUES = "big_int_values"

"""The version and advertised capabilities reported by a sysml-grpc service."""
struct ServerInfo
    version::String
    capabilities::Set{String}
    answered::Bool
    origin::String
    ServerInfo(version, capabilities, answered, origin) =
        new(String(version), Set(String(c) for c in capabilities), Bool(answered), String(origin))
end

"""Return whether `info` advertises `capability`."""
Base.in(capability::AbstractString, info::ServerInfo) =
    String(capability) in info.capabilities

"""Describe the service and its reported capabilities."""
function describe(info::ServerInfo)
    version = isempty(info.version) ? "unknown" : info.version
    if !info.answered
        return "$(info.origin) (version unknown: too old to answer GetServerInfo, so it predates every capability)"
    end
    reported = join(sort!(collect(info.capabilities)), ", ")
    isempty(reported) && (reported = "none")
    return "$(info.origin) (version $(version), capabilities: $(reported))"
end

Base.show(io::IO, info::ServerInfo) = print(io, describe(info))
Base.length(::ServerInfo) = 2
Base.iterate(info::ServerInfo, state::Int=1) =
    state == 1 ? (info.version, 2) : state == 2 ? (info.capabilities, 3) : nothing

"""Return the recommended way to obtain a service with `capability`."""
function upgrade_remedy(capability::AbstractString)
    return "run a sysml-grpc whose GetServerInfo reports '$(capability)': set \$OPENSYSML_GRPC_VERSION to a release that has it, or build one with `make build-grpc` and start it yourself"
end

"""Explain why a server's version or capabilities do not meet a request."""
function mismatch_reason(info::ServerInfo; version=nothing, capabilities=())
    reasons = String[]
    if version !== nothing
        if !info.answered
            push!(reasons, "it did not answer GetServerInfo, so it cannot be shown to be the $(version) that was asked for")
        elseif info.version != version
            reported = isempty(info.version) ? "unknown" : info.version
            push!(reasons, "it reports version $(reported), but $(version) was asked for")
        end
    end
    missing = sort!(String[c for c in capabilities if !(c in info)])
    if !isempty(missing)
        named = join(repr.(missing), ", ")
        noun = length(missing) == 1 ? "capability" : "capabilities"
        push!(reasons, "it does not report the $(named) $(noun) this client requires")
    end
    return isempty(reasons) ? nothing : join(reasons, "; ")
end

"""Require a capability from a server description or connection."""
function require_capability(info::ServerInfo, capability::AbstractString)
    capability in info ||
        throw(MissingCapabilityError(String(capability), info, upgrade_remedy(capability)))
    return nothing
end

_engine_field(engine) =
    engine === nothing || isempty(String(engine)) || String(engine) == "auto" ? "" :
    String(engine)

_engine_capabilities(engine) = begin
    field = _engine_field(engine)
    field == "explore" ? (CAPABILITY_ENGINES, CAPABILITY_SCHEDULE_EXPLORE) :
    isempty(field) ? () : (CAPABILITY_ENGINES,)
end

_question_field(question) =
    question === nothing || isempty(String(question)) || String(question) == "evaluate" ? "" :
    String(question)

_question_capabilities(question) =
    isempty(_question_field(question)) ? () : (CAPABILITY_VERIFICATION_QUESTIONS,)

_is_exploring(schedule) = schedule !== nothing &&
    (String(schedule) == "explore" || startswith(String(schedule), "explore:"))

function _schedule_capabilities(schedule)
    (schedule === nothing || isempty(String(schedule))) && return ()
    _is_exploring(schedule) ?
        (CAPABILITY_SCHEDULE, CAPABILITY_SCHEDULE_EXPLORE) :
        (CAPABILITY_SCHEDULE,)
end

function _require_capabilities(conn, capabilities)
    for capability in capabilities
        require_capability(conn, capability)
    end
    return nothing
end
