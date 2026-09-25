# Conformance runner: drives every scenario through OpenSysML.call. Run with
# julia --project=client/julia/OpenSysML run.jl [--binary P | --address H:P] [flags].

module ConformanceRunner

using OpenSysML
using HTTP
using JSON

include("compare.jl")

const SERVICE = "sysml.SysMLService"

function repository_root()
    dir = pwd()
    while true
        isdir(joinpath(dir, "conformance", "scenarios")) && return dir
        parent = dirname(dir)
        parent == dir && error("could not locate repository root")
        dir = parent
    end
end

function parse_options(args)
    opts = Dict{String,Any}("binary" => nothing, "address" => nothing,
                            "scenarios" => nothing, "fixtures" => nothing,
                            "run" => nothing, "report" => nothing,
                            "allow_skips" => false, "verbose" => false)
    i = 1
    takes_value = ("--binary", "-binary", "--address", "-address",
                   "--scenarios", "-scenarios", "--fixtures", "-fixtures",
                   "--run", "-run", "--report", "-report")
    while i <= length(args)
        arg = args[i]
        if arg in takes_value
            i += 1
            i <= length(args) || error("$arg needs a value")
            key = replace(arg, r"^--?|-" => s -> "")
            opts[lstrip(arg, '-')] = args[i]
        elseif arg in ("--allow-skips", "-allow-skips")
            opts["allow_skips"] = true
        elseif arg in ("-v", "--verbose")
            opts["verbose"] = true
        elseif arg in ("-h", "--help")
            println("run.jl [--binary PATH | --address HOST:PORT] [--scenarios DIR] [--fixtures DIR] [--run SUBSTRING] [--report FILE|-] [--allow-skips] [-v]")
            exit(0)
        else
            error("unknown flag $(repr(arg))")
        end
        i += 1
    end
    return opts
end

function load_scenarios(dir)
    paths = sort(filter(p -> endswith(p, ".json"),
                        [joinpath(dir, f) for f in readdir(dir)]))
    scenarios = Any[]
    seen = Set{String}()
    for path in paths
        suite = JSON.parsefile(path)
        for sc in suite["scenarios"]
            (isempty(get(sc, "id", "")) || isempty(get(sc, "rpc", ""))) &&
                error("$path: scenario needs id and rpc")
            sc["id"] in seen && error("duplicate scenario id $(repr(sc["id"]))")
            push!(seen, sc["id"])
            push!(scenarios, sc)
        end
    end
    isempty(scenarios) && error("no scenario files in $dir")
    return scenarios
end

method_of(scenario) = last(split(scenario["rpc"], '/'))

function fixture_path(dir, name)
    (isabspath(name) || occursin('\\', name) || occursin(r"^[A-Za-z]:", name)) &&
        error("fixture $(repr(name)) is outside $dir")
    parts = split(name, '/')
    any(p -> p == "..", parts) && error("fixture $(repr(name)) is outside $dir")
    return joinpath(dir, parts...)
end

function resolve_placeholders!(value, model_hash, fixtures)
    if value isa AbstractDict
        for (k, child) in collect(value)
            value[k] = resolve_placeholders!(child, model_hash, fixtures)
        end
    elseif value isa AbstractVector
        for i in eachindex(value)
            value[i] = resolve_placeholders!(value[i], model_hash, fixtures)
        end
    elseif value isa AbstractString
        if value == "\${model_hash}"
            model_hash === nothing &&
                error("request names \${model_hash} but scenario declares no model")
            return model_hash
        elseif startswith(value, "\${fixture:") && endswith(value, "}")
            name = value[length("\${fixture:")+1:end-1]
            return read(fixture_path(fixtures, name), String)
        end
    end
    return value
end

mutable struct Runner
    conn::Connection
    fixtures::String
    models::Dict{String,String}
end

