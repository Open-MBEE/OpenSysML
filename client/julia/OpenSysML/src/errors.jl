struct Diagnostic
    severity::String
    message::String
    file::String
    line::Int
    col::Int
    code::String
end

function Diagnostic(d::AbstractDict)
    span = get(d, "span", Dict{String,Any}())
    Diagnostic(String(get(d, "severity", "")), String(get(d, "message", "")),
               String(get(span, "file", "")), Int(get(span, "startLine", 0)),
               Int(get(span, "startCol", 0)), String(get(d, "code", "")))
end

abstract type OpenSysMLError <: Exception end
abstract type ConnectError <: OpenSysMLError end
const ServiceError = ConnectError

for name in (:ServiceCallError, :ModelNotFoundError, :ModelFileNotFoundError,
             :InvalidRequestError, :ServiceTimeoutError, :UnsupportedOperationError,
             :ServiceUnavailableError)
    @eval begin
        struct $name <: ConnectError
            code::String
            message::String
            http_status::Int
        end
    end
end

function _not_found_type(message, default)
    text = lowercase(message)
    occursin("file not found", text) || occursin("no such file", text) ? ModelFileNotFoundError :
    occursin("model not found", text) ? ModelNotFoundError :
    default
end

function ConnectError(code::AbstractString, message::AbstractString, http_status::Integer;
                      not_found=ModelNotFoundError)
    c = lowercase(String(code))
    m = String(message)
    cls = c == "not_found" ? _not_found_type(m, not_found) :
          c in ("invalid_argument", "failed_precondition", "out_of_range") ? InvalidRequestError :
          c in ("deadline_exceeded", "cancelled", "canceled") ? ServiceTimeoutError :
          c == "unimplemented" ? UnsupportedOperationError :
          c == "unavailable" ? ServiceUnavailableError : ServiceCallError
    cls(String(code), m, Int(http_status))
end

function Base.showerror(io::IO, e::ConnectError)
    print(io, "$(nameof(typeof(e)))($(e.code), HTTP $(e.http_status)): $(e.message)")
end

struct TransportError <: OpenSysMLError
    message::String
end
Base.showerror(io::IO, e::TransportError) = print(io, "TransportError: $(e.message)")

struct ChecksumMismatchError <: OpenSysMLError
    message::String
end
Base.showerror(io::IO, e::ChecksumMismatchError) = print(io, "ChecksumMismatchError: $(e.message)")

struct UnpinnedReleaseError <: OpenSysMLError
    message::String
end
Base.showerror(io::IO, e::UnpinnedReleaseError) = print(io, "UnpinnedReleaseError: $(e.message)")

abstract type DiagnosticError <: OpenSysMLError end
abstract type ExecutionError <: DiagnosticError end

struct ExecutionFailure <: ExecutionError
    message::String
    diagnostics::Vector{Diagnostic}
end

DiagnosticError(message, diagnostics=Diagnostic[]) =
    ExecutionFailure(String(message), Diagnostic[d for d in diagnostics])
ExecutionError(message, diagnostics=Diagnostic[]) =
    ExecutionFailure(String(message), Diagnostic[d for d in diagnostics])

struct WrongKindError <: ExecutionError
    message::String
    diagnostics::Vector{Diagnostic}
    WrongKindError(message, diagnostics=Diagnostic[]) =
        new(String(message), Diagnostic[d for d in diagnostics])
end

struct AnalysisRunError <: ExecutionError
    message::String
    diagnostics::Vector{Diagnostic}
    result::Any
end

struct ModelError <: DiagnosticError
    message::String
    diagnostics::Vector{Diagnostic}
    model::Any
end
ModelError(message, diagnostics=Diagnostic[]; model=nothing) =
    ModelError(String(message), Diagnostic[d for d in diagnostics], model)

"""The service could not read the SysML v1 model it was asked to migrate."""
struct MigrationError <: OpenSysMLError
    message::String
end
Base.showerror(io::IO, e::MigrationError) = print(io, "MigrationError: ", e.message)

struct ConversionError <: DiagnosticError
    message::String
    diagnostics::Vector{Diagnostic}
    ConversionError(message, diagnostics=Diagnostic[]) =
        new(String(message), Diagnostic[d for d in diagnostics])
end

abstract type EditError <: DiagnosticError end

"""A declaration referring to an edited target."""
struct Referrer
    name::String
    document::String
end
Base.:(==)(a::Referrer, b::Referrer) =
    a.name == b.name && a.document == b.document
Base.hash(r::Referrer, h::UInt) = hash((r.name, r.document), h)
Base.show(io::IO, r::Referrer) =
    print(io, "Referrer(name=$(repr(r.name)), document=$(repr(r.document)))")

