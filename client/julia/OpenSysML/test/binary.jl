using SHA

@testset "binary release downloads" begin
    repo = "example/OpenSysML"
    version = "v1.2.3"
    asset = OpenSysML.release_asset_name()
    bytes = Vector{UInt8}(codeunits("sysml-grpc binary"))
    digest = bytes2hex(sha256(bytes))
    checksum = Vector{UInt8}(codeunits("$(digest)  $(asset)\n"))
    base_url = "https://release.invalid/download"

    mktempdir() do directory
        binary_path = joinpath(directory, "sysml-grpc")
        fetcher = (url, limit) -> begin
            endswith(url, ".sha256") ? checksum : bytes
        end
        pins = Dict(repo => Dict(version => Dict(asset => digest)))
        installed = download_binary(version; github_repo=repo, base_url=base_url,
            fetcher=fetcher, pinned_digests=pins, binary_path=binary_path)
        @test installed == binary_path
        @test read(binary_path) == bytes
        @test (stat(binary_path).mode & 0o111) != 0
        @test !isfile(binary_path * ".tmp")
        @test !isfile(binary_path * ".json.tmp")
        @test OpenSysML.cached_release(; github_repo=repo, binary_path=binary_path) == version
        @test OpenSysML.cached_release(; github_repo="elsewhere/repo", binary_path=binary_path) === nothing
        @test OpenSysML.read_metadata(binary_path)["sha256"] == digest

        mismatch_path = joinpath(directory, "mismatch")
        bad_bytes = Vector{UInt8}(codeunits("tampered binary"))
        bad_fetcher = (url, limit) -> endswith(url, ".sha256") ? checksum : bad_bytes
        @test_throws ChecksumMismatchError download_binary(version;
            github_repo=repo, base_url=base_url, fetcher=bad_fetcher,
            pinned_digests=pins, binary_path=mismatch_path)
        @test !isfile(mismatch_path)

        sidecar_mismatch = (url, limit) ->
            endswith(url, ".sha256") ? Vector{UInt8}(codeunits(repeat("0", 64))) : bytes
        @test_throws ChecksumMismatchError download_binary(version;
            github_repo=repo, base_url=base_url, fetcher=sidecar_mismatch,
            pinned_digests=pins, binary_path=joinpath(directory, "sidecar-mismatch"))

        unpinned = (url, limit) -> endswith(url, ".sha256") ? checksum : bytes
        withenv(OpenSysML.ALLOW_UNPINNED_ENV => "0") do
            @test_throws UnpinnedReleaseError download_binary(version;
                github_repo=repo, base_url=base_url, fetcher=unpinned,
                pinned_digests=Dict(), binary_path=joinpath(directory, "refused"))
        end
        withenv(OpenSysML.ALLOW_UNPINNED_ENV => "1") do
            @test_logs (:warn, r"No digest is pinned") download_binary(version;
                github_repo=repo, base_url=base_url, fetcher=unpinned,
                pinned_digests=Dict(), binary_path=joinpath(directory, "allowed"))
            @test read(joinpath(directory, "allowed")) == bytes
        end

        latest_fetcher = (url, limit) ->
            occursin("/releases/latest", url) ?
                Vector{UInt8}(codeunits(JSON.json(Dict("tag_name" => version)))) :
                endswith(url, ".sha256") ? checksum : bytes
        latest_path = joinpath(directory, "latest")
        @test download_binary("latest"; github_repo=repo, base_url=base_url,
            api_url="https://api.invalid/releases/latest", fetcher=latest_fetcher,
            pinned_digests=pins, binary_path=latest_path) == latest_path
        @test OpenSysML.cached_release(; github_repo=repo, binary_path=latest_path) == version
    end

    @test_throws TransportError OpenSysML._fetch("fixture", 3;
        fetcher=(url, limit) -> UInt8[1, 2, 3, 4])
end

@testset "binary resolution" begin
    mktempdir() do directory
        first_binary = joinpath(directory, "first")
        second_binary = joinpath(directory, "second")
        write(first_binary, "binary")
        write(second_binary, "binary")
        chmod(first_binary, 0o700)
        chmod(second_binary, 0o700)
        withenv("OPENSYSML_BINARY" => first_binary,
                "OPENSYSML_GRPC_BINARY" => second_binary) do
            @test OpenSysML.named_binary() == first_binary
        end
        withenv("OPENSYSML_BINARY" => joinpath(directory, "missing"),
                "OPENSYSML_GRPC_BINARY" => second_binary) do
            @test_throws TransportError OpenSysML.named_binary()
        end
        path_binary = joinpath(directory, OpenSysML.service_binary_name())
        write(path_binary, "binary")
        chmod(path_binary, 0o700)
        withenv("OPENSYSML_BINARY" => "", "OPENSYSML_GRPC_BINARY" => "",
                "PATH" => directory) do
            @test OpenSysML.binary_on_path() == path_binary
            @test ensure_binary(binary_path=joinpath(directory, "empty-cache")) == path_binary
        end

        cache = joinpath(directory, "cached-service")
        write(cache, "cached build")
        chmod(cache, 0o700)
        withenv("OPENSYSML_BINARY" => "", "OPENSYSML_GRPC_BINARY" => "",
                "OPENSYSML_GRPC_VERSION" => "", "PATH" => "") do
            resolved = ensure_binary(binary_path=cache)
            @test resolved == OpenSysML.stable_binary_path(
                bytes2hex(sha256(read(cache))); binary_path=cache)
            @test read(resolved) == read(cache)
            @test_throws TransportError ensure_binary(force_download=true,
                binary_path=joinpath(directory, "not-installed"))
        end
    end
end