# One parse per distinct model spec, keyed by its fields.
function model_hash(r::Runner, spec)
    key = JSON.json(spec)
    haskey(r.models, key) && return r.models[key]
    language = get(spec, "language", "")
    strict = get(spec, "strict_conformance", false)
    if haskey(spec, "fixture") && !isempty(spec["fixture"])
        name = spec["fixture"]
        request = Dict{String,Any}("content" => read(fixture_path(r.fixtures, name), String))
        isempty(language) || (request["language"] = language)
        strict && (request["strictConformance"] = true)
        answer = call(r.conn, "ParseFile", request)
    else
        docs = Any[Dict{String,Any}("name" => f,
                                    "content" => read(fixture_path(r.fixtures, f), String))
                   for f in get(spec, "fixtures", Any[])]
        for doc in docs
            isempty(language) || (doc["language"] = language)
        end
        request = Dict{String,Any}("documents" => docs)
        strict && (request["strictConformance"] = true)
        answer = call(r.conn, "ParseSources", request)
    end
    hash = answer["modelHash"]
    r.models[key] = hash
    return hash
end

# Raw POST so the scenario request is sent exactly as written; the typed helpers
# are exercised by the package's own tests instead.
function raw_call(conn, method, body_text)
    response = HTTP.post("$(conn.base)/$SERVICE/$method",
                         ["Content-Type" => "application/json"], body_text;
                         status_exception=false, readtimeout=max(1, round(Int, conn.timeout)))
    return (status=response.status,
            content_type=lowercase(String(HTTP.header(response, "Content-Type"))),
            body=String(response.body))
end

function run_scenario(r::Runner, scenario, capabilities)
    result = Dict{String,Any}("id" => scenario["id"], "outcome" => "pass",
                              "rpc" => method_of(scenario), "status" => "OK",
                              "duration_ms" => 0.0)
    started = time()
    missing = [c for c in get(scenario, "requires_capabilities", Any[])
               if !(c in capabilities)]
    expect = scenario["expect"]
    if !isempty(missing)
        if haskey(scenario, "expect_without_capability")
            expect = scenario["expect_without_capability"]
        else
            result["outcome"] = "skip"
            result["status"] = "-"
            result["reason"] = "missing capability $(join(missing, ", "))"
            result["duration_ms"] = (time() - started) * 1000
            return result
        end
    end
    hash = nothing
    if haskey(scenario, "model") && scenario["model"] !== nothing
        try
            hash = model_hash(r, scenario["model"])
        catch e
            result["outcome"] = "error"
            result["status"] = "-"
            result["reason"] = sprint(showerror, e)
            result["duration_ms"] = (time() - started) * 1000
            return result
        end
    end
    request = deepcopy(get(scenario, "request", Dict{String,Any}()))
    try
        resolve_placeholders!(request, hash, r.fixtures)
    catch e
        result["outcome"] = "error"
        result["status"] = "-"
        result["reason"] = sprint(showerror, e)
        result["duration_ms"] = (time() - started) * 1000
        return result
    end
    failures = String[]
    local response
    try
        response = raw_call(r.conn, method_of(scenario), JSON.json(request))
    catch e
        result["outcome"] = "error"
        result["status"] = "-"
        result["reason"] = sprint(showerror, e)
        result["duration_ms"] = (time() - started) * 1000
        return result
    end
    want_status = get(expect, "status", "OK")
    want_status === nothing && (want_status = "OK")
    if response.status != 200 || !occursin("application/json", response.content_type)
        if response.status != 200 && occursin("application/json", response.content_type)
            out = JSON.parse(response.body)
            result["status"] = status_canonical(get(out, "code", "unknown"))
            if result["status"] != want_status
                result["outcome"] = "fail"
                push!(failures, "status: $(result["status"]) ($(get(out, "message", ""))), want $want_status")
            elseif haskey(expect, "status_message_contains") &&
                   !occursin(expect["status_message_contains"], get(out, "message", ""))
                result["outcome"] = "fail"
                push!(failures, "status message $(repr(get(out, "message", ""))) does not contain $(repr(expect["status_message_contains"]))")
            end
        else
            result["outcome"] = "error"
            result["status"] = "-"
            result["reason"] = "transport answered HTTP $(response.status) with a non-JSON body"
        end
    else
        actual = JSON.parse(response.body)
        normalize_response!(actual, hash === nothing ? "" : hash)
        label_instance_ids(actual)
        if want_status != "OK"
            result["outcome"] = "fail"
            push!(failures, "the call succeeded, want status $want_status")
        else
            failures = compare(expect, actual)
            isempty(failures) || (result["outcome"] = "fail")
        end
    end
    isempty(failures) || (result["failures"] = failures)
    result["duration_ms"] = (time() - started) * 1000
    return result
