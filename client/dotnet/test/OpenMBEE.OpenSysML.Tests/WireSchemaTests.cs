using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf.Reflection;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The committed protocol code is what api/proto/sysml.proto generates, and the comparison proving it misses no change.</summary>
public sealed class WireSchemaTests
{
    private static readonly FileDescriptorProto Committed = Wire.SysmlReflection.Descriptor.ToProto();

    private static readonly Dictionary<string, (Action<FileDescriptorProto> Make, string Named)> Changes = new(StringComparer.Ordinal)
    {
        ["add a field"] = (
            file => Message(file, "Verdict").Field.Add(new FieldDescriptorProto
            {
                Name = "forecast",
                Number = 99,
                Label = FieldDescriptorProto.Types.Label.Optional,
                Type = FieldDescriptorProto.Types.Type.String,
            }),
            "the schema has field sysml.Verdict.forecast = 99, Optional String, where the code has "),
        ["renumber a field"] = (file => Field(file, "Verdict", "kind").Number = 101,
            "the schema has field sysml.Verdict.kind = 101, Optional String, where the code has field sysml.Verdict.kind = 1, Optional String."),
        ["retype a field"] = (file => Field(file, "Verdict", "kind").Type = FieldDescriptorProto.Types.Type.Bytes,
            "the schema has field sysml.Verdict.kind = 1, Optional Bytes, where the code has field sysml.Verdict.kind = 1, Optional String."),
        ["repeat a field"] = (file => Field(file, "Verdict", "kind").Label = FieldDescriptorProto.Types.Label.Repeated,
            "the schema has field sysml.Verdict.kind = 1, Repeated String, where the code has field sysml.Verdict.kind = 1, Optional String."),
        ["take a field out of its oneof"] = (file => Field(file, "Value", "int_value").ClearOneofIndex(),
            "the schema has field sysml.Value.int_value = 1, Optional Int64, where the code has field sysml.Value.int_value = 1, Optional Int64 in oneof kind."),
        ["make an optional field plain"] = (file => Field(file, "ExecuteActionRequest", "advance").Proto3Optional = false,
            "the schema has field sysml.ExecuteActionRequest.advance = 6, Optional Double in oneof _advance, where the code has field sysml.ExecuteActionRequest.advance = 6, Optional Double in oneof _advance proto3_optional."),
        ["retype a map's values"] = (file => Message(file, "Instance").NestedType.Single(nested => nested.Name == "FeatureValuesEntry")
            .Field.Single(field => field.Name == "value").TypeName = ".sysml.Value",
            "the schema has field sysml.Instance.FeatureValuesEntry.value = 2, Optional Message .sysml.Value, where the code has field sysml.Instance.FeatureValuesEntry.value = 2, Optional Message .sysml.FeatureValue."),
        ["free a reserved number"] = (file => Message(file, "Instance").ReservedRange.Clear(),
            "where the code has reserved sysml.Instance 3 to 3."),
        ["add an enum value"] = (file => file.EnumType.Single(declared => declared.Name == "PrimitiveOperator")
            .Value.Add(new EnumValueDescriptorProto { Name = "PRIMITIVE_OPERATOR_LIKE", Number = 8 }),
            "the schema has value sysml.PrimitiveOperator.PRIMITIVE_OPERATOR_LIKE = 8, where the code has "),
        ["rename a message"] = (file => Message(file, "Verdict").Name = "Judgement",
            "the schema has message sysml.Judgement, where the code has message sysml.Verdict."),
        ["retarget an RPC"] = (file => Rpc(file, "Evaluate").InputType = ".sysml.ParseFileRequest",
            "the schema has rpc sysml.SysMLService.Evaluate(.sysml.ParseFileRequest) returns (.sysml.EvaluateResponse), where the code has rpc sysml.SysMLService.Evaluate(.sysml.EvaluateRequest) returns (.sysml.EvaluateResponse)."),
        ["stream an RPC's response"] = (file => Rpc(file, "Evaluate").ServerStreaming = true,
            "the schema has rpc sysml.SysMLService.Evaluate(.sysml.EvaluateRequest) returns (stream .sysml.EvaluateResponse), where "),
        ["add an RPC"] = (
            file => file.Service[0].Method.Add(new MethodDescriptorProto
            {
                Name = "Forecast",
                InputType = ".sysml.EvaluateRequest",
                OutputType = ".sysml.EvaluateResponse",
            }),
            "the schema has rpc sysml.SysMLService.Forecast(.sysml.EvaluateRequest) returns (.sysml.EvaluateResponse), where the code has nothing more."),
    };