for name in (
    :EditResultError, :EditTargetError, :InvalidEditError, :NoEditsError,
    :OverlappingEditsError, :RenameReferencedError, :OwnerNotFoundError,
    :OwnerNotNamespaceError, :IllegalMemberKindError, :MemberNameTakenError,
    :DeleteReferencedError, :OwnerInsideTargetError, :MoveReferencedError,
    :ReferencedElsewhereError, :EditFailureError,
)
    @eval begin
        struct $name <: EditError
            message::String
            failure::String
            diagnostics::Vector{Diagnostic}
            referring_elements::Vector{String}
            referrers::Vector{Referrer}
        end
        function $name(message::AbstractString; failure="", diagnostics=Diagnostic[],
                       referring_elements=String[], referrers=Referrer[])
            $name(String(message), String(failure),
                 Diagnostic[d isa Diagnostic ? d : Diagnostic(d) for d in diagnostics],
                 String[String(v) for v in referring_elements],
                 Referrer[Referrer(r.name, r.document) for r in referrers])
        end
    end
end
Base.showerror(io::IO, err::EditError) = print(io, err.message)
Base.show(io::IO, err::EditError) = print(io, err.message)

struct SymbolNotFoundError <: OpenSysMLError
    name::String
    suggestions::Vector{String}
    SymbolNotFoundError(name, suggestions=String[]) =
        new(String(name), String[s for s in suggestions])
end

struct MissingCapabilityError <: OpenSysMLError
    capability::String
    info::Any
    remedy::String
end

struct StaleServiceError <: OpenSysMLError
    address::String
    reason::String
    remedy::String
    info::Any
end

struct QueryError <: OpenSysMLError
    message::String
end
struct DocumentQueryError <: OpenSysMLError
    message::String
end
struct UnsupportedValueError <: OpenSysMLError
    message::String
end
struct FeatureValueError <: OpenSysMLError
    feature_name::String
    message::String
    FeatureValueError(feature_name, message) =
        new(String(feature_name), String(message))
end
struct TypeMismatchError <: OpenSysMLError
    feature_name::String
    expected::String
    value::Any
end
struct InstanceTypeError <: OpenSysMLError
    expected::String
    actual::String
end
struct IncommensurableUnitsError <: OpenSysMLError
    message::String
    operation::String
    left::Any
    right::Any
end
IncommensurableUnitsError(message::AbstractString) =
    IncommensurableUnitsError(String(message), "", nothing, nothing)
IncommensurableUnitsError(operation, left, right) =
    IncommensurableUnitsError(
        "cannot $(operation) a quantity in [$(left)] and one in [$(right)]: " *
        "$(left) reduces to $(reduction(left)) and $(right) to $(reduction(right)), " *
        "which measure different things",
        String(operation), left, right)

Base.showerror(io::IO, e::DiagnosticError) = begin
    print(io, "$(nameof(typeof(e))): $(e.message)")
    for d in e.diagnostics
        print(io, "\n  $(d.severity): $(d.message)")
    end
end
Base.showerror(io::IO, e::SymbolNotFoundError) = begin
    msg = "no symbol named $(repr(e.name)) in this model"
    isempty(e.suggestions) || (msg *= "; did you mean " * join(repr.(e.suggestions), ", ") * "?")
    print(io, msg)
end
Base.showerror(io::IO, e::MissingCapabilityError) =
    print(io, "the sysml-grpc service does not support the $(repr(e.capability)) capability, which this operation requires.\n  service: $(e.info === nothing ? "service information unavailable" : describe(e.info))\n  fix:     $(e.remedy)")
Base.showerror(io::IO, e::StaleServiceError) =
    print(io, "the sysml-grpc service already listening on $(e.address) is not the one this client asked for: $(e.reason).\n  service: $(e.info === nothing ? e.address : describe(e.info))\n  fix:     $(e.remedy)")
for name in (:QueryError, :DocumentQueryError, :UnsupportedValueError, :IncommensurableUnitsError)
    @eval Base.showerror(io::IO, e::$name) = print(io, "$(nameof(typeof(e))): $(e.message)")
end
Base.showerror(io::IO, e::FeatureValueError) = print(io, "feature value $(repr(e.feature_name)): $(e.message)")
Base.showerror(io::IO, e::TypeMismatchError) = print(io, "feature value $(repr(e.feature_name)): expected $(e.expected), got $(repr(e.value))")
Base.showerror(io::IO, e::InstanceTypeError) = print(io, "instance of $(repr(e.actual)) is not a $(repr(e.expected))")
