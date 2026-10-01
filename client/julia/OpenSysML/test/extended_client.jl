function recording_service(capabilities; handler=(method, body) -> Dict{String,Any}())
    listener = listen(ip"127.0.0.1", 0)
    port = getsockname(listener)[2]
    close(listener)
    requests = Any[]
    server = HTTP.serve!(ip"127.0.0.1", port) do request
        method = last(split(String(request.target), '/'))
        body = isempty(request.body) ? Dict{String,Any}() :
            JSON.parse(String(request.body))
        push!(requests, (method=method, body=body))
        answer = method == "GetServerInfo" ?
            Dict("version" => "test", "capabilities" => capabilities) :
            handler(method, body)
        HTTP.Response(200, ["Content-Type" => "application/json"], JSON.json(answer))
    end
    server, requests, "127.0.0.1:$(port)"
end

@testset "closed connections reject calls" begin
    server, requests, address = recording_service(String[])
    conn = external(address)
    @test isopen(conn)
    close(conn)
    @test !isopen(conn)
    failure = try
        parse_source(conn, "package Closed;")
        nothing
    catch error
        error
    end
    @test failure isa TransportError
    @test failure.message == "the connection is closed; open a new one"
    @test isempty(requests)
    @test close(conn) === nothing
    close(server)

    if isfile(GRPC_BINARY)
        private_conn = private(binary=GRPC_BINARY)
        close(private_conn)
        @test !isopen(private_conn)
        @test_throws TransportError parse_source(private_conn, "package Closed;")
        @test close(private_conn) === nothing
    end
end

@testset "transport failures hide request bodies and classify timeouts" begin
    listener = listen(ip"127.0.0.1", 0)
    port = getsockname(listener)[2]
    disconnected = @async begin
        socket = accept(listener)
        close(socket)
        close(listener)
    end
    source = "package SecretSource;"
    conn = external("127.0.0.1:$(port)"; timeout=2)
    failure = try
        parse_source(conn, source)
        nothing
    catch error
        error
    finally
        close(conn)
    end
    wait(disconnected)
    @test failure isa TransportError
    @test !occursin(source, failure.message)
    @test ncodeunits(failure.message) < 300

    server, _, address = recording_service(String[];
        handler=(method, body) -> begin
            sleep(2)
            Dict("modelHash" => "slow")
        end)
    slow_conn = external(address; timeout=1)
    timeout = try
        parse_source(slow_conn, "package Slow;")
        nothing
    catch error
        error
    finally
        close(slow_conn)
        close(server)
    end
    @test timeout isa ServiceTimeoutError
    @test occursin("ParseFile failed:", timeout.message)
end

@testset "live IEEE-754 values use protobuf JSON spellings" begin
    if !isfile(GRPC_BINARY)
        @test_skip false
    else
        conn = private(binary=GRPC_BINARY)
        try
            model = parse_source(conn, """
                package Wire {
                    private import ScalarValues::*;
                    action Echo {
                        attribute data;
                        attribute result;
                        first start;
                        action inner { assign result := data; }
                        done;
                        succession first start then inner;
                        succession first inner then done;
                    }
                }
            """)
            for input in (NaN, Inf, -Inf, -0.0)
                output = execute_action(model, "Wire::Echo"; inputs=Dict("data" => input))["result"]
                @test output isa Float64
                if isnan(input)
                    @test isnan(output)
                elseif isinf(input)
                    @test output == input
                else
                    @test output == input && signbit(output)
                end
            end
        finally
            close(conn)
        end
    end
end

@testset "errors and capabilities" begin
    @test ConnectError("not_found", "model not found: abc", 404) isa ModelNotFoundError
    @test ConnectError("not_found", "file not found: a.sysml", 404) isa ModelFileNotFoundError
    @test ConnectError("invalid_argument", "bad request", 400) isa InvalidRequestError
    @test ConnectError("deadline_exceeded", "timed out", 504) isa ServiceTimeoutError
    @test ConnectError("unimplemented", "unknown method", 501) isa UnsupportedOperationError
    @test ConnectError("unavailable", "offline", 503) isa ServiceUnavailableError
    @test SymbolNotFoundError("Demo::Car", ["Demo::car"]).name == "Demo::Car"

    info = ServerInfo("1.2.3", ["query", "verification"], true, "test service")
    version, capabilities = info
    @test version == "1.2.3"
    @test capabilities == Set(["query", "verification"])
    @test "query" in info
    @test occursin("1.2.3", OpenSysML.describe(info))
    @test mismatch_reason(info; version="1.2.4") !== nothing
    @test_throws MissingCapabilityError require_capability(info, "convert")
