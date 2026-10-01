const DEFAULT_GITHUB_REPO = "Open-MBEE/OpenSysML"
const NETWORK_TIMEOUT = 15
const MAX_BINARY_BYTES = 512 * 1024 * 1024
const MAX_METADATA_BYTES = 8 * 1024 * 1024
const ALLOW_UNPINNED_ENV = "OPENSYSML_ALLOW_UNPINNED_DOWNLOAD"
const BINARY_ENV = "OPENSYSML_BINARY"
const PINNED_DIGESTS_FILE = normpath(joinpath(@__DIR__, "..", "release-digests.json"))
const PINNED_SHA256 = JSON.parsefile(PINNED_DIGESTS_FILE)
const _BINARY_CACHE_LOCK = ReentrantLock()
const _BINARY_CACHE_LOCK_DEPTH = Ref(0)

function _default_github_repo()
    get(ENV, "OPENSYSML_GITHUB_REPO", "") == "" ?
        DEFAULT_GITHUB_REPO : ENV["OPENSYSML_GITHUB_REPO"]
end

function detect_platform()
    system = lowercase(string(Sys.KERNEL))
    goos = system in ("linux", "darwin", "windows") ? system :
        throw(TransportError("Unsupported operating system: $(system)"))
    machine = lowercase(string(Sys.ARCH))
    goarch = machine in ("x86_64", "amd64") ? "amd64" :
        machine in ("aarch64", "arm64") ? "arm64" :
        throw(TransportError("Unsupported architecture: $(machine)"))
    goos, goarch
end

function release_asset_name(goos=nothing, goarch=nothing)
    if goos === nothing || goarch === nothing
        detected_os, detected_arch = detect_platform()
        goos = goos === nothing ? detected_os : String(goos)
        goarch = goarch === nothing ? detected_arch : String(goarch)
    end
    name = "sysml-grpc-$(goos)-$(goarch)"
    goos == "windows" ? name * ".exe" : name
end

service_binary_name(goos=lowercase(string(Sys.KERNEL))) =
    goos == "windows" ? "sysml-grpc.exe" : "sysml-grpc"

get_binary_path() = joinpath(homedir(), ".opensysml", "bin", service_binary_name())
lock_path() = get_binary_path() * ".lock"
lock_path(binary_path::AbstractString) = String(binary_path) * ".lock"
metadata_path() = get_binary_path() * ".json"
metadata_path(binary_path::AbstractString) = String(binary_path) * ".json"

_is_executable_file(path) =
    isfile(path) && (Sys.iswindows() || (stat(path).mode & 0o111) != 0)

function _try_file_lock(io, path)
    try
        seekstart(io)
        if Sys.iswindows()
            ccall(:_locking, Cint, (Cint, Cint, Clong), Base.fd(io), 1, 1) == 0 ||
                error("lock failed")
        else
            ccall(:lockf, Cint, (Cint, Cint, Clong), Base.fd(io), 1, 0) == 0 ||
                error("lock failed")
        end
        return true
    catch err
        @warn "Could not lock $(path) against other processes: $(sprint(showerror, err))"
        false
    end
end

function _release_file_lock(io, path)
    try
        seekstart(io)
        if Sys.iswindows()
            ccall(:_locking, Cint, (Cint, Cint, Clong), Base.fd(io), 0, 1) == 0 ||
                error("unlock failed")
        else
            ccall(:lockf, Cint, (Cint, Cint, Clong), Base.fd(io), 0, 0) == 0 ||
                error("unlock failed")
        end
    catch err
        @warn "Could not unlock $(path): $(sprint(showerror, err))"
    end
    nothing
end

function _with_binary_cache_lock(f::Function, binary_path::AbstractString)
    lock(_BINARY_CACHE_LOCK) do
        if _BINARY_CACHE_LOCK_DEPTH[] > 0
            _BINARY_CACHE_LOCK_DEPTH[] += 1
            try
                return f()
            finally
                _BINARY_CACHE_LOCK_DEPTH[] -= 1
            end
        end
        path = lock_path(binary_path)
        try
            mkpath(dirname(path))
        catch err
            @warn "Could not lock $(path) against other processes: $(sprint(showerror, err))"
            _BINARY_CACHE_LOCK_DEPTH[] = 1
            try
                return f()
            finally
                _BINARY_CACHE_LOCK_DEPTH[] = 0
            end
        end
        io = try
            open(path, "a+")
        catch err
            @warn "Could not lock $(path) against other processes: $(sprint(showerror, err))"
            nothing
        end
        io === nothing && begin
            _BINARY_CACHE_LOCK_DEPTH[] = 1
            try
                return f()
            finally
                _BINARY_CACHE_LOCK_DEPTH[] = 0
            end
        end
        locked = _try_file_lock(io, path)
        _BINARY_CACHE_LOCK_DEPTH[] = 1
        try
            f()
        finally
            _BINARY_CACHE_LOCK_DEPTH[] = 0
            locked && _release_file_lock(io, path)
            close(io)
        end
    end
