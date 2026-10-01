struct AppliedEdit
    operation_index::Int
    target::String
    offset::Int
    length::Int
    old_text::String
    new_text::String
    document::String
end
AppliedEdit(operation_index::Integer, target::AbstractString, offset::Integer,
            length::Integer, old_text::AbstractString, new_text::AbstractString,
            document::AbstractString="") =
    AppliedEdit(Int(operation_index), String(target), Int(offset), Int(length),
                String(old_text), String(new_text), String(document))
Base.:(==)(a::AppliedEdit, b::AppliedEdit) =
    all(getfield(a, field) == getfield(b, field) for field in fieldnames(AppliedEdit))
Base.show(io::IO, edit::AppliedEdit) =
    print(io, "$(edit.target): $(repr(edit.old_text)) -> $(repr(edit.new_text))")

struct EditedDocument
    name::String
    content::String
end
EditedDocument(name::AbstractString, content::AbstractString) =
    EditedDocument(String(name), String(content))
Base.:(==)(a::EditedDocument, b::EditedDocument) =
    a.name == b.name && a.content == b.content
Base.show(io::IO, document::EditedDocument) = print(io, document.content)

struct EditResult
    content::String
    from_format::String
    to_format::String
    diagnostics::Vector{Diagnostic}
    experimental::Bool
    experimental_notice::String
    applied::Vector{AppliedEdit}
    documents::Vector{EditedDocument}
end
EditResult(content, from_format, to_format, diagnostics, applied, documents) =
    EditResult(String(content), String(from_format), String(to_format),
        Diagnostic[d isa Diagnostic ? d : Diagnostic(d) for d in diagnostics],
        false, "", AppliedEdit[a for a in applied], EditedDocument[d for d in documents])
Base.show(io::IO, result::EditResult) = print(io, result.content)
function save(result::EditResult, path::AbstractString)
    open(path, "w") do io
        write(io, result.content)
    end
    String(path)
end

function _edit_failure_name(failure)
    failure isa AbstractString && return String(failure)
    failure isa Integer || return ""
    names = (
        "EDIT_FAILURE_UNSPECIFIED", "EDIT_FAILURE_NO_OPERATIONS",
        "EDIT_FAILURE_UNKNOWN_TARGET", "EDIT_FAILURE_AMBIGUOUS_TARGET",
        "EDIT_FAILURE_NOT_VALUED", "EDIT_FAILURE_INVALID_VALUE",
        "EDIT_FAILURE_INVALID_NAME", "EDIT_FAILURE_NOT_NAMED",
        "EDIT_FAILURE_RENAME_REFERENCED", "EDIT_FAILURE_OVERLAPPING_EDITS",
        "EDIT_FAILURE_RESULT_INVALID", "EDIT_FAILURE_OWNER_UNKNOWN",
        "EDIT_FAILURE_OWNER_NOT_NAMESPACE", "EDIT_FAILURE_ILLEGAL_KIND",
        "EDIT_FAILURE_MEMBER_NAME_TAKEN", "EDIT_FAILURE_DELETE_REFERENCED",
        "EDIT_FAILURE_OWNER_INSIDE_TARGET", "EDIT_FAILURE_MOVE_REFERENCED",
        "EDIT_FAILURE_REFERENCED_ELSEWHERE",
    )
    index = Int(failure) + 1
    1 <= index <= length(names) ? names[index] : "EDIT_FAILURE_$(failure)"
end

const _EDIT_FAILURE_ERRORS = Dict{String,DataType}(
    "EDIT_FAILURE_NO_OPERATIONS" => NoEditsError,
    "EDIT_FAILURE_UNKNOWN_TARGET" => EditTargetError,
    "EDIT_FAILURE_AMBIGUOUS_TARGET" => EditTargetError,
    "EDIT_FAILURE_NOT_VALUED" => EditTargetError,
    "EDIT_FAILURE_NOT_NAMED" => EditTargetError,
    "EDIT_FAILURE_INVALID_VALUE" => InvalidEditError,
    "EDIT_FAILURE_INVALID_NAME" => InvalidEditError,
    "EDIT_FAILURE_RENAME_REFERENCED" => RenameReferencedError,
    "EDIT_FAILURE_OVERLAPPING_EDITS" => OverlappingEditsError,
    "EDIT_FAILURE_RESULT_INVALID" => EditResultError,
    "EDIT_FAILURE_OWNER_UNKNOWN" => OwnerNotFoundError,
    "EDIT_FAILURE_OWNER_NOT_NAMESPACE" => OwnerNotNamespaceError,
    "EDIT_FAILURE_ILLEGAL_KIND" => IllegalMemberKindError,
    "EDIT_FAILURE_MEMBER_NAME_TAKEN" => MemberNameTakenError,
    "EDIT_FAILURE_DELETE_REFERENCED" => DeleteReferencedError,
    "EDIT_FAILURE_OWNER_INSIDE_TARGET" => OwnerInsideTargetError,
    "EDIT_FAILURE_MOVE_REFERENCED" => MoveReferencedError,
    "EDIT_FAILURE_REFERENCED_ELSEWHERE" => ReferencedElsewhereError,
)

function _edit_error(answer)
    failure = _edit_failure_name(get(answer, "failure", ""))
    T = get(_EDIT_FAILURE_ERRORS, failure, EditFailureError)
    T(String(get(answer, "error", "edit was refused"));
      failure=failure,
      diagnostics=Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])],
      referring_elements=String[String(v) for v in get(answer, "referringElements", Any[])],
      referrers=Referrer[Referrer(String(get(r, "name", "")),
                                 String(get(r, "document", "")))
                         for r in get(answer, "referrers", Any[])])
end

function _edit_result(answer)
    diagnostics = Diagnostic[Diagnostic(d) for d in get(answer, "diagnostics", Any[])]
    message = String(get(answer, "error", ""))
    isempty(message) || throw(_edit_error(answer))
    applied = AppliedEdit[AppliedEdit(
        Int(get(a, "operationIndex", 0)), String(get(a, "target", "")),
        Int(get(a, "offset", 0)), Int(get(a, "length", 0)),
        String(get(a, "oldText", "")), String(get(a, "newText", "")),
        String(get(a, "document", ""))) for a in get(answer, "applied", Any[])]
    documents = EditedDocument[EditedDocument(String(get(d, "name", "")),
        String(get(d, "content", ""))) for d in get(answer, "documents", Any[])]
    EditResult(String(get(answer, "content", "")), "sysml", "sysml",
               diagnostics, applied, documents)
end
@doc "IllegalMemberKindError is a flat EditError subtype, not an InvalidEditError subtype." IllegalMemberKindError

const _EDIT_CAPABILITY_ORDER = (
    CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING, CAPABILITY_CONNECTION_AUTHORING,
    CAPABILITY_SATISFY_AUTHORING, CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
    CAPABILITY_TRANSITION_AUTHORING, CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
    CAPABILITY_METADATA_AUTHORING, CAPABILITY_METADATA_PREFIX_AUTHORING,
    CAPABILITY_SEQUENCE_AUTHORING, CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
    CAPABILITY_IMPORT_AUTHORING, CAPABILITY_DOCUMENTATION_AUTHORING,
    CAPABILITY_COMMENT_AUTHORING, CAPABILITY_MEMBER_MODIFIERS,
    CAPABILITY_IMPLICIT_PARAMETERS, CAPABILITY_CONSTRAINT_BODY_AUTHORING,
    CAPABILITY_STATE_ACTION_AUTHORING, CAPABILITY_EDIT_DOCUMENTS,
)
const _EDIT_CAPABILITIES_CHECKED_LAST = (
    CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
    CAPABILITY_MEMBER_MODIFIERS, CAPABILITY_IMPLICIT_PARAMETERS,
)
const _ASSERTED_CONSTRAINT_KINDS =
    ("assert", "assert not", "assert constraint", "assert not constraint")
