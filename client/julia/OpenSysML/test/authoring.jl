const AUTHORING_CAPABILITIES = String[
    OpenSysML.CAPABILITY_APPLY_EDITS,
    OpenSysML.CAPABILITY_AUTHORING,
    OpenSysML.CAPABILITY_CONNECTION_AUTHORING,
    OpenSysML.CAPABILITY_SATISFY_AUTHORING,
    OpenSysML.CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
    OpenSysML.CAPABILITY_TRANSITION_AUTHORING,
    OpenSysML.CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
    OpenSysML.CAPABILITY_METADATA_AUTHORING,
    OpenSysML.CAPABILITY_METADATA_PREFIX_AUTHORING,
    OpenSysML.CAPABILITY_SEQUENCE_AUTHORING,
    OpenSysML.CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
    OpenSysML.CAPABILITY_IMPORT_AUTHORING,
    OpenSysML.CAPABILITY_DOCUMENTATION_AUTHORING,
    OpenSysML.CAPABILITY_COMMENT_AUTHORING,
    OpenSysML.CAPABILITY_MEMBER_MODIFIERS,
    OpenSysML.CAPABILITY_IMPLICIT_PARAMETERS,
    OpenSysML.CAPABILITY_CONSTRAINT_BODY_AUTHORING,
    OpenSysML.CAPABILITY_STATE_ACTION_AUTHORING,
    OpenSysML.CAPABILITY_EDIT_DOCUMENTS,
]

function authoring_service(capabilities, response)
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
            JSON.json(Dict("version" => "test", "capabilities" => capabilities[])) :
            response[]
        HTTP.Response(200, ["Content-Type" => "application/json"], answer)
    end
    server, requests, "127.0.0.1:$(port)"
end

@testset "authoring shorthand serialization and accessors" begin
    capabilities = Ref(copy(AUTHORING_CAPABILITIES))
    response = Ref(JSON.json(Dict("content" => "edited")))
    server, requests, address = authoring_service(capabilities, response)
    conn = external(address)
    try
        cases = Any[
            ("add_entry_transition",
             e -> add_entry_transition(e, "P::Machine", "idle"),
             e -> OpenSysML._editor_add!(
                 e, ("add_transition", "P::Machine", "", "", "idle", "", "", "", true))),
            ("add_require_constraint",
             e -> add_require_constraint(e, "P::R", "true", "rule"),
             e -> add_requirement_constraint(e, "P::R", "require", "true", "rule")),
            ("add_assume_constraint",
             e -> add_assume_constraint(e, "P::R", "true", "rule"),
             e -> add_requirement_constraint(e, "P::R", "assume", "true", "rule")),
            ("add_allocation",
             e -> add_allocation(e, "P::S", "a", "b"; name="alloc", type="T"),
             e -> add_connection(e, "P::S", "allocation", "a", "b";
                                 name="alloc", type="T")),
            ("add_flow",
             e -> add_flow(e, "P::S", "a", "b"; name="transfer", type="T"),
             e -> add_connection(e, "P::S", "flow", "a", "b";
                                 name="transfer", type="T")),
            ("add_succession",
             e -> add_succession(e, "P::A", "first", "next"; name="steps"),
             e -> add_connection(e, "P::A", "succession", "first", "next";
                                 name="steps")),
            ("add_return",
             e -> add_return(e, "P::A", "result"; type="Integer"),
             e -> add_member(e, "P::A", "return", "result"; type="Integer")),
            ("add_action_def",
             e -> add_action_def(e, "P", "Run", [("x", "Integer")],
                                 [("y", "Integer")]; doc="Runs"),
             e -> (add_member(e, "P", "action def", "Run"; doc="Runs");
                   add_parameter(e, "P::Run", "in", "x"; type="Integer");
                   add_parameter(e, "P::Run", "out", "y"; type="Integer"))),
            ("add_perform_action",
             e -> add_perform_action(e, "P", "run", "P::Run"; doc="Runs"),
             e -> add_member(e, "P", "perform action", "run";
                             type="P::Run", doc="Runs")),
            ("add_perform",
             e -> add_perform(e, "P", "run", "Runs"),
             e -> add_member(e, "P", "perform", "run"; doc="Runs")),
            ("add_exhibit_state",
             e -> add_exhibit_state(e, "P", "active", "P::Active"),
             e -> add_member(e, "P", "exhibit state", "active"; type="P::Active")),
            ("add_exhibit",
             e -> add_exhibit(e, "P", "active"),
             e -> add_member(e, "P", "exhibit", "active")),
        ]
        member_helpers = [
            (:add_package, "package"), (:add_part_def, "part def"), (:add_part, "part"),
            (:add_attribute_def, "attribute def"), (:add_attribute, "attribute"),
            (:add_item_def, "item def"), (:add_item, "item"),
            (:add_port_def, "port def"), (:add_port, "port"), (:add_class, "class"),
            (:add_struct, "struct"), (:add_datatype, "datatype"),
            (:add_classifier, "classifier"), (:add_feature, "feature"),
            (:add_assoc, "assoc"), (:add_behavior, "behavior"),
            (:add_function, "function"), (:add_predicate, "predicate"),
            (:add_interaction, "interaction"), (:add_metaclass, "metaclass"),
            (:add_state_def, "state def"), (:add_state, "state"),
            (:add_requirement_def, "requirement def"), (:add_requirement, "requirement"),
        ]
        member_case = (function_name, kind) -> (
            String(function_name),
            e -> getfield(OpenSysML, function_name)(e, "P", "X"; type="T"),
            e -> add_member(e, "P", kind, "X"; type="T"),
        )
        append!(cases, member_case(function_name, kind)
                for (function_name, kind) in member_helpers)

        body = Body()
        add_first(body, "start")
        body_copy = operations(body)
        empty!(body_copy)
        @test length(body) == 1

        for (name, shorthand, equivalent) in cases
            @testset "$name" begin
                shorthand_editor = Editor("model-hash", conn)
                equivalent_editor = Editor("model-hash", conn)
                shorthand(shorthand_editor)
                equivalent(equivalent_editor)
                snapshot = operations(shorthand_editor)
                empty!(snapshot)
                @test length(shorthand_editor) > 0
                @test !applied(shorthand_editor)
                apply_edits(shorthand_editor)
                shorthand_json = deepcopy(last(requests).body["operations"])
                apply_edits(equivalent_editor)
                @test shorthand_json == last(requests).body["operations"]
                @test applied(shorthand_editor)
            end
        end
    finally
        close(conn)
        close(server)
    end