end

function print_result(result, verbose)
    mark = get(Dict("pass" => "PASS", "fail" => "FAIL", "skip" => "SKIP"), result["outcome"], "ERR ")
    println("$mark $(rpad(result["id"], 46)) $(result["status"])")
    haskey(result, "reason") && println("       $(result["reason"])")
    for f in get(result, "failures", String[])
        println("       $f")
    end
    verbose && println("       duration_ms=$(round(result["duration_ms"], digits=3))")
end

function main(args)
    opts = parse_options(args)
    root = repository_root()
    scenarios_dir = something(opts["scenarios"], joinpath(root, "conformance", "scenarios"))
    fixtures_dir = something(opts["fixtures"], joinpath(root, "conformance", "fixtures"))
    scenarios = load_scenarios(scenarios_dir)
    conn = opts["address"] !== nothing ? external(opts["address"]) :
           opts["binary"] !== nothing ? OpenSysML.private(binary=opts["binary"]) :
           OpenSysML.private()
    service_name = something(opts["binary"], opts["address"], "private")
    try
        runner = Runner(conn, fixtures_dir, Dict{String,String}())
        _, caps = server_info(conn)
        capabilities = sort(String.(caps))
        results = Any[]
        for sc in scenarios
            if opts["run"] !== nothing && !occursin(opts["run"], sc["id"])
                continue
            end
            result = run_scenario(runner, sc, capabilities)
            print_result(result, opts["verbose"])
            push!(results, result)
        end
        counts = Dict(o => count(r -> r["outcome"] == o, results)
                      for o in ("pass", "fail", "skip", "error"))
        summary = Dict{String,Any}(
            "protocol" => "connect-json",
            "service" => String(service_name),
            "capabilities" => capabilities,
            "total" => length(results),
            "passed" => counts["pass"],
            "failed" => counts["fail"],
            "skipped" => counts["skip"],
            "errored" => counts["error"],
            "results" => results)
        report = Dict{String,Any}(
            "service" => String(service_name),
            "total" => length(results),
            "passed" => counts["pass"],
            "failed" => counts["fail"],
            "skipped" => counts["skip"],
            "errored" => counts["error"],
            "protocols" => Any[summary])
        println("total=$(report["total"]) passed=$(report["passed"]) failed=$(report["failed"]) skipped=$(report["skipped"]) errored=$(report["errored"])")
        if opts["report"] !== nothing
            data = JSON.json(report, 4) * "\n"
            opts["report"] == "-" ? print(data) : write(opts["report"], data)
        end
        if counts["fail"] > 0 || counts["error"] > 0
            error("$(counts["fail"] + counts["error"]) failed or errored scenarios")
        end
        if counts["skip"] > 0 && !opts["allow_skips"]
            error("$(counts["skip"]) scenarios skipped for missing capabilities; use --allow-skips")
        end
    finally
        close(conn)
    end
    return 0
end

end # module

if abspath(PROGRAM_FILE) == @__FILE__
    try
        ConformanceRunner.main(ARGS)
    catch e
        e isa InterruptException && rethrow()
        println(stderr, "conformance: $(sprint(showerror, e))")
        exit(1)
    end
end
