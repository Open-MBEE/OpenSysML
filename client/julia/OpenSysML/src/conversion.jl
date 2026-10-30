"""
A standard-library element an api-json conversion references outside its element array, in
place of the library stub releases before 0.11.0 wrote: its `id`, `qualified_name` (such as
`SI::second`) and `type`, the metaclass as API JSON's `@type` spells it.
"""
struct LibraryReference
    id::String
    qualified_name::String
    type::String
end
Base.:(==)(a::LibraryReference, b::LibraryReference) =
    a.id == b.id && a.qualified_name == b.qualified_name && a.type == b.type
Base.hash(ref::LibraryReference, h::UInt) = hash((ref.id, ref.qualified_name, ref.type), h)

"""
A model written out in another format. `library_references` lists the standard-library
elements an api-json conversion names outside its element array; empty for any other format.
"""
struct Conversion
    content::String
    from_format::String
    to_format::String
    diagnostics::Vector{Diagnostic}
    experimental::Bool
    experimental_notice::String
    library_references::Vector{LibraryReference}
end
Conversion(content, from_format, to_format, diagnostics, experimental, experimental_notice) =
    Conversion(content, from_format, to_format, diagnostics, experimental, experimental_notice,
               LibraryReference[])

"""`id_form` of a conversion: derived ids spelled from each element's qualified name, the default."""
const ID_FORM_QUALIFIED = "qualified"
"""`id_form` of a conversion: name-based uuids under each root package, the library convention."""
const ID_FORM_UUID = "uuid"
Base.show(io::IO, result::Conversion) =
    print(io, "Conversion($(result.from_format) → $(result.to_format), $(ncodeunits(result.content)) bytes)")

const EXPERIMENTAL_NOTICE =
    "RDF conversion — Turtle and the API's JSON element form alike — is experimental: " *
    "the mapping covers model structure and the behavior its bodies state, refuses what " *
    "it cannot write back, and its vocabulary may change without a compatibility path; " *
    "see docs/reference/rdf-mapping.md § Status"

is_experimental(from_format, to_format) =
    lowercase(String(from_format)) in ("ttl", "turtle", "rdf", "api-json", "json", "xmi", "uml", "mdzip") ||
    lowercase(String(to_format)) in ("ttl", "turtle", "rdf", "api-json", "json")

"""Infer the output format from a supported file extension."""
function format_of_path(path::AbstractString)
    extension = lowercase(splitext(String(path))[2])
    formats = Dict(".sysml" => "sysml", ".kerml" => "sysml", ".ttl" => "ttl",
                   ".turtle" => "ttl", ".json" => "api-json")
    haskey(formats, extension) ||
        throw(ArgumentError("cannot tell the format to write $(repr(String(path))) as: " *
                            "expected one of .json, .kerml, .sysml, .ttl, .turtle, " *
                            "or pass to_format explicitly"))
    formats[extension]
end

"""
Convert exactly one file, inline source, or cached model to another format. `id_form`
(`ID_FORM_QUALIFIED`, the default, or `ID_FORM_UUID`) spells the derived ids of a graph
notation; `include_library_stubs` restores, in api-json, the ownerless library stubs releases
before 0.11.0 wrote, and needs the service's `convert_library_stubs` capability. For notation
converted to api-json, `compact` writes the compact document (`api-json-compact/1`) in place
of the element array, `omit_derived` leaves out every derived property of the metamodel and
`keep_derived` names the ones still written (all three need `convert_compact`);
`include_check_diagnostics` reports the semantic check's diagnostics, each with its severity,
in the result's `diagnostics` beside the output (notation sources only; needs
`convert_check_diagnostics`).
"""
function _convert(conn::Connection, to_format::AbstractString; file_path=nothing,
                  content=nothing, model_hash=nothing, from_format="",
                  tolerate_syntax_errors=false, id_form="", include_library_stubs::Bool=false,
                  compact::Bool=false, omit_derived::Bool=false, keep_derived=String[],
                  include_check_diagnostics::Bool=false)
    sources = Pair{String,Any}["file_path" => file_path, "content" => content,
                               "model_hash" => model_hash]
    given = Pair{String,Any}[p for p in sources if last(p) !== nothing]
    length(given) == 1 || throw(ArgumentError("provide exactly one of file_path, content or model_hash; got " *
        (isempty(given) ? "none" : join(first.(given), ", "))))
    source_name, source_value = only(given)
    source_name == "content" && isempty(String(from_format)) &&
        throw(ArgumentError("from_format is required for inline content"))
    if is_v1(from_format) || (source_name == "file_path" && isempty(String(from_format)) &&
                              path_is_v1(source_value))
        name = source_name == "file_path" ? String(source_value) : "the source"
        throw(ArgumentError("$name $MIGRATED_NOT_CONVERTED; call migrate_file or " *
                            "migrate_source with the same source"))
    end
    require_capability(conn, CAPABILITY_CONVERT)
    include_library_stubs && require_capability(conn, CAPABILITY_CONVERT_LIBRARY_STUBS)
    kept = String[String(name) for name in keep_derived]
    (compact || omit_derived || !isempty(kept)) &&
        require_capability(conn, CAPABILITY_CONVERT_COMPACT)
    include_check_diagnostics && require_capability(conn, CAPABILITY_CONVERT_CHECK_DIAGNOSTICS)
    request = Dict{String,Any}("toFormat" => String(to_format),
        "fromFormat" => String(from_format), "tolerateSyntaxErrors" => Bool(tolerate_syntax_errors))
    isempty(String(id_form)) || (request["idForm"] = String(id_form))
    include_library_stubs && (request["includeLibraryStubs"] = true)
    compact && (request["compact"] = true)
    omit_derived && (request["omitDerived"] = true)
    isempty(kept) || (request["keepDerived"] = kept)
    include_check_diagnostics && (request["includeCheckDiagnostics"] = true)
    request[source_name == "file_path" ? "filePath" :
            source_name == "model_hash" ? "modelHash" : "content"] = String(source_value)
    source_not_found = source_name == "file_path" ? ModelFileNotFoundError : ModelNotFoundError
    answer = _translate(; not_found=source_not_found, capabilities=(CAPABILITY_CONVERT,),
                        connection=conn) do
        call(conn, "Convert", request)
    end
    diags = _diagnostics(answer)
    message = String(get(answer, "error", ""))
    from_format_answer = String(get(answer, "fromFormat", ""))
    to_format_answer = String(get(answer, "toFormat", ""))
    experimental = Bool(get(answer, "experimental", false)) ||
                   is_experimental(from_format_answer, to_format_answer)
    notice = String(get(answer, "experimentalNotice", ""))
    experimental && isempty(notice) && (notice = EXPERIMENTAL_NOTICE)
    references = LibraryReference[LibraryReference(_render_string(ref, "id"),
        _render_string(ref, "qualifiedName"), _render_string(ref, "type"))
        for ref in get(answer, "libraryReferences", Any[])]
    result = Conversion(String(get(answer, "content", "")), from_format_answer,
        to_format_answer, diags, experimental, notice, references)
    if result.experimental
        @warn result.experimental_notice
    end
    isempty(message) || throw(ConversionError(message, diags))
    result