end

function named_binary()
    for variable in (BINARY_ENV, "OPENSYSML_GRPC_BINARY")
        named = get(ENV, variable, "")
        isempty(named) && continue
        _is_executable_file(named) ||
            throw(TransportError("\$$(variable) names $(named), which is not an executable file"))
        return named
    end
    nothing
end

function binary_on_path()
    path = Sys.which(service_binary_name())
    path !== nothing && _is_executable_file(path) ? path : nothing
end

function _fetch_url(url::AbstractString, limit::Integer)
    startswith(url, "file://") && return _read_bounded_file(url[8:end], limit)
    try
        return HTTP.open("GET", String(url); readtimeout=NETWORK_TIMEOUT) do stream
            status = stream.message.status
            200 <= status < 300 ||
                throw(TransportError("GET $(url) answered HTTP $(status)"))
            _read_bounded_stream(stream, url, limit)
        end
    catch err
        err isa OpenSysMLError && rethrow()
        throw(TransportError("GET $(url) failed: $(sprint(showerror, err))"))
    end
end

function _read_bounded_file(path, limit)
    try
        open(path, "r") do io
            body = read(io, limit + 1)
            length(body) <= limit ||
                throw(TransportError("file://$(path) is larger than $(limit) bytes"))
            body
        end
    catch err
        err isa OpenSysMLError && rethrow()
        throw(TransportError("could not read file://$(path): $(sprint(showerror, err))"))
    end
end

function _read_bounded_stream(stream, url, limit)
    body = UInt8[]
    while !eof(stream)
        remaining = limit + 1 - length(body)
        chunk = read(stream, min(8192, remaining))
        isempty(chunk) && break
        append!(body, chunk)
        length(body) <= limit ||
            throw(TransportError("$(url) is larger than $(limit) bytes"))
    end
    body
end

function _fetch(url, limit; fetcher=_fetch_url)
    bytes = fetcher(String(url), Int(limit))
    bytes isa AbstractVector{UInt8} ||
        throw(TransportError("GET $(url) returned a non-byte response"))
    length(bytes) <= limit ||
        throw(TransportError("$(url) is larger than $(limit) bytes"))
    Vector{UInt8}(bytes)
end

function _release_download_url(version, asset, repo; base_url=nothing)
    root = base_url === nothing ?
        "https://github.com/$(repo)/releases/download" : rstrip(String(base_url), '/')
    "$(root)/$(version)/$(asset)"
end

function resolve_latest_version(; github_repo=nothing, api_url=nothing, fetcher=_fetch_url)
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    url = api_url === nothing ?
        "https://api.github.com/repos/$(repo)/releases/latest" : String(api_url)
    release = try
        JSON.parse(String(_fetch(url, MAX_METADATA_BYTES; fetcher=fetcher)))
    catch err
        err isa TransportError && rethrow(TransportError("Failed to resolve latest release from $(url): $(err.message)"))
        throw(TransportError("Failed to resolve latest release from $(url): $(sprint(showerror, err))"))
    end
    release isa AbstractDict || throw(TransportError("Latest release of $(repo) is not a JSON object"))
    tag = get(release, "tag_name", "")
    isempty(String(tag)) && throw(TransportError("Latest release of $(repo) has no tag name"))
    String(tag)
end

function pinned_digest(version, asset; github_repo=nothing, pinned_digests=PINNED_SHA256)
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    String(get(get(get(pinned_digests, repo, Dict()), String(version), Dict()), String(asset), ""))
end

function unpinned_downloads_allowed(github_repo=nothing)
    allowed = lowercase(strip(get(ENV, ALLOW_UNPINNED_ENV, "")))
    allowed in ("", "0", "false", "no") && return false
    allowed in ("1", "true", "yes") && return true
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    repo in strip.(split(allowed, ','; keepempty=false))
end

