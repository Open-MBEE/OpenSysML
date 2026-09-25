using Test
using JSON
using OpenSysML

include(joinpath(@__DIR__, "..", "conformance", "compare.jl"))

const FIXTURES = normpath(joinpath(@__DIR__, "..", "..", "..", "..", "conformance", "fixtures"))

@testset "decode_value: the nineteen arms" begin
    @test decode_value(nothing) === missing
    @test decode_value(JSON.parse("""{"intValue":"9007199254740993"}""")) === Int64(9007199254740993)
    @test decode_value(JSON.parse("""{"intValue":"-9223372036854775808"}""")) === typemin(Int64)
    @test decode_value(JSON.parse("""{"realValue":0.3333333333333333}""")) ≈ 1/3
    @test decode_value(JSON.parse("""{"realValue":20}""")) === 20.0
    @test decode_value(JSON.parse("""{"realValue":"NaN"}""")) |> isnan
    @test decode_value(JSON.parse("""{"realValue":"Infinity"}""")) === Inf
    @test decode_value(JSON.parse("""{"realValue":"-Infinity"}""")) === -Inf
    @test decode_value(JSON.parse("""{"boolValue":true}""")) === true
    @test decode_value(JSON.parse("""{"stringValue":"abc"}""")) == "abc"
    @test decode_value(JSON.parse("""{"instanceId":"2"}""")) == InstanceRef(2)
    @test decode_value(JSON.parse("""{"sequence":{"elements":[{"stringValue":"nav"},{"intValue":"1"}]}}""")) == Any["nav", 1]
    @test decode_value(JSON.parse("""{"sequence":{}}""")) == Any[]
    @test decode_value(JSON.parse("""{"null":""}""")) === nothing
    @test_throws ErrorException decode_value(JSON.parse("""{"null":"unsupported: function F::f closing over a body's bindings"}"""))
    @test decode_value(JSON.parse("""{"unset":true}""")) isa Unset
    q = decode_value(JSON.parse("""{"quantity":{"realMagnitude":5.4,"unit":"SI::km/SI::h","unitTerm":{"scaleNum":5,"scaleDen":18,"factors":[{"unitId":"SI::metre","exponent":1}]}}}"""))
    @test q isa Quantity && q.magnitude === 5.4 && q.unit == "SI::km/SI::h" && q.unit_term["scaleNum"] == 5
    qi = decode_value(JSON.parse("""{"quantity":{"intMagnitude":"7","unit":"kg"}}"""))
    @test qi.magnitude === Int64(7)
    l = decode_value(JSON.parse("""{"enumLiteral":{"literalId":"Rover::Mode::idle","enumerationId":"Rover::Mode","name":"Mode::idle"}}"""))
    @test l == EnumLiteral("Rover::Mode::idle", "Rover::Mode", "Mode::idle", nothing)
    lv = decode_value(JSON.parse("""{"enumLiteral":{"literalId":"D::Level::high","enumerationId":"D::Level","name":"Level::high","value":{"intValue":"3"}}}"""))
    @test lv.value === Int64(3)
    @test decode_value(JSON.parse("""{"complex":{"real":1.5,"imaginary":-2}}""")) == complex(1.5, -2.0)
    @test decode_value(JSON.parse("""{"complex":{"imaginary":2}}""")) == complex(0.0, 2.0)
    a = decode_value(JSON.parse("""{"array":{"dimensions":["2","3"],"elements":[{"intValue":"1"},{"intValue":"2"},{"intValue":"3"},{"intValue":"4"},{"intValue":"5"},{"intValue":"6"}]}}"""))
    @test a.dimensions == [2, 3] && length(a.elements) == 6
    @test_throws ErrorException decode_value(JSON.parse("""{"array":{"dimensions":["2","3"],"elements":[{"intValue":"1"}]}}"""))
    v = decode_value(JSON.parse("""{"vector":{"components":[{"realValue":3},{"intValue":"4"}]}}"""))
    @test v.components == [3.0, 4]
    @test_throws ErrorException decode_value(JSON.parse("""{"vector":{"components":[{"stringValue":"x"}]}}"""))
    vq = decode_value(JSON.parse("""{"vectorQuantity":{"components":[{"realMagnitude":3,"unit":"m"},{"realMagnitude":4,"unit":"m"}]}}"""))
    @test length(vq.components) == 2 && vq.components[1] isa Quantity
    @test_throws ErrorException decode_value(JSON.parse("""{"vectorQuantity":{"components":[]}}"""))
    m = decode_value(JSON.parse("""{"measurementRef":{"unit":"m","unitTerm":{"scaleNum":1,"scaleDen":1},"unitId":"SI::metre"}}"""))
    @test m.unit == "m" && m.unit_id == "SI::metre"
    mc = decode_value(JSON.parse("""{"measurementRef":{"unit":"m/s","unitTerm":{"scaleNum":1,"scaleDen":1}}}"""))
    @test mc.unit_id === nothing
    @test_throws ErrorException decode_value(JSON.parse("""{"measurementRef":{"unit":"m"}}"""))
    @test_throws ErrorException decode_value(JSON.parse("""{"measurementRef":{}}"""))
    @test decode_value(JSON.parse("""{"infinity":true}""")) isa Infinity
    @test_throws ErrorException decode_value(JSON.parse("""{"infinity":false}"""))
    f = decode_value(JSON.parse("""{"function":{"calcId":"F::Sq"}}"""))
    @test f == FunctionRef("F::Sq", nothing)
    fs = decode_value(JSON.parse("""{"function":{"calcId":"F::Scaler::scale","selfId":"1"}}"""))
    @test fs.self == InstanceRef(1)
    @test_throws ErrorException decode_value(JSON.parse("""{"function":{"calcId":""}}"""))
    s = decode_value(JSON.parse("""{"set":{"elements":[{"intValue":"1"},{"intValue":"2"}]}}"""))
    @test s == Set([1, 2])
    @test_throws ErrorException decode_value(JSON.parse("""{"set":{"elements":[{"intValue":"1"},{"intValue":"1"}]}}"""))
    @test decode_value(JSON.parse("""{"set":{}}""")) == Set()
    t = decode_value(JSON.parse("""{"tensorQuantity":{"dimensions":["2","2"],"components":[{"realMagnitude":1,"unit":"m"},{"realMagnitude":2,"unit":"m"},{"realMagnitude":3,"unit":"m"},{"realMagnitude":4,"unit":"m"}]}}"""))
    @test t.dimensions == [2, 2] && length(t.components) == 4
    @test_throws ErrorException decode_value(JSON.parse("""{"tensorQuantity":{"dimensions":["2","2"],"components":[{"realMagnitude":1,"unit":"m"}]}}"""))
    mo = decode_value(JSON.parse("""{"metaobject":{"elementId":"Meta::seatBelt","metaclassId":"SysML::Systems::PartUsage"}}"""))
    @test mo == Metaobject("Meta::seatBelt", "SysML::Systems::PartUsage")
    @test_throws ErrorException decode_value(JSON.parse("""{"metaobject":{"metaclassId":"X"}}"""))
    u = decode_value(JSON.parse("""{"undetermined":{"reason":"P::Q::d has no value in the model","count":{"lower":"1","upper":"*"}}}"""))
    @test u == Undetermined("P::Q::d has no value in the model", "1", "*")
    @test_throws ErrorException decode_value(JSON.parse("""{"fromTheFuture":1}"""))