const _STATE_ACTION_KINDS =
    ("exhibit state", "exhibit", "entry action", "do action", "exit action")
const _ACTION_BODY_MEMBER_KINDS =
    ("accept", "send", "assign", "if", "while", "loop", "for", "terminate")
const _SEQUENCE_DETAIL_FIELDS =
    ("condition", "value", "target", "via", "until", "body", "else_body",
     "multiplicity", "parameter")

function _edit_require!(conn, requested, capabilities...)
    for capability in capabilities
        require_capability(conn, capability)
        push!(requested, String(capability))
    end
end
_edit_note!(requested, capability, needed=true) =
    needed && push!(requested, String(capability))

_edit_text(value) = value isa AbstractString ? String(value) :
    throw(ArgumentError("expected notation text, got $(typeof(value))"))
_edit_all_text(values) = all(v -> v isa AbstractString, values)

function _edit_target_id(target)
    target isa AbstractString && return String(target)
    if target isa AbstractDict && haskey(target, "id")
        id = target["id"]
        id isa AbstractString && !isempty(id) && return String(id)
    end
    if hasproperty(target, :id)
        id = getproperty(target, :id)
        id isa AbstractString && !isempty(id) && return String(id)
    end
    throw(ArgumentError("target must be a symbol id (FQN) or an object with an id"))
end
_edit_owner_id(owner) = owner isa AbstractString ? String(owner) : _edit_target_id(owner)

function _notation_references(label, values)
    values === nothing && return String[]
    values isa AbstractString && return String[String(values)]
    values isa AbstractVector || values isa Tuple ||
        throw(ArgumentError("$(label) must be a notation string or sequence of strings"))
    _edit_all_text(values) ||
        throw(ArgumentError("$(label) must contain only notation strings"))
    String[String(v) for v in values]
end

function _member_extras(extra)
    metadata = Any[]
    body_expression = ""
    doc = ""
    if !isempty(extra)
        if first(extra) isa AbstractString
            body_expression = _edit_text(first(extra))
            length(extra) == 2 && (doc = _edit_text(extra[2]))
        else
            metadata = first(extra)
            length(extra) > 1 && (body_expression = _edit_text(extra[2]))
            length(extra) > 2 && (doc = _edit_text(extra[3]))
        end
    end
    return metadata, body_expression, doc
end

function _edit_member_json!(conn, operation, requested)
    length(operation) in (8, 12, 13, 14, 15) ||
        throw(ArgumentError("malformed add_member operation: expected 8, 12, 13, 14 or 15 fields"))
    _, owner, kind, name, type_name, multiplicity, value, specializes = operation[1:8]
    all(_edit_all_text((owner, kind, name, type_name, multiplicity, value)) for _ in 1:1) ||
        throw(ArgumentError("malformed add_member operation: text fields must be notation strings"))
    specializes isa AbstractVector || specializes isa Tuple ||
        throw(ArgumentError("malformed add_member operation: specializes must be a sequence"))
    _edit_all_text(specializes) ||
        throw(ArgumentError("malformed add_member operation: specializes must be notation strings"))
    modifiers = length(operation) >= 12 ? operation[9:12] : ()
    metadata, body_expression, doc = _member_extras(operation[13:end])
    _edit_require!(conn, requested, CAPABILITY_AUTHORING)
    _edit_note!(requested, CAPABILITY_IMPLICIT_PARAMETERS, kind == "")
    kind == "objective" && isempty(name) &&
        _edit_require!(conn, requested, CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING)
    !isempty(doc) && _edit_require!(conn, requested, CAPABILITY_DOCUMENTATION_AUTHORING)
    add = Dict{String,Any}(
        "owner" => String(owner), "kind" => String(kind), "name" => String(name),
        "type" => String(type_name), "multiplicity" => String(multiplicity),
        "value" => String(value), "specializes" => String[String(s) for s in specializes],
    )
    if !isempty(modifiers)
        abstract, redefines, default, direction = modifiers
        (abstract isa Bool && default isa Bool) ||
            throw(ArgumentError("malformed add_member modifiers: abstract and default must be bool"))
        (redefines isa AbstractVector || redefines isa Tuple) &&
            !(redefines isa AbstractString) && _edit_all_text(redefines) ||
            throw(ArgumentError("malformed add_member modifiers: redefines must be a sequence"))
        direction isa AbstractString ||
            throw(ArgumentError("malformed add_member modifiers: direction must be notation text"))
        add["isAbstract"] = abstract
        add["redefines"] = String[String(v) for v in redefines]
        add["isDefault"] = default
        add["direction"] = String(direction)
        _edit_note!(requested, CAPABILITY_MEMBER_MODIFIERS,
            abstract || !isempty(redefines) || default || !isempty(direction) ||
            kind in ("ref", "return"))
    end
    (metadata isa AbstractVector || metadata isa Tuple) && _edit_all_text(metadata) ||
        throw(ArgumentError("malformed add_member metadata: expected a sequence of notation strings"))
    add["metadataPrefixes"] = String[String(v) for v in metadata]
    isempty(metadata) || _edit_require!(conn, requested, CAPABILITY_METADATA_AUTHORING)
    add["bodyExpression"] = body_expression
    (isempty(body_expression) && !(kind in _ASSERTED_CONSTRAINT_KINDS)) ||
        _edit_require!(conn, requested, CAPABILITY_CONSTRAINT_BODY_AUTHORING)
    kind in _STATE_ACTION_KINDS &&
        _edit_require!(conn, requested, CAPABILITY_STATE_ACTION_AUTHORING)
    isempty(doc) || (add["doc"] = doc)
    Dict("addMember" => add)
end

function _sequence_json!(conn, operation, requested, depth=0)
    depth <= 128 || throw(ArgumentError("nested action-body items exceed the maximum depth"))
    (operation isa Tuple || operation isa AbstractVector) &&
        length(operation) in (8, 9) ||
        throw(ArgumentError("malformed add_sequence operation: expected 8 or 9 fields"))
    _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_SEQUENCE_AUTHORING)
    _, owner, keyword, ref, member_kind, member_name, type_name, after = operation[1:8]
    _edit_all_text((owner, keyword, ref, member_kind, member_name, type_name, after)) ||
        throw(ArgumentError("malformed add_sequence operation: fields must be text"))
    fields = length(operation) == 9 ? operation[9] : Dict{String,Any}()
    fields isa AbstractDict ||
        throw(ArgumentError("malformed add_sequence operation: ninth field must be a dictionary"))
    for (key, value) in fields
        if String(key) in ("body", "else_body")
            (value isa Tuple || value isa AbstractVector) ||
                throw(ArgumentError("malformed add_sequence operation: $(key) must be a list"))
        else
            value isa AbstractString ||
                throw(ArgumentError("malformed add_sequence operation: $(key) must be text"))
        end
    end
    keys_text = String[String(k) for k in keys(fields)]
    unknown = setdiff(keys_text, collect(_SEQUENCE_DETAIL_FIELDS))
    isempty(unknown) ||
        throw(ArgumentError("malformed add_sequence operation: unknown fields $(join(sort!(unknown), ", "))"))
    add = Dict{String,Any}(
        "owner" => String(owner), "keyword" => String(keyword), "ref" => String(ref),
        "memberKind" => String(member_kind), "memberName" => String(member_name),
        "type" => String(type_name), "after" => String(after),
    )
    for (key, value) in fields
        field = String(key)
        if field == "body" || field == "else_body"
            camel = field == "else_body" ? "elseBody" : "body"
            add[camel] = Any[_sequence_json!(conn, child, requested, depth + 1)["addSequence"]
                             for child in value]
        else
            add[field] = String(value)
        end
    end
    extended = keyword in ("if", "else") ||
        (isempty(keyword) && !isempty(member_kind)) ||
        member_kind in _ACTION_BODY_MEMBER_KINDS || !isempty(fields)
    _edit_note!(requested, CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING, extended)
    Dict("addSequence" => add)