end

@testset "empty service version requests" begin
    withenv("OPENSYSML_GRPC_VERSION" => "") do
        @test OpenSysML._version_request(nothing) === nothing
        @test OpenSysML._version_request("") === nothing
    end
    withenv("OPENSYSML_GRPC_VERSION" => "v1.2.3") do
        @test OpenSysML._version_request("") == "v1.2.3"
    end
end

@testset "OSLC query capability preflight" begin
    server, requests, address = recording_service(["query"])
    conn = external(address)
    try
        err = try
            query(Model(conn, "hash", Diagnostic[]), "name = 'car'")
            nothing
        catch exception
            exception
        end
        @test err isa MissingCapabilityError
        @test err.capability == CAPABILITY_OSLC_QUERY
        @test [request.method for request in requests] == ["GetServerInfo"]
    finally
        close(conn)
        close(server)
    end
end

@testset "file parsing does not require inline language capability" begin
    server, requests, address = recording_service(String[];
        handler=(method, body) -> Dict("modelHash" => "file-hash"))
    conn = external(address)
    try
        info = server_info(conn)
        @test CAPABILITY_INLINE_LANGUAGE ∉ info
        model = parse_file(conn, "model.sysml"; language="sysml")
        @test model.hash == "file-hash"
        parse_request = only(request for request in requests if request.method == "ParseFile")
        @test parse_request.body["filePath"] == "model.sysml"
        @test parse_request.body["language"] == "sysml"
    finally
        close(conn)
        close(server)
    end
end

@testset "document verdict and event decoding" begin
    verdict = OpenSysML._document_value(JSON.parse(
        """{"verdict":{"assertion":{"elementType":"SysML::RequirementUsage"},
        "kind":"requirement","verdict":"pass"}}"""))
    @test verdict.assertion == ElementRef("", "SysML::RequirementUsage")

    event = OpenSysML._document_value(JSON.parse(
        """{"event":{"kind":"transition","from":"Idle","to":"Running"}}"""))
    @test event.from_state == "Idle"
    @test event.to_state == "Running"
end

@testset "sweep seeds use uint64" begin
    seed = typemax(UInt64)
    server, requests, address = recording_service([
        CAPABILITY_VERIFICATION, CAPABILITY_COMPLEX_VALUES, CAPABILITY_STRUCTURED_VALUES
    ]; handler=(method, body) -> Dict("seed" => string(seed)))
    conn = external(address)
    try
        table = run_sweep(Model(conn, "hash", Diagnostic[]), "Demo::Sweep",
            "x" => (1, 2); seed=seed)
        @test table.seed === seed
        request = only(request for request in requests if request.method == "RunSweep")
        @test request.body["seed"] == string(seed)
    finally
        close(conn)
        close(server)
    end
end

@testset "protected RPCs are preflighted" begin
    listener = listen(ip"127.0.0.1", 0)
    port = getsockname(listener)[2]
    close(listener)
    requests = String[]
    legacy = Ref(false)
    server = HTTP.serve!(ip"127.0.0.1", port) do req
        method = last(split(String(req.target), '/'))
        push!(requests, method)
        if method == "GetServerInfo" && legacy[]
            return HTTP.Response(501, ["Content-Type" => "application/json"],
                JSON.json(Dict("code" => "unimplemented", "message" => "unknown method")))
        end
        body = method == "GetServerInfo" ?
            JSON.json(Dict("version" => "old", "capabilities" => ["query"])) : "{}"
        HTTP.Response(200, ["Content-Type" => "application/json"], body)
    end
    conn = external("127.0.0.1:$(port)")
    try
        info = server_info(conn)
        @test info.answered
        @test info.version == "old"
        @test_throws MissingCapabilityError execute_action(
            Model(conn, "hash", Diagnostic[]), "Demo::action"; schedule="linear")
        @test requests == ["GetServerInfo"]
        @test_throws StaleServiceError external("127.0.0.1:$(port)"; version="new")
        @test_throws MissingCapabilityError external(
            "127.0.0.1:$(port)"; require_capabilities=["verification"])
        legacy[] = true
        older = external("127.0.0.1:$(port)")
        old_info = server_info(older)
        @test !old_info.answered
        @test isempty(old_info.capabilities)
        close(older)
    finally
        close(conn)
        close(server)
    end