end

@testset "encode_value" begin
    @test encode_value(Int64(4)) == Dict("intValue" => "4")
    @test encode_value(5.4) == Dict("realValue" => 5.4)
    @test encode_value(true) == Dict("boolValue" => true)
    @test encode_value("x") == Dict("stringValue" => "x")
    @test encode_value(nothing) == Dict("null" => "")
    @test encode_value(complex(1.5, -2.0)) == Dict("complex" => Dict("real" => 1.5, "imaginary" => -2.0))
    @test encode_value(Any[1, "a"]) == Dict("sequence" => Dict("elements" => Any[Dict("intValue" => "1"), Dict("stringValue" => "a")]))
    @test encode_value(Set([1, 2]))["set"]["elements"] |> length == 2
    @test encode_value(Quantity(Int64(3), "kg", nothing)) == Dict("quantity" => Dict("intMagnitude" => "3", "unit" => "kg"))
    @test encode_value(InstanceRef(7)) == Dict("instanceId" => "7")
    @test encode_value(FunctionRef("F::Sq", nothing)) == Dict("function" => Dict("calcId" => "F::Sq"))
    @test_throws ErrorException encode_value(FunctionRef("F::Sq", InstanceRef(1)))
    @test_throws ErrorException encode_value(Undetermined("r", "1", "1"))
    @test_throws ErrorException encode_value(Unset())
    @test encode_value(EnumLiteral("E::a", "E", "a"))["enumLiteral"]["literalId"] == "E::a"
    for x in (Int64(4), 5.4, true, "x", nothing, complex(1.5, -2.0), Any[1, 2])
        @test decode_value(encode_value(x)) == x
    end
end