end

function _edit_operation_json(conn, operation, requested)
    (operation isa Tuple || operation isa AbstractVector) && !isempty(operation) ||
        throw(ArgumentError("edit operation must be a non-empty tuple"))
    kind = operation[1] isa AbstractString ? String(operation[1]) : ""
    if kind == "set_value" || kind == "rename"
        length(operation) == 3 ||
            throw(ArgumentError("malformed $(kind) operation: expected 3 fields"))
        _, target, text = operation
        _edit_all_text((target, text)) ||
            throw(ArgumentError("malformed $(kind) operation: fields must be notation text"))
        nested = kind == "set_value" ?
            Dict{String,Any}("target" => String(target), "value" => String(text)) :
            Dict{String,Any}("target" => String(target), "newName" => String(text))
        return Dict((kind == "set_value" ? "setValue" : "rename") => nested)
    elseif kind == "add_member"
        return _edit_member_json!(conn, operation, requested)
    elseif kind == "add_connection"
        length(operation) == 7 ||
            throw(ArgumentError("malformed add_connection operation: expected 7 fields"))
        _, owner, connection_kind, from_end, to_end, name, type_name = operation
        _edit_all_text((owner, connection_kind, from_end, to_end, name, type_name)) ||
            throw(ArgumentError("malformed add_connection operation: fields must be notation text"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_CONNECTION_AUTHORING)
        return Dict("addConnection" => Dict("owner" => String(owner), "kind" => String(connection_kind),
            "fromEnd" => String(from_end), "toEnd" => String(to_end), "name" => String(name),
            "type" => String(type_name)))
    elseif kind == "add_satisfy"
        length(operation) == 6 ||
            throw(ArgumentError("malformed add_satisfy operation: expected 6 fields"))
        _, owner, requirement, satisfying_feature, asserted, negated = operation
        _edit_all_text((owner, requirement, satisfying_feature)) &&
            asserted isa Bool && negated isa Bool ||
            throw(ArgumentError("malformed add_satisfy operation: fields must be notation text and bool"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_SATISFY_AUTHORING)
        return Dict("addSatisfy" => Dict("owner" => String(owner), "requirement" => String(requirement),
            "satisfyingFeature" => String(satisfying_feature), "isAsserted" => asserted,
            "isNegated" => negated))
    elseif kind == "add_requirement_constraint"
        length(operation) == 5 ||
            throw(ArgumentError("malformed add_requirement_constraint operation: expected 5 fields"))
        _, owner, constraint_kind, expression, name = operation
        _edit_all_text((owner, constraint_kind, expression, name)) ||
            throw(ArgumentError("malformed add_requirement_constraint operation: fields must be notation text"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING)
        return Dict("addRequirementConstraint" => Dict("owner" => String(owner),
            "kind" => String(constraint_kind), "expression" => String(expression), "name" => String(name)))
    elseif kind == "add_transition"
        length(operation) == 9 ||
            throw(ArgumentError("malformed add_transition operation: expected 9 fields"))
        _, owner, name, source, target, trigger, guard, effect, initial = operation
        _edit_all_text((owner, name, source, target, trigger, guard, effect)) && initial isa Bool ||
            throw(ArgumentError("malformed add_transition operation: text fields and initial must be valid"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_TRANSITION_AUTHORING)
        return Dict("addTransition" => Dict("owner" => String(owner), "name" => String(name),
            "source" => String(source), "target" => String(target), "trigger" => String(trigger),
            "guard" => String(guard), "effect" => String(effect), "initial" => initial))
    elseif kind == "add_verify"
        length(operation) == 3 ||
            throw(ArgumentError("malformed add_verify operation: expected 3 fields"))
        _, owner, requirement = operation
        _edit_all_text((owner, requirement)) ||
            throw(ArgumentError("malformed add_verify operation: fields must be notation text"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING)
        return Dict("addVerify" => Dict("owner" => String(owner), "requirement" => String(requirement)))
    elseif kind == "add_metadata"
        length(operation) == 7 ||
            throw(ArgumentError("malformed add_metadata operation: expected 7 fields"))
        _, owner, metadata_type, name, about, values, shorthand = operation
        _edit_all_text((owner, metadata_type, name)) && shorthand isa Bool ||
            throw(ArgumentError("malformed add_metadata operation: text fields and shorthand must be valid"))
        (about isa Tuple || about isa AbstractVector) && _edit_all_text(about) ||
            throw(ArgumentError("malformed add_metadata operation: about must be notation strings"))
        (values isa Tuple || values isa AbstractVector) && !(values isa AbstractString) ||
            throw(ArgumentError("malformed add_metadata operation: values must be feature-value pairs"))
        bindings = Any[]
        for (index, pair) in enumerate(values)
            (pair isa Tuple || pair isa AbstractVector) && length(pair) == 2 &&
                _edit_all_text(pair) ||
                throw(ArgumentError("malformed add_metadata operation: values[$(index - 1)] must be a pair of strings"))
            push!(bindings, Dict("feature" => String(pair[1]), "value" => String(pair[2])))
        end
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_METADATA_AUTHORING)
        return Dict("addMetadata" => Dict("owner" => String(owner), "metadataType" => String(metadata_type),
            "name" => String(name), "about" => String[String(v) for v in about],
            "values" => bindings, "shorthand" => shorthand))
    elseif kind == "add_metadata_prefix"
        length(operation) == 3 ||
            throw(ArgumentError("malformed add_metadata_prefix operation: expected 3 fields"))
        _, target, metadata_type = operation
        _edit_all_text((target, metadata_type)) ||
            throw(ArgumentError("malformed add_metadata_prefix operation: fields must be notation text"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_METADATA_PREFIX_AUTHORING)
        return Dict("addMetadataPrefix" => Dict("target" => String(target),
            "metadataType" => String(metadata_type)))
    elseif kind == "add_sequence"
        return _sequence_json!(conn, operation, requested)
    elseif kind == "add_import"
        length(operation) == 7 ||
            throw(ArgumentError("malformed add_import operation: expected 7 fields"))
        _, owner, visibility, target, recursive, import_all, filters = operation
        _edit_all_text((owner, visibility, target)) && recursive isa Bool && import_all isa Bool &&
            (filters isa Tuple || filters isa AbstractVector) && _edit_all_text(filters) ||
            throw(ArgumentError("malformed add_import operation: fields must be valid notation text and bools"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_IMPORT_AUTHORING)
        return Dict("addImport" => Dict("owner" => String(owner), "visibility" => String(visibility),
            "target" => String(target), "isRecursive" => recursive, "isImportAll" => import_all,
            "filters" => String[String(v) for v in filters]))
    elseif kind == "add_documentation"
        length(operation) == 6 ||
            throw(ArgumentError("malformed add_documentation operation: expected 6 fields"))
        _, target, body, name, locale, replace = operation
        _edit_all_text((target, body, name, locale)) && replace isa Bool ||
            throw(ArgumentError("malformed add_documentation operation: text fields and replace must be valid"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_DOCUMENTATION_AUTHORING)
        return Dict("addDocumentation" => Dict("target" => String(target), "body" => String(body),
            "name" => String(name), "locale" => String(locale), "replace" => replace))
    elseif kind == "add_comment"
        length(operation) == 6 ||
            throw(ArgumentError("malformed add_comment operation: expected 6 fields"))
        _, owner, body, name, about, locale = operation
        _edit_all_text((owner, body, name, locale)) &&
            (about isa Tuple || about isa AbstractVector) && _edit_all_text(about) ||
            throw(ArgumentError("malformed add_comment operation: text fields and about must be valid"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING)
        return Dict("addComment" => Dict("owner" => String(owner), "body" => String(body),
            "name" => String(name), "about" => String[String(v) for v in about],
            "locale" => String(locale)))
    elseif kind == "add_note"
        length(operation) == 3 && _edit_all_text(operation[2:3]) ||
            throw(ArgumentError("malformed add_note operation: expected target and text"))
        _, target, text = operation
        _edit_require!(conn, requested, CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING)
        return Dict("addNote" => Dict("target" => String(target), "text" => String(text)))
    elseif kind == "delete"
        length(operation) == 3 && operation[3] isa Bool ||
            throw(ArgumentError("malformed delete operation: expected target and bool cascade"))
        _, target, cascade = operation
        target isa AbstractString ||
            throw(ArgumentError("malformed delete operation: target must be notation text"))
        _edit_require!(conn, requested, CAPABILITY_AUTHORING)
        return Dict("delete" => Dict("target" => String(target), "cascade" => cascade))
    elseif kind == "move"
        length(operation) == 3 && _edit_all_text(operation[2:3]) ||
            throw(ArgumentError("malformed move operation: expected target and owner"))
        _, target, owner = operation
        _edit_require!(conn, requested, CAPABILITY_AUTHORING)
        return Dict("move" => Dict("target" => String(target), "owner" => String(owner)))
    end
    throw(ArgumentError("unknown edit operation $(repr(kind)): expected set_value, rename, add_member, add_connection, add_satisfy, add_requirement_constraint, add_transition, add_verify, add_metadata, add_metadata_prefix, add_sequence, add_import, add_documentation, add_comment, add_note, delete or move"))
end

function _apply_edits(conn::Connection, model_hash::AbstractString, operations;
                      document="", accept_documents=true, multi_document=false)
    require_capability(conn, CAPABILITY_APPLY_EDITS)
    requested = Set{String}([CAPABILITY_APPLY_EDITS])
    request_operations = Any[_edit_operation_json(conn, op, requested) for op in operations]
    for capability in _EDIT_CAPABILITIES_CHECKED_LAST
        capability in requested && require_capability(conn, capability)
    end
    multi_document && require_capability(conn, CAPABILITY_EDIT_DOCUMENTS)
    multi_document && push!(requested, CAPABILITY_EDIT_DOCUMENTS)
    request = Dict{String,Any}("modelHash" => String(model_hash),
        "operations" => request_operations, "acceptDocuments" => Bool(accept_documents))
    isempty(document) || (request["document"] = String(document))
    capabilities = Tuple(capability for capability in _EDIT_CAPABILITY_ORDER
                         if capability in requested)
    answer = _translate(; capabilities=capabilities, connection=conn) do
        call(conn, "ApplyEdits", request)
    end
    _edit_result(answer)
end

"""An item builder for nested action-body statements."""
mutable struct Body
    operations::Vector{Any}
end
Body() = Body(Any[])
Base.length(body::Body) = length(body.operations)
Base.isempty(body::Body) = isempty(body.operations)

"""Collect source-preserving edits for one loaded model."""
mutable struct Editor
    model_hash::String
    connection::Union{Connection,Nothing}
    operations::Vector{Any}
    applied::Bool
    model::Union{Model,Nothing}
end
Editor(model_hash::AbstractString, conn::Connection) =
    Editor(String(model_hash), conn, Any[], false, nothing)
Editor(model_hash::AbstractString, ::Nothing) =
    Editor(String(model_hash), nothing, Any[], false, nothing)
Editor(model::Model) = Editor(model.hash, model.connection, Any[], false, model)
operations(body::Body) = copy(body.operations)
operations(editor::Editor) = copy(editor.operations)
applied(editor::Editor) = editor.applied
Base.length(editor::Editor) = length(editor.operations)
Base.isempty(editor::Editor) = isempty(editor.operations)
Base.show(io::IO, editor::Editor) =
    print(io, "Editor(model_hash=$(repr(editor.model_hash)), operations=$(length(editor)), applied=$(editor.applied))")

function _editor_add!(editor::Editor, operation)
    editor.applied && throw(ArgumentError(
        "this editor has already been applied: build another editor from the edited model to edit further"))
    push!(editor.operations, operation)
    editor
end

function set_value(editor::Editor, target, value)
    value isa AbstractString ||
        throw(ArgumentError("value must be SysML notation for an expression, not $(typeof(value))"))
    _editor_add!(editor, ("set_value", _edit_target_id(target), String(value)))
end
function rename(editor::Editor, target, new_name)
    new_name isa AbstractString || throw(ArgumentError("new_name must be a name, not $(typeof(new_name))"))
    _editor_add!(editor, ("rename", _edit_target_id(target), String(new_name)))
end

function add_member(editor::Editor, owner, kind, name; type=nothing, multiplicity=nothing,
                    value=nothing, specializes=nothing, abstract=false, redefines=nothing,
                    default=false, direction=nothing, metadata=nothing, expression=nothing,
                    doc=nothing)
    kind isa AbstractString || throw(ArgumentError("kind must be notation text, not $(typeof(kind))"))
    name isa AbstractString || throw(ArgumentError("name must be notation text, not $(typeof(name))"))
    for (label, text) in (("type", type), ("multiplicity", multiplicity), ("value", value),
                          ("expression", expression))
        text === nothing || text isa AbstractString ||
            throw(ArgumentError("$(label) must be notation text, not $(typeof(text))"))
    end
    (abstract isa Bool && default isa Bool) || throw(ArgumentError("abstract and default must be bool"))
    direction === nothing || direction isa AbstractString ||
        throw(ArgumentError("direction must be notation text, not $(typeof(direction))"))
    doc === nothing || doc isa AbstractString ||
        throw(ArgumentError("doc must be text, not $(typeof(doc))"))
    specializations = _notation_references("specializes", specializes)
    redefinition = _notation_references("redefines", redefines)
    has_metadata = metadata !== nothing
    prefixes = _notation_references("metadata", metadata)
    base = ("add_member", _edit_owner_id(owner), String(kind), String(name),
        type === nothing ? "" : String(type), multiplicity === nothing ? "" : String(multiplicity),
        value === nothing ? "" : String(value), specializations)
    modifiers_needed = abstract || !isempty(redefinition) || default ||
        direction !== nothing || has_metadata || String(kind) in ("ref", "return") ||
        String(kind) == "" || expression !== nothing || (doc !== nothing && !isempty(doc))
    operation = modifiers_needed ? (base..., abstract, redefinition, default,
        direction === nothing ? "" : String(direction)) : base
    has_metadata && (operation = (operation..., prefixes))
    if expression !== nothing || (doc !== nothing && !isempty(doc))
        operation = (operation..., expression === nothing ? "" : String(expression))
    end
    doc !== nothing && !isempty(doc) && (operation = (operation..., String(doc)))
    _editor_add!(editor, operation)
end

function add_objective(editor::Editor, owner; name=nothing, type=nothing)
    name === nothing || name isa AbstractString ||
        throw(ArgumentError("name must be notation text, not $(typeof(name))"))
    type === nothing || type isa AbstractString ||
        throw(ArgumentError("type must be notation text, not $(typeof(type))"))
    add_member(editor, owner, "objective", name === nothing ? "" : String(name); type=type)
end

function add_verify(editor::Editor, owner, requirement)
    requirement isa AbstractString || throw(ArgumentError("requirement must be notation text"))
    _editor_add!(editor, ("add_verify", _edit_owner_id(owner), String(requirement)))
end

function add_metadata(editor::Editor, owner, metadata_type; values=nothing, name=nothing,
                      about=nothing, shorthand=false)
    metadata_type isa AbstractString ||
        throw(ArgumentError("metadata_type must be notation text"))
    name === nothing || name isa AbstractString || throw(ArgumentError("name must be notation text"))
    shorthand isa Bool || throw(ArgumentError("shorthand must be bool"))
    bindings = Any[]
    if values isa AbstractDict
        append!(bindings, collect(pairs(values)))
    elseif values !== nothing
        (values isa AbstractVector || values isa Tuple) ||
            throw(ArgumentError("values must be a mapping or a sequence of (feature, value) pairs"))
        append!(bindings, values)
    end
    normalized = Tuple{String,String}[]
    for (index, pair) in enumerate(bindings)
        (pair isa Tuple || pair isa Pair || pair isa AbstractVector) && length(pair) == 2 ||
            throw(ArgumentError("values[$(index - 1)] must be a pair of strings"))
        _edit_all_text(pair) || throw(ArgumentError("values[$(index - 1)] feature and value must be strings"))
        push!(normalized, (String(pair[1]), String(pair[2])))
    end
    about_values = _notation_references("about", about)
    _editor_add!(editor, ("add_metadata", _edit_owner_id(owner), String(metadata_type),
        name === nothing ? "" : String(name), about_values, normalized, shorthand))
end

function add_metadata_prefix(editor::Editor, target, metadata_type)
    metadata_type isa AbstractString || throw(ArgumentError("metadata_type must be notation text"))
    _editor_add!(editor, ("add_metadata_prefix", _edit_target_id(target), String(metadata_type)))
end

function add_documentation(editor::Editor, target, body; name=nothing, locale=nothing,
                           replace=false)
    body isa AbstractString || throw(ArgumentError("body must be text, not $(typeof(body))"))
    name === nothing || name isa AbstractString || throw(ArgumentError("name must be text"))
    locale === nothing || locale isa AbstractString || throw(ArgumentError("locale must be text"))
    replace isa Bool || throw(ArgumentError("replace must be a bool, not $(typeof(replace))"))
    _editor_add!(editor, ("add_documentation", _edit_target_id(target), String(body),
        name === nothing ? "" : String(name), locale === nothing ? "" : String(locale), replace))
end

function add_comment(editor::Editor, owner, body; name=nothing, about=nothing, locale=nothing)
    body isa AbstractString || throw(ArgumentError("body must be text, not $(typeof(body))"))
    name === nothing || name isa AbstractString || throw(ArgumentError("name must be text"))
    locale === nothing || locale isa AbstractString || throw(ArgumentError("locale must be text"))
    about isa AbstractString && throw(ArgumentError("about must be a sequence of names, not one name"))
    about_names = String[_edit_target_id(value) for value in (about === nothing ? () : about)]
    _editor_add!(editor, ("add_comment", _edit_owner_id(owner), String(body),
        name === nothing ? "" : String(name), about_names,
        locale === nothing ? "" : String(locale)))
end

function add_note(editor::Editor, target, text)
    text isa AbstractString || throw(ArgumentError("text must be text, not $(typeof(text))"))
    occursin('\n', text) || occursin('\r', text) ?
        throw(ArgumentError("a note is one line: its text may not contain a line break")) : nothing
    _editor_add!(editor, ("add_note", _edit_target_id(target), String(text)))
end

function add_satisfy(editor::Editor, owner, requirement; by=nothing, asserted=false, negated=false)
    requirement isa AbstractString || throw(ArgumentError("requirement must be notation text"))
    by === nothing || by isa AbstractString || throw(ArgumentError("by must be notation text"))
    (asserted isa Bool && negated isa Bool) || throw(ArgumentError("asserted and negated must be bool"))
    _editor_add!(editor, ("add_satisfy", _edit_owner_id(owner), String(requirement),
        by === nothing ? "" : String(by), asserted, negated))
end

function add_requirement_constraint(editor::Editor, owner, kind, expression,
                                    positional_name=nothing; name=positional_name)
    kind isa AbstractString && expression isa AbstractString ||
        throw(ArgumentError("kind and expression must be notation text"))
    name === nothing || name isa AbstractString || throw(ArgumentError("name must be notation text"))
    _editor_add!(editor, ("add_requirement_constraint", _edit_owner_id(owner), String(kind),
        String(expression), name === nothing ? "" : String(name)))
end

function add_transition(editor::Editor, owner, source, target; name=nothing, trigger=nothing,
                        guard=nothing, effect=nothing)
    source isa AbstractString && target isa AbstractString ||
        throw(ArgumentError("source and target must be notation text"))
    for (label, value) in (("name", name), ("trigger", trigger), ("guard", guard), ("effect", effect))
        value === nothing || value isa AbstractString ||
            throw(ArgumentError("$(label) must be notation text, not $(typeof(value))"))
    end
    _editor_add!(editor, ("add_transition", _edit_owner_id(owner),
        name === nothing ? "" : String(name), String(source), String(target),
        trigger === nothing ? "" : String(trigger), guard === nothing ? "" : String(guard),
        effect === nothing ? "" : String(effect), false))
end

add_entry_transition(editor::Editor, owner, target) =
    target isa AbstractString ?
    _editor_add!(editor, ("add_transition", _edit_owner_id(owner), "", "", String(target), "", "", "", true)) :
    throw(ArgumentError("target must be notation text, not $(typeof(target))"))

function _sequence_text(label, text; optional=false)
    text === nothing && optional && return ""
    text isa AbstractString ||
        throw(ArgumentError("$(label) must be notation text, not $(typeof(text))"))
    String(text)
end

function _sequence_options(after, multiplicity=nothing)
    _sequence_text("multiplicity", multiplicity; optional=true)
    _sequence_text("after", after; optional=true)
    (after === nothing ? "" : String(after), multiplicity)
end

function _sequence_keyword_options(keyword, options)
    last(options) === nothing || keyword == "then" ||
        throw(ArgumentError("multiplicity requires then=true"))
    nothing
end

function _sequence_statement(owner, keyword, member_kind=""; ref="", member_name="",
                             type_name="", options=("", nothing), fields...)
    after, multiplicity = options
    values = Dict{String,Any}()
    for (key, value) in fields
        value === nothing || (values[String(key)] = value)
    end
    multiplicity === nothing || (values["multiplicity"] = String(multiplicity))
    base = ("add_sequence", String(owner), String(keyword), String(ref),
        String(member_kind), String(member_name), String(type_name), String(after))
    isempty(values) ? base : (base..., values)
end

function _body_statement(body::Body, kind; then=nothing, type_name="", fields...)
    then_keyword = if then === nothing
        isempty(body.operations) ? "" : "then"
    else
        then isa Bool || throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
        then ? "then" : ""
    end
    options = _sequence_options(get(fields, :after, nothing),
                                get(fields, :multiplicity, nothing))
    rest = Pair{Symbol,Any}[p for p in fields if !(first(p) in (:after, :multiplicity))]
    _sequence_keyword_options(then_keyword, options)
    operation = _sequence_statement("", then_keyword, String(kind);
        type_name=String(type_name), options=options, rest...)
    push!(body.operations, operation)
    body
end

function add_first(body::Body, ref)
    _sequence_text("ref", ref)
    push!(body.operations, _sequence_statement("", "first"; ref=String(ref)))
    body
end

function add_then(body::Body, positional_ref=nothing, positional_action=nothing,
        positional_type=nothing, positional_kind="action", positional_multiplicity=nothing;
        ref=positional_ref, action=positional_action, type=positional_type,
        kind=positional_kind, multiplicity=positional_multiplicity)
    for (label, value) in (("ref", ref), ("action", action), ("type", type),
                           ("kind", kind), ("multiplicity", multiplicity))
        _sequence_text(label, value; optional=true)
    end
    (ref === nothing) != (action === nothing) ||
        throw(ArgumentError("exactly one of ref and action is required"))
    options = _sequence_options(nothing, multiplicity)
    if ref !== nothing
        (type === nothing && (kind === nothing || kind == "action")) ||
            throw(ArgumentError("a then reference takes no type or kind"))
        push!(body.operations, _sequence_statement("", "then"; ref=String(ref), options=options))
    else
        push!(body.operations, _sequence_statement("", "then",
            kind === nothing || isempty(kind) ? "action" : String(kind); member_name=String(action),
            type_name=type === nothing ? "" : String(type), options=options))
    end
    body
end

function add_action(body::Body, positional_name=nothing, positional_type=nothing,
        positional_kind="action"; name=positional_name, type=positional_type,
        kind=positional_kind)
    for (label, value) in (("name", name), ("type", type), ("kind", kind))
        _sequence_text(label, value; optional=true)
    end
    push!(body.operations, _sequence_statement("", "",
        kind === nothing || isempty(kind) ? "action" : String(kind);
        member_name=name === nothing ? "" : String(name),
        type_name=type === nothing ? "" : String(type)))
    body
end

function add_accept(body::Body, payload, positional_type=nothing, positional_via=nothing;
        type=positional_type, via=positional_via, then=nothing, multiplicity=nothing)
    for (label, value) in (("payload", payload), ("type", type), ("via", via))
        _sequence_text(label, value; optional=label != "payload")
    end
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "accept";
        type_name=type === nothing ? "" : String(type), options=options,
        parameter=String(payload), via=via === nothing ? "" : String(via)))
    body
end

function add_send(body::Body, payload, positional_to=nothing, positional_via=nothing;
        to=positional_to, via=positional_via, then=nothing, multiplicity=nothing)
    for (label, value) in (("payload", payload), ("to", to), ("via", via))
        _sequence_text(label, value; optional=label != "payload")
    end
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "send"; options=options,
        value=String(payload), target=to === nothing ? "" : String(to),
        via=via === nothing ? "" : String(via)))
    body
end

function add_assign(body::Body, target, value; then=nothing, multiplicity=nothing)
    _sequence_text("target", target)
    _sequence_text("value", value)
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "assign";
        options=options, target=String(target), value=String(value)))
    body
end

function _validate_body(label, value; optional=false)
    value === nothing && optional && return nothing
    value isa Body || throw(ArgumentError("$(label) must be Body, not $(typeof(value))"))
    value
end

function add_if(body::Body, condition, branch::Body, positional_else_body=nothing;
        else_body=positional_else_body, then=nothing, multiplicity=nothing)
    _sequence_text("condition", condition)
    _validate_body("body", branch)
    _validate_body("else_body", else_body; optional=true)
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "if"; options=options,
        condition=String(condition), body=branch.operations,
        else_body=else_body === nothing || isempty(else_body.operations) ?
                  Any[] : else_body.operations))
    body