end

@testset "value round trips and quantity semantics" begin
    metre = Unit("m"; factors=[UnitFactor("SI::metre", 1)])
    kilometre = Unit("km"; scale_num=1000, factors=[UnitFactor("SI::metre", 1)])
    quantity = Quantity(2, "km", kilometre)
    @test in_unit(quantity, metre) == 2000.0
    @test to_unit(quantity, metre).magnitude == 2000.0
    @test quantity + Quantity(500, "m", metre) == Quantity(2.5, "km", kilometre)
    @test_throws IncommensurableUnitsError quantity + Quantity(1, "s", Unit("s"; factors=[UnitFactor("SI::second", 1)]))
    @test in_unit(Quantity(5, "m"), Unit("m")) == 5
    @test_throws IncommensurableUnitsError in_unit(Quantity(5, "m"), Unit("s"))
    @test_throws IncommensurableUnitsError Quantity(5, "m") + Quantity(1, "s")
    @test_throws IncommensurableUnitsError Quantity(5, "m") < Quantity(1, "s")
    huge = Unit("u"; scale_num=1, factors=[UnitFactor("SI::u", 1)])
    @test Quantity(9007199254740992, "u", huge) != Quantity(9007199254740993, "u", huge)
    @test Quantity(1, "km", kilometre) == Quantity(1000, "m", metre)
    @test hash(Quantity(1, "km", kilometre)) == hash(Quantity(1000, "m", metre))
    @test MeasurementRef("m", "SI::metre", metre) ==
          MeasurementRef("SI::m", "SI::metre", Unit("SI::m"; factors=[UnitFactor("SI::metre", 1)]))
    @test !same_value(MeasurementRef("rad", "SI::rad", Unit("rad"; reduction_given=true)),
                      MeasurementRef("sr", "SI::sr", Unit("sr"; reduction_given=true)))
    @test Set([true]) == Set([1])
    @test same_value(Set([true]), Set([1]))
    @test same_value(Set(), nothing)
    @test same_value(Set([ArrayValue([1], Any[1])]), Set([ArrayValue([1], Any[1])]))
    @test_throws UnsupportedValueError decode_value(Dict("set" => Dict("elements" => [
        Dict("array" => Dict("dimensions" => ["1"], "elements" => [Dict("intValue" => "1")])),
        Dict("array" => Dict("dimensions" => ["1"], "elements" => [Dict("intValue" => "1")])),
    ])))
    @test OpenSysML.nested(ArrayValue([2, 2], Any[1, 2, 3, 4])) == Any[Any[1, 2], Any[3, 4]]

    values = Any[
        Quantity(3, "m", metre),
        EnumLiteral("Demo::Mode::on", "Demo::Mode", "Mode::on", 1),
        FunctionRef("Demo::square", nothing),
        InstanceRef(4),
        Metaobject("Demo::wheel", "SysML::PartUsage"),
        MeasurementRef("m", "SI::metre", metre),
        ArrayValue([2], Any[1, "two"]),
        VectorValue(Union{Int64,Float64}[1, 2.5]),
        VectorQuantity([Quantity(1, "m", metre), Quantity(2, "m", metre)]),
        TensorQuantity([2], [Quantity(1, "m", metre), Quantity(2, "m", metre)]),
        complex(2.0, -1.0),
        Set([1, 2]),
        Infinity(),
    ]
    for value in values
        @test same_value(decode_value(encode_value(value)), value)
    end
    @test same_value(nothing, Any[])
    @test CAPABILITY_STRUCTURED_VALUES in value_capabilities(values[7])
    @test CAPABILITY_MEASUREMENT_REFS in value_capabilities(values[6])
    @test CAPABILITY_FUNCTION_VALUES in value_capabilities(values[3])
    @test CAPABILITY_SET_VALUES in value_capabilities(values[12])
    @test CAPABILITY_TENSOR_VALUES in value_capabilities(values[10])
    @test OpenSysML.unit(values[9]) == metre
    @test collect(OpenSysML.magnitudes(values[9])) == [1, 2]
    @test_throws ArgumentError VectorValue([true])
    @test OpenSysML.decode_values(Dict("outputs" => Dict(
        "intValue" => Dict("intValue" => "7"))))["outputs"]["intValue"] == 7

    bad_feature = OpenSysML._decode_instance(Dict(
        "id" => "1",
        "typeSymbolId" => "Demo::Thing",
        "featureValues" => Dict("items" => Dict("values" => [
            Dict("stringValue" => "supported"),
            Dict("null" => "unsupported: value"),
        ])),
    ))
    @test bad_feature.feature_values["items"] isa FeatureValueError
    @test bad_feature.feature_values["items"].message == "unsupported: value"
    @test_throws FeatureValueError bad_feature.items

    coordinate_error = OpenSysML._decode_instance(Dict(
        "id" => "1",
        "typeSymbolId" => "Demo::Context",
        "featureValues" => Dict("accelarationCF" => Dict("value" =>
            Dict("null" => "unsupported: coordinate frame accelarationCF [m/s**2, m/s**2, m/s**2]"))),
    )).feature_values["accelarationCF"]
    @test coordinate_error isa FeatureValueError
    @test coordinate_error.message ==
          "unsupported: coordinate frame accelarationCF [m/s**2, m/s**2, m/s**2]"
