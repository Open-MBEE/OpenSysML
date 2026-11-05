using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using Microsoft.CodeAnalysis;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Holds the edits to the schema, the inventory and the service: one sealed record per arm of <c>EditOperation</c>,
/// each written as its arm; the gates and their order, and every predicate behind them, read from the service's own
/// source rather than restated; and no derivation from outside the library.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed partial class EditInventoryTests
{
    private static readonly ParityInventory Committed = ParityInventory.LoadCommitted();

    private static readonly string EditSource = Repository.PathOf("internal", "frontend", "grpc", "edit.go");

    [Fact]
    public void EveryArmHasOneSealedRecordWrittenAsThatArmAndOneImplementedInventoryArm()
    {
        var oneof = Wire.EditOperation.Descriptor.Oneofs.Single(oneof => oneof.Name == "operation");
        var records = typeof(Edit).GetNestedTypes().Where(type => type.IsSubclassOf(typeof(Edit))).ToList();

        Assert.All(records, type => Assert.True(type.IsSealed, type.Name));
        Assert.Equal(oneof.Fields.Count, records.Count);
        Assert.Equal(
            oneof.Fields.Select(field => Committed.Edits.Arms[field.Name].Edit).Order(StringComparer.Ordinal),
            records.Select(type => type.Name).Order(StringComparer.Ordinal));
        Assert.Equal(records.Select(type => type.Name).Order(StringComparer.Ordinal), CompleteEditingWire.Edits.Select(edit => edit.GetType().Name).Order(StringComparer.Ordinal));
        foreach (var edit in CompleteEditingWire.Edits)
        {
            var written = EditWire.ToWire(edit);
            Assert.Equal(edit.Arm, oneof.Fields.Single(field => field.FieldNumber == (int)written.OperationCase).Name);
            Assert.Equal(Committed.Edits.Arms[edit.Arm].Edit, edit.GetType().Name);
        }

        Assert.All(
            Committed.Edits.Arms.Values,
            arm => Assert.Equal(("implemented", "ModelEditingTests.EveryArmAndEveryOptionIsSentAsTheSchemaWritesIt"), (arm.Status, arm.Test)));
    }

    [Fact]
    public void TheGatesAreTheServicesInTheOrderItChecksThem()
    {
        var constants = Directory.GetFiles(Repository.PathOf("internal", "frontend", "grpc"), "*.go")
            .Where(path => !path.EndsWith("_test.go", StringComparison.Ordinal))
            .SelectMany(path => GoCapability().Matches(File.ReadAllText(path)))
            .ToDictionary(match => match.Groups[1].Value, match => match.Groups[2].Value, StringComparer.Ordinal);
        var checks = GoGate().Matches(GoFunction(EditSource, "func (s *Service) ApplyEdits(")).Select(match => constants[match.Groups[2].Value]).ToList();

        Assert.Equal(EditWire.GateOrder, checks);
        Assert.Equal(Committed.Edits.GateOrder, EditWire.GateOrder);
    }

    [Fact]
    public void EachArmNeedsTheCapabilitiesTheServicesPredicatesNameForIt()
    {
        var authoring = GoArms().Matches(GoFunction(EditSource, "func requestsAuthoring(")).Select(match => match.Groups[1].Value).ToHashSet(StringComparer.Ordinal);
        var dedicated = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            ["requestsConnectionAuthoring"] = Capabilities.ConnectionAuthoring,
            ["requestsSatisfyAuthoring"] = Capabilities.SatisfyAuthoring,
            ["requestsRequirementConstraintAuthoring"] = Capabilities.RequirementConstraintAuthoring,
            ["requestsTransitionAuthoring"] = Capabilities.TransitionAuthoring,
            ["requestsVerificationObjectiveAuthoring"] = Capabilities.VerificationObjectiveAuthoring,
            ["requestsMetadataAuthoring"] = Capabilities.MetadataAuthoring,
            ["requestsMetadataPrefixAuthoring"] = Capabilities.MetadataPrefixAuthoring,
            ["requestsSequenceAuthoring"] = Capabilities.SequenceAuthoring,
            ["requestsImportAuthoring"] = Capabilities.ImportAuthoring,
            ["requestsDocumentationAuthoring"] = Capabilities.DocumentationAuthoring,
            ["requestsCommentAuthoring"] = Capabilities.CommentAuthoring,
        };
        var bare = Bare().ToDictionary(edit => edit.GetType().Name, StringComparer.Ordinal);

        foreach (var (name, edit) in bare)
        {
            var needed = EditWire.Needed([edit]).ToHashSet(StringComparer.Ordinal);
            Assert.Equal(authoring.Contains(name), needed.Contains(Capabilities.Authoring));
            foreach (var (function, capability) in dedicated)
            {
                var arms = GoArms().Matches(GoFunction(EditSource, $"func {function}(")).Select(match => match.Groups[1].Value);
                Assert.Equal(arms.Contains(name), needed.Contains(capability));
            }
        }
    }

    [Fact]
    public void AMembersKindAndPartsNeedTheCapabilitiesTheServicesPredicatesName()
    {
        var modifiers = GoKinds().Matches(GoFunction(EditSource, "func requestsMemberModifiers(")).Select(match => match.Groups[1].Value).ToList();
        var constraint = GoKinds().Matches(GoFunction(EditSource, "func requestsConstraintBodyAuthoring(")).Select(match => match.Groups[1].Value).ToList();
        var states = GoCaseKinds(GoFunction(EditSource, "func requestsStateActionAuthoring("));
        var objective = GoFunction(EditSource, "func requestsVerificationObjectiveAuthoring(");

        Assert.Equal(["ref", "return"], modifiers);
        Assert.Equal(["assert", "assert not", "assert constraint", "assert not constraint"], constraint);
        Assert.Equal(["exhibit state", "exhibit", "entry action", "do action", "exit action"], states);
        Assert.Contains("add.GetKind() == \"objective\" && add.GetName() == \"\"", objective, StringComparison.Ordinal);
        Assert.All(modifiers, kind => Assert.Contains(Capabilities.MemberModifiers, Needed(new Edit.AddMember("A", kind, "x"))));
        Assert.All(constraint, kind => Assert.Contains(Capabilities.ConstraintBodyAuthoring, Needed(new Edit.AddMember("A", kind, "x"))));
        Assert.All(states, kind => Assert.Contains(Capabilities.StateActionAuthoring, Needed(new Edit.AddMember("A", kind, "x"))));
        Assert.Equal([Capabilities.Authoring], Needed(new Edit.AddMember("A", "part", "x")));
        Assert.Equal([Capabilities.Authoring, Capabilities.ImplicitParameters], Needed(new Edit.AddMember("A", string.Empty, "x")));
        Assert.Equal([Capabilities.Authoring, Capabilities.VerificationObjectiveAuthoring], Needed(new Edit.AddMember("A", "objective", string.Empty)));
        Assert.Equal([Capabilities.Authoring], Needed(new Edit.AddMember("A", "objective", "named")));
    }

    [Fact]
    public void AStatementNeedsItsCapabilityForEveryPartTheServicesPredicateReadsAtAnyDepth()
    {
        var predicate = GoFunction(EditSource, "func sequenceNeedsActionBodyStatementAuthoring(");
        var parts = GoGetters().Matches(predicate).Select(match => match.Groups[1].Value).Distinct(StringComparer.Ordinal).Order(StringComparer.Ordinal).ToList();
        var keywords = GoKeywords().Matches(predicate).Select(match => match.Groups[1].Value).ToList();
        var kinds = GoCaseKinds(predicate);

        Assert.Equal(["Body", "Condition", "ElseBody", "Keyword", "MemberKind", "Multiplicity", "Parameter", "Target", "Until", "Value", "Via"], parts);
        Assert.Equal(["if", "else"], keywords);
        Assert.Equal(["accept", "send", "assign", "if", "while", "loop", "for", "terminate"], kinds);
        Assert.Contains("add.GetKeyword() == \"\" && add.GetMemberKind() != \"\"", predicate, StringComparison.Ordinal);
        Assert.Contains("depth > 128", predicate, StringComparison.Ordinal);
        Assert.Equal(128, Edit.AddSequence.MaxDepth);

        var plain = Edit.AddSequence.ThenMember(string.Empty, "action", "a");
        foreach (var part in parts.Except(["Keyword", "MemberKind"]))
        {
            var item = part switch
            {
                "Body" => plain with { Body = [Edit.AddSequence.Then(string.Empty, "x")] },
                "ElseBody" => plain with { ElseBody = [Edit.AddSequence.Then(string.Empty, "x")] },
                "Condition" => plain with { Condition = "x" },
                "Value" => plain with { Value = "x" },
                "Target" => plain with { Target = "x" },
                "Via" => plain with { Via = "x" },
                "Until" => plain with { Until = "x" },
                "Multiplicity" => plain with { Multiplicity = "[1]" },
                "Parameter" => plain with { Parameter = "x" },
                _ => throw new InvalidOperationException($"The service reads a part {part} of a statement that this client does not judge."),
            };
            var nested = plain with { Owner = "A", Body = [Edit.AddSequence.ThenMember(string.Empty, "action", "b") with { ElseBody = [item] }] };
            Assert.Contains(Capabilities.ActionBodyStatementAuthoring, Needed(item with { Owner = "A" }));
            Assert.Contains(Capabilities.ActionBodyStatementAuthoring, Needed(nested));
        }

        Assert.All(keywords, keyword => Assert.Contains(Capabilities.ActionBodyStatementAuthoring, Needed(new Edit.AddSequence("A", keyword) { Ref = "x" })));
        Assert.All(kinds, kind => Assert.Contains(Capabilities.ActionBodyStatementAuthoring, Needed(Edit.AddSequence.ThenMember("A", kind, "x"))));
        Assert.Contains(Capabilities.ActionBodyStatementAuthoring, Needed(Edit.AddSequence.Node("A", "action", "x")));
        Assert.DoesNotContain(Capabilities.ActionBodyStatementAuthoring, Needed(plain with { Owner = "A" }));
        Assert.DoesNotContain(Capabilities.ActionBodyStatementAuthoring, Needed(Edit.AddSequence.First("A", "x")));
    }

    [Fact]
    public void ACompilationOutsideTheLibraryCannotDeriveFromAnEdit()
    {
        Assert.Empty(ValueInventoryTests.Errors("public static class Use { public static Edit Make() => new Edit.SetValue(\"A\", \"1\") with { Value = \"2\" }; }"));
        Assert.Contains("CS0534", ValueInventoryTests.Errors("public sealed record Outside : Edit;"));
        Assert.Contains("CS0534", ValueInventoryTests.Errors("public sealed record Outside : Edit { public Outside(Edit edit) : base(edit) { } }"));
        Assert.Contains("CS0534", ValueInventoryTests.Errors("public sealed record Outside : Edit;", MetadataImportOptions.All));
        foreach (var arm in typeof(Edit).GetNestedTypes().Where(type => type.IsSubclassOf(typeof(Edit))))
        {
            Assert.Contains("CS0509", ValueInventoryTests.Errors($"public sealed record Outside : Edit.{arm.Name};"));
        }
    }

    // One edit of each arm with nothing optional written.
    private static IEnumerable<Edit> Bare() =>
    [
        new Edit.SetValue("A", "1"),
        new Edit.Rename("A", "B"),
        new Edit.AddMember("A", "part", "p"),
        new Edit.Delete("A"),
        new Edit.Move("A", "B"),
        new Edit.AddConnection("A", "flow", "a", "b"),
        new Edit.AddSatisfy("A", "R"),
        new Edit.AddRequirementConstraint("A", "require", "x"),
        new Edit.AddTransition("A", "s", "t"),
        new Edit.AddImport("A", "L::*"),
        new Edit.AddDocumentation("A", "d"),
        new Edit.AddVerify("A", "R"),
        new Edit.AddMetadata("A", "M"),
        Edit.AddSequence.Then("A", "x"),
        new Edit.AddMetadataPrefix("A", "M"),
        new Edit.AddComment("A", "c"),
        new Edit.AddNote("A", "n"),
    ];

    private static List<string> Needed(Edit edit) => [.. EditWire.Needed([edit])];

    private static string GoFunction(string path, string signature)
    {
        var source = File.ReadAllText(path);
        var start = source.IndexOf(signature, StringComparison.Ordinal);
        Assert.True(start >= 0, $"{path} has no {signature}");
        var end = source.IndexOf("\n}\n", start, StringComparison.Ordinal);
        return source[start..end];
    }

    private static List<string> GoCaseKinds(string body) =>
        [.. GoCase().Matches(body).SelectMany(match => GoString().Matches(match.Groups[1].Value)).Select(match => match.Groups[1].Value)];

    [GeneratedRegex("""^const Capability(\w+) = "([a-z_]+)"$""", RegexOptions.Multiline)]
    private static partial Regex GoCapability();

    [GeneratedRegex("""if requests(\w+)\(req\.Operations\) \{\s*if err := s\.requireCapability\(Capability(\w+)\)""")]
    private static partial Regex GoGate();

    [GeneratedRegex("""\*pb\.EditOperation_(\w+)""")]
    private static partial Regex GoArms();

    [GeneratedRegex("GetKind\\(\\) == \"([^\"]+)\"")]
    private static partial Regex GoKinds();

    [GeneratedRegex("""add\.Get(\w+)\(\)""")]
    private static partial Regex GoGetters();

    [GeneratedRegex("GetKeyword\\(\\) == \"([a-z]+)\"")]
    private static partial Regex GoKeywords();

    [GeneratedRegex("""^\s*case ("[^:\n]+"):""", RegexOptions.Multiline)]
    private static partial Regex GoCase();

    [GeneratedRegex("\"([^\"]*)\"")]
    private static partial Regex GoString();
}