    private static readonly Dictionary<string, Action<FileDescriptorProto>> Ignored = new(StringComparer.Ordinal)
    {
        ["the C# namespace managed mode sets"] = file => file.Options.CsharpNamespace = "Elsewhere.Wire",
        ["every file option"] = file => file.Options = null,
        ["a field option"] = file => Field(file, "Verdict", "kind").Options = new FieldOptions { Deprecated = true },
        ["source information"] = file => file.SourceCodeInfo = new SourceCodeInfo { Location = { new SourceCodeInfo.Types.Location { Path = { 4, 0 } } } },
        ["a JSON name"] = file => Field(file, "Verdict", "kind").JsonName = "verdictKind",
    };

    /// <summary>Every change the comparison must name, by name.</summary>
    public static TheoryData<string> ChangeNames => [.. Changes.Keys];

    /// <summary>Every difference the comparison must ignore, by name.</summary>
    public static TheoryData<string> IgnoredNames => [.. Ignored.Keys];

    [Fact]
    [Trait(Category.Name, Category.Integration)]
    public async Task TheCommittedStubsAreTheSchema()
    {
        var schema = await WireSchema.BuildAsync(BufGate.FromProcess.RequireCommand());
        var difference = WireSchema.FirstDifference(schema, Committed);

        Assert.True(difference is null, difference);
    }

    [Fact]
    [Trait(Category.Name, Category.Unit)]
    public void TheOutlineHoldsEveryDeclarationOfTheCommittedCode()
    {
        var file = Wire.SysmlReflection.Descriptor;
        var messages = Nested(file.MessageTypes).ToList();
        var enums = file.EnumTypes.Concat(messages.SelectMany(message => message.EnumTypes)).ToList();
        var outline = WireSchema.Outline(Committed);

        Assert.Equal(messages.Count, Count(outline, "message "));
        Assert.Equal(messages.Sum(message => message.Fields.InDeclarationOrder().Count), Count(outline, "field "));
        Assert.Equal(messages.Sum(message => message.Oneofs.Count), Count(outline, "oneof "));
        Assert.Equal(enums.Count, Count(outline, "enum "));
        Assert.Equal(enums.Sum(declared => declared.Values.Count), Count(outline, "value "));
        Assert.Equal(Assert.Single(file.Services).Methods.Count, Count(outline, "rpc "));
        Assert.Contains("reserved sysml.Instance 3 to 3", outline);
        Assert.Contains("reserved sysml.Instance \"slots\"", outline);
        Assert.Null(WireSchema.FirstDifference(Committed, Committed.Clone()));
    }

    [Theory]
    [Trait(Category.Name, Category.Unit)]
    [MemberData(nameof(ChangeNames))]
    public void AChangeTheGeneratedCodeDependsOnIsTheFirstDifferenceNamed(string change)
    {
        var schema = Committed.Clone();
        Changes[change].Make(schema);

        var difference = WireSchema.FirstDifference(schema, Committed);

        Assert.NotNull(difference);
        Assert.StartsWith("api/proto/sysml.proto and the committed C# protocol code first differ at declaration ", difference, StringComparison.Ordinal);
        Assert.Contains(Changes[change].Named, difference, StringComparison.Ordinal);
        Assert.EndsWith(WireSchema.Remedy, difference, StringComparison.Ordinal);
    }

    [Theory]
    [Trait(Category.Name, Category.Unit)]
    [MemberData(nameof(IgnoredNames))]
    public void WhatOnlyTheCSharpPluginsSeeIsNotADifference(string difference)
    {
        var schema = Committed.Clone();
        Ignored[difference](schema);

        Assert.NotEqual(Committed, schema);
        Assert.Null(WireSchema.FirstDifference(schema, Committed));
    }

    private static int Count(IReadOnlyList<string> outline, string kind) => outline.Count(line => line.StartsWith(kind, StringComparison.Ordinal));

    private static IEnumerable<MessageDescriptor> Nested(IEnumerable<MessageDescriptor> messages) =>
        messages.SelectMany(message => Nested(message.NestedTypes).Prepend(message));

    private static DescriptorProto Message(FileDescriptorProto file, string name) => file.MessageType.Single(message => message.Name == name);

    private static FieldDescriptorProto Field(FileDescriptorProto file, string message, string name) =>
        Message(file, message).Field.Single(field => field.Name == name);

    private static MethodDescriptorProto Rpc(FileDescriptorProto file, string name) => file.Service[0].Method.Single(method => method.Name == name);
}