end

@testset "authoring request serialization and results" begin
    capabilities = Ref(copy(AUTHORING_CAPABILITIES))
    response = Ref(JSON.json(Dict(
        "content" => "edited",
        "applied" => [Dict("operationIndex" => 0, "target" => "P::x",
                           "offset" => 2, "length" => 1,
                           "oldText" => "1", "newText" => "2",
                           "document" => "p.sysml")],
        "documents" => [Dict("name" => "p.sysml", "content" => "edited")],
    )))
    server, requests, address = authoring_service(capabilities, response)
    conn = external(address)
    try
        operations = Any[
            ("set_value", "P::x", "2"),
            ("rename", "P::x", "y"),
            ("add_member", "P", "part", "item", "", "", "", String[]),
            ("add_connection", "P", "flow", "a", "b", "", ""),
            ("add_satisfy", "P", "P::R", "", true, false),
            ("add_requirement_constraint", "P", "require", "true", ""),
            ("add_transition", "P", "", "a", "b", "", "", "", false),
            ("add_verify", "P", "P::R"),
            ("add_metadata", "P", "P::M", "", String[], [("kind", "test")], false),
            ("add_metadata_prefix", "P::x", "P::M"),
            ("add_sequence", "P::A", "then", "", "action", "step", "P::A", "",
                Dict("body" => [("add_sequence", "", "", "done", "", "", "", "")])),
            ("add_import", "P", "", "ScalarValues::*", false, false, String[]),
            ("add_documentation", "P::x", "docs", "", "", false),
            ("add_comment", "P", "comment", "", ["P::x"], "en"),
            ("add_note", "P::x", "note"),
            ("delete", "P::x", false),
            ("move", "P::x", "P::Q"),
        ]
        result = apply_edits(conn, "model-hash", operations)
        @test result isa EditResult
        @test result.content == "edited"
        @test result.from_format == result.to_format == "sysml"
        @test !result.experimental
        @test result.applied == [AppliedEdit(0, "P::x", 2, 1, "1", "2", "p.sysml")]
        @test result.documents == [EditedDocument("p.sysml", "edited")]
        @test [request.method for request in requests] == ["GetServerInfo", "ApplyEdits"]
        payload = last(requests).body
        @test payload["modelHash"] == "model-hash"
        @test payload["acceptDocuments"]
        @test [first(keys(operation)) for operation in payload["operations"]] == [
            "setValue", "rename", "addMember", "addConnection", "addSatisfy",
            "addRequirementConstraint", "addTransition", "addVerify", "addMetadata",
            "addMetadataPrefix", "addSequence", "addImport", "addDocumentation",
            "addComment", "addNote", "delete", "move",
        ]
        nested = Dict("owner" => "", "keyword" => "", "ref" => "done",
                      "memberKind" => "", "memberName" => "", "type" => "", "after" => "")
        expected = Any[
            Dict("setValue" => Dict("target" => "P::x", "value" => "2")),
            Dict("rename" => Dict("target" => "P::x", "newName" => "y")),
            Dict("addMember" => Dict("owner" => "P", "kind" => "part", "name" => "item",
                "type" => "", "multiplicity" => "", "value" => "", "specializes" => String[],
                "metadataPrefixes" => String[], "bodyExpression" => "")),
            Dict("addConnection" => Dict("owner" => "P", "kind" => "flow",
                "fromEnd" => "a", "toEnd" => "b", "name" => "", "type" => "")),
            Dict("addSatisfy" => Dict("owner" => "P", "requirement" => "P::R",
                "satisfyingFeature" => "", "isAsserted" => true, "isNegated" => false)),
            Dict("addRequirementConstraint" => Dict("owner" => "P", "kind" => "require",
                "expression" => "true", "name" => "")),
            Dict("addTransition" => Dict("owner" => "P", "name" => "", "source" => "a",
                "target" => "b", "trigger" => "", "guard" => "", "effect" => "", "initial" => false)),
            Dict("addVerify" => Dict("owner" => "P", "requirement" => "P::R")),
            Dict("addMetadata" => Dict("owner" => "P", "metadataType" => "P::M", "name" => "",
                "about" => String[], "values" => [Dict("feature" => "kind", "value" => "test")],
                "shorthand" => false)),
            Dict("addMetadataPrefix" => Dict("target" => "P::x", "metadataType" => "P::M")),
            Dict("addSequence" => Dict("owner" => "P::A", "keyword" => "then", "ref" => "",
                "memberKind" => "action", "memberName" => "step", "type" => "P::A",
                "after" => "", "body" => [nested])),
            Dict("addImport" => Dict("owner" => "P", "visibility" => "",
                "target" => "ScalarValues::*", "isRecursive" => false, "isImportAll" => false,
                "filters" => String[])),
            Dict("addDocumentation" => Dict("target" => "P::x", "body" => "docs",
                "name" => "", "locale" => "", "replace" => false)),
            Dict("addComment" => Dict("owner" => "P", "body" => "comment", "name" => "",
                "about" => ["P::x"], "locale" => "en")),
            Dict("addNote" => Dict("target" => "P::x", "text" => "note")),
            Dict("delete" => Dict("target" => "P::x", "cascade" => false)),
            Dict("move" => Dict("target" => "P::x", "owner" => "P::Q")),
        ]
        @test payload["operations"] == expected
    finally
        close(conn)
        close(server)
    end
