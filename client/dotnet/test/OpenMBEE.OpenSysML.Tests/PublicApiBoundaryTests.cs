using System;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The generated protocol code is public and mutable, and stays outside the handwritten API: the API
/// baseline holds none of it, and it is exempted from the handwritten API checks by its exact paths alone.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed partial class PublicApiBoundaryTests
{
    private const string WireNamespace = "OpenMBEE.OpenSysML.Wire";
    private static readonly string[] GeneratedFiles = ["src/OpenMBEE.OpenSysML/Wire/Sysml.cs", "src/OpenMBEE.OpenSysML/Wire/SysmlGrpc.cs"];

    // Every generated file of the client tree, in ordinal order: the buf output and the metamodel types.
    private static readonly string[] ExemptedFiles = ["src/OpenMBEE.OpenSysML.Metamodel/Generated.cs", .. GeneratedFiles];
    private static readonly string[] BaselineFiles = ["PublicAPI.Shipped.txt", "PublicAPI.Unshipped.txt"];
    private static readonly string Library = Repository.PathOf("client", "dotnet", "src", "OpenMBEE.OpenSysML");

    [GeneratedRegex(@"^DOTNET_GENERATED := (.+)$", RegexOptions.Multiline)]
    private static partial Regex MakeGenerated();

    [GeneratedRegex(@"^\[(?<glob>[^\]\n]+)\]\n(?<body>(?:[^\[\n].*\n)*)", RegexOptions.Multiline)]
    private static partial Regex EditorConfigSection();

    [GeneratedRegex(@"^(?<prefix>[^{]*)(?:\{(?<names>[^}]+)\}(?<suffix>.*))?$")]
    private static partial Regex BraceGlob();

    [Fact]
    public void TheApiBaselineHoldsNoGeneratedProtocolCode()
    {
        Assert.DoesNotContain(Baseline(), line => line.Contains(WireNamespace, StringComparison.Ordinal));
    }

    [Fact]
    public void TheGeneratedExemptionsNameExactlyTheGeneratedFiles()
    {
        var make = MakeGenerated().Match(File.ReadAllText(Repository.PathOf("Makefile")));
        Assert.True(make.Success, "The Makefile declares no DOTNET_GENERATED.");
        Assert.Equal(ExemptedFiles, make.Groups[1].Value.Split(' ', StringSplitOptions.RemoveEmptyEntries).Order(StringComparer.Ordinal));

        var exemptions = EditorConfigSection().Matches(File.ReadAllText(Repository.PathOf("client", "dotnet", ".editorconfig")).ReplaceLineEndings("\n"))
            .Where(section => section.Groups["body"].Value.Contains("generated_code = true\n", StringComparison.Ordinal))
            .ToList();
        Assert.All(exemptions, section => Assert.Contains("dotnet_diagnostic.RS0016.severity = none\n", section.Groups["body"].Value, StringComparison.Ordinal));
        var exempted = exemptions.SelectMany(section =>
        {
            var glob = BraceGlob().Match(section.Groups["glob"].Value);
            return glob.Groups["names"].Success
                ? glob.Groups["names"].Value.Split(',').Select(name => $"{glob.Groups["prefix"].Value}{name}{glob.Groups["suffix"].Value}")
                : [glob.Groups["prefix"].Value];
        });
        Assert.Equal(ExemptedFiles, exempted.Order(StringComparer.Ordinal));
    }

    private static string[] Baseline() =>
        [.. BaselineFiles
            .SelectMany(file => File.ReadAllLines(Path.Combine(Library, file)))
            .Where(line => line.Length > 0 && !line.StartsWith('#'))];
}