end

@testset "query and document binding builders" begin
    query = build_query(scope=["Demo::Vehicle"], select=["name"],
        var"where"=Dict("property" => "name", "operator" => "=", "value" => "car"))
    @test query["scope"] == ["Demo::Vehicle"]
    @test query["select"] == ["name"]
    @test query["where"]["primitive"]["operator"] == "PRIMITIVE_OPERATOR_EQUAL"
    @test_throws QueryError build_query(scope=["Demo::Vehicle"], select=["name"],
        var"where"=Dict("property" => "name", "operator" => "!=", "value" => "car"))

    cases = [
        (() -> build_query(; payload=Dict("@type" => "Query"), scope=["Demo::Vehicle"]),
         "pass a query payload or scope/select/where keywords, not both"),
        (() -> build_query(["not a mapping"]), "a query is an object, not list"),
        (() -> build_query(Dict("@type" => "Other")),
         "expected a 'Query' payload, got 'Other'"),
        (() -> build_query(Dict("@type" => "Query", "unexpected" => true)),
         "a query has no unexpected; the standard's query is scope, select and where"),
        (() -> build_query(scope=1), "scope is a list, not int"),
        (() -> build_query(scope=[1]),
         "a scope entry is an element's qualified name or a {'@id': ...} reference, not 1"),
        (() -> build_query(select=[1]), "a selected property is a name, not 1"),
        (() -> build_query(var"where"=1), "a constraint is an object, not int"),
        (() -> build_query(var"where"=Dict("@type" => "Other")),
         "unknown constraint type 'Other'; the standard's constraints are PrimitiveConstraint and CompositeConstraint"),
        (() -> build_query(var"where"=Dict("@type" => "PrimitiveConstraint", "extra" => true)),
         "a PrimitiveConstraint has no extra"),
        (() -> build_query(var"where"=Dict("operator" => "!=", "property" => "x")),
         "unknown primitive operator '!='; expected one of <, =, >"),
        (() -> build_query(var"where"=Dict("operator" => "=", "property" => "")),
         "a primitive constraint names one property, not ''"),
        (() -> build_query(var"where"=Dict("operator" => "=", "property" => "x", "value" => Dict("x" => 1))),
         "cannot compare against {'x': 1}"),
        (() -> build_query(var"where"=Dict("operator" => "=", "property" => "x", "value" => Set([1]))),
         "cannot compare against {1}"),
        (() -> build_query(var"where"=Dict("@type" => "CompositeConstraint", "extra" => true)),
         "a CompositeConstraint has no extra"),
        (() -> build_query(var"where"=Dict("@type" => "CompositeConstraint", "operator" => "xor")),
         "unknown composite operator 'xor'; expected one of and, or"),
        (() -> build_query(var"where"=Dict("@type" => "CompositeConstraint", "operator" => "and", "constraint" => [])),
         "a composite constraint combines a non-empty list of constraints, not []"),
    ]
    for (build, expected) in cases
        failure = try
            build()
            nothing
        catch error
            error
        end
        @test failure isa QueryError
        @test failure.message == expected
    end

    bindings = build_document_bindings(Dict(
        "root" => ElementRef("Demo::car"),
        "object" => ObjectRef(0, "car"),
        "values" => [1, true, "x"],
    ))
    binding_by_parameter = Dict(binding["parameter"] => binding for binding in bindings)
    @test binding_by_parameter["root"]["values"][1] == Dict("elementId" => "Demo::car")
    @test binding_by_parameter["object"]["values"][1]["object"]["path"] == "car"
    @test binding_by_parameter["values"]["values"] == [
        Dict("intValue" => "1"), Dict("boolValue" => true), Dict("stringValue" => "x")]
    @test_throws DocumentQueryError build_document_bindings(Dict("empty" => ObjectRef()))
    @test format_of_path("model.ttl") == "ttl"
    @test_throws ArgumentError format_of_path("model.unknown")