@testset "runner comparison rules" begin
    # relative tolerance, both sides of the boundary
    @test isempty(compare(Dict("response" => 100.0), 100.0 + 1e-10))
    @test !isempty(compare(Dict("response" => 100.0), 100.0 + 1e-6))
    # a named list must have exactly the expected length
    @test !isempty(compare(Dict("response" => Any[1, 2]), Any[1, 2, 3]))
    @test isempty(compare(Dict("response" => Any[1, 2]), Any[1, 2]))
    # an int64 on the wire is a string; an expected integer matches it
    @test isempty(compare(Dict("response" => Dict("n" => 4)), Dict("n" => "4")))
    @test !isempty(compare(Dict("response" => Dict("n" => 5)), Dict("n" => "4")))
    # snake_case expectations find lowerCamel wire keys
    @test isempty(compare(Dict("response" => Dict("model_hash" => "x")), Dict("modelHash" => "x")))
    # absent/non_empty read the default the same way, "0" like 0
    actual = Dict{String,Any}("error" => "", "n" => "0", "elements" => Any[Dict("id" => "1"), Dict("id" => "2")])
    @test isempty(compare(Dict("response" => Dict("error" => ""), "absent" => Any["missing", "error", "n"],
                               "non_empty" => Any["elements"]), actual))
    @test !isempty(compare(Dict("non_empty" => Any["n"]), actual))
    # contains_all collects over *
    @test isempty(compare(Dict("contains_all" => Dict("elements.*.id" => ["1", "2"])), actual))
    @test !isempty(compare(Dict("contains_all" => Dict("elements.*.id" => ["3"])), actual))
    @test lookup(actual, "elements.1.id") == "2"
    # counts and min_counts
    @test isempty(compare(Dict("counts" => Dict("elements" => 2), "min_counts" => Dict("elements" => 1)), actual))
    @test !isempty(compare(Dict("counts" => Dict("elements" => 3)), actual))
    # contains
    @test isempty(compare(Dict("contains" => Dict("error" => "model not found")), Dict("error" => "model not found: abc")))
    @test !isempty(compare(Dict("contains" => Dict("error" => "nope")), Dict("error" => "model not found")))
    # status canonicalization
    @test status_canonical("not_found") == "NOT_FOUND"
    @test status_canonical("invalid_argument") == "INVALID_ARGUMENT"
    @test status_canonical("OK") == "OK"
end

@testset "runner normalization" begin
    actual = JSON.parse("""{
        "instance": {"id": "9", "typeSymbolId": "T", "featureValues": {}},
        "instances": [{"id": "9", "typeSymbolId": "T", "featureValues": {}},
                      {"id": "12", "typeSymbolId": "T", "featureValues": {}}],
        "value": {"instanceId": "12"},
        "function": {"calcId": "T::f", "selfId": "9"},
        "unbound": {"selfId": "0"},
        "version": "1.2.3",
        "capabilities": [],
        "file": "/tmp/x.sysml",
        "hash": "abc123"
    }""")
    normalize_response!(actual, "abc123")
    @test actual["version"] == "\${version}"
    @test actual["hash"] == "\${model_hash}"
    @test actual["file"] == "\${path}"
    @test !haskey(actual["unbound"], "selfId")
    label_instance_ids(actual)
    @test actual["instance"]["id"] == "@1"
    @test actual["instances"][1]["id"] == "@1"
    @test actual["instances"][2]["id"] == "@2"
    @test actual["value"]["instanceId"] == "@2"
    @test actual["function"]["selfId"] == "@1"
end

# Live tests need a service binary; they skip with a message without one.
const GRPC_BINARY = get(ENV, "OPENSYSML_GRPC_BINARY",
                        joinpath(@__DIR__, "..", "..", "..", "..", "bin", "sysml-grpc"))

@testset "live service" begin
    if !isfile(GRPC_BINARY)
        @test_skip false
        @info "skipping live tests: no sysml-grpc binary ($(GRPC_BINARY))"
    else
        conn = OpenSysML.private(binary=GRPC_BINARY)
        try
            version, caps = server_info(conn)
            @test !isempty(version) && "evaluate_subject" in caps
            @test has_capability(conn, "query")

            model = parse_source(conn, read(joinpath(FIXTURES, "simple_part.sysml"), String);
                                 name="simple_part.sysml")
            @test !isempty(model.hash)
            @test all(d -> d.severity != "error", diagnostics(model))

            bmodel = parse_source(conn, read(joinpath(FIXTURES, "behavior.sysml"), String);
                                  name="behavior.sysml")
            qmodel = parse_source(conn, read(joinpath(FIXTURES, "query.sysml"), String);
                                  name="query.sysml")

            sym = symbol(model, "Test::SimplePart")
            @test get(sym, "id", "") == "Test::SimplePart"

            @test evaluate(model, "2 + 2") == 4
            inst = instantiate(model, "Test::SimplePart")
            @test inst.type_symbol_id == "Test::SimplePart"
            @test !isempty(inst.feature_values)

            ran = execute_action(bmodel, "Test::addFive"; inputs=Dict("result" => 10))
            @test ran["outputs"]["result"] == 15
            states = execute_state(bmodel, "Test::Machine")
            @test states["statesVisited"] == ["init", "Running", "done"]

            rows = query(qmodel, "oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name")
            @test length(get(rows, "elements", Any[])) == 3

            @test_throws DiagnosticError evaluate(model, "1 +")
            @test_throws ConnectError symbol(Model(conn, "0"^64, Diagnostic[]), "X")
            @test_throws TransportError call(conn, "NoSuchMethod", Dict{String,Any}())

            dup = parse_sources(conn, [("a.sysml", "package A {}"), ("b.sysml", "package B {}")])
            @test !isempty(dup.hash)
        finally
            close(conn)
        end
    end
end