end

"""Convert a file-backed source to `to_format`."""
convert_file(conn::Connection, path::AbstractString, to_format::AbstractString;
             from_format="", tolerate_syntax_errors=false, id_form="",
             include_library_stubs::Bool=false, compact::Bool=false, omit_derived::Bool=false,
             keep_derived=String[], include_check_diagnostics::Bool=false) =
    _convert(conn, to_format; file_path=path, from_format=from_format,
             tolerate_syntax_errors=tolerate_syntax_errors, id_form=id_form,
             include_library_stubs=include_library_stubs, compact=compact,
             omit_derived=omit_derived, keep_derived=keep_derived,
             include_check_diagnostics=include_check_diagnostics)

"""Convert inline source to `to_format`, naming its input format explicitly."""
convert_source(conn::Connection, content::AbstractString, to_format::AbstractString;
               from_format, tolerate_syntax_errors=false, id_form="",
               include_library_stubs::Bool=false, compact::Bool=false, omit_derived::Bool=false,
               keep_derived=String[], include_check_diagnostics::Bool=false) =
    _convert(conn, to_format; content=content, from_format=from_format,
             tolerate_syntax_errors=tolerate_syntax_errors, id_form=id_form,
             include_library_stubs=include_library_stubs, compact=compact,
             omit_derived=omit_derived, keep_derived=keep_derived,
             include_check_diagnostics=include_check_diagnostics)

"""Convert a cached model by hash to `to_format`."""
convert_model(conn::Connection, model_hash::AbstractString, to_format::AbstractString;
              tolerate_syntax_errors=false, id_form="", include_library_stubs::Bool=false,
              compact::Bool=false, omit_derived::Bool=false, keep_derived=String[],
              include_check_diagnostics::Bool=false) =
    _convert(conn, to_format; model_hash=model_hash,
             tolerate_syntax_errors=tolerate_syntax_errors, id_form=id_form,
             include_library_stubs=include_library_stubs, compact=compact,
             omit_derived=omit_derived, keep_derived=keep_derived,
             include_check_diagnostics=include_check_diagnostics)

"""Convert the source retained for `model` to `to_format`."""
convert_model(model::Model, to_format::AbstractString;
              tolerate_syntax_errors=false, id_form="", include_library_stubs::Bool=false,
              compact::Bool=false, omit_derived::Bool=false, keep_derived=String[],
              include_check_diagnostics::Bool=false) =
    convert_model(model.connection, model.hash, to_format;
                  tolerate_syntax_errors=tolerate_syntax_errors, id_form=id_form,
                  include_library_stubs=include_library_stubs, compact=compact,
                  omit_derived=omit_derived, keep_derived=keep_derived,
                  include_check_diagnostics=include_check_diagnostics)

"""Write `model` back out as SysML notation."""
function to_sysml(model::Model; tolerate_syntax_errors=false)
    convert_model(model, "sysml"; tolerate_syntax_errors=tolerate_syntax_errors)
end
"""Write `model` in experimental Turtle form."""
to_turtle(model::Model) = convert_model(model, "turtle")
"""Write `model` in the API's experimental JSON element form."""
to_api_json(model::Model) = convert_model(model, "api-json")

"""Convert and write `model` to a path, inferring its format when omitted."""
function save(model::Model, path::AbstractString; to_format=nothing,
              tolerate_syntax_errors=false, id_form="", include_library_stubs::Bool=false)
    format = to_format === nothing ? format_of_path(path) : String(to_format)
    result = convert_model(model, format; tolerate_syntax_errors=tolerate_syntax_errors,
                           id_form=id_form, include_library_stubs=include_library_stubs)
    open(path, "w") do io
        write(io, result.content)
    end
    result
end
