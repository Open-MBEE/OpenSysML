using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Tests;
using Xunit;

namespace OpenMBEE.OpenSysML.Generate.Tests;

/// <summary>
/// The committed goldens are what their documents generate. Offline, each records the stamp of its documents'
/// current bytes and this generator's version, so an edited input without a regenerated golden fails even
/// without a service; live, generating from the documents over any wire gives the committed file exactly.
/// </summary>
public sealed class GoldenTests(LiveServices services) : IClassFixture<LiveServices>
{
    internal const string Regenerate = "make dotnet-generate-goldens";

    private static readonly string Manifest = Repository.PathOf("client", "dotnet", "test", "testdata", "generate", "goldens.json");
    private static readonly JsonSerializerOptions ManifestOptions = new() { PropertyNameCaseInsensitive = true };

    public static TheoryData<string> Outputs => [.. Goldens().Select(golden => golden.Output)];

    public static TheoryData<ClientWire, string> WiresAndOutputs
    {
        get
        {
            var data = new TheoryData<ClientWire, string>();
            foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
            {
                foreach (var golden in Goldens())
                {
                    data.Add(wire, golden.Output);
                }
            }

            return data;
        }
    }

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Fact]
    [Trait(Category.Name, Category.Unit)]
    public void TheManifestNamesEveryGoldenTheConsumerCompiles()
    {
        var compiled = Directory.GetFiles(Repository.PathOf("client", "dotnet", "test", "OpenMBEE.OpenSysML.Generate.Consumer", "Golden"))
            .Select(path => Path.GetRelativePath(Repository.Root, path).Replace('\\', '/'))
            .Order(StringComparer.Ordinal);

        Assert.Equal(compiled, Goldens().Select(golden => golden.Output).Order(StringComparer.Ordinal));
    }

    [Theory]
    [MemberData(nameof(Outputs))]
    [Trait(Category.Name, Category.Unit)]
    public void AGoldenRecordsItsDocumentsCurrentStampAndThisGeneratorsVersion(string output)
    {
        var golden = Goldens().Single(entry => entry.Output == output);
        var text = File.ReadAllText(Repository.PathOf(output));
        var documents = SourceClosure.Read(golden.Sources.Select(source => Repository.PathOf(source)));

        Assert.Contains($"public const string ModelHash = \"{SourceClosure.Stamp(documents)}\";", text, StringComparison.Ordinal);
        Assert.Contains($"public const string GeneratorVersion = \"{Emitter.GeneratorVersion}\";", text, StringComparison.Ordinal);
        Assert.Contains($"namespace {golden.Namespace}\n", text, StringComparison.Ordinal);
        Assert.All(documents, document => Assert.Contains($"//     Document: {document.LogicalName}\n", text, StringComparison.Ordinal));
    }

    [Theory]
    [MemberData(nameof(WiresAndOutputs))]
    [Trait(Category.Name, Category.Integration)]
    public async Task GeneratingFromTheDocumentsGivesTheGoldenOverEveryWire(ClientWire wire, string output)
    {
        var golden = Goldens().Single(entry => entry.Output == output);
        await using var connection = await services.OpenAsync(wire, Token);

        var generated = await GenerateCommand.GenerateAsync(
            connection, SourceClosure.Read(golden.Sources.Select(source => Repository.PathOf(source))), golden.Namespace, Token);

        Assert.True(generated == File.ReadAllText(Repository.PathOf(output)), $"{output} is not what its documents generate; regenerate it with {Regenerate}");
    }

    internal static IReadOnlyList<Golden> Goldens() =>
        JsonSerializer.Deserialize<List<Golden>>(File.ReadAllText(Manifest), ManifestOptions)
            ?? throw new InvalidOperationException($"{Manifest} names no golden.");

    internal sealed record Golden(IReadOnlyList<string> Sources, string Namespace, string Output);
}