end

function add_while(body::Body, condition, branch::Body, positional_until=nothing;
        until=positional_until, then=nothing, multiplicity=nothing)
    _sequence_text("condition", condition)
    _sequence_text("until", until; optional=true)
    _validate_body("body", branch)
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "while"; options=options,
        condition=String(condition), until=until === nothing ? "" : String(until),
        body=branch.operations))
    body
end

function add_loop(body::Body, branch::Body, positional_until=nothing;
        until=positional_until, then=nothing, multiplicity=nothing)
    _sequence_text("until", until; optional=true)
    _validate_body("body", branch)
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "loop"; options=options,
        until=until === nothing ? "" : String(until), body=branch.operations))
    body
end

function add_for(body::Body, variable, collection, branch::Body, positional_type=nothing;
        type=positional_type, then=nothing, multiplicity=nothing)
    for (label, value) in (("variable", variable), ("collection", collection), ("type", type))
        _sequence_text(label, value; optional=label == "type")
    end
    _validate_body("body", branch)
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "for";
        type_name=type === nothing ? "" : String(type), options=options,
        parameter=String(variable), value=String(collection), body=branch.operations))
    body
end

function add_terminate(body::Body, positional_occurrence=nothing;
        occurrence=positional_occurrence, then=nothing, multiplicity=nothing)
    _sequence_text("occurrence", occurrence; optional=true)
    keyword = then === nothing ? (isempty(body.operations) ? "" : "then") :
              then isa Bool ? (then ? "then" : "") :
              throw(ArgumentError("then must be bool or nothing, not $(typeof(then))"))
    options = _sequence_options(nothing, multiplicity)
    _sequence_keyword_options(keyword, options)
    push!(body.operations, _sequence_statement("", keyword, "terminate";
        options=options, value=occurrence === nothing ? "" : String(occurrence)))
    body
