mutable struct SymbolInfo
    raw::Dict{String,Any}
    model::Model
    children_cache::Union{Nothing,Vector{SymbolInfo}}
    attributes_cache::Union{Nothing,Vector{SymbolInfo}}
end
SymbolInfo(raw::AbstractDict, model::Model) =
    SymbolInfo(Dict{String,Any}(String(k) => v for (k, v) in raw), model, nothing, nothing)

function Base.getproperty(info::SymbolInfo, name::Symbol)
    name === :raw && return getfield(info, :raw)
    name === :model && return getfield(info, :model)
    name === :children_cache && return getfield(info, :children_cache)
    name === :attributes_cache && return getfield(info, :attributes_cache)
    raw = getfield(info, :raw)
    name === :id && return String(get(raw, "id", ""))
    name === :name && return String(get(raw, "name", ""))
    name === :kind && return String(get(raw, "kind", ""))
    name === :metadata && return Dict{String,String}(String(k) => String(v) for (k, v) in get(raw, "metadata", Dict()))
    name === :type_facts && return haskey(raw, "typeInfo") ? TypeFacts(raw["typeInfo"]) : nothing
    name === :multiplicity && return haskey(raw, "multiplicity") ? Multiplicity(raw["multiplicity"]) : nothing
    name === :specializations && return Specialization[Specialization(s) for s in get(raw, "specializations", Any[])]
    name === :withheld_library_attributes && return Int(get(raw, "withheldLibraryAttributes", 0))
    return getfield(info, name)
end

function Base.show(io::IO, info::SymbolInfo)
    print(io, "$(info.id) ($(info.kind))")
end

"""Return typed symbol metadata for a qualified identifier, or `nothing`."""
function get_symbol(model::Model, id::AbstractString)
    answer = call(model.connection, "GetSymbol",
                  Dict{String,Any}("modelHash" => model.hash, "symbolId" => String(id)))
    haskey(answer, "error") && !isempty(answer["error"]) && return nothing
    raw = get(answer, "symbol", nothing)
    raw isa AbstractDict || return nothing
    return SymbolInfo(raw, model)
end

"""Return the direct child symbols of `info`."""
function children(info::SymbolInfo)
    info.children_cache !== nothing && return info.children_cache
    result = SymbolInfo[]
    for id in get(info.raw, "childIds", Any[])
        child = get_symbol(info.model, String(id))
        child === nothing || push!(result, child)
    end
    info.children_cache = result
    return result
end

const ATTRIBUTE_KINDS = Set(["attributedef", "attributeusage", "referenceusage"])
const PART_KINDS = Set(["partdef", "partusage"])
_kind_is(info::SymbolInfo, kinds) = lowercase(info.kind) in kinds

"""Return declared and inherited attributes of `info`."""
function attributes(info::SymbolInfo)
    info.attributes_cache !== nothing && return info.attributes_cache
    declared = SymbolInfo[c for c in children(info) if _kind_is(c, ATTRIBUTE_KINDS)]
    by_name = Dict(c.name => c for c in declared)
    for inherited in _inherited_attributes(info, Set{String}())
        get!(by_name, inherited.name, inherited)
    end
    reported = String[String(get(f, "name", "")) for f in get(info.raw, "attributes", Any[])]
    ordered = SymbolInfo[by_name[n] for n in reported if haskey(by_name, n)]
    append!(ordered, SymbolInfo[c for c in declared if !(c.name in reported)])
    info.attributes_cache = ordered
    return ordered
end

function _inherited_attributes(info::SymbolInfo, visited::Set{String})
    (info.model === nothing || info.id in visited) && return SymbolInfo[]
    push!(visited, info.id)
    inherited = SymbolInfo[]
    for specialization in info.specializations
        target = specialization.target_id
        (isempty(target) || target in visited) && continue
        supertype = get_symbol(info.model, target)
        supertype === nothing && continue
        append!(inherited, SymbolInfo[c for c in children(supertype)
                                      if _kind_is(c, ATTRIBUTE_KINDS)])
        append!(inherited, _inherited_attributes(supertype, visited))
    end
    inherited
end

"""Return the part definitions and usages directly contained by `info`."""
parts(info::SymbolInfo) = SymbolInfo[c for c in children(info) if _kind_is(c, PART_KINDS)]
"""Return a named attribute symbol, or `nothing` when it is absent."""
get_attr(info::SymbolInfo, name::AbstractString) =
    begin
        for attr in attributes(info)
            attr.name == name && return attr
        end
        nothing
    end