end

@testset "authoring option serialization" begin
    capabilities = Ref(copy(AUTHORING_CAPABILITIES))
    response = Ref(JSON.json(Dict("content" => "edited")))
    server, requests, address = authoring_service(capabilities, response)
    conn = external(address)
    try
        operations = [
            ("add_member", "P", "constraint", "check", "", "", "", ["P::Base"],
             true, ["P::Base::old"], true, "out", ["prefix"], "x > 0", "Checks input"),
            ("add_documentation", "P::check", "Details", "summary", "en", true),
        ]
        result = apply_edits(conn, "model-hash", operations)
        @test result.content == "edited"
        @test [request.method for request in requests] == ["GetServerInfo", "ApplyEdits"]
        @test last(requests).body["operations"] == Any[
            Dict("addMember" => Dict("owner" => "P", "kind" => "constraint",
                "name" => "check", "type" => "", "multiplicity" => "", "value" => "",
                "specializes" => ["P::Base"], "isAbstract" => true,
                "redefines" => ["P::Base::old"], "isDefault" => true, "direction" => "out",
                "metadataPrefixes" => ["prefix"], "bodyExpression" => "x > 0",
                "doc" => "Checks input")),
            Dict("addDocumentation" => Dict("target" => "P::check", "body" => "Details",
                "name" => "summary", "locale" => "en", "replace" => true)),
        ]
    finally
        close(conn)
        close(server)
    end