end

function add_guarded_then(body::Body, guard, ref)
    _sequence_text("guard", guard)
    _sequence_text("ref", ref)
    push!(body.operations, _sequence_statement("", "if"; ref=String(ref), condition=String(guard)))
    body
end

function add_else(body::Body, ref)
    _sequence_text("ref", ref)
    push!(body.operations, _sequence_statement("", "else"; ref=String(ref)))
    body
end

function _editor_sequence(editor, owner, keyword; ref="", member_kind="",
                          member_name="", type_name="", after=nothing,
                          multiplicity=nothing, fields...)
    options = _sequence_options(after, multiplicity)
    _sequence_keyword_options(String(keyword), options)
    _editor_add!(editor, _sequence_statement(_edit_owner_id(owner), String(keyword),
        String(member_kind); ref=String(ref), member_name=String(member_name),
        type_name=String(type_name), options=options, fields...))
end

function add_first(editor::Editor, owner, ref; after=nothing)
    _sequence_text("ref", ref)
    _editor_sequence(editor, owner, "first"; ref=String(ref), after=after)
end

function add_then(editor::Editor, owner; ref=nothing, action=nothing, type=nothing,
                  after=nothing, kind="action", multiplicity=nothing)
    for (label, text) in (("ref", ref), ("action", action), ("type", type),
                          ("after", after), ("kind", kind), ("multiplicity", multiplicity))
        _sequence_text(label, text; optional=true)
    end
    (ref === nothing) != (action === nothing) ||
        throw(ArgumentError("exactly one of ref and action is required"))
    if ref !== nothing
        (type === nothing && (kind === nothing || kind == "action")) ||
            throw(ArgumentError("a then reference takes no type or kind"))
        return _editor_sequence(editor, owner, "then"; ref=String(ref), after=after,
                                multiplicity=multiplicity)
    end
    _editor_sequence(editor, owner, "then";
        member_kind=kind === nothing ? "action" : String(kind),
        member_name=String(action), type_name=type === nothing ? "" : String(type),
        after=after, multiplicity=multiplicity)