"""Return typed facts for the attributes reported on `info`."""
function attribute_facts(info::SymbolInfo)
    require_capability(info.model.connection, CAPABILITY_SYMBOL_ATTRIBUTES)
    return AttributeFacts[AttributeFacts(a) for a in get(info.raw, "attributes", Any[])]
end

"""Return typed semantic facts for `info`."""
function facts(info::SymbolInfo)
    require_capability(info.model.connection, CAPABILITY_TYPE_FACTS)
    raw_attrs = AttributeFacts[AttributeFacts(a) for a in get(info.raw, "attributes", Any[])]
    SymbolFacts(info.id, info.name, info.kind, info.type_facts, info.multiplicity,
                info.specializations, raw_attrs, info.withheld_library_attributes)
end

function Base.getproperty(model::Model, name::Symbol)
    name === :roots && return SymbolInfo[SymbolInfo(raw, model) for raw in getfield(model, :roots)]
    name === :root && return isempty(getfield(model, :roots)) ? nothing :
        SymbolInfo(first(getfield(model, :roots)), model)
    return getfield(model, name)
end

"""Return the symbol with qualified identifier `fqn`, or `default`."""
function Base.get(model::Model, fqn::AbstractString, default=nothing)
    for r in model.roots
        r.id == fqn && return r
    end
    info = get_symbol(model, fqn)
    (info === nothing || info.id != fqn) ? default : info
end

function _walk_symbols(model::Model)
    found = SymbolInfo[]
    queue = copy(model.roots)
    while !isempty(queue)
        current = popfirst!(queue)
        append!(found, children(current))
        append!(queue, children(current))
    end
    found
end

function _edit_distance(left::String, right::String)
    previous = collect(0:length(right))
    for (i, a) in enumerate(left)
        current = Vector{Int}(undef, length(right) + 1)
        current[1] = i
        for (j, b) in enumerate(right)
            current[j + 1] = min(current[j] + 1, previous[j + 1] + 1,
                                 previous[j] + (a == b ? 0 : 1))
        end
        previous = current
    end
    previous[end]
end

function _near_names(model::Model, name::String)
    candidates = String[]
    for info in Iterators.flatten((model.roots, _walk_symbols(model)))
        isempty(info.name) || push!(candidates, info.name)
        isempty(info.id) || info.id == info.name || push!(candidates, info.id)
    end
    unique!(candidates)
    scored = Tuple{Float64,String}[]
    for candidate in candidates
        width = max(length(name), length(candidate))
        width == 0 && continue
        score = 1 - _edit_distance(name, candidate) / width
        score >= 0.6 && push!(scored, (score, candidate))
    end
    sort!(scored; by=entry -> (-entry[1], entry[2]))
    String[entry[2] for entry in Iterators.take(scored, 3)]
end

"""Find a model symbol by short name or qualified identifier."""
function find(model::Model, name::AbstractString)
    for r in model.roots
        (r.name == name || r.id == name) && return r
    end
    if occursin("::", name)
        exact = get(model, name, nothing)
        exact === nothing || return exact
    end
    if has_capability(model.connection, CAPABILITY_QUERY)
        elements = query(model; select=["owner"],
                         where=Dict("@type" => "PrimitiveConstraint", "operator" => "=",
                                    "property" => "name", "value" => String(name)))
        for element in elements
            info = get(model, element.id, nothing)
            info === nothing || return info
        end
    end
    candidates = _walk_symbols(model)
    for candidate in candidates
        candidate.name == name && return candidate
    end
    if occursin("::", name)
        elements = query(model; select=[], where=Dict("@type" => "PrimitiveConstraint",
            "operator" => "=", "property" => "qualifiedName", "value" => String(name)))
        for element in elements
            info = get(model, element.id, nothing)
            info === nothing || return info
        end
    end
    return nothing
end

function Base.getindex(model::Model, name::AbstractString)
    found = find(model, name)
    found === nothing && throw(SymbolNotFoundError(name, _near_names(model, String(name))))
    found
end
"""Return whether a short name or qualified identifier resolves in `model`."""
Base.haskey(model::Model, name::AbstractString) = find(model, name) !== nothing
Base.in(name::AbstractString, model::Model) = haskey(model, name)
