using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text.Json;
using Google.Protobuf;
using Google.Protobuf.Reflection;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class ParityInventoryTests
{
    private static readonly ParityInventory Committed = ParityInventory.LoadCommitted();

    private static readonly IReadOnlySet<string> TestNames = typeof(ParityInventoryTests).Assembly.GetTypes()
        .SelectMany(type => type.GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
        .Where(method => method.IsDefined(typeof(FactAttribute), inherit: true))
        .Select(method => $"{method.DeclaringType!.Name}.{method.Name}")
        .ToHashSet(StringComparer.Ordinal);

    [Fact]
    public void TheCommittedInventoryMatchesTheSchemaScenariosAndCapabilities()
    {
        var problems = Check(Committed);

        Assert.True(problems.Count == 0, "client/dotnet/test/testdata/parity.json is stale:\n" + string.Join("\n", problems));
    }

    [Fact]
    public void TheInventoryClassifiesWhatTheDescriptorDeclares()
    {
        var schema = Wire.SysmlReflection.Descriptor;
        var service = Assert.Single(schema.Services);

        Assert.Equal(service.Methods.Select(method => method.Name), Committed.Rpcs.Select(row => row.Rpc));
        Assert.Equal(OneofArms(schema, "sysml.Value", "kind"), Committed.Values.Arms.Keys.Order(StringComparer.Ordinal));
        Assert.Equal(OneofArms(schema, "sysml.EditOperation", "operation"), Committed.Edits.Arms.Keys.Order(StringComparer.Ordinal));
        Assert.Equal(
            ParityChecker.Reachable(service.Methods.SelectMany(method => new[] { method.InputType, method.OutputType }))
                .Sum(message => message.Fields.InDeclarationOrder().Count),
            Committed.Fields.Values.Sum(fields => fields.Count));
    }

    [Fact]
    public void EveryValueKindIsSentAndReceivedUnderTheServicesGates()
    {
        var arms = Committed.Values.Arms;

        Assert.Equal("response-only", arms["unset"].Send);
        Assert.Equal("response-only", arms["undetermined"].Send);
        Assert.Equal("ungated", arms["enum_literal"].Send);
        Assert.Equal("enum_values", arms["enum_literal"].Receive);
        Assert.Equal(["array", "vector", "vector_quantity"], arms.Where(arm => arm.Value.Send == "structured_values").Select(arm => arm.Key).Order(StringComparer.Ordinal));
        Assert.All(arms.Values.Where(arm => arm.Send is not ("ungated" or "response-only")), arm => Assert.Equal(arm.Send, arm.Receive));
    }

    [Fact]
    public void AnAddedRpcFailsClassification()
    {
        var schema = Mutated(file => file.Service[0].Method.Add(new MethodDescriptorProto
        {
            Name = "Forecast",
            InputType = ".sysml.EvaluateRequest",
            OutputType = ".sysml.EvaluateResponse",
        }));

        Assert.Contains("RPC Forecast has no inventory row.", Check(Committed, schema));
    }

    [Fact]
    public void ARenamedRpcFailsClassification()
    {
        var schema = Mutated(file => file.Service[0].Method.Single(method => method.Name == "Evaluate").Name = "EvaluateExpression");
        var problems = Check(Committed, schema);

        Assert.Contains("RPC EvaluateExpression has no inventory row.", problems);
        Assert.Contains("Row Evaluate names no RPC of sysml.SysMLService.", problems);
    }

    [Fact]
    public void AnRpcWhoseRequestTypeChangedFailsClassification()
    {
        var schema = Mutated(file => file.Service[0].Method.Single(method => method.Name == "Evaluate").InputType = ".sysml.InstantiateRequest");

        Assert.Contains(
            "Row Evaluate gives request sysml.EvaluateRequest; the schema's is sysml.InstantiateRequest.",
            Check(Committed, schema));
    }

    [Fact]
    public void AnAddedFieldFailsClassification()
    {
        var schema = Mutated(file => Message(file, "EvaluateRequest").Field.Add(Field("deadline_seconds", 99, FieldDescriptorProto.Types.Type.Double)));

        Assert.Contains("Field sysml.EvaluateRequest.deadline_seconds has no classification.", Check(Committed, schema));
    }

    [Fact]
    public void ARenamedFieldFailsClassification()
    {
        var schema = Mutated(file => Message(file, "EvaluateRequest").Field.Single(field => field.Name == "expression").Name = "text");
        var problems = Check(Committed, schema);

        Assert.Contains("Field sysml.EvaluateRequest.text has no classification.", problems);
        Assert.Contains("Field sysml.EvaluateRequest.expression is classified but not declared.", problems);
    }

    [Fact]
    public void AnAddedMessageFailsClassification()
    {
        var schema = Mutated(file =>
        {
            file.MessageType.Add(new DescriptorProto { Name = "Deadline", Field = { Field("seconds", 1, FieldDescriptorProto.Types.Type.Double) } });
            Message(file, "EvaluateRequest").Field.Add(Field("deadline", 99, FieldDescriptorProto.Types.Type.Message, ".sysml.Deadline"));
        });

        Assert.Contains("Message sysml.Deadline has no field classification.", Check(Committed, schema));
    }

    [Fact]
    public void AnAddedValueArmFailsClassification()
    {
        var schema = Mutated(file => Message(file, "Value").Field.Add(Field("date_value", 99, FieldDescriptorProto.Types.Type.String, oneof: 0)));
        var problems = Check(Committed, schema);

        Assert.Contains("The values section's arms: arm of sysml.Value.kind date_value is not classified.", problems);
        Assert.Contains("Field sysml.Value.date_value has no classification.", problems);
    }

    [Fact]
    public void ARenamedValueArmFailsClassification()
    {
        var schema = Mutated(file => Message(file, "Value").Field.Single(field => field.Name == "big_int_value").Name = "wide_int_value");
        var problems = Check(Committed, schema);

        Assert.Contains("The values section's arms: arm of sysml.Value.kind wide_int_value is not classified.", problems);
        Assert.Contains("The values section's arms: arm of sysml.Value.kind big_int_value is not declared.", problems);
    }

    [Fact]
    public void AnAddedEditArmFailsClassification()
    {
        var schema = Mutated(file => Message(file, "EditOperation").Field.Add(
            Field("add_alias", 99, FieldDescriptorProto.Types.Type.Message, ".sysml.RenameEdit", oneof: 0)));

        Assert.Contains(
            "The edits section's arms: arm of sysml.EditOperation.operation add_alias is not classified.",
            Check(Committed, schema));
    }

    [Fact]
    public void AValueAddedToARequestFailsTheGateInventory()
    {
        var schema = Mutated(file => Message(file, "EvaluateRequest").Field.Add(
            Field("seed", 99, FieldDescriptorProto.Types.Type.Message, ".sysml.Value")));

        Assert.Contains(
            "Row Evaluate's valueFields: the request field seed carries Values but is not listed.",
            Check(Committed, schema));
    }

    [Fact]
    public void AnUnlistedOrStaleScenarioFails()
    {
        var added = new Scenario("evaluate/synthetic", "Evaluate", []);
        var problems = Check(Committed, scenarios: [.. Scenario.All, added]);
        Assert.Contains("Row Evaluate's scenarios: scenario evaluate/synthetic is not listed.", problems);

        var stale = WithRow(Committed, "Evaluate", row => row with { Scenarios = [.. row.Scenarios, "evaluate/removed"] });
        Assert.Contains("Row Evaluate's scenarios: scenario evaluate/removed is no scenario of the RPC.", Check(stale));

        var unknown = new Scenario("forecast/synthetic", "Forecast", []);
        Assert.Contains(
            "Scenario forecast/synthetic calls Forecast, which is no RPC of sysml.SysMLService.",
            Check(Committed, scenarios: [.. Scenario.All, unknown]));
    }

    [Fact]
    public void AScenarioRequirementNoGateCoversFails()
    {
        var inventory = WithRow(Committed, "Evaluate", row => row with { Gates = [] });

        Assert.Contains(Check(inventory), problem => problem.StartsWith("Scenario evaluate/", StringComparison.Ordinal)
            && problem.Contains("requires evaluate_subject, which no gate of Row Evaluate", StringComparison.Ordinal));
    }

    [Fact]
    public void GatesNameDeclaredCapabilitiesAndFields()
    {
        var inventory = WithRow(Committed, "Evaluate", row => row with
        {
            Gates =
            [
                new Gate("evaluate_subjects", "subject_symbol_id is set", "subject_symbol_id"),
                new Gate("evaluate_subject", "subject is set", "subject"),
                new Gate("evaluate_subject", "subject_symbol_id is set"),
                new Gate("evaluate_subject", "always", "subject_symbol_id"),
            ],
        });
        var problems = Check(inventory);

        Assert.Contains("Row Evaluate's gate evaluate_subjects names capability evaluate_subjects, which the service does not declare.", problems);
        Assert.Contains("Row Evaluate's gate evaluate_subject names field subject, which sysml.EvaluateRequest does not declare.", problems);
        Assert.Contains("Row Evaluate's gate evaluate_subject is conditional but names no field.", problems);
        Assert.Contains("Row Evaluate's gate evaluate_subject names field subject_symbol_id but applies always.", problems);
    }

    [Fact]
    public void ArmGatesAreOrderedAndDeclared()
    {
        var values = Committed with { Values = Committed.Values with { SendGateOrder = [.. Committed.Values.SendGateOrder.Skip(1), "enum_values"] } };
        var problems = Check(values);
        Assert.Contains("The values section's sendGateOrder: capability complex_values is used but not ordered.", problems);
        Assert.Contains("The values section's sendGateOrder: capability enum_values is ordered but no arm uses it.", problems);

        var arms = Committed.Edits.Arms.ToDictionary();
        arms["add_note"] = arms["add_note"] with { Gates = [new Gate("authoring", "always"), new Gate("note_authoring", "always")] };
        Assert.Contains(
            "Edit arm add_note's gate note_authoring names capability note_authoring, which the service does not declare.",
            Check(Committed with { Edits = Committed.Edits with { Arms = arms } }));
    }

    [Fact]
    public void RowsAreUniqueAndInSchemaOrder()
    {
        var duplicated = Committed with { Rpcs = [.. Committed.Rpcs, Committed.Rpcs[0]] };
        Assert.Contains($"RPC {Committed.Rpcs[0].Rpc} has more than one row.", Check(duplicated));

        var reordered = Committed with { Rpcs = [Committed.Rpcs[1], Committed.Rpcs[0], .. Committed.Rpcs.Skip(2)] };
        Assert.Contains("The rows are not in the order the schema declares the RPCs.", Check(reordered));
    }

    [Fact]
    public void TheReadmeCoverageTableIsTheInventorysRows()
    {
        var readme = File.ReadAllText(Repository.PathOf("client", "dotnet", "README.md")).ReplaceLineEndings("\n");
        var start = readme.IndexOf("\n## Coverage\n", StringComparison.Ordinal);
        Assert.True(start >= 0, "client/dotnet/README.md has no Coverage section");
        var end = readme.IndexOf("\n## ", start + 1, StringComparison.Ordinal);
        var section = readme[start..(end < 0 ? readme.Length : end)].Split('\n');

        Assert.Single(section, line => line == "| RPC | Status | Called by |");
        Assert.Equal(
            Committed.Rpcs.Select(row => $"| `{row.Rpc}` | {row.Status} | {string.Join(", ", row.Methods.Select(method => $"`{method}`"))} |"),
            section.Where(line => line.StartsWith("| `", StringComparison.Ordinal)));
    }

    [Fact]
    public void AnImplementedRowNamesTestsTheSuiteHas()
    {
        var unproven = WithRow(Committed, "Evaluate", row => row with { Status = "implemented", Tests = [] });
        Assert.Contains("Row Evaluate is implemented but names no test.", Check(unproven));

        var phantom = WithRow(Committed, "Evaluate", row => row with { Status = "implemented", Tests = ["ModelTests.EvaluatesAnExpression"] });
        Assert.Contains("Row Evaluate names test ModelTests.EvaluatesAnExpression, which the suite does not have.", Check(phantom));

        var proven = WithRow(Committed, "Evaluate", row => row with { Status = "implemented", Tests = [$"{nameof(ParityInventoryTests)}.{nameof(AnImplementedRowNamesTestsTheSuiteHas)}"] });
        Assert.Empty(Check(proven));

        var unknown = WithRow(Committed, "Evaluate", row => row with { Status = "done" });
        Assert.Contains("Row Evaluate has status done, which is neither deferred nor implemented.", Check(unknown));
    }

    [Fact]
    public void FieldClassificationsCarryTheirReasonAndTest()
    {
        var proof = $"{nameof(ParityInventoryTests)}.{nameof(FieldClassificationsCarryTheirReasonAndTest)}";

        Assert.Contains("Field sysml.EvaluateRequest.expression is typed but names no test.", Check(WithField("expression", new FieldClass("typed"))));
        Assert.Contains("Field sysml.EvaluateRequest.expression is raw-only but gives no reason.", Check(WithField("expression", new FieldClass("raw-only", Test: proof))));
        Assert.Contains("Field sysml.EvaluateRequest.expression is deferred but gives a reason or a test.", Check(WithField("expression", new FieldClass("deferred", "later"))));
        Assert.Contains(
            "Field sysml.EvaluateRequest.expression has status mapped, which is not one of deferred, typed, raw-only, unsupported.",
            Check(WithField("expression", new FieldClass("mapped"))));
        Assert.Empty(Check(WithField("expression", new FieldClass("typed", Test: proof))));
        Assert.Empty(Check(WithField("expression", new FieldClass("unsupported", "no service populates it", proof))));
    }

    [Fact]
    public void PopulationEntriesNameDeclaredFieldsAndCapabilities()
    {
        var population = Committed.Population.ToDictionary();
        population["sysml.Diagnostic.codes"] = "diagnostic_codes";
        population["sysml.Diagnostic.code"] = "diagnostic_code";
        var problems = Check(Committed with { Population = population });

        Assert.Contains("Population entry sysml.Diagnostic.codes names no field of the schema.", problems);
        Assert.Contains("Population entry sysml.Diagnostic.code names capability diagnostic_code, which the service does not declare.", problems);
    }

    [Theory]
    [InlineData("\"unexpected\": 1,", "unexpected")]
    [InlineData("\"service\": \"sysml.SysMLService\",", "service")]
    public void TheInventoryRefusesUnknownAndRepeatedMembers(string inserted, string member)
    {
        var json = System.IO.File.ReadAllText(ParityInventory.CommittedPath);
        var mutated = "{" + inserted + json.TrimStart()[1..];

        var error = Assert.ThrowsAny<JsonException>(() => ParityInventory.Parse(mutated));
        Assert.Contains(member, error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void TheInventoryRefusesAMissingMember()
    {
        using var document = JsonDocument.Parse(System.IO.File.ReadAllText(ParityInventory.CommittedPath));
        var members = document.RootElement.EnumerateObject().Where(member => member.Name != "population")
            .Select(member => $"\"{member.Name}\": {member.Value.GetRawText()}");

        Assert.ThrowsAny<JsonException>(() => ParityInventory.Parse("{" + string.Join(",", members) + "}"));
    }

    [Fact]
    public void FieldGroupsExpandToOneClassificationPerField()
    {
        var inventory = WithFieldsJson("""
            {
              "sysml.A": [
                {"status": "typed", "test": "T.Proves", "fields": ["a", "b"]},
                {"status": "unsupported", "reason": "never populated", "test": "T.Shows", "fields": ["c"]},
                {"status": "deferred", "fields": ["d"]}
              ],
              "sysml.B": [{"status": "typed", "test": "T.Proves", "fields": ["a"]}]
            }
            """);

        Assert.Equal(["sysml.A", "sysml.B"], inventory.Fields.Keys);
        Assert.Equal(new FieldClass("typed", Test: "T.Proves"), inventory.Fields["sysml.A"]["a"]);
        Assert.Equal(new FieldClass("typed", Test: "T.Proves"), inventory.Fields["sysml.A"]["b"]);
        Assert.Equal(new FieldClass("unsupported", "never populated", "T.Shows"), inventory.Fields["sysml.A"]["c"]);
        Assert.Equal(new FieldClass("deferred"), inventory.Fields["sysml.A"]["d"]);
        Assert.Equal(["a"], inventory.Fields["sysml.B"].Keys);
    }

    [Theory]
    [InlineData("""{"sysml.A": [{"status": "typed", "test": "T.P", "fields": ["a", "a"]}]}""", "Field sysml.A.a is classified twice.")]
    [InlineData("""{"sysml.A": [{"status": "typed", "test": "T.P", "fields": ["a"]}, {"status": "deferred", "fields": ["a"]}]}""", "Field sysml.A.a is classified twice.")]
    [InlineData("""{"sysml.A": [{"status": "deferred", "fields": ["a"]}, {"status": "deferred", "fields": ["b"]}]}""", "Message sysml.A has two groups classified deferred; they are one group.")]
    [InlineData("""{"sysml.A": [{"status": "deferred", "fields": []}]}""", "A group of message sysml.A classifies no field.")]
    [InlineData("""{"sysml.A": [{"status": "deferred"}]}""", "A group of message sysml.A has no fields.")]
    [InlineData("""{"sysml.A": [{"fields": ["a"]}]}""", "A group of message sysml.A has no status.")]
    [InlineData("""{"sysml.A": [{"status": "deferred", "status": "typed", "fields": ["a"]}]}""", "A group of message sysml.A repeats its member status.")]
    [InlineData("""{"sysml.A": [{"status": "deferred", "note": "x", "fields": ["a"]}]}""", "A group of message sysml.A has an unknown member note.")]
    [InlineData("""{"sysml.A": [{"status": "deferred", "fields": [1]}]}""", "A group's fields of message sysml.A is text, not Number.")]
    [InlineData("""{"sysml.A": [], "sysml.A": []}""", "Message sysml.A is classified twice.")]
    [InlineData("""{"sysml.A": {"a": "deferred"}}""", "Message sysml.A's fields are a list of groups, not StartObject.")]
    public void TheInventoryRefusesAFieldGroupThatClassifiesAFieldTwiceNoneOrAmbiguously(string fields, string problem)
    {
        var error = Assert.ThrowsAny<JsonException>(() => WithFieldsJson(fields));

        Assert.Equal(problem, error.Message);
    }

    private static IReadOnlyList<string> Check(ParityInventory inventory, FileDescriptor? schema = null, IReadOnlyList<Scenario>? scenarios = null) =>
        ParityChecker.Check(inventory, schema ?? Wire.SysmlReflection.Descriptor, scenarios ?? Scenario.All, ServiceCapabilities.Names, TestNames);

    private static ParityInventory WithRow(ParityInventory inventory, string rpc, Func<RpcRow, RpcRow> change) =>
        inventory with { Rpcs = [.. inventory.Rpcs.Select(row => row.Rpc == rpc ? change(row) : row)] };

    // The committed inventory read with its field classifications written as given.
    private static ParityInventory WithFieldsJson(string fields)
    {
        using var document = JsonDocument.Parse(System.IO.File.ReadAllText(ParityInventory.CommittedPath));
        var members = document.RootElement.EnumerateObject()
            .Select(member => $"\"{member.Name}\": {(member.Name == "fields" ? fields : member.Value.GetRawText())}");
        return ParityInventory.Parse("{" + string.Join(",", members) + "}");
    }

    private static ParityInventory WithField(string field, FieldClass classification)
    {
        var fields = Committed.Fields.ToDictionary();
        var request = fields["sysml.EvaluateRequest"].ToDictionary();
        request[field] = classification;
        fields["sysml.EvaluateRequest"] = request;
        return Committed with { Fields = fields };
    }

    private static IEnumerable<string> OneofArms(FileDescriptor schema, string message, string oneof) =>
        ParityChecker.Messages(schema)[message].Oneofs.Single(o => o.Name == oneof).Fields.Select(field => field.Name).Order(StringComparer.Ordinal);

    private static FileDescriptor Mutated(Action<FileDescriptorProto> mutate)
    {
        var file = FileDescriptorProto.Parser.ParseFrom(Wire.SysmlReflection.Descriptor.SerializedData);
        mutate(file);
        return FileDescriptor.BuildFromByteStrings([file.ToByteString()])[0];
    }

    private static DescriptorProto Message(FileDescriptorProto file, string name) => file.MessageType.Single(message => message.Name == name);

    private static FieldDescriptorProto Field(string name, int number, FieldDescriptorProto.Types.Type type, string? typeName = null, int? oneof = null)
    {
        var field = new FieldDescriptorProto { Name = name, Number = number, Type = type, Label = FieldDescriptorProto.Types.Label.Optional, JsonName = name };
        if (typeName is not null)
        {
            field.TypeName = typeName;
        }

        if (oneof is { } index)
        {
            field.OneofIndex = index;
        }

        return field;
    }
}
