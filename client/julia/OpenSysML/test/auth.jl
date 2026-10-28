# A service other machines reach admits only the bearer token it was given.
function guarded_service(token)
    listener = listen(ip"127.0.0.1", 0)
    port = getsockname(listener)[2]
    close(listener)
    server = HTTP.serve!(ip"127.0.0.1", port) do request
        if HTTP.header(request, "Authorization") != "Bearer $(token)"
            return HTTP.Response(401, ["Content-Type" => "application/json",
                                       "WWW-Authenticate" => "Bearer realm=\"sysml-grpc\""],
                                 JSON.json(Dict("code" => "unauthenticated", "message" => "a bearer token is required")))
        end
        HTTP.Response(200, ["Content-Type" => "application/json"],
                      JSON.json(Dict("version" => "test", "capabilities" => ["query"])))
    end
    server, "127.0.0.1:$(port)"
end

@testset "a guarded service admits the right token and refuses the rest" begin
    token = "julia-token_1"
    server, address = guarded_service(token)
    try
        refused(conn) = try
            server_info(conn)
            false
        catch e
            e isa ConnectError && occursin("unauthenticated", sprint(showerror, e))
        end
        withenv(TOKEN_ENV => nothing) do
            @test refused(external(address))
            @test refused(external(address; token="nope"))
            @test external(address; token=token).token == token
            @test server_info(external(address; token=token)).version == "test"
            @test server_info(external(address; token=token, require_capabilities=["query"])).version == "test"
            @test external(address; token="").token === nothing
        end
        withenv(TOKEN_ENV => token) do
            @test external(address).token == token
            @test server_info(external(address)).version == "test"
            @test external(address; token="nope").token == "nope"
        end
        withenv(TOKEN_ENV => nothing, "OPENSYSML_SERVICE" => address) do
            @test server_info(OpenSysML.connect(; token=token)).version == "test"
            @test refused(OpenSysML.connect())
        end
    finally
        close(server)
    end
end
