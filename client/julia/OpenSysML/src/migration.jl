"""The forms a SysML v1 model comes in: UML XMI, an Eclipse UML2 file or a Cameo/MagicDraw archive."""
const V1_FORMATS = ("xmi", "uml", "mdzip")

"""Why `convert_file` refuses a SysML v1 model, as the `sysml` command words it."""
const MIGRATED_NOT_CONVERTED =
    "is a SysML v1 model, which is migrated, not converted: every element is mapped, " *
    "approximated or left unmapped and reported element by element"

"""What the service says of a migration when it says nothing itself."""
const MIGRATION_NOTICE =
    "SysML v1 migration is experimental: the mapping covers structure, ports and connectors, " *
    "requirements, constraints, instances and allocations, reports every element it " *
    "approximates or leaves behind, and what it writes for a v1 element may change without a " *
    "compatibility path; see docs/reference/sysml-v1-migration.md § Status"

const VERDICT_MAPPED = "mapped"
const VERDICT_APPROXIMATED = "approximated"
const VERDICT_UNMAPPED = "unmapped"
const VERDICT_SKIPPED = "skipped"

"""Whether `from_format` names a SysML v1 model, in any case and padding."""
is_v1(from_format) = lowercase(strip(String(from_format))) in V1_FORMATS

"""Whether a path's extension names a SysML v1 model: `.xmi`, `.uml` or `.mdzip`."""
path_is_v1(path) = lstrip(lowercase(splitext(String(path))[2]), '.') in V1_FORMATS

"""One SysML v1 element's verdict in a migration."""
struct MigrationEntry
    id::String
    kind::String
    name::String
    target::String
    verdict::String
    note::String
end
MigrationEntry(record::AbstractDict) = MigrationEntry(
    String(get(record, "id", "")), String(get(record, "kind", "")),
    String(get(record, "name", "")), String(get(record, "target", "")),
    String(get(record, "verdict", "")), String(get(record, "note", "")))

"""
The account a migration gives of itself: what became of every element. The summary and the
four counts come with every migration; `entries` and `text` when `report=true` asks for them.
"""
struct MigrationReport
    source::String
    exporter::String
    summary::String
    mapped::Int
    approximated::Int
    unmapped::Int
    skipped::Int
    entries::Vector{MigrationEntry}
    text::String
end
MigrationReport() = MigrationReport("", "", "", 0, 0, 0, 0, MigrationEntry[], "")
MigrationReport(record::AbstractDict) = MigrationReport(
    String(get(record, "source", "")), String(get(record, "exporter", "")),
    String(get(record, "summary", "")),
    Int(get(record, "mapped", 0)), Int(get(record, "approximated", 0)),
    Int(get(record, "unmapped", 0)), Int(get(record, "skipped", 0)),
    MigrationEntry[MigrationEntry(e) for e in get(record, "entries", Any[])],
    String(get(record, "text", "")))
Base.show(io::IO, report::MigrationReport) = print(io,
    "MigrationReport($(report.mapped) mapped, $(report.approximated) approximated, " *
    "$(report.unmapped) unmapped, $(report.skipped) skipped)")

"""The entries with `verdict`: `mapped`, `approximated`, `unmapped` or `skipped`."""
by_verdict(report::MigrationReport, verdict::AbstractString) =
    MigrationEntry[e for e in report.entries if e.verdict == verdict]

"""
A migrated model: the notation or Turtle written, with its report and the image files its
diagrams embed, keyed by the relative path `content` refers to them with. `source_path` is the
v1 file it came from, absolute, or `nothing` for inline content.
"""
struct Migration
    content::String
    from_format::String
    to_format::String
    report::MigrationReport
    results::String
    files::Dict{String,Vector{UInt8}}
    source_path::Union{Nothing,String}
    experimental::Bool
    experimental_notice::String
end
Base.show(io::IO, result::Migration) = print(io,
    "Migration($(result.from_format) → $(result.to_format), $(ncodeunits(result.content)) bytes, " *
    "$(result.report.mapped) mapped, $(result.report.approximated) approximated, " *
    "$(result.report.unmapped) unmapped, $(result.report.skipped) skipped)")

function _migrate(conn::Connection, to_format::AbstractString; file_path=nothing, content=nothing,
                  from_format="", report=false, results=false, layout_path=nothing,
                  layout_content=nothing, image_base_url="", strict=false)
    (file_path === nothing) == (content === nothing) &&
        throw(ArgumentError("provide exactly one of file_path or content"))
    name = file_path === nothing ? "the source" : String(file_path)
    from = String(from_format)
    if !isempty(from) && !is_v1(from)
        throw(ArgumentError("$name is $from input, which is converted, not migrated: only a " *
                            "SysML v1 model (xmi, uml or mdzip) is migrated; call convert_file " *
                            "or convert_source with the same source"))
    end
    content !== nothing && isempty(from) &&
        throw(ArgumentError("from_format is required for inline content: xmi, uml or mdzip"))
    layout_path !== nothing && layout_content !== nothing &&
        throw(ArgumentError("provide at most one of layout_path or layout_content"))
    require_capability(conn, CAPABILITY_MIGRATE)
    request = Dict{String,Any}("toFormat" => String(to_format), "fromFormat" => from,
        "report" => Bool(report), "results" => Bool(results),
        "imageBaseUrl" => String(image_base_url), "strict" => Bool(strict))
    if file_path === nothing
        bytes = content isa AbstractString ? Vector{UInt8}(codeunits(content)) : Vector{UInt8}(content)
        request["content"] = base64encode(bytes)
    else
        request["filePath"] = String(file_path)
    end
    layout_path === nothing || (request["layoutPath"] = String(layout_path))
    layout_content === nothing || (request["layoutContent"] = String(layout_content))
    answer = _translate(; not_found=ModelFileNotFoundError, capabilities=(CAPABILITY_MIGRATE,),
                        connection=conn) do
        call(conn, "Migrate", request)
    end
    message = String(get(answer, "error", ""))
    isempty(message) || throw(MigrationError(message))
    notice = String(get(answer, "experimentalNotice", ""))
    isempty(notice) && (notice = MIGRATION_NOTICE)
    files = Dict{String,Vector{UInt8}}()
    for file in get(answer, "files", Any[])
        files[String(get(file, "path", ""))] = base64decode(String(get(file, "content", "")))
    end
    report_record = get(answer, "report", nothing)
    result = Migration(String(get(answer, "content", "")), String(get(answer, "fromFormat", from)),
        String(get(answer, "toFormat", to_format)),
        report_record === nothing ? MigrationReport() : MigrationReport(report_record),
        String(get(answer, "results", "")), files,
        file_path === nothing ? nothing : abspath(String(file_path)), true, notice)
    @warn result.experimental_notice
    result
