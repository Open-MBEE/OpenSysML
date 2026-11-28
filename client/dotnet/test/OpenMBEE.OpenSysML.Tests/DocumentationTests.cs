using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Holds the published .NET client guide to the sample the build runs, so the walkthrough a reader
/// copies is the program <c>make dotnet-test</c> runs against a real service.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class DocumentationTests
{
    private const string RegionStart = "// <walkthrough>";
    private const string RegionEnd = "// </walkthrough>";
    private const string SampleProject = "client/dotnet/samples/Vehicle";
    private const string SampleCommand = "OPENSYSML_GRPC_BINARY=\"$PWD/bin/sysml-grpc\" dotnet run --project " + SampleProject + " -c Release";
    private static readonly string[] GuidePath = ["docs", "clients", "dotnet.md"];
    private static readonly string[] SamplePath = ["client", "dotnet", "samples", "Vehicle", "Program.cs"];

    [Fact]
    public void TheGuideWalkthroughIsTheSamplesProgram()
    {
        var expected = Walkthrough(Read(SamplePath));
        var blocks = FencedBlocks(Read(GuidePath), "csharp");

        Assert.True(
            blocks.Contains(expected),
            $"docs/clients/dotnet.md has no csharp block that is {SampleProject}/Program.cs's using directives and walkthrough region:\n{expected}");
    }

    [Fact]
    public void TheGuideRunsTheSampleTheGateRuns()
    {
        Assert.True(HasSampleCommand(Read(GuidePath)), $"docs/clients/dotnet.md must run the walkthrough with:\n{SampleCommand}");
        Assert.True(File.Exists(Repository.PathOf([.. SampleProject.Split('/'), "Vehicle.csproj"])), $"{SampleProject} has no project");
    }

    [Theory]
    [InlineData(SampleCommand, true)]
    [InlineData("dotnet run --project " + SampleProject + " -c Release", false)]
    [InlineData("OPENSYSML_GRPC_BINARY=\"\" dotnet run --project " + SampleProject + " -c Release", false)]
    [InlineData("OPENSYSML_GRPC_BINARY=\"$PWD/bin/missing\" dotnet run --project " + SampleProject + " -c Release", false)]
    [InlineData("# " + SampleCommand, false)]
    [InlineData("false && " + SampleCommand, false)]
    [InlineData(SampleCommand + " --help", false)]
    public void OnlyTheCompleteServiceEnabledCommandIsAccepted(string command, bool accepted)
    {
        Assert.Equal(accepted, HasSampleCommand("```bash\n" + command + "\n```\n"));
    }

    [Fact]
    public void AWalkthroughTheGuideDoesNotShowIsRefused()
    {
        const string program = "using System;\nusing OpenMBEE.OpenSysML;\n\nif (true) { }\n\n// <walkthrough>\nConsole.WriteLine(1);\n// </walkthrough>\n";
        var expected = Walkthrough(program);

        Assert.Equal("using System;\nusing OpenMBEE.OpenSysML;\n\nConsole.WriteLine(1);", expected);
        Assert.Contains(expected, FencedBlocks("```csharp\n" + expected + "\n```\n", "csharp"));
        Assert.DoesNotContain(expected, FencedBlocks("```csharp\n" + expected.Replace("1", "2", StringComparison.Ordinal) + "\n```\n", "csharp"));
        Assert.DoesNotContain(expected, FencedBlocks("```csharp\nusing System;\n\nConsole.WriteLine(1);\n```\n", "csharp"));
        Assert.Throws<InvalidOperationException>(() => Walkthrough("using System;\nConsole.WriteLine(1);\n"));
        Assert.Throws<InvalidOperationException>(() => Walkthrough(program + RegionStart + "\n" + RegionEnd + "\n"));
    }

    private static string Read(string[] path) => File.ReadAllText(Repository.PathOf(path)).ReplaceLineEndings("\n");

    private static bool HasSampleCommand(string markdown) =>
        FencedBlocks(markdown, "bash").Any(block => block.Split('\n').Contains(SampleCommand, StringComparer.Ordinal));

    // The program's leading using directives, a blank line, then the lines between the markers.
    private static string Walkthrough(string program)
    {
        var lines = program.Split('\n');
        var usings = lines.TakeWhile(line => line.StartsWith("using ", StringComparison.Ordinal)).ToList();
        var starts = Indexes(lines, RegionStart);
        var ends = Indexes(lines, RegionEnd);
        if (usings.Count == 0 || starts.Count != 1 || ends.Count != 1 || ends[0] <= starts[0])
        {
            throw new InvalidOperationException(
                $"The program must open with its using directives and mark one region with {RegionStart} and {RegionEnd}.");
        }

        var region = lines[(starts[0] + 1)..ends[0]];
        return string.Join("\n", usings) + "\n\n" + string.Join("\n", region);
    }

    private static List<int> Indexes(string[] lines, string marker) =>
        [.. lines.Select((line, index) => (line, index)).Where(entry => entry.line == marker).Select(entry => entry.index)];

    // The content of each fenced block of the language, without its fences.
    private static List<string> FencedBlocks(string markdown, string language)
    {
        var blocks = new List<string>();
        var lines = markdown.Split('\n');
        for (var i = 0; i < lines.Length; i++)
        {
            if (lines[i] != "```" + language)
            {
                continue;
            }

            var end = Array.IndexOf(lines, "```", i + 1);
            if (end < 0)
            {
                break;
            }

            blocks.Add(string.Join("\n", lines[(i + 1)..end]));
            i = end;
        }

        return blocks;
    }
}
