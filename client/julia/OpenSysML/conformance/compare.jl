# Comparison and normalization rules shared by conformance/run.jl and the
# package's unit tests. Operates on JSON.parse output: Dict{String,Any} objects,
# Vector lists and typed scalars. Actual responses are proto3-JSON (lowerCamel);
# expectations spell proto snake_case.

function lower_camel(k::AbstractString)
    parts = split(k, '_')
    return join([parts[1]; [uppercasefirst(p) for p in parts[2:end] if !isempty(p)]])
end

# Every expected key is looked up as written first, then lowerCamel.
function lookup(value, path::AbstractString)
    lookup_parts(value, split(path, '.'))
end

function _get_member(value, key)
    if value isa AbstractDict
        haskey(value, key) && return (value[key], true)
        camel = lower_camel(key)
        camel != key && haskey(value, camel) && return (value[camel], true)
        return (nothing, false)
    elseif value isa AbstractVector
        i = tryparse(Int, key)
        i !== nothing && 0 <= i < length(value) && return (value[i+1], true)
        return (nothing, false)
    end
    return (nothing, false)
end

function lookup_parts(value, parts)
    isempty(parts) && return value
    head, rest = parts[1], parts[2:end]
    if head == "*"
        items = value isa AbstractVector ? value :
                value isa AbstractDict ? collect(values(value)) : Any[]
        for item in items
            got = lookup_parts(item, rest)
            got !== nothing && return got
        end
        return nothing
    end
    next, found = _get_member(value, head)
    found || return nothing
    return lookup_parts(next, rest)
end

function values_at(value, path::AbstractString)
    values_parts(value, split(path, '.'))
end

function values_parts(value, parts)
    isempty(parts) && return Any[value]
    head, rest = parts[1], parts[2:end]
    if head == "*"
        items = value isa AbstractVector ? value :
                value isa AbstractDict ? collect(values(value)) : Any[]
        return reduce(vcat, Any[values_parts(item, rest) for item in items]; init=Any[])
    end
    next, found = _get_member(value, head)
    found || return Any[]
    return values_parts(next, rest)
end

# An int64 arrives as a JSON string on the wire; an expected integer compares
# equal to the string that parses to it.
_parse_int(s) = tryparse(Int64, String(s))

function numbers_equal(expected, actual)
    if expected isa Integer && actual isa AbstractString
        got = _parse_int(actual)
        return got !== nothing && got == expected
    end
    if expected isa Number && actual isa Number && !(expected isa Bool) && !(actual isa Bool)
        expected == actual && return true
        e, a = Float64(expected), Float64(actual)
        scale = max(abs(e), abs(a))
        return scale != 0.0 && abs(e - a) / scale <= 1e-9
    end
    return isequal(expected, actual)
end

# An absent field and one holding its default are the same thing on the wire:
# the expectation below is satisfied by a key that is not present.
function is_default_expectation(expected)
    expected === nothing && return true
    expected === false && return true
    expected isa AbstractString && isempty(expected) && return true
    expected isa Number && !(expected isa Bool) && expected == 0 && return true
    expected isa AbstractVector && isempty(expected) && return true
    expected isa AbstractDict && all(is_default_expectation(v) for v in values(expected)) && return true
    return false
end

function is_default(value)
    value === nothing && return true
    value === false && return true
    value == "" && value isa AbstractString && return true
    if value isa Number && !(value isa Bool)
        value == 0 && return true
    end
    if value isa AbstractString
        i = _parse_int(value)
        i !== nothing && i == 0 && return true
    end
    value isa AbstractVector && isempty(value) && return true
    value isa AbstractDict && isempty(value) && return true
    return false
end

const RUNTIME_ID_KEYS = ["instance_id", "instanceId", "self_id", "selfId"]

# Runtime ids: an Instance is an object carrying type_symbol_id/typeSymbolId,
# feature_values/featureValues and id; its id, and every instance_id/self_id in
# the response, become @1, @2, ... in order of first appearance.
function label_instance_ids(value)
    labels = Dict{Int64,String}()
    _label_ids!(value, labels)
end

function _label_id(id, labels)
    n = id isa AbstractString ? _parse_int(id) : (id isa Integer ? Int64(id) : nothing)
    n === nothing && return id
    return get!(labels, n) do
        "@$(length(labels) + 1)"
    end
end

function _label_ids!(value, labels)
    if value isa AbstractDict
        is_instance = (haskey(value, "type_symbol_id") || haskey(value, "typeSymbolId")) &&
                      (haskey(value, "feature_values") || haskey(value, "featureValues")) &&
                      haskey(value, "id")
        if is_instance && haskey(value, "id")
            value["id"] = _label_id(value["id"], labels)
        end
        for key in RUNTIME_ID_KEYS
            if haskey(value, key)
                value[key] = _label_id(value[key], labels)
            end
        end
        for (k, child) in collect(value)
            skip = (is_instance && k == "id") || k in RUNTIME_ID_KEYS
            skip || _label_ids!(child, labels)
        end
    elseif value isa AbstractVector
        for item in value
            _label_ids!(item, labels)
        end
    end
end

function _is_absolute_path(s::AbstractString)
    startswith(s, '/') || (length(s) > 2 && s[2] == ':' && s[3] == '\\')
end