end

function add_accept(editor::Editor, owner, payload, positional_type=nothing,
        positional_via=nothing; type=positional_type, via=positional_via, then=true,
        multiplicity=nothing, after=nothing)
    _sequence_text("payload", payload)
    _sequence_text("type", type; optional=true)
    _sequence_text("via", via; optional=true)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="accept",
        type_name=type === nothing ? "" : String(type), after=after, multiplicity=multiplicity,
        parameter=String(payload), via=via === nothing ? "" : String(via))
end

function add_send(editor::Editor, owner, payload, positional_to=nothing,
        positional_via=nothing; to=positional_to, via=positional_via, then=true,
        multiplicity=nothing, after=nothing)
    _sequence_text("payload", payload)
    _sequence_text("to", to; optional=true)
    _sequence_text("via", via; optional=true)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="send",
        after=after, multiplicity=multiplicity, value=String(payload),
        target=to === nothing ? "" : String(to), via=via === nothing ? "" : String(via))
end

function add_assign(editor::Editor, owner, target, value; then=true, multiplicity=nothing,
                    after=nothing)
    _sequence_text("target", target)
    _sequence_text("value", value)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="assign",
        after=after, multiplicity=multiplicity, target=String(target), value=String(value))
end

function add_if(editor::Editor, owner, condition, branch::Body, positional_else_body=nothing;
        else_body=positional_else_body, then=true, multiplicity=nothing, after=nothing)
    _sequence_text("condition", condition)
    _validate_body("body", branch)
    _validate_body("else_body", else_body; optional=true)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="if",
        after=after, multiplicity=multiplicity, condition=String(condition),
        body=branch.operations, else_body=else_body === nothing ||
            isempty(else_body.operations) ? Any[] : else_body.operations)
