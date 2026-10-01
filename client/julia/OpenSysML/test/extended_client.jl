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

@testset "migration is told from conversion before anything is sent" begin
    @test is_v1("xmi") && is_v1("UML") && is_v1(" mdzip ")
    @test !is_v1("sysml") && !is_v1("") && !is_v1("xmi2")
    @test path_is_v1("Model.mdzip") && path_is_v1("dir/Model.XMI") && path_is_v1("Model.uml")
    @test !path_is_v1("Model.sysml") && !path_is_v1("xmi")
    server, requests, address = recording_service(["convert", "migrate"])
    conn = external(address)
    try
        for refused in (() -> convert_file(conn, "Model.mdzip", "sysml"),
                        () -> convert_source(conn, "<xmi/>", "sysml"; from_format="XMI"),
                        () -> migrate_file(conn, "Model.sysml", "sysml"; from_format="sysml"),
                        () -> migrate_source(conn, "package P;", "sysml"; from_format="sysml"),
                        () -> migrate_source(conn, "<xmi/>", "sysml"; from_format=""),
                        () -> migrate_file(conn, "Model.mdzip", "sysml"; layout_path="a.xml",
                                           layout_content="<mtip/>"))
            @test_throws ArgumentError refused()
        end
        err = try
            convert_file(conn, "Model.mdzip", "sysml")
        catch exception
            exception
        end
        @test occursin(MIGRATED_NOT_CONVERTED, err.msg)
        @test occursin("migrate_file", err.msg)
        @test isempty(requests)
    finally
        close(conn)
        close(server)
    end
end

