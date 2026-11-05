using System;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapter renders what ExportGraphs, ImportData and CompareResults returned as the wire messages,
/// reading only public types. Every field those responses can reach is set in a sample, and each sample, read by the
/// client and rendered back, comes out as it went in: a byte of a graph's form, a file, an assignment, a document, a
/// configuration, a distribution or a case statistic the client dropped, or one the rendering forgot, fails here, as does
/// a distribution, difference or seed the client fabricated where the answer had none.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceGraphDataRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheGraphDataResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(Wire.ExportGraphsResponse.Descriptor, Wire.ImportDataResponse.Descriptor, Wire.CompareResultsResponse.Descriptor);
        IMessage[] samples =
        [
            .. CompleteGraphDataWire.All,
            .. CompleteWire.Values,
            CompleteWire.Diagnostic,
            CompleteWire.Instance,
            CompleteVerificationWire.Verdict,
            CompleteVerificationWire.Undecided,
            CompleteVerificationWire.Sweep,
            CompleteVerificationWire.Analysis,
        ];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void AGraphFormIsRenderedByteForByte()
    {
        var sample = CompleteGraphDataWire.Graphs;

        Assert.Equal(sample, Rendering.Graphs(GraphDataWire.Graphs(sample, Model.ExportGraphsMethod)));
    }

    [Fact]
    public async Task AnImportIsRenderedWithEveryFileDocumentAndTheModelItMadeAndADryRunWithNone()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var nothing = new Wire.ImportDataResponse { Files = { new Wire.ImportedFile { Name = "fleet.csv" }, new Wire.ImportedFile { Name = "fleet.txt" } }, ModelHash = "h" };
        foreach (var (sample, dryRun, documents) in new (Wire.ImportDataResponse, bool, int?)[]
        {
            (CompleteGraphDataWire.Imported, false, 1),
            (CompleteGraphDataWire.Imported, false, null),
            (CompleteGraphDataWire.ImportedSeveral, false, 2),
            (CompleteGraphDataWire.ImportedSeveral, false, null),
            (CompleteGraphDataWire.DryRun, true, null),
            (nothing, false, 1),
        })
        {
            var read = GraphDataWire.Import(sample, Model.ImportDataMethod, CompleteGraphDataWire.Imports, dryRun, "h", documents, connection.ModelByHash);

            Assert.Equal(sample, Rendering.ImportResult(read));
        }
    }

    [Fact]
    public void ARefusedImportIsRenderedInBandWithItsDiagnostics()
    {
        var sample = CompleteGraphDataWire.Refusal;

        var refused = Assert.Throws<ModelException>(() => GraphDataWire.Import(sample, Model.ImportDataMethod, CompleteGraphDataWire.Imports, false, "h", null, _ => throw new InvalidOperationException()));

        Assert.Equal(sample, Rendering.ImportResult(refused));
    }

    [Fact]
    public void AComparisonIsRenderedWithEveryFactAndNoneItLacked()
    {
        foreach (var sample in new[] { CompleteGraphDataWire.Comparison, new Wire.CompareResultsResponse() })
        {
            Assert.Equal(sample, Rendering.Comparison(GraphDataWire.Comparison(sample, Model.CompareResultsMethod)));
        }
    }

    [Fact]
    public void ResultsNothingOfWhichWasComparedAreRenderedInBandWithTheirKind()
    {
        var newer = new Wire.CompareResultsResponse { Error = "a failure of a newer kind", FailureReason = (Wire.FailureReason)99 };
        var partial = CompleteGraphDataWire.Comparison;
        partial.Error = "only some were compared";
        foreach (var sample in new[] { CompleteGraphDataWire.CompareRefusal, newer, partial })
        {
            var refused = Assert.Throws<CompareException>(() => GraphDataWire.Comparison(sample, Model.CompareResultsMethod));

            Assert.Equal(sample, Rendering.Comparison(refused));
        }
    }
}
