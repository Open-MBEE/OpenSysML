using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// <see cref="Value"/> and <see cref="Edit"/> are closed: one sealed arm class and one arm test per value kind, one
/// sealed record per arm of <c>EditOperation</c> written as that arm, and no derivation from outside the library.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SealedHierarchyTests
{
    [Fact]
    public void EveryKindHasOneSealedArmClassAndOneArmTest()
    {
        var kinds = Enum.GetNames<ValueKind>();
        var nested = typeof(Value).GetNestedTypes();
        var arms = nested.Where(type => !type.IsAbstract).ToList();
        var armTests = typeof(ValueArmTests).GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly)
            .Where(method => method.IsDefined(typeof(FactAttribute)))
            .Select(method => method.Name);

        Assert.Equal(kinds.Select(kind => kind + "Value").Order(StringComparer.Ordinal), arms.Select(type => type.Name).Order(StringComparer.Ordinal));
        Assert.All(arms, type => Assert.True(type.IsSealed && type.IsSubclassOf(typeof(Value)), type.Name));
        Assert.Equal([typeof(Value.NumberValue)], nested.Where(type => type.IsAbstract));
        Assert.Equal(
            ["BigIntegerValue", "IntegerValue", "RationalValue", "RealValue"],
            arms.Where(type => type.IsSubclassOf(typeof(Value.NumberValue))).Select(type => type.Name).Order(StringComparer.Ordinal));
        Assert.Equal(kinds.Select(kind => kind + "Arm").Order(StringComparer.Ordinal), armTests.Order(StringComparer.Ordinal));
    }

    [Fact]
    public void OnlyTheLibraryCanCallTheConstructorsAnArmDerivesFrom()
    {
        const BindingFlags Constructors = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;

        Assert.All(typeof(Value).GetConstructors(Constructors), constructor => Assert.True(constructor.IsPrivate));
        Assert.All(typeof(Value.NumberValue).GetConstructors(Constructors), constructor => Assert.True(constructor.IsFamilyAndAssembly));
    }

    [Fact]
    public void ACompilationOutsideTheLibraryCannotDeriveFromAValue()
    {
        const string Members = """
            public override ValueKind Kind => ValueKind.Null;
            public override bool Equals(Value? other) => false;
            public override int GetHashCode() => 0;
            public override string ToString() => string.Empty;
            """;

        Assert.Empty(Errors("public static class Use { public static Value Make() => new Value.SequenceValue([new Value.IntegerValue(1)]); }"));
        var outsideValue = $"public sealed class Outside : Value {{ {Members} }}";
        var outsideNumber = $"public sealed class Outside : Value.NumberValue {{ {Members} public override double ToDouble() => 0; }}";
        Assert.Equal(["CS1729"], Errors(outsideValue));
        Assert.Equal(["CS0122"], Errors(outsideValue, MetadataImportOptions.All));
        Assert.Equal(["CS0122"], Errors(outsideNumber));
        Assert.Equal(["CS0122"], Errors(outsideNumber, MetadataImportOptions.All));

        Assert.Contains("CS8864", Errors("public sealed record Outside : Value;"));
        foreach (var arm in typeof(Value).GetNestedTypes().Where(type => type.IsSealed))
        {
            Assert.Contains("CS0509", Errors($"public sealed class Outside : Value.{arm.Name} {{ }}"));
        }
    }

    [Fact]
    public void EveryArmHasOneSealedRecordWrittenAsThatArm()
    {
        var oneof = Wire.EditOperation.Descriptor.Oneofs.Single(oneof => oneof.Name == "operation");
        var records = typeof(Edit).GetNestedTypes().Where(type => type.IsSubclassOf(typeof(Edit))).ToList();

        Assert.All(records, type => Assert.True(type.IsSealed, type.Name));
        Assert.Equal(oneof.Fields.Count, records.Count);
        Assert.Equal(records.Select(type => type.Name).Order(StringComparer.Ordinal), CompleteEditingWire.Edits.Select(edit => edit.GetType().Name).Order(StringComparer.Ordinal));
        Assert.Equal(oneof.Fields.Select(field => field.Name).Order(StringComparer.Ordinal), CompleteEditingWire.Edits.Select(edit => edit.Arm).Order(StringComparer.Ordinal));
        foreach (var edit in CompleteEditingWire.Edits)
        {
            var written = EditWire.ToWire(edit);
            Assert.Equal(edit.Arm, oneof.Fields.Single(field => field.FieldNumber == (int)written.OperationCase).Name);
        }
    }

    [Fact]
    public void ACompilationOutsideTheLibraryCannotDeriveFromAnEdit()
    {
        Assert.Empty(Errors("public static class Use { public static Edit Make() => new Edit.SetValue(\"A\", \"1\") with { Value = \"2\" }; }"));
        Assert.Contains("CS0534", Errors("public sealed record Outside : Edit;"));
        Assert.Contains("CS0534", Errors("public sealed record Outside : Edit { public Outside(Edit edit) : base(edit) { } }"));
        Assert.Contains("CS0534", Errors("public sealed record Outside : Edit;", MetadataImportOptions.All));
        foreach (var arm in typeof(Edit).GetNestedTypes().Where(type => type.IsSubclassOf(typeof(Edit))))
        {
            Assert.Contains("CS0509", Errors($"public sealed record Outside : Edit.{arm.Name};"));
        }
    }

    // A consumer imports only what the library makes visible; importing all of it shows why a constructor is unusable.
    private static List<string> Errors(string source, MetadataImportOptions imported = MetadataImportOptions.Public)
    {
        var library = typeof(Value).Assembly.Location;
        var references = ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!)
            .Split(Path.PathSeparator)
            .Where(path => !string.Equals(Path.GetFileName(path), Path.GetFileName(library), StringComparison.Ordinal))
            .Append(library)
            .Select(path => MetadataReference.CreateFromFile(path));
        var compilation = CSharpCompilation.Create(
            "Consumer",
            [CSharpSyntaxTree.ParseText($"#nullable enable\nusing OpenMBEE.OpenSysML;\n{source}", new CSharpParseOptions(LanguageVersion.Latest))],
            references,
            new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary).WithMetadataImportOptions(imported));

        return compilation.GetDiagnostics()
            .Where(diagnostic => diagnostic.Severity == Microsoft.CodeAnalysis.DiagnosticSeverity.Error)
            .Select(diagnostic => diagnostic.Id)
            .Distinct()
            .ToList();
    }
}