@testset "migrate sends the Migrate request and reads the answer" begin
    answer = Dict{String,Any}("content" => "package Vehicle;", "fromFormat" => "xmi",
        "toFormat" => "sysml", "experimental" => true,
        "report" => Dict("source" => "Vehicle.xmi", "exporter" => "Cameo", "summary" => "s",
            "mapped" => 2, "approximated" => 1, "unmapped" => 0, "skipped" => 1,
            "entries" => [Dict("id" => "a", "kind" => "Class", "name" => "A", "target" => "part def A",
                               "verdict" => "mapped", "note" => ""),
                          Dict("id" => "d", "kind" => "Diagram", "name" => "D", "target" => "",
                               "verdict" => "skipped", "note" => "diagram")],
            "text" => "report"),
        "results" => "{}",
        "files" => [Dict("path" => "images/a.png", "content" => base64encode(UInt8[0x89, 0x50]))])
    server, requests, address = recording_service(["migrate"]; handler=(method, body) -> answer)
    conn = external(address)
    try
        migrated = @test_logs (:warn, MIGRATION_NOTICE) migrate_source(conn, UInt8[0x3c, 0x78],
            "sysml"; from_format="xmi", report=true, results=true, layout_content="<mtip/>",
            image_base_url="https://img.example/", strict=true)
        request = requests[end]
        @test request.method == "Migrate"
        @test request.body["content"] == base64encode(UInt8[0x3c, 0x78])
        @test request.body["fromFormat"] == "xmi" && request.body["toFormat"] == "sysml"
        @test request.body["report"] && request.body["results"] && request.body["strict"]
        @test request.body["layoutContent"] == "<mtip/>" && !haskey(request.body, "layoutPath")
        @test request.body["imageBaseUrl"] == "https://img.example/"
        @test migrated.content == "package Vehicle;"
        @test migrated.experimental && migrated.experimental_notice == MIGRATION_NOTICE
        @test migrated.source_path === nothing
        @test migrated.report.mapped == 2 && migrated.report.skipped == 1
        @test length(migrated.report.entries) == 2
        @test [e.id for e in by_verdict(migrated.report, VERDICT_SKIPPED)] == ["d"]
        @test migrated.results == "{}"
        @test migrated.files["images/a.png"] == UInt8[0x89, 0x50]
        @test occursin("2 mapped", sprint(show, migrated))

        answer["error"] = "cannot read the archive"
        @test_throws MigrationError migrate_file(conn, "Model.mdzip", "sysml")
        @test requests[end].body["filePath"] == "Model.mdzip"
        delete!(answer, "error")

        mktempdir() do directory
            path = joinpath(directory, "Vehicle.sysml")
            @test save(migrated, path) === migrated
            @test read(path, String) == "package Vehicle;"
            @test read(joinpath(directory, "images", "a.png")) == UInt8[0x89, 0x50]
            for escaping in ("../escaped.png", "images/../../escaped.png", "/tmp/escaped.png",
                             "images//x.png", "images\\x.png", "Other.sysml")
                bad = Migration(migrated.content, "xmi", "sysml", migrated.report, "",
                    Dict(escaping => UInt8[1]), nothing, true, MIGRATION_NOTICE)
                other = joinpath(directory, "Other.sysml")
                @test_throws ArgumentError save(bad, other)
                @test !isfile(other)
            end
            @test !isfile(joinpath(dirname(directory), "escaped.png"))
            source = joinpath(directory, "Vehicle.xmi")
            write(source, "<xmi/>")
            from_source = Migration(migrated.content, "xmi", "sysml", migrated.report, "",
                Dict{String,Vector{UInt8}}(), source, true, MIGRATION_NOTICE)
            @test_throws ArgumentError save(from_source, source)
            @test read(source, String) == "<xmi/>"
            if Sys.isunix()
                outside = mktempdir()
                symlink(outside, joinpath(directory, "linked"))
                linked = Migration(migrated.content, "xmi", "sysml", migrated.report, "",
                    Dict("linked/escaped.png" => UInt8[1]), nothing, true, MIGRATION_NOTICE)
                @test_throws ArgumentError save(linked, joinpath(directory, "Linked.sysml"))
                @test isempty(readdir(outside))
                mkdir(joinpath(directory, "alias"))
                symlink(joinpath(directory, "alias", "real.png"),
                        joinpath(directory, "alias", "alias.png"))
                aliased = Migration(migrated.content, "xmi", "sysml", migrated.report, "",
                    Dict("alias/alias.png" => UInt8[1]), nothing, true, MIGRATION_NOTICE)
                @test_throws ArgumentError save(aliased, joinpath(directory, "Aliased.sysml"))
                @test !isfile(joinpath(directory, "alias", "real.png"))
                @test !isfile(joinpath(directory, "Aliased.sysml"))
            end
        end
    finally
        close(conn)
        close(server)
    end
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
    @test_throws ErrorException decode_value(Dict("set" => Dict("elements" => [
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
    @test_throws FeatureValueError bad_feature.items
end

@testset "query and document binding builders" begin
    query = build_query(scope=["Demo::Vehicle"], select=["name"],
        var"where"=Dict("property" => "name", "operator" => "=", "value" => "car"))
    @test query["scope"] == ["Demo::Vehicle"]
    @test query["select"] == ["name"]
    @test query["where"]["primitive"]["operator"] == "PRIMITIVE_OPERATOR_EQUAL"
    @test_throws QueryError build_query(scope=["Demo::Vehicle"], select=["name"],
        var"where"=Dict("property" => "name", "operator" => "!=", "value" => "car"))

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
                vehicle = joinpath(FIXTURES, "vehicle.xmi")
                @test_throws ArgumentError convert_file(conn, vehicle, "sysml")
                migrated = migrate_file(conn, vehicle, "sysml"; report=true)
                @test occursin("part def Vehicle", migrated.content)
                @test migrated.from_format == "xmi"
                @test (migrated.report.mapped, migrated.report.approximated,
                       migrated.report.unmapped, migrated.report.skipped) == (77, 13, 3, 2)
                @test length(migrated.report.entries) == 95
                @test length(by_verdict(migrated.report, VERDICT_UNMAPPED)) == 3
                @test migrated.source_path == abspath(vehicle)
                inline = migrate_source(conn, read(vehicle), "ttl"; from_format=" XMI ")
                @test inline.to_format == "ttl" && inline.report.mapped == 77
                @test isempty(inline.report.entries) && inline.source_path === nothing
                mktempdir() do directory
                    output = joinpath(directory, "Vehicle.sysml")
                    save(migrated, output)
                    @test read(output, String) == migrated.content
                end
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
                @test run["outputs"]["seen"] === true
                state_run = execute_state(performer_model, "Wire::Craft::modes";
                    performer="Wire::pair.craft")
                @test state_run["statesVisited"][end] == "active"
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
