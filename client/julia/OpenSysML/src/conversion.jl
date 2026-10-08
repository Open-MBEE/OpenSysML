struct Conversion
    content::String
    from_format::String
    to_format::String
    diagnostics::Vector{Diagnostic}
    experimental::Bool
    experimental_notice::String
end
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

"""Convert exactly one file, inline source, or cached model to another format."""
function _convert(conn::Connection, to_format::AbstractString; file_path=nothing,
                  content=nothing, model_hash=nothing, from_format="",
                  tolerate_syntax_errors=false)
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
    request = Dict{String,Any}("toFormat" => String(to_format),
        "fromFormat" => String(from_format), "tolerateSyntaxErrors" => Bool(tolerate_syntax_errors))
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
    result = Conversion(String(get(answer, "content", "")), from_format_answer,
        to_format_answer, diags, experimental, notice)
    if result.experimental
        @warn result.experimental_notice
    end
    isempty(message) || throw(ConversionError(message, diags))
    result
end

"""Convert a file-backed source to `to_format`."""
convert_file(conn::Connection, path::AbstractString, to_format::AbstractString;
             from_format="", tolerate_syntax_errors=false) =
    _convert(conn, to_format; file_path=path, from_format=from_format,
             tolerate_syntax_errors=tolerate_syntax_errors)

"""Convert inline source to `to_format`, naming its input format explicitly."""
convert_source(conn::Connection, content::AbstractString, to_format::AbstractString;
               from_format, tolerate_syntax_errors=false) =
    _convert(conn, to_format; content=content, from_format=from_format,
             tolerate_syntax_errors=tolerate_syntax_errors)

"""Convert a cached model by hash to `to_format`."""
convert_model(conn::Connection, model_hash::AbstractString, to_format::AbstractString;
              tolerate_syntax_errors=false) =
    _convert(conn, to_format; model_hash=model_hash,
             tolerate_syntax_errors=tolerate_syntax_errors)

"""Convert the source retained for `model` to `to_format`."""
convert_model(model::Model, to_format::AbstractString;
              tolerate_syntax_errors=false) =
    convert_model(model.connection, model.hash, to_format;
                  tolerate_syntax_errors=tolerate_syntax_errors)

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
              tolerate_syntax_errors=false)
    format = to_format === nothing ? format_of_path(path) : String(to_format)
    result = convert_model(model, format; tolerate_syntax_errors=tolerate_syntax_errors)
    open(path, "w") do io
        write(io, result.content)
    end
    result
end