end

"""
    migrate_file(conn, path, to_format; from_format="", report=false, results=false,
                 layout_path=nothing, layout_content=nothing, image_base_url="", strict=false)

Migrate a SysML v1 model — a Cameo/MagicDraw `.mdzip`, a UML XMI `.xmi` or an Eclipse UML2
`.uml` export the service can read — to `to_format`: `sysml`, `kerml`, `ttl`, `turtle` or `rdf`.

A migration is ledgered, not lossless: every v1 element lands in the result's `report` as
`mapped`, `approximated`, `unmapped` or `skipped`; `report=true` asks for every element's
verdict and the report text, `results=true` for the JSON index of the v1 tool's result
snapshots. Migration is experimental and says so with `@warn`.
"""
migrate_file(conn::Connection, path::AbstractString, to_format::AbstractString; kwargs...) =
    _migrate(conn, to_format; file_path=path, kwargs...)

"""
    migrate_source(conn, content, to_format; from_format, kwargs...)

Migrate the bytes of a SysML v1 file carried inline; `from_format` names which of `xmi`,
`uml` or `mdzip` they are. Takes the options of [`migrate_file`](@ref).
"""
migrate_source(conn::Connection, content, to_format::AbstractString; from_format, kwargs...) =
    _migrate(conn, to_format; content=content, from_format=from_format, kwargs...)

const _MAX_SYMLINK_HOPS = 40

"""`path` with `.` and `..` resolved lexically, absolute."""
_normalized(path) = normpath(abspath(String(path)))

"""Whether `file` is strictly below `base`, both normalized."""
function _within(base, file)
    rel = relpath(file, base)
    parts = splitpath(rel)
    !isempty(parts) && !isabspath(rel) && !(parts[1] in (".", ".."))
end

"""
Where a path lands once every symbolic link on it is followed, existing or dangling: the real
path of its deepest existing ancestor, with the missing tail below it.
"""
function _landing(path)
    head = _normalized(path)
    tail = String[]
    hops = 0
    while true
        if ispath(head)
            return joinpath(realpath(head), tail...)
        elseif islink(head)
            hops += 1
            hops >= _MAX_SYMLINK_HOPS &&
                throw(ArgumentError("$path: too many levels of symbolic links"))
            head = _normalized(joinpath(dirname(head), readlink(head)))
        else
            parent = dirname(head)
            parent == head && return joinpath(head, tail...)
            pushfirst!(tail, basename(head))
            head = parent
        end
    end
end

"""Whether two paths name one file: by identity where both exist, else by where they land."""
function _same_path(a, b)
    if ispath(a) && ispath(b)
        sa, sb = stat(a), stat(b)
        return sa.device == sb.device && sa.inode == sb.inode
    end
    _landing(a) == _landing(b)
end

"""
    save(migration::Migration, path)

Write the migrated model to `path` and its image files beside it, at their relative paths under
`path`'s directory, as `sysml -migrate -o` writes them. Nothing is written until every
destination is judged: a `path` naming the v1 model the migration came from, an image that
would land outside the model's directory — through `..`, an absolute path or a symbolic
link — or one whose own path is a symbolic link is refused with an `ArgumentError`. A
directory on the way replaced while the write is under way is not guarded against, as
`sysml -migrate -o` does not either. Returns the migration.
"""
function save(migration::Migration, path::AbstractString)
    source = migration.source_path
    source !== nothing && ispath(source) && _same_path(path, source) &&
        throw(ArgumentError("$path names the model being migrated; the v1 model would be " *
                            "replaced by its migration"))
    absolute = _normalized(path)
    base = dirname(absolute)
    landed_base = _landing(base)
    destinations = Pair{String,Vector{UInt8}}[]
    for (name, data) in sort!(collect(migration.files); by=first)
        segments = split(name, '/')
        malformed = any(s -> s in ("", ".", "..") || occursin('\\', s), segments)
        file = joinpath(base, segments...)
        if malformed || !_within(base, file) || !_within(landed_base, _landing(file))
            throw(ArgumentError("the migration's image $name would land outside $base"))
        end
        for guarded in (absolute, source)
            guarded === nothing && continue
            _same_path(file, guarded) &&
                throw(ArgumentError("the migration's image $name would replace $guarded"))
        end
        islink(file) && throw(ArgumentError("the migration's image $name would be written " *
                                            "through a symbolic link at $file"))
        push!(destinations, file => data)
    end
    open(path, "w") do io
        write(io, migration.content)
    end
    for (file, data) in destinations
        mkpath(dirname(file))
        write(file, data)
    end
    migration
end
