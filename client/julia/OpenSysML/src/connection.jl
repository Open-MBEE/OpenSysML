mutable struct Connection
    base::String
    private::Bool
    process::Union{Base.Process,Nothing}
    stdin::Union{IO,Nothing}
    timeout::Float64
    info::Union{Dict{String,Any},Nothing}
    Connection(base, private, process, stdin, timeout) = new(base, private, process, stdin, timeout, nothing)
end

function _base_url(address::AbstractString)
    a = String(strip(address))
    if !startswith(a, "http://") && !startswith(a, "https://")
        a = "http://" * a
    end
    return rstrip(a, '/')
end

function resolve_binary()
    if haskey(ENV, "OPENSYSML_GRPC_BINARY") && !isempty(ENV["OPENSYSML_GRPC_BINARY"])
        return ENV["OPENSYSML_GRPC_BINARY"]
    end
    name = Sys.iswindows() ? "sysml-grpc.exe" : "sysml-grpc"
    cached = joinpath(homedir(), ".opensysml", "bin", name)
    isfile(cached) && return cached
    found = Sys.which(name)
    found === nothing &&
        throw(TransportError("no sysml-grpc binary found; set OPENSYSML_GRPC_BINARY, install to ~/.opensysml/bin/$(name), or put it on PATH"))
    return found
end

function external(address::AbstractString; timeout::Real=30)
    return Connection(_base_url(address), false, nothing, nothing, Float64(timeout))
end

function private(; binary::Union{AbstractString,Nothing}=nothing, timeout::Real=30)
    bin = binary === nothing ? resolve_binary() : String(binary)
    cmd = `$bin -port 0 -health-port 0 -report-address -exit-with-parent`
    proc = try
        open(cmd, "r+")
    catch e
        throw(TransportError("could not start $(bin): $(sprint(showerror, e))"))
    end
    address = try
        readline(proc.out)
    catch e
        kill(proc)
        throw(TransportError("sysml-grpc exited without reporting an address"))
    end
    isempty(address) && throw(TransportError("sysml-grpc exited without reporting an address"))
    return Connection(_base_url(address), true, proc, proc.in, Float64(timeout))
end

function connect(; timeout::Real=30)
    if haskey(ENV, "OPENSYSML_SERVICE") && !isempty(ENV["OPENSYSML_SERVICE"])
        return external(ENV["OPENSYSML_SERVICE"]; timeout=timeout)
    end
    return private(; timeout=timeout)
end

function Base.close(conn::Connection)
    conn.stdin === nothing || close(conn.stdin)
    if conn.process !== nothing
        deadline = time() + 5
        while process_running(conn.process) && time() < deadline
            sleep(0.05)
        end
        process_running(conn.process) && kill(conn.process)
        try
            wait(conn.process)
        catch
        end
    end
    conn.stdin = nothing
    conn.process = nothing
    return nothing
end

function call(conn::Connection, method::AbstractString, request)
    body = JSON.json(request)
    response = try
        HTTP.post("$(conn.base)/sysml.SysMLService/$(method)",
                  ["Content-Type" => "application/json"], body;
                  status_exception=false, readtimeout=max(1, round(Int, conn.timeout)))
    catch e
        throw(TransportError("$(method) failed: $(sprint(showerror, e))"))
    end
    content_type = lowercase(String(HTTP.header(response, "Content-Type")))
    is_json = occursin("application/json", content_type)
    if response.status != 200
        if is_json
            out = _parse_json(String(response.body), method, response.status)
            throw(ConnectError(get(out, "code", "unknown"), get(out, "message", ""), response.status))
        end
        throw(TransportError("$(method) answered HTTP $(response.status) with a non-JSON body"))
    end
    is_json || throw(TransportError("$(method) answered HTTP 200 with a non-JSON body"))
    return _parse_json(String(response.body), method, response.status)
end

function _parse_json(text::AbstractString, method::AbstractString, status::Integer)
    return try
        JSON.parse(text)
    catch
        throw(TransportError("$(method) answered HTTP $(status) with undecodable JSON"))
    end
end

function server_info(conn::Connection)
    conn.info === nothing && (conn.info = call(conn, "GetServerInfo", Dict{String,Any}()))
    return (get(conn.info, "version", ""), get(conn.info, "capabilities", Any[]))
end

has_capability(conn::Connection, name::AbstractString) =
    name in server_info(conn)[2]
