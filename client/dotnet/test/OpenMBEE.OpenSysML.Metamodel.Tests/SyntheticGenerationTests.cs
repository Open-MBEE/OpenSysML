using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.Loader;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using OpenMBEE.OpenSysML.Tests;
using Xunit;

namespace OpenMBEE.OpenSysML.Metamodel.Tests;

/// <summary>
/// The generator's output for a synthetic table holding what the real metamodel lacks, same-named redefinitions
/// that narrow a range over two generations and a diamond under a property-declaring ancestor, compiled with
/// the metamodel runtime under the client's build policy and read through every interface.
/// </summary>
/// <remarks>
/// tools/gen/csmetamodel writes the golden and its TestSyntheticGolden keeps it current.
/// </remarks>
[Trait(Category.Name, Category.Unit)]
public sealed class SyntheticGenerationTests
{
    private const string Root = "OpenMBEE.OpenSysML.Metamodel";

    private static readonly string[] Aliases = ["a", "a"];

    private static readonly string Golden = File.ReadAllText(Repository.PathOf("client", "dotnet", "test", "testdata", "metamodel", "synthetic.golden.cs"));

    /// <summary>The output compiles with no warning, a documentation comment required on every public member.</summary>
    [Fact]
    public void SyntheticOutputCompilesCleanly()
    {
        var (diagnostics, _) = Compile(Golden);
        Assert.Empty(diagnostics);
    }

    /// <summary>The compile check is load-bearing: output without its <c>new</c> modifiers, an implementation or a comment fails it.</summary>
    [Fact]
    public void BrokenOutputFailsTheCompile()
    {
        Assert.Contains(Compile(Golden.Replace("        new IPartDefinition? Definition { get; }", "        IPartDefinition? Definition { get; }", StringComparison.Ordinal)).Diagnostics, diagnostic => diagnostic.StartsWith("CS0108", StringComparison.Ordinal));
        Assert.Contains(Compile(RemoveLine(Golden, "        IPartDefinition? INarrower.Definition => ")).Diagnostics, diagnostic => diagnostic.StartsWith("CS0535", StringComparison.Ordinal));
        Assert.Contains(Compile(RemoveLine(Golden, "    /// <summary>Metaclass <c>Narrower</c>, specializing")).Diagnostics, diagnostic => diagnostic.StartsWith("CS1591", StringComparison.Ordinal));
    }

    /// <summary>A redefinition read through any interface of the chain reads the most specific declaration's value.</summary>
    [Fact]
    public void RedefinitionsReadTheNarrowestDeclarationThroughEveryInterface()
    {
        var model = new Loaded(Compile(Golden).Assembly!);
        var narrower = model.Create("Narrower", "n");
        var definition = model.Create("PartDefinition", "d");
        var properties = model.Properties("Narrower");
        Assert.Equal("Narrower", Loaded.DeclaringName(properties["definition"]));
        Assert.Equal("Narrow", Loaded.DeclaringName(properties["definitions"]));
        Assert.Equal("Base", Loaded.DeclaringName(properties["size"]));
        Assert.Equal("Base", Loaded.DeclaringName(model.Properties("Left")["definition"]));

        Assert.Throws<ArgumentException>(() => model.Supply(narrower, properties["definition"], model.Create("Definition", "wide")));
        model.Supply(narrower, properties["definition"], definition);
        model.Supply(narrower, properties["definitions"], model.ListOf("IPartDefinition", definition));
        foreach (var owner in new[] { "IBase", "INarrow", "INarrower" })
        {
            Assert.Same(definition, model.Read(owner, "Definition", narrower));
        }

        foreach (var owner in new[] { "IBase", "INarrow" })
        {
            Assert.Equal([definition], ((IEnumerable)model.Read(owner, "Definitions", narrower)!).Cast<object>());
        }

        Assert.Equal(model.Type("IPartDefinition"), model.Type("INarrower").GetProperty("Definition")!.PropertyType);
        Assert.Equal(model.Type("IDefinition"), model.Type("IBase").GetProperty("Definition")!.PropertyType);
    }

    /// <summary>A diamond inherits its shared ancestor's properties once and reads them through that ancestor's interface.</summary>
    [Fact]
    public void DiamondsInheritSharedPropertiesOnce()
    {
        var model = new Loaded(Compile(Golden).Assembly!);
        var leafInfo = model.Metaclass("Leaf");
        Assert.Equal(["Left", "Right", "Base", "Element"], ((IEnumerable)Member(leafInfo, "Ancestors")!).Cast<object>().Select(ancestor => ancestor.ToString()!));
        var properties = model.Properties("Leaf");
        Assert.Equal(
            ["aliasIds", "declaredName", "definition", "definitions", "direction", "isLibraryElement", "leftName", "ownedElement", "owner", "size", "weight"],
            properties.Keys.Order(StringComparer.Ordinal));
        var leaf = model.Create("Leaf", "l");
        Assert.True(model.Type("IBase").IsInstanceOfType(leaf) && model.Type("ILeft").IsInstanceOfType(leaf) && model.Type("IRight").IsInstanceOfType(leaf));
        Assert.False(model.Type("INarrow").IsInstanceOfType(leaf));

        var inout = Enum.ToObject(model.Type("Direction"), 1);
        model.Supply(leaf, properties["direction"], inout);
        model.Supply(leaf, properties["size"], 3);
        model.Supply(leaf, properties["weight"], 2.5);
        model.Supply(leaf, properties["aliasIds"], Aliases);
        Assert.Equal(inout, model.Read("IBase", "Direction", leaf));
        Assert.Equal(3, model.Read("IBase", "Size", leaf));
        Assert.Equal(2.5, model.Read("IBase", "Weight", leaf));
        Assert.Equal(["a", "a"], ((IEnumerable)model.Read("IElement", "AliasIds", leaf)!).Cast<object>());
        var failure = Assert.Throws<TargetInvocationException>(() => model.Read("ILeft", "LeftName", leaf));
        Assert.Equal("NotSuppliedException", failure.InnerException!.GetType().Name);
    }