end

@testset "live RPC wrappers" begin
    if !isfile(GRPC_BINARY)
        @test_skip false
    else
        conn = private(binary=GRPC_BINARY)
        try
            @testset "model, symbols, and source parsing" begin
                simple = parse_file(conn, joinpath(FIXTURES, "simple_part.sysml"))
                @test !isempty(simple.hash)
                @test !isempty(simple.documents)
                raw_symbol = symbol(simple, "Test::SimplePart")
                @test raw_symbol isa AbstractDict
                typed_symbol = get_symbol(simple, "Test::SimplePart")
                @test typed_symbol isa SymbolInfo
                @test typed_symbol.id == "Test::SimplePart"
                instance = instantiate(simple, "Test::SimplePart")
                @test instance.type_symbol_id == "Test::SimplePart"
                @test instance.graph[instance.id] === instance

                coordinate_path = joinpath(@__DIR__, "..", "..", "..", "..", "examples",
                    "pilot-corpora", "sysml-examples", "Vehicle Example",
                    "SysML v2 Spec Annex A SimpleVehicleModel.sysml")
                coordinate_model = parse_file(conn, coordinate_path)
                coordinate_instance = instantiate(coordinate_model,
                    "SimpleVehicleModel::Definitions::GenericContext::Context")
                coordinate_value = features(coordinate_instance)["accelarationCF"]
                @test coordinate_value isa FeatureValueError
                @test coordinate_value.message ==
                      "unsupported: coordinate frame accelarationCF [m/s**2, m/s**2, m/s**2]"

                sources = parse_sources(conn, [
                    SourceDocument("a.sysml", "package A {}"),
                    SourceDocument("b.sysml", "package B {}"),
                ])
                @test sources.documents == ["a.sysml", "b.sysml"]
                @test isok(sources)
            end

            @testset "verification, calculation, analysis, and sweep" begin
                verification = parse_file(conn, joinpath(FIXTURES, "verification.sysml"))
                @test verify_constraint(verification, "Demo::Vehicle::massPositive").holds
                @test verify_requirement(verification, "Demo::Vehicle::lightEnough";
                    subject="Demo::sedan").holds
                @test !isempty(verify_satisfaction(verification))
                @test !isempty(verify_satisfaction(verification; symbol="Demo::analysis"))
                @test !satisfied(verification; symbol="Demo::analysis")
                validation = validate_instance(verification, "Demo::sedan")
                @test !valid(validation)
                @test validation.summary.kind == "object"
                @test !validation.summary.holds
                @test evaluate(verification, "mass"; subject="Demo::sedan") == 1200.0
                @test calc(verification, "Demo::add"; arguments=[2, 3]).value == 5

                analysis = parse_file(conn, joinpath(FIXTURES, "analysis.sysml"))
                result = run_analysis(analysis, "An::plain")
                @test result.outputs["x"] == 3.0
                @test explore_analysis(analysis, "An::plain").complete

                sweep_model = parse_file(conn, joinpath(FIXTURES, "sweep.sysml"))
                table = run_sweep(sweep_model, "Sw::Ratio", Dict("b" => (1, 2));
                    named_arguments=Dict("a" => 4.0))
                @test length(table) == 2
                @test [row.outputs["result"] for row in table] == [4.0, 2.0]
                @test isempty(failures(table))
                @test Bool(table)
                @test !isempty(list_engines(conn))
            end

            @testset "query, documents, and conversion" begin
                query_model = parse_file(conn, joinpath(FIXTURES, "simple_part.sysml"))
                elements = query(query_model; scope=["Test::SimplePart"], select=["name"])
                @test !isempty(elements)
                err = try
                    query_model["SimplePartt"]
                    nothing
                catch error
                    error
                end
                @test err isa SymbolNotFoundError
                @test "SimplePart" in err.suggestions

                document_model = parse_file(conn, joinpath(FIXTURES, "document.sysml"))
                rows = run_document_query(document_model, "Observatory::SubsystemTable";
                    bindings=Dict("root" => ElementRef("Observatory::telescope")))
                @test rows.columns == ["name", "mass"]
                @test length(rows) == 4
                @test !isempty(render_document(document_model, "Observatory::MassReport"))
                @test !isempty(render_document(document_model, "Observatory::MassReport"; form="html"))
                @test !isempty(convert_model(document_model, "sysml").content)
                @test !isempty(to_turtle(document_model).content)
                @test !isempty(to_api_json(document_model).content)
                @test !isempty(convert_file(conn, joinpath(FIXTURES, "simple_part.sysml"), "sysml").content)
                @test !isempty(convert_source(conn, "package Inline {}", "sysml";
                    from_format="sysml").content)
                mktempdir() do directory
                    output = joinpath(directory, "roundtrip.sysml")
                    @test !isempty(save(document_model, output).content)
                    @test isfile(output)
                end
            end

            @testset "exploration and performer" begin
                behavior = parse_file(conn, joinpath(FIXTURES, "behavior.sysml"))
                exploration = explore_state(behavior, "Test::Machine")
                @test exploration.complete
                @test length(exploration) == 1
                @test explore_action(behavior, "Test::addFive";
                    inputs=Dict("result" => 10)).complete

                performer_source = """
                package Wire {
                    private import ScalarValues::*;
                    item def Ping;
                    port def Link { in item ping : Ping; }
                    part def Ground {
                        port p : ~Link;
                        exhibit state hail { entry; then go; state go { entry send new Ping() via p; } }
                    }
                    part def Craft {
                        port p : Link;
                        attribute pinged : Boolean = false;
                        exhibit state modes {
                            entry; then waiting;
                            state waiting;
                            transition first waiting accept Ping via p then active;
                            state active { entry assign pinged := true; }
                        }
                        action look { out seen : Boolean; first start; then action read assign seen := pinged; then done; }
                    }
                    part def Pair {
                        part ground : Ground;
                        part craft : Craft;
                        connect craft.p to ground.p;
                    }
                    part pair : Pair;
                }
                """
                performer_model = parse_source(conn, performer_source; name="performer.sysml")
                run = execute_action(performer_model, "Wire::Craft::look";
                    performer="Wire::pair.craft")
                @test run["seen"] === true
                @test run.performer["this.pinged"] === true
                state_run = execute_state(performer_model, "Wire::Craft::modes";
                    performer="Wire::pair.craft")
                @test state_run.states_visited[end] == "active"
                @test state_run.final_context["this.pinged"] === true
                @test state_run.final_time isa Float64
                explored = explore_state(performer_model, "Wire::Craft::modes";
                    performer="Wire::pair.craft")
                @test explored.complete
                @test only(explored).final_state == "active"
            end
        finally
            close(conn)
        end
    end