end

@testset "authoring capability preflights" begin
    response = Ref("{}")
    capabilities = Ref(String[])
    server, requests, address = authoring_service(capabilities, response)
    cases = [
        (OpenSysML.CAPABILITY_APPLY_EDITS, ("set_value", "P::x", "2"), false),
        (OpenSysML.CAPABILITY_AUTHORING, ("delete", "P::x", false), false),
        (OpenSysML.CAPABILITY_CONNECTION_AUTHORING,
            ("add_connection", "P", "flow", "a", "b", "", ""), false),
        (OpenSysML.CAPABILITY_SATISFY_AUTHORING,
            ("add_satisfy", "P", "P::R", "", false, false), false),
        (OpenSysML.CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
            ("add_requirement_constraint", "P", "require", "true", ""), false),
        (OpenSysML.CAPABILITY_TRANSITION_AUTHORING,
            ("add_transition", "P", "", "a", "b", "", "", "", false), false),
        (OpenSysML.CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
            ("add_verify", "P", "P::R"), false),
        (OpenSysML.CAPABILITY_METADATA_AUTHORING,
            ("add_metadata", "P", "P::M", "", String[], [], false), false),
        (OpenSysML.CAPABILITY_METADATA_PREFIX_AUTHORING,
            ("add_metadata_prefix", "P::x", "P::M"), false),
        (OpenSysML.CAPABILITY_SEQUENCE_AUTHORING,
            ("add_sequence", "P::A", "first", "start", "", "", "", ""), false),
        (OpenSysML.CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
            ("add_sequence", "P::A", "if", "", "", "", "", ""), false),
        (OpenSysML.CAPABILITY_IMPORT_AUTHORING,
            ("add_import", "P", "", "ScalarValues::*", false, false, String[]), false),
        (OpenSysML.CAPABILITY_DOCUMENTATION_AUTHORING,
            ("add_documentation", "P::x", "docs", "", "", false), false),
        (OpenSysML.CAPABILITY_COMMENT_AUTHORING,
            ("add_comment", "P", "comment", "", String[], ""), false),
        (OpenSysML.CAPABILITY_MEMBER_MODIFIERS,
            ("add_member", "P", "part", "X", "", "", "", String[],
             true, String[], false, ""), false),
        (OpenSysML.CAPABILITY_IMPLICIT_PARAMETERS,
            ("add_member", "P", "", "x", "", "", "", String[]), false),
        (OpenSysML.CAPABILITY_CONSTRAINT_BODY_AUTHORING,
            ("add_member", "P", "constraint", "c", "", "", "", String[],
             false, String[], false, "", "true"), false),
        (OpenSysML.CAPABILITY_STATE_ACTION_AUTHORING,
            ("add_member", "P", "entry action", "enter", "", "", "", String[]), false),
        (OpenSysML.CAPABILITY_EDIT_DOCUMENTS, ("set_value", "P::x", "2"), true),
    ]
    try
        for (missing, operation, multi_document) in cases
            capabilities[] = String[c for c in AUTHORING_CAPABILITIES if c != missing]
            empty!(requests)
            conn = external(address)
            err = try
                apply_edits(conn, "model-hash", [operation]; multi_document=multi_document)
                nothing
            catch exception
                exception
            finally
                close(conn)
            end
            @test err isa MissingCapabilityError
            err isa MissingCapabilityError && @test err.capability == missing
            @test [request.method for request in requests] == ["GetServerInfo"]
        end
    finally
        close(server)
    end
end