function _served_digest(text, url)
    parts = split(strip(String(text)))
    isempty(parts) && throw(TransportError("checksum response from $(url) is empty"))
    digest = lowercase(String(first(parts)))
    occursin(r"^[0-9a-f]{64}$", digest) ||
        throw(TransportError("checksum response from $(url) does not start with a SHA-256 digest"))
    digest
end

function expected_digest(version, asset, served_digest; github_repo=nothing,
                         pinned_digests=PINNED_SHA256)
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    pinned = pinned_digest(version, asset; github_repo=repo, pinned_digests=pinned_digests)
    if isempty(pinned)
        unpinned_downloads_allowed(repo) ||
            throw(UnpinnedReleaseError(
                "no SHA-256 digest is pinned for $(asset) of $(version) of $(repo); " *
                "the only available checksum is served beside the binary, so set " *
                "\$$(ALLOW_UNPINNED_ENV)=$(repo) (or =1) to accept same-origin trust"))
        @warn "No digest is pinned for $(asset) of $(version) of $(repo); verifying the served checksum because \$$(ALLOW_UNPINNED_ENV) is set"
        return String(served_digest)
    end
    String(served_digest) == pinned ||
        throw(ChecksumMismatchError(
            "Checksum mismatch for $(asset) of $(version): $(repo) serves $(served_digest), " *
            "but the client pins $(pinned); it was not installed"))
    pinned
end

function release_download_url(version, asset; github_repo=nothing, base_url=nothing)
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    _release_download_url(String(version), String(asset), repo; base_url=base_url)
end

function download_binary(version="latest"; github_repo=nothing, base_url=nothing,
                         api_url=nothing, fetcher=_fetch_url,
                         pinned_digests=PINNED_SHA256, binary_path=get_binary_path())
    _with_binary_cache_lock(binary_path) do
        _download_binary_locked(version; github_repo=github_repo, base_url=base_url,
            api_url=api_url, fetcher=fetcher, pinned_digests=pinned_digests,
            binary_path=binary_path)
    end
end

function _download_binary_locked(version; github_repo, base_url, api_url, fetcher,
                                 pinned_digests, binary_path)
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    resolved = String(version)
    if resolved == "latest"
        resolved = resolve_latest_version(; github_repo=repo, api_url=api_url, fetcher=fetcher)
    end
    asset = release_asset_name()
    binary_url = _release_download_url(resolved, asset, repo; base_url=base_url)
    checksum_url = _release_download_url(resolved, asset * ".sha256", repo; base_url=base_url)
    checksum = _served_digest(_fetch(checksum_url, MAX_METADATA_BYTES; fetcher=fetcher), checksum_url)
    expected = expected_digest(resolved, asset, checksum;
        github_repo=repo, pinned_digests=pinned_digests)
    bytes = _fetch(binary_url, MAX_BINARY_BYTES; fetcher=fetcher)
    actual = bytes2hex(sha256(bytes))
    actual == expected ||
        throw(ChecksumMismatchError(
            "Checksum mismatch for $(asset): expected $(expected), but the downloaded " *
            "binary has digest $(actual); it was not installed"))
    _install_binary(binary_path, bytes, resolved, expected, repo)
    String(binary_path)
end

function _install_binary(binary_path, bytes, version, digest, repo)
    mkpath(dirname(binary_path))
    temp = String(binary_path) * ".tmp"
    metadata_file = metadata_path(binary_path)
    metadata_temp = metadata_file * ".tmp"
    try
        open(temp, "w") do io
            write(io, bytes)
        end
        chmod(temp, 0o700)
        mv(temp, binary_path; force=true)
        open(metadata_temp, "w") do io
            JSON.print(io, Dict("version" => version, "sha256" => digest, "repo" => repo))
        end
        mv(metadata_temp, metadata_file; force=true)
    catch err
        isfile(temp) && rm(temp; force=true)
        isfile(metadata_temp) && rm(metadata_temp; force=true)
        err isa OpenSysMLError && rethrow()
        throw(TransportError("Downloaded $(version) but could not install it at $(binary_path): $(sprint(showerror, err))"))
    end
    nothing
end

function file_digest(path)
    open(path, "r") do io
        context = SHA.SHA2_256_CTX()
        while !eof(io)
            SHA.update!(context, read(io, 8192))
        end
        bytes2hex(SHA.digest!(context))
    end
