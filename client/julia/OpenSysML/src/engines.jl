struct Bound
    name::String
    limit::Int64
    reached::Bool
end
Bound(raw::AbstractDict) =
    Bound(String(get(raw, "name", "")), parse(Int64, string(get(raw, "limit", 0))),
          Bool(get(raw, "reached", false)))
Base.show(io::IO, b::Bound) = print(io, "$(b.name) $(b.limit)$(b.reached ? " reached" : "")")

struct Standing
    engine::String
    strength::String
    bounds::Vector{Bound}
end
Standing(raw::AbstractDict=Dict{String,Any}()) =
    Standing(String(get(raw, "engine", "")), String(get(raw, "strength", "")),
             Bound[Bound(b) for b in get(raw, "bounds", Any[])])
reported(s::Standing) = !isempty(s.strength)
reached(s::Standing) = Bound[b for b in s.bounds if b.reached]
function explain(s::Standing)
    reported(s) || return ""
    line = s.strength * (isempty(s.engine) ? "" : " by $(s.engine)")
    ends = reached(s)
    isempty(ends) || (line *= " (" * join(string.(ends), ", ") * ")")
    line
end
Base.show(io::IO, s::Standing) = print(io, explain(s))

struct EngineInfo
    name::String
    authority::String
    answers::Vector{String}
    bounds::Vector{String}
    process::String
    process_found::String
    ready::Bool
    unavailable::String
    kind::String
    protocol::String
    source::String
    command::String
    version::String
    served::Bool
end
function EngineInfo(raw::AbstractDict)
    EngineInfo(String(get(raw, "name", "")), String(get(raw, "authority", "")),
        String[String(v) for v in get(raw, "answers", Any[])],
        String[String(v) for v in get(raw, "bounds", Any[])],
        String(get(raw, "process", "")), String(get(raw, "processFound", "")),
        Bool(get(raw, "ready", false)), String(get(raw, "unavailable", "")),
        String(get(raw, "kind", "")), String(get(raw, "protocol", "")),
        String(get(raw, "source", "")), String(get(raw, "command", "")),
        String(get(raw, "version", "")), Bool(get(raw, "served", true)))
end
Base.show(io::IO, engine::EngineInfo) = print(io, "$(engine.name) ($(engine.authority))")

"""List the engines and authorities advertised by the service."""
function list_engines(conn::Connection)
    require_capability(conn, CAPABILITY_ENGINES)
    answer = _translate(; capabilities=(CAPABILITY_ENGINES,), connection=conn) do
        call(conn, "ListEngines", Dict{String,Any}())
    end
    return EngineInfo[EngineInfo(e) for e in get(answer, "engines", Any[])]
end