function _zero_self_id(v)
    v == 0 || v === "0"
end

# Rewrites string leaves in place where possible; dicts and vectors are
# mutated, so the caller takes the return value.
function normalize_response!(value, model_hash::AbstractString)
    _norm(v) = _normalize_value(v, model_hash)
    return _norm(value)
end

function _normalize_value(value, model_hash)
    if value isa AbstractDict
        is_instance = (haskey(value, "type_symbol_id") || haskey(value, "typeSymbolId")) &&
                      (haskey(value, "feature_values") || haskey(value, "featureValues")) &&
                      haskey(value, "id")
        is_server_info = haskey(value, "capabilities") && haskey(value, "version")
        for key in ("self_id", "selfId")
            haskey(value, key) && _zero_self_id(value[key]) && delete!(value, key)
        end
        for (k, child) in collect(value)
            if is_server_info && k == "version"
                value[k] = "\${version}"
            elseif (is_instance && k == "id") || k in RUNTIME_ID_KEYS
                # labelled after normalization, in one pass over the response
            else
                value[k] = _normalize_value(child, model_hash)
            end
        end
    elseif value isa AbstractVector
        for i in eachindex(value)
            value[i] = _normalize_value(value[i], model_hash)
        end
    elseif value isa AbstractString
        if !isempty(model_hash) && value == model_hash
            return "\${model_hash}"
        elseif _is_absolute_path(value)
            return "\${path}"
        end
    end
    return value
end

function status_canonical(code::AbstractString)
    code == "OK" && return "OK"
    join(uppercase.(split(code, '_')), "_")
end

function compare(expect, actual)
    failures = String[]
    if haskey(expect, "response") && expect["response"] !== nothing
        compare_value(expect["response"], actual, raw"$", failures)
    end
    for path in get(expect, "non_empty", Any[])
        got = lookup(actual, path)
        if got === nothing
            push!(failures, "$path is absent")
        elseif is_default(got)
            push!(failures, "$path is empty")
        end
    end
    for path in get(expect, "absent", Any[])
        got = lookup(actual, path)
        if got !== nothing && !is_default(got)
            push!(failures, "$path is not absent/default: $(JSON.json(got))")
        end
    end
    for (path, needle) in get(expect, "contains", Dict())
        got = lookup(actual, path)
        if got isa AbstractString && occursin(needle, got)
        elseif got === nothing
            push!(failures, "$path is absent")
        else
            push!(failures, "$path=$(JSON.json(got)) does not contain $(repr(needle))")
        end
    end
    for (path, needles) in get(expect, "contains_all", Dict())
        check_contains_all(path, needles, actual, failures)
    end
    for (path, wanted) in get(expect, "counts", Dict())
        check_count(path, wanted, false, actual, failures)
    end
    for (path, wanted) in get(expect, "min_counts", Dict())
        check_count(path, wanted, true, actual, failures)
    end
    return failures
end

function check_contains_all(path, needles, actual, failures)
    if any(==("*"), split(path, '.'))
        check_members_at(path, needles, actual, failures)
        return
    end
    got = lookup(actual, path)
    if got isa AbstractString
        for needle in needles
            occursin(needle, got) ||
                push!(failures, "$path does not contain $(repr(needle))")
        end
    elseif got isa AbstractVector
        for needle in needles
            any(v -> v == needle, got) ||
                push!(failures, "$path does not contain member $(repr(needle))")
        end
    else
        check_members_at(path, needles, actual, failures)
    end
end

function check_members_at(path, needles, actual, failures)
    values = values_at(actual, path)
    if isempty(values)
        push!(failures, "$path is absent or not text/list")
        return
    end
    for needle in needles
        any(v -> v isa AbstractString && v == needle, values) ||
            push!(failures, "$path does not contain member $(repr(needle))")
    end
end

function check_count(path, wanted, minimum, actual, failures)
    got = lookup(actual, path)
    count = if got === nothing
        # an absent list or map holds zero entries, as the wire spells it
        0
    elseif got isa AbstractVector || got isa AbstractDict
        length(got)
    else
        push!(failures, "$path is not a list/map")
        return
    end
    if (minimum && count < wanted) || (!minimum && count != wanted)
        relation = minimum ? "at least" : "exactly"
        push!(failures, "$path has $count entries, want $relation $wanted")
    end
end

function compare_value(expected, actual, path, failures)
    if expected isa AbstractDict && actual isa AbstractDict
        for (key, want) in expected
            got, found = _get_member(actual, key)
            if !found
                is_default_expectation(want) ||
                    push!(failures, "$path.$key is absent")
            else
                compare_value(want, got, "$path.$key", failures)
            end
        end
    elseif expected isa AbstractVector && actual isa AbstractVector
        if length(expected) != length(actual)
            push!(failures, "$path has $(length(actual)) entries, want $(length(expected))")
            return
        end
        for i in eachindex(expected)
            compare_value(expected[i], actual[i], "$path.$(i-1)", failures)
        end
    elseif expected isa Number && !(expected isa Bool)
        numbers_equal(expected, actual) ||
            push!(failures, "$path: got $(JSON.json(actual)), want $(JSON.json(expected))")
    else
        isequal(expected, actual) ||
            push!(failures, "$path: got $(JSON.json(actual)), want $(JSON.json(expected))")
    end
end