end

function read_metadata(binary_path=get_binary_path())
    try
        value = JSON.parsefile(metadata_path(binary_path))
        value isa AbstractDict ? value : Dict{String,Any}()
    catch
        Dict{String,Any}()
    end
end

function cached_release(; github_repo=nothing, binary_path=get_binary_path())
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    recorded = read_metadata(binary_path)
    version = get(recorded, "version", "")
    digest = get(recorded, "sha256", "")
    isempty(String(version)) && return nothing
    isempty(String(digest)) && return nothing
    get(recorded, "repo", "") == repo || return nothing
    try
        file_digest(binary_path) == digest || return nothing
    catch
        return nothing
    end
    String(version)
end

function stale_cache_reason(version; github_repo=nothing, binary_path=get_binary_path())
    version === nothing && return nothing
    repo = github_repo === nothing ? _default_github_repo() : String(github_repo)
    requested = String(version)
    if requested == "latest"
        try
            requested = resolve_latest_version(; github_repo=repo)
        catch err
            err isa TransportError && return nothing
            rethrow()
        end
    end
    have = cached_release(; github_repo=repo, binary_path=binary_path)
    have == requested && return nothing
    if have === nothing
        recorded_repo = get(read_metadata(binary_path), "repo", "")
        if !isempty(String(recorded_repo)) && recorded_repo != repo
            return "the cached binary was downloaded from $(recorded_repo), but $(requested) of $(repo) was asked for"
        end
        return "the cached binary was not downloaded by this client, so its release is unknown, and $(requested) was asked for"
    end
    "the cached binary is $(have), but $(requested) was asked for"
end

function stable_binary_path(digest; binary_path=get_binary_path())
    root, ext = endswith(binary_path, ".exe") ?
        (binary_path[1:end-4], ".exe") : (binary_path, "")
    "$(root)-$(first(String(digest), 16))$(ext)"
end

function stable_binary(; binary_path=get_binary_path())
    digest = file_digest(binary_path)
    stable = stable_binary_path(digest; binary_path=binary_path)
    _is_executable_file(stable) && return stable
    temp = stable * ".tmp"
    try
        cp(binary_path, temp; force=true)
        chmod(temp, 0o700)
        mv(temp, stable; force=true)
        return stable
    catch
        isfile(temp) && rm(temp; force=true)
        return binary_path
    end
end

function ensure_binary(; force_download=false, version=nothing, github_repo=nothing,
                       base_url=nothing, api_url=nothing, fetcher=_fetch_url,
                       pinned_digests=PINNED_SHA256, binary_path=get_binary_path())
    named = named_binary()
    named !== nothing && return named
    requested = version === nothing ? get(ENV, "OPENSYSML_GRPC_VERSION", "") : String(version)
    isempty(requested) && (requested = nothing)
    return _with_binary_cache_lock(binary_path) do
        cached = nothing
        if !force_download && _is_executable_file(binary_path)
            reason = stale_cache_reason(requested; github_repo=github_repo,
                                        binary_path=binary_path)
            if reason === nothing
                return stable_binary(; binary_path=binary_path)
            end
            cached = binary_path
            @warn "Replacing the cached sysml-grpc: $(reason). Downloading $(requested)."
        end
        if requested === nothing
            force_download && throw(TransportError(
                "A download was asked for without a release to download. Set \$OPENSYSML_GRPC_VERSION, or pass version= here."))
            on_path = binary_on_path()
            on_path !== nothing && return on_path
            throw(TransportError(
                "Binary not found at $(binary_path), named by \$OPENSYSML_BINARY or on \$PATH, and auto-download disabled."))
        end
        try
            download_binary(requested; github_repo=github_repo, base_url=base_url,
                api_url=api_url, fetcher=fetcher, pinned_digests=pinned_digests,
                binary_path=binary_path)
            stable_binary(; binary_path=binary_path)
        catch err
            if cached !== nothing && (err isa UnpinnedReleaseError || err isa TransportError)
                @warn "Keeping the cached sysml-grpc at $(cached): $(requested) was not downloaded ($(sprint(showerror, err)))."
                return cached
            end
            rethrow()
        end
    end
end

function resolve_binary(; version=nothing, github_repo=nothing, kwargs...)
    ensure_binary(; version=version, github_repo=github_repo, kwargs...)
end
