mutable struct Connection
    base::String
    private::Bool
    process::Union{Base.Process,Nothing}
    stdin::Union{IO,Nothing}
    timeout::Float64
    info::Union{ServerInfo,Nothing}
    origin::String
    expected_version::Union{String,Nothing}
    required_capabilities::Set{String}
    Connection(base, private, process, stdin, timeout; origin="service at $(base)", version=nothing,
               require_capabilities=()) =
        new(String(base), Bool(private), process, stdin, Float64(timeout), nothing,
            String(origin), version === nothing ? nothing : String(version),
            Set(String(c) for c in require_capabilities))
end

function _base_url(address::AbstractString)
    a = String(strip(address))
    if !startswith(a, "http://") && !startswith(a, "https://")
        a = "http://" * a
    end
    return rstrip(a, '/')
end

function _version_request(version)
    requested = version === nothing ? get(ENV, "OPENSYSML_GRPC_VERSION", nothing) : String(version)
    requested == "latest" || return requested
    try
        resolve_latest_version()
    catch err
        err isa TransportError && return nothing
        rethrow()
    end
end

"""Connect to a running sysml-grpc service."""
function external(address::AbstractString; timeout::Real=30, version=nothing, require_capabilities=())
    expected = _version_request(version)
    conn = Connection(_base_url(address), false, nothing, nothing, Float64(timeout);
                      origin="service at $(_base_url(address)) (not started by this client)",
                      version=expected, require_capabilities=require_capabilities)
    if expected !== nothing || !isempty(conn.required_capabilities)
        try
            _validate_connection(conn)
        catch
            close(conn)
            rethrow()
        end
    end
    return conn
end

"""Start and connect to a private sysml-grpc service process."""
function private(; binary::Union{AbstractString,Nothing}=nothing, timeout::Real=30,
                 version=nothing, require_capabilities=())
    bin = binary === nothing ? resolve_binary(; version=version) : String(binary)
    _is_executable_file(bin) || throw(TransportError("sysml-grpc binary is not an executable file: $(bin)"))
    cmd = `$bin -port 0 -health-port 0 -report-address -exit-with-parent`
    proc = try
        open(cmd, "r+")
    catch e
        throw(TransportError("could not start $(bin): $(sprint(showerror, e))"))
    end
    task = @async readline(proc.out)
    deadline = time() + Float64(timeout)
    while !istaskdone(task) && time() < deadline
        sleep(0.05)
    end
    if !istaskdone(task)
        _abort_child(proc)
        throw(TransportError("sysml-grpc reported no address within $(timeout)s"))
    end
    address = try
        fetch(task)
    catch
        nothing
    end
    if address === nothing || isempty(address)
        _abort_child(proc)
        throw(TransportError("sysml-grpc exited without reporting an address"))
    end
    expected = _version_request(version)
    conn = Connection(_base_url(address), true, proc, proc.in, Float64(timeout);
                      origin="binary $(bin)", version=expected,
                      require_capabilities=require_capabilities)
    try
        _validate_connection(conn)
    catch
        close(conn)
        rethrow()
    end
    return conn
end

function _abort_child(proc)
    try
        close(proc.in)
    catch
    end
    kill(proc)
    deadline = time() + 2
    while process_running(proc) && time() < deadline
        sleep(0.05)
    end
    process_running(proc) && kill(proc, Base.SIGKILL)
    try
        wait(proc)
    catch
    end
    return nothing
end

"""Connect to the configured service or start a private service."""
function connect(; timeout::Real=30, version=nothing, require_capabilities=())
    if haskey(ENV, "OPENSYSML_SERVICE") && !isempty(ENV["OPENSYSML_SERVICE"])
        return external(ENV["OPENSYSML_SERVICE"]; timeout=timeout, version=version,
                        require_capabilities=require_capabilities)
    end
    return private(; timeout=timeout, version=version, require_capabilities=require_capabilities)
end

"""Run `f` with a connection that is closed when `f` returns."""
function connect(f::Function; kwargs...)
    conn = connect(; kwargs...)
    try
        return f(conn)
    finally
        close(conn)
    end
end

"""Close a connection and stop its private service process, if any."""
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

"""Call a sysml-grpc JSON-transcoded RPC with a dictionary request."""
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

"""Return cached service version and capability information."""
function server_info(conn::Connection)
    if conn.info === nothing
        try
            out = call(conn, "GetServerInfo", Dict{String,Any}())
            conn.info = ServerInfo(get(out, "version", ""), get(out, "capabilities", Any[]), true, conn.origin)
        catch e
            if e isa UnsupportedOperationError
                conn.info = ServerInfo("", Set{String}(), false, conn.origin)
            else
                rethrow()
            end
        end
    end
    info = conn.info::ServerInfo
    _raise_if_mismatch(conn, info)
    return info
end

"""Return whether the connected service advertises `name`."""
has_capability(conn::Connection, name::AbstractString) = name in server_info(conn)
require_capability(conn::Connection, capability::AbstractString) =
    require_capability(server_info(conn), capability)

function _raise_if_mismatch(conn::Connection, info::ServerInfo)
    reason = mismatch_reason(info; version=conn.expected_version)
    reason === nothing && return nothing
    remedy = conn.private ?
        "the binary this client started is not $(conn.expected_version); install that release or pass version=nothing" :
        "stop the service at $(conn.base) and start the requested release"
    throw(StaleServiceError(conn.base, reason, remedy, info))
end

function _validate_connection(conn::Connection)
    info = server_info(conn)
    for capability in sort!(collect(conn.required_capabilities))
        require_capability(info, capability)
    end
    return nothing
end

function _translate(f::Function; not_found=ModelNotFoundError, capabilities=(), connection=nothing)
    try
        return f()
    catch e
        e isa ConnectError || rethrow()
        if e.code == "unimplemented" && !isempty(capabilities)
            capability = findfirst(c -> occursin(String(c), e.message), capabilities)
            name = capability === nothing ? first(capabilities) : capabilities[capability]
            info = connection === nothing ? nothing : server_info(connection)
            throw(MissingCapabilityError(String(name), info, upgrade_remedy(name)))
        elseif e.code == "not_found" && occursin("symbol not found: ", lowercase(e.message))
            name = split(e.message, "symbol not found: "; limit=2)[end]
            throw(SymbolNotFoundError(name))
        elseif e.code == "not_found" && not_found !== ModelNotFoundError
            not_found === SymbolNotFoundError && begin
                name = split(e.message, "symbol not found: "; limit=2)[end]
                throw(SymbolNotFoundError(name))
            end
            throw(ConnectError(e.code, e.message, e.http_status; not_found=not_found))
        end
        rethrow()
    end
end
