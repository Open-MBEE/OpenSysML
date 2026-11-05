using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapter renders what ApplyEdits returned as the wire message, reading only public types. Every field
/// the response can reach is set in a sample, and each sample, read by the client and rendered back, comes out as it
/// went in: an applied edit, a document, a diagnostic or a referrer the client dropped, or one the rendering forgot,
/// fails here, as does notation a model of several documents was given that it never had.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceEditingRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheApplyEditsResponseReaches()
    {
        var reachable = CompleteWire.ReachableFields(Wire.ApplyEditsResponse.Descriptor);
        IMessage[] samples = [.. CompleteEditingWire.All];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void AnEditedModelIsRenderedWithEveryDocumentAndAppliedEditAndNoContentForSeveral()
    {
        foreach (var (sample, documents, editsDocuments) in new (Wire.ApplyEditsResponse, int?, bool)[]
        {
            (CompleteEditingWire.Single, 1, true),
            (CompleteEditingWire.Single, null, true),
            (CompleteEditingWire.Several, 2, true),
            (CompleteEditingWire.Several, null, true),
            (CompleteEditingWire.Bare, null, false),
            (new Wire.ApplyEditsResponse(), null, true),
            (new Wire.ApplyEditsResponse(), 1, false),
        })
        {
            Assert.Equal(sample, Rendering.EditResult(EditWire.Result(sample, Model.ApplyEditsMethod, 2, documents, editsDocuments)));
        }
    }

    [Fact]
    public void ARefusalIsRenderedInBandWithItsKindDiagnosticsAndReferrers()
    {
        var newer = new Wire.ApplyEditsResponse { Error = "a refusal of a newer kind", Failure = (Wire.EditFailure)99 };
        foreach (var sample in new[] { CompleteEditingWire.Refusal, newer })
        {
            var refused = Assert.Throws<EditException>(() => EditWire.Result(sample, Model.ApplyEditsMethod, 1, null, true));

            Assert.Equal(sample, Rendering.EditResult(refused));
        }
    }
}