@testset "authoring builders and errors" begin
    editor = Editor("hash", nothing)
    @test isempty(editor)
    add_parameter(editor, "P::C", "in", "x"; type="Integer")
    add_return(editor, "P::C"; type="Integer")
    add_calc_def(editor, "P", "C"; inputs=[("x", "Integer")],
                 return_type="Integer", return_expression="x * 2")
    add_action_def(editor, "P", "A"; inputs=[("x", "Integer")],
                   outputs=[("y", "Integer")])
    add_perform_action(editor, "P", "run"; type="P::A")
    add_perform(editor, "P", "run")
    add_exhibit_state(editor, "P", "running"; type="P::State")
    add_exhibit(editor, "P", "running")
    add_state_action(editor, "P::State", "entry", "enter")
    add_assert_constraint(editor, "P", "checked"; expression="true")
    add_assert(editor, "P", "checked")
    @test length(editor) == 15
    @test editor.operations[1][1:4] == ("add_member", "P::C", "", "x")
    @test editor.operations[3][3:4] == ("calc def", "C")
    @test editor.operations[end][3] == "assert"
    @test_throws ArgumentError add_state_action(editor, "P", "invalid", "x")
    @test_throws ArgumentError add_calc(editor, "P", "C";
        return_expression="x", return_type="")

    member_helpers = [
        (:add_package, "package"), (:add_part_def, "part def"), (:add_part, "part"),
        (:add_attribute_def, "attribute def"), (:add_attribute, "attribute"),
        (:add_item_def, "item def"), (:add_item, "item"),
        (:add_port_def, "port def"), (:add_port, "port"), (:add_class, "class"),
        (:add_struct, "struct"), (:add_datatype, "datatype"),
        (:add_classifier, "classifier"), (:add_feature, "feature"),
        (:add_assoc, "assoc"), (:add_behavior, "behavior"),
        (:add_function, "function"), (:add_predicate, "predicate"),
        (:add_interaction, "interaction"), (:add_metaclass, "metaclass"),
        (:add_state_def, "state def"), (:add_state, "state"),
        (:add_requirement_def, "requirement def"), (:add_requirement, "requirement"),
        (:add_constraint_def, "constraint def"), (:add_constraint, "constraint"),
    ]
    members = Editor("hash", nothing)
    for (function_name, _) in member_helpers
        getfield(OpenSysML, function_name)(members, "P", "Name")
    end
    @test [operation[3] for operation in members.operations] == last.(member_helpers)

    add_objective(members, "P")
    @test last(members.operations)[3:4] == ("objective", "")

    body = Body()
    add_first(body, "start")
    add_then(body, "done")
    add_action(body, "run", "P::Action")
    branch = Body()
    add_assign(branch, "ready", "true")
    add_if(body, "ready", branch; else_body=Body())
    add_while(body, "ready", branch, "done")
    add_loop(body, branch, "done")
    add_for(body, "item", "items", branch, "Item")
    add_accept(body, "input", "Item", "port")
    add_send(body, "output", "target", "port")
    add_terminate(body)
    add_guarded_then(body, "ready", "done")
    add_else(body, "done")
    @test length(body) == 12
    @test body.operations[3][1] == "add_sequence"
    @test !isempty(body.operations[4][9]["body"])

    response = Ref(JSON.json(Dict(
        "error" => "the value is invalid",
        "failure" => "EDIT_FAILURE_INVALID_VALUE",
        "diagnostics" => [Dict("severity" => "error", "message" => "invalid")],
        "referringElements" => ["P::owner"],
        "referrers" => [Dict("name" => "P::owner", "document" => "p.sysml")],
    )))
    capabilities = Ref(copy(AUTHORING_CAPABILITIES))
    server, requests, address = authoring_service(capabilities, response)
    conn = external(address)
    try
        err = try
            apply_edits(conn, "hash", [("set_value", "P::x", "1[")])
            nothing
        catch exception
            exception
        end
        @test err isa InvalidEditError
        @test err isa EditError
        @test err.failure == "EDIT_FAILURE_INVALID_VALUE"
        @test sprint(showerror, err) == "the value is invalid"
        @test first(err.diagnostics).message == "invalid"
        @test err.referring_elements == ["P::owner"]
        @test err.referrers == [Referrer("P::owner", "p.sysml")]
        edit_error_types = (
            EditResultError, EditTargetError, InvalidEditError, NoEditsError,
            OverlappingEditsError, RenameReferencedError, OwnerNotFoundError,
            OwnerNotNamespaceError, IllegalMemberKindError, MemberNameTakenError,
            DeleteReferencedError, OwnerInsideTargetError, MoveReferencedError,
            ReferencedElsewhereError, EditFailureError,
        )
        @test all(T -> supertype(T) === EditError, edit_error_types)
        @test !(IllegalMemberKindError("bad") isa InvalidEditError)
        empty_editor = Editor("hash", conn)
        @test_throws NoEditsError apply_edits(empty_editor)
        @test [request.method for request in requests] == ["GetServerInfo", "ApplyEdits"]
    finally
        close(conn)
        close(server)
    end
end