    /// <summary>Associations and subsets link both ways, as the table names them.</summary>
    [Fact]
    public void LinksFollowTheTable()
    {
        var model = new Loaded(Compile(Golden).Assembly!);
        var element = model.Properties("Element");
        Assert.Same(element["ownedElement"], Member(element["owner"], "Opposite"));
        Assert.Same(element["owner"], Member(element["ownedElement"], "Opposite"));
        Assert.Equal([element["declaredName"]], ((IEnumerable)Member(model.Properties("Left")["leftName"], "Subsets")!).Cast<object>());
        Assert.Equal([model.Properties("Narrow")["definition"]], ((IEnumerable)Member(model.Properties("Narrower")["definition"], "Redefines")!).Cast<object>());
        Assert.Equal("false", Member(element["isLibraryElement"], "DefaultLiteral"));
        Assert.Equal(false, Member(element["aliasIds"], "IsUnique"));
    }

    private static (List<string> Diagnostics, Assembly? Assembly) Compile(string generated)
    {
        var directory = Repository.PathOf("client", "dotnet", "src", Root);
        var sources = Directory.GetFiles(directory, "*.cs")
            .Where(path => Path.GetFileName(path) != "Generated.cs")
            .Order(StringComparer.Ordinal)
            .Select(path => File.ReadAllText(path))
            .Append(generated);
        var metamodel = Path.GetFileName(typeof(MetamodelElement).Assembly.Location);
        var references = ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!)
            .Split(Path.PathSeparator)
            .Where(path => Path.GetFileName(path) != metamodel)
            .Select(path => MetadataReference.CreateFromFile(path));
        var parse = new CSharpParseOptions(LanguageVersion.Latest, DocumentationMode.Diagnose);
        var compilation = CSharpCompilation.Create(
            Root,
            sources.Select(source => CSharpSyntaxTree.ParseText(source, parse)),
            references,
            new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary)
                .WithNullableContextOptions(NullableContextOptions.Enable)
                .WithSpecificDiagnosticOptions(new Dictionary<string, ReportDiagnostic> { ["CS1591"] = ReportDiagnostic.Error }));
        var diagnostics = compilation.GetDiagnostics()
            .Where(diagnostic => diagnostic.Severity >= Microsoft.CodeAnalysis.DiagnosticSeverity.Warning)
            .Select(diagnostic => $"{diagnostic.Id}: {diagnostic.GetMessage(CultureInfo.InvariantCulture)}")
            .ToList();
        if (diagnostics.Count > 0)
        {
            return (diagnostics, null);
        }

        using var image = new MemoryStream();
        var emitted = compilation.Emit(image);
        Assert.True(emitted.Success, string.Join("\n", emitted.Diagnostics));
        image.Position = 0;
        return (diagnostics, new AssemblyLoadContext("synthetic-metamodel", isCollectible: true).LoadFromStream(image));
    }

    private static string RemoveLine(string source, string prefix)
    {
        var lines = source.Split('\n').ToList();
        Assert.Equal(1, lines.RemoveAll(line => line.StartsWith(prefix, StringComparison.Ordinal)));
        return string.Join('\n', lines);
    }

    private static object? Member(object target, string name) =>
        target.GetType().GetProperty(name)!.GetValue(target);

    /// <summary>The compiled synthetic assembly, reached by reflection: its types are not this test's.</summary>
    private sealed class Loaded(Assembly assembly)
    {
        private const BindingFlags Internal = BindingFlags.Instance | BindingFlags.NonPublic;

        internal System.Type Type(string name) => assembly.GetType($"{Root}.{name}", throwOnError: true)!;

        internal object Metaclass(string name) => Type("Metaclasses").GetProperty(name)!.GetValue(null)!;

        internal Dictionary<string, object> Properties(string metaclass) =>
            ((IEnumerable)Member(Metaclass(metaclass), "Properties")!).Cast<object>().ToDictionary(property => (string)Member(property, "JsonKey")!);

        internal static string DeclaringName(object property) => Member(property, "DeclaringMetaclass")!.ToString()!;

        internal object Create(string metaclass, string id)
        {
            var info = Metaclass(metaclass);
            return info.GetType().GetMethod("CreateElement", Internal)!.Invoke(info, [id, metaclass])!;
        }

        internal void Supply(object element, object property, object? value)
        {
            try
            {
                Type("MetamodelElement").GetMethod("Supply", Internal)!.Invoke(element, [property, value]);
            }
            catch (TargetInvocationException failure) when (failure.InnerException is not null)
            {
                throw failure.InnerException;
            }
        }

        internal object? Read(string interfaceName, string property, object element) =>
            Type(interfaceName).GetProperty(property)!.GetValue(element);

        internal Array ListOf(string interfaceName, params object[] items)
        {
            var list = Array.CreateInstance(Type(interfaceName), items.Length);
            Array.Copy(items, list, items.Length);
            return list;
        }
    }
}