end

@testset "capability selection" begin
    @test OpenSysML._engine_capabilities(nothing) == ()
    @test OpenSysML._engine_capabilities("auto") == ()
    @test OpenSysML._engine_capabilities("solver") ==
          (OpenSysML.CAPABILITY_ENGINES,)
    @test OpenSysML._engine_capabilities("explore") ==
          (OpenSysML.CAPABILITY_ENGINES, OpenSysML.CAPABILITY_SCHEDULE_EXPLORE)
    @test OpenSysML._question_capabilities("evaluate") == ()
    @test OpenSysML._question_capabilities("why") ==
          (OpenSysML.CAPABILITY_VERIFICATION_QUESTIONS,)
    @test OpenSysML._schedule_capabilities(nothing) == ()
    @test OpenSysML._schedule_capabilities("default") ==
          (OpenSysML.CAPABILITY_SCHEDULE,)
    @test OpenSysML._schedule_capabilities("explore:runs=2") ==
          (OpenSysML.CAPABILITY_SCHEDULE, OpenSysML.CAPABILITY_SCHEDULE_EXPLORE)
end

@testset "ordered sweep range inputs" begin
    normalize = OpenSysML._ordered_sweep_ranges
    one = "x" => (1, 3)
    ordered = ["second" => (1, 2), "first" => (4, 5)]
    @test normalize(one) == [one]
    @test normalize(ordered) == ordered
    @test normalize(Dict("x" => (1, 3))) == [one]
    error = try
        normalize(Dict("x" => (1, 3), "y" => (2, 4)))
        nothing
    catch exception
        exception
    end
    @test error isa ArgumentError
    @test occursin("vector of pairs", sprint(showerror, error))
end