@testset "live authoring operations" begin
    if !isfile(GRPC_BINARY)
        @test_skip false
    else
        conn = private(binary=GRPC_BINARY)
        try
            cases = [
                ("set value", "package P { attribute x : Integer = 1; }",
                 e -> set_value(e, "P::x", "2")),
                ("rename", "package P { part def Old; }",
                 e -> rename(e, "P::Old", "New")),
                ("add member", "package P { }",
                 e -> add_part_def(e, "P", "Vehicle")),
                ("delete", "package P { part def Old; }",
                 e -> delete(e, "P::Old")),
                ("move", "package P { part def A { attribute x : Integer; } part def B; }",
                 e -> move(e, "P::A::x", "P::B")),
                ("connection", "package P { part def S { part a; part b; } }",
                 e -> add_allocation(e, "P::S", "a", "b")),
                ("satisfy", "package P { requirement def R; requirement r : R; part def H; }",
                 e -> add_satisfy(e, "P::H", "P::r")),
                ("requirement constraint", "package P { requirement def R; }",
                 e -> add_require_constraint(e, "P::R", "true")),
                ("transition", "package P { state def M { state idle; state active; } }",
                 e -> add_transition(e, "P::M", "idle", "active")),
                ("verify", "package P { requirement def R; requirement r : R; verification def V; }",
                 e -> add_verify(e, "P::V", "P::r")),
                ("metadata", "package P { metadata def M; part def H; }",
                 e -> add_metadata(e, "P::H", "P::M")),
                ("metadata prefix", "metadata def Safety; part def H;",
                 e -> add_metadata_prefix(e, "H", "Safety")),
                ("sequence", "package P { action def A; }",
                 e -> (add_first(e, "P::A", "start"); add_then(e, "P::A"; ref="done"))),
                ("import", "package P { }",
                 e -> add_import(e, "P", "ScalarValues::*")),
                ("documentation", "package P { part def H; }",
                 e -> add_documentation(e, "P::H", "A host.")),
                ("comment", "package P { part def H; }",
                 e -> add_comment(e, "P", "A comment.")),
                ("note", "package P { part def H { attribute x : Integer; } }",
                 e -> add_note(e, "P::H::x", "A note.")),
            ]
            for (name, source, build) in cases
                @testset "$name" begin
                    model = parse_source(conn, source; name="$(replace(name, ' ' => '_')).sysml")
                    result = apply_edits(build(edit(model)))
                    @test result isa EditResult
                    @test !isempty(result.content)
                end
            end

            invalid = parse_source(conn,
                "package P { attribute x : Integer = 1; }"; name="invalid.sysml")
            @test_throws InvalidEditError apply_edits(
                set_value(edit(invalid), "P::x", "1["))

            multiple = parse_sources(conn, [
                ("a.sysml", "package A { part def Vehicle; }"),
                ("b.sysml", "package B { part def Engine; }"),
            ])
            edited = apply_edits(rename(edit(multiple), "A::Vehicle", "Car"))
            @test isempty(edited.content)
            @test edited.documents == [EditedDocument(
                "a.sysml", "package A { part def Car; }")]
        finally
            close(conn)
        end
    end
end

@testset "live authoring shorthands" begin
    if !isfile(GRPC_BINARY)
        @test_skip false
    else
        conn = private(binary=GRPC_BINARY)
        try
            model = parse_source(conn, """
                package P {
                    part def Vehicle;
                    part def Engine {
                        attribute fuel : ScalarValues::Integer;
                    }
                    state def Machine;
                }
                """; name="shorthands.sysml")
            editor = edit(model)
            add_package(editor, "P", "Nested")
            add_part_def(editor, "P", "NewEngine")
            add_part(editor, "P::Vehicle", "source"; type="P::Engine")
            add_part(editor, "P::Vehicle", "target"; type="P::Engine")
            add_attribute(editor, "P::Vehicle", "mass";
                          type="ScalarValues::Integer", value="1")
            add_action_def(editor, "P::Vehicle", "Run",
                [("input", "ScalarValues::Integer")],
                [("output", "ScalarValues::Integer")])
            add_state_def(editor, "P", "AddedMachine")
            add_state(editor, "P::Machine", "idle")
            add_entry_transition(editor, "P::Machine", "idle")
            add_flow(editor, "P::Vehicle", "source.fuel", "target.fuel")

            result = apply_edits(editor)
            reparsed = parse_source(conn, result.content; name="shorthands-edited.sysml")
            @test !isempty(result.content)
            @test all(d -> d.severity != "error", diagnostics(reparsed))
            @test occursin("package Nested", result.content)
            @test occursin("action def Run", result.content)
            @test occursin("entry; then idle;", result.content)
        finally
            close(conn)
        end
    end
end