end

function add_while(editor::Editor, owner, condition, branch::Body, positional_until=nothing;
        until=positional_until, then=true, multiplicity=nothing, after=nothing)
    _sequence_text("condition", condition)
    _sequence_text("until", until; optional=true)
    _validate_body("body", branch)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="while",
        after=after, multiplicity=multiplicity, condition=String(condition),
        until=until === nothing ? "" : String(until), body=branch.operations)
end

function add_loop(editor::Editor, owner, branch::Body, positional_until=nothing;
        until=positional_until, then=true, multiplicity=nothing, after=nothing)
    _sequence_text("until", until; optional=true)
    _validate_body("body", branch)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="loop",
        after=after, multiplicity=multiplicity,
        until=until === nothing ? "" : String(until), body=branch.operations)
end

function add_for(editor::Editor, owner, variable, collection, branch::Body, positional_type=nothing;
        type=positional_type, then=true, multiplicity=nothing, after=nothing)
    _sequence_text("variable", variable)
    _sequence_text("collection", collection)
    _sequence_text("type", type; optional=true)
    _validate_body("body", branch)
    then isa Bool || throw(ArgumentError("then must be bool, not $(typeof(then))"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="for",
        type_name=type === nothing ? "" : String(type), after=after,
        multiplicity=multiplicity, parameter=String(variable), value=String(collection),
        body=branch.operations)
end

function add_terminate(editor::Editor, owner, positional_occurrence=nothing;
        occurrence=positional_occurrence, then=true, multiplicity=nothing, after=nothing)
    _sequence_text("occurrence", occurrence; optional=true)
    then isa Bool || throw(ArgumentError("then must be bool"))
    _editor_sequence(editor, owner, then ? "then" : ""; member_kind="terminate",
        after=after, multiplicity=multiplicity,
        value=occurrence === nothing ? "" : String(occurrence))
end

function add_guarded_then(editor::Editor, owner, guard, ref; after=nothing)
    _sequence_text("guard", guard)
    _sequence_text("ref", ref)
    _editor_sequence(editor, owner, "if"; ref=String(ref), after=after,
                     condition=String(guard))
end

function add_else(editor::Editor, owner, ref; after=nothing)
    _sequence_text("ref", ref)
    _editor_sequence(editor, owner, "else"; ref=String(ref), after=after)
end

function add_import(editor::Editor, owner, target; visibility=nothing, recursive=false,
                    all=false, filter=nothing)
    target isa AbstractString || throw(ArgumentError("target must be notation text"))
    visibility === nothing || visibility isa AbstractString ||
        throw(ArgumentError("visibility must be notation text or nothing"))
    recursive isa Bool || throw(ArgumentError("recursive must be a bool"))
    all isa Bool || throw(ArgumentError("all must be a bool"))
    filters = if filter === nothing
        String[]
    elseif filter isa AbstractString
        String[String(filter)]
    elseif filter isa AbstractVector || filter isa Tuple
        _edit_all_text(filter) || throw(ArgumentError("filter must contain notation text"))
        String[String(value) for value in filter]
    else
        throw(ArgumentError("filter must be notation text or a list of it"))
    end
    _editor_add!(editor, ("add_import", _edit_owner_id(owner),
        visibility === nothing ? "" : String(visibility), String(target),
        recursive, all, filters))
end

add_require_constraint(editor::Editor, owner, expression, positional_name=nothing;
                       name=positional_name) =
    add_requirement_constraint(editor, owner, "require", expression; name=name)
add_assume_constraint(editor::Editor, owner, expression, positional_name=nothing;
                      name=positional_name) =
    add_requirement_constraint(editor, owner, "assume", expression; name=name)

function add_connection(editor::Editor, owner, kind, from_, to; name=nothing, type=nothing)
    for (label, text) in (("kind", kind), ("from_", from_), ("to", to))
        text isa AbstractString || throw(ArgumentError("$(label) must be notation text"))
    end
    for (label, text) in (("name", name), ("type", type))
        text === nothing || text isa AbstractString ||
            throw(ArgumentError("$(label) must be notation text or nothing"))
    end
    _editor_add!(editor, ("add_connection", _edit_owner_id(owner), String(kind),
        String(from_), String(to), name === nothing ? "" : String(name),
        type === nothing ? "" : String(type)))
end

add_allocation(editor::Editor, owner, from_, to; kwargs...) =
    add_connection(editor, owner, "allocation", from_, to; kwargs...)
add_flow(editor::Editor, owner, from_, to; kwargs...) =
    add_connection(editor, owner, "flow", from_, to; kwargs...)
add_succession(editor::Editor, owner, from_, to; kwargs...) =
    add_connection(editor, owner, "succession", from_, to; kwargs...)

function delete(editor::Editor, target; cascade=false)
    cascade isa Bool || throw(ArgumentError("cascade must be bool"))
    _editor_add!(editor, ("delete", _edit_target_id(target), cascade))
end

move(editor::Editor, target, owner) =
    _editor_add!(editor, ("move", _edit_target_id(target), _edit_owner_id(owner)))

function _parameter_pairs(values, label)
    values === nothing && return Tuple{String,String}[]
    (values isa AbstractVector || values isa Tuple) ||
        throw(ArgumentError("$(label) must be a sequence of pairs of strings"))
    result = Tuple{String,String}[]
    for (index, pair) in enumerate(values)
        if pair isa Pair
            left, right = first(pair), last(pair)
        elseif (pair isa Tuple || pair isa AbstractVector) && length(pair) == 2
            left, right = pair[1], pair[2]
        else
            throw(ArgumentError("$(label)[$(index - 1)] must be a 2-tuple of strings"))
        end
        (left isa AbstractString && right isa AbstractString) ||
            throw(ArgumentError("$(label)[$(index - 1)] name and type must be strings"))
        push!(result, (String(left), String(right)))
    end
    result
end

function add_parameter(editor::Editor, owner, direction, name; type=nothing, kind=nothing,
                       kwargs...)
    add_member(editor, owner, kind === nothing ? "" : String(kind), name;
        type=type, direction=direction, kwargs...)
end
add_return(editor::Editor, owner, positional_name="";
           name=positional_name, kwargs...) =
    add_member(editor, owner, "return", name; kwargs...)

function add_calc_def(editor::Editor, owner, name; inputs=nothing, return_type=nothing,
                      return_expression=nothing, expression=nothing, kwargs...)
    input_pairs = _parameter_pairs(inputs, "inputs")
    for (label, value) in (("return_type", return_type),
                           ("return_expression", return_expression), ("expression", expression))
        value === nothing || value isa AbstractString ||
            throw(ArgumentError("$(label) must be notation text or nothing"))
    end
    expression !== nothing && return_expression !== nothing &&
        throw(ArgumentError("expression and return_expression both bind the result; give one"))
    return_expression !== nothing && (return_type === nothing || isempty(return_type)) &&
        throw(ArgumentError("return_expression requires return_type"))
    owner_id = _edit_owner_id(owner)
    add_member(editor, owner_id, "calc def", name; expression=expression, kwargs...)
    qualified_name = isempty(owner_id) ? String(name) : owner_id * "::" * String(name)
    for (parameter_name, parameter_type) in input_pairs
        add_parameter(editor, qualified_name, "in", parameter_name; type=parameter_type)
    end
    (return_type !== nothing || return_expression !== nothing) &&
        add_return(editor, qualified_name; type=return_type, value=return_expression)
    editor
end

function add_calc(editor::Editor, owner, name; inputs=nothing, return_type=nothing,
                  return_expression=nothing, expression=nothing, kwargs...)
    input_pairs = _parameter_pairs(inputs, "inputs")
    for (label, value) in (("return_type", return_type),
                           ("return_expression", return_expression), ("expression", expression))
        value === nothing || value isa AbstractString ||
            throw(ArgumentError("$(label) must be notation text or nothing"))
    end
    expression !== nothing && return_expression !== nothing &&
        throw(ArgumentError("expression and return_expression both bind the result; give one"))
    return_expression !== nothing && (return_type === nothing || isempty(return_type)) &&
        throw(ArgumentError("return_expression requires return_type"))
    owner_id = _edit_owner_id(owner)
    add_member(editor, owner_id, "calc", name; expression=expression, kwargs...)
    qualified_name = isempty(owner_id) ? String(name) : owner_id * "::" * String(name)
    for (parameter_name, parameter_type) in input_pairs
        add_parameter(editor, qualified_name, "in", parameter_name; type=parameter_type)
    end
    (return_type !== nothing || return_expression !== nothing) &&
        add_return(editor, qualified_name; type=return_type, value=return_expression)
    editor
end

function _add_action_with_parameters(editor, owner, kind, name, inputs, outputs; kwargs...)
    input_pairs = _parameter_pairs(inputs, "inputs")
    output_pairs = _parameter_pairs(outputs, "outputs")
    owner_id = _edit_owner_id(owner)
    add_member(editor, owner_id, kind, name; kwargs...)
    qualified_name = isempty(owner_id) ? String(name) : owner_id * "::" * String(name)
    for (parameter_name, parameter_type) in input_pairs
        add_parameter(editor, qualified_name, "in", parameter_name; type=parameter_type)
    end
    for (parameter_name, parameter_type) in output_pairs
        add_parameter(editor, qualified_name, "out", parameter_name; type=parameter_type)
    end
    editor
end
function add_action_def(editor::Editor, owner, name, positional_inputs=nothing,
                        positional_outputs=nothing; inputs=positional_inputs,
                        outputs=positional_outputs, kwargs...)
    _add_action_with_parameters(editor, owner, "action def", name, inputs, outputs; kwargs...)
end
function add_action(editor::Editor, owner, name, positional_inputs=nothing,
                    positional_outputs=nothing; inputs=positional_inputs,
                    outputs=positional_outputs, kwargs...)
    _add_action_with_parameters(editor, owner, "action", name, inputs, outputs; kwargs...)
end

add_perform_action(editor::Editor, owner, name, positional_type=nothing;
                   type=positional_type, kwargs...) =
    add_member(editor, owner, "perform action", name; type=type, kwargs...)
add_perform(editor::Editor, owner, action, positional_doc=nothing;
            doc=positional_doc) =
    add_member(editor, owner, "perform", action; doc=doc)
add_exhibit_state(editor::Editor, owner, name, positional_type=nothing;
                  type=positional_type) =
    add_member(editor, owner, "exhibit state", name; type=type)
add_exhibit(editor::Editor, owner, state) = add_member(editor, owner, "exhibit", state)

function add_state_action(editor::Editor, owner, kind, name, positional_type=nothing;
                          type=positional_type)
    kind isa AbstractString || throw(ArgumentError("kind must be notation text"))
    kind in ("entry", "do", "exit") ||
        throw(ArgumentError("kind must be 'entry', 'do' or 'exit'"))
    add_member(editor, owner, "$(kind) action", name; type=type)
end

function add_assert_constraint(editor::Editor, owner, positional_name=nothing,
        positional_type=nothing, positional_expression=nothing, positional_negated=false;
        name=positional_name, type=positional_type, expression=positional_expression,
        negated=positional_negated)
    negated isa Bool || throw(ArgumentError("negated must be bool"))
    add_member(editor, owner, negated ? "assert not constraint" : "assert constraint",
        name === nothing ? "" : name; type=type, expression=expression)
end

function add_assert(editor::Editor, owner, ref, positional_negated=false;
                    negated=positional_negated)
    ref isa AbstractString || throw(ArgumentError("ref must be notation text, not $(typeof(ref))"))
    negated isa Bool || throw(ArgumentError("negated must be bool"))
    add_member(editor, owner, negated ? "assert not" : "assert", String(ref))
end

for (function_name, member_kind) in (
    (:add_package, "package"), (:add_part_def, "part def"), (:add_part, "part"),
    (:add_attribute_def, "attribute def"), (:add_attribute, "attribute"),
    (:add_item_def, "item def"), (:add_item, "item"), (:add_port_def, "port def"),
    (:add_port, "port"), (:add_class, "class"), (:add_struct, "struct"),
    (:add_datatype, "datatype"), (:add_classifier, "classifier"),
    (:add_feature, "feature"), (:add_assoc, "assoc"), (:add_behavior, "behavior"),
    (:add_function, "function"), (:add_predicate, "predicate"),
    (:add_interaction, "interaction"), (:add_metaclass, "metaclass"),
    (:add_state_def, "state def"), (:add_state, "state"),
    (:add_requirement_def, "requirement def"), (:add_requirement, "requirement"),
)
    @eval function $function_name(editor::Editor, owner, member_name; kwargs...)
        add_member(editor, owner, $member_kind, member_name; kwargs...)
    end
end

function add_constraint_def(editor::Editor, owner, name, positional_expression=nothing;
        expression=positional_expression, kwargs...)
    add_member(editor, owner, "constraint def", name; expression=expression, kwargs...)
end
function add_constraint(editor::Editor, owner, name, positional_expression=nothing;
        expression=positional_expression, kwargs...)
    add_member(editor, owner, "constraint", name; expression=expression, kwargs...)
end

function apply_edits(editor::Editor)
    editor.applied && throw(ArgumentError(
        "this editor has already been applied: build another editor from the edited model to edit further"))
    isempty(editor.operations) && throw(NoEditsError(
        "this editor has no operations: add an edit before applying it";
        failure="EDIT_FAILURE_NO_OPERATIONS"))
    editor.connection === nothing &&
        throw(ArgumentError("this editor has no connection and cannot be applied"))
    require_capability(editor.connection, CAPABILITY_APPLY_EDITS)
    multi = editor.model !== nothing && length(editor.model.documents) > 1
    result = _apply_edits(editor.connection, editor.model_hash, editor.operations;
                          accept_documents=true, multi_document=multi)
    editor.applied = true
    result
end
apply(editor::Editor) = apply_edits(editor)

"""Create an editor, or apply a callback's collected edits and return its result."""
edit(model::Model) = Editor(model)
edit(model::Model, f::Function) = edit(f, model)
function edit(f::Function, model::Model)
    editor = Editor(model)
    f(editor)
    apply_edits(editor)
end

function apply_edits(conn::Connection, model_hash::AbstractString, operations;
                     document="", accept_documents=true, multi_document=false)
    _apply_edits(conn, model_hash, operations; document=document,
                 accept_documents=Bool(accept_documents),
                 multi_document=Bool(multi_document))
end

function apply_edits(model::Model, operations; document="")
    require_capability(model.connection, CAPABILITY_APPLY_EDITS)
    multi = length(model.documents) > 1
    _apply_edits(model.connection, model.hash, operations; document=document,
                 accept_documents=true, multi_document=multi)
end
