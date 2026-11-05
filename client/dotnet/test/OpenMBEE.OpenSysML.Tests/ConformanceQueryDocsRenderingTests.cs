using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters render what the query, document query, document and view calls returned as wire messages,
/// reading only public types. Every field those responses can reach is set in a sample, and each sample, read by the
/// client and rendered back, comes out as it went in: a property, a cell, a port, a geometry, a style, a route point, a
/// note or a notice the client dropped, or one the rendering forgot, fails here, as does an absent message the client
/// filled in.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceQueryDocsRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheQueryDocumentAndViewResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(
            Wire.QueryResponse.Descriptor, Wire.RunDocumentQueryResponse.Descriptor, Wire.RenderDocumentResponse.Descriptor, Wire.RenderViewResponse.Descriptor);
        IMessage[] samples = [.. CompleteQueryDocsWire.All, .. CompleteWire.Values];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void WhatAQuerySelectedIsRenderedWithEveryProperty()
    {
        foreach (var sample in new[] { CompleteQueryDocsWire.Query, new Wire.QueryResponse() })
        {
            Assert.Equal(sample, Rendering.QueryResult(QueryWire.Result(sample, Model.QueryMethod)));
        }
    }

    [Fact]
    public void ADocumentQuerysRowsAreRenderedWithEveryValueInItsArm()
    {
        var empty = new Wire.RunDocumentQueryResponse { Columns = { new Wire.DocumentQueryColumn { Name = "name" } } };
        var sparse = new Wire.RunDocumentQueryResponse
        {
            Rows = { new Wire.DocumentQueryRow { Element = new Wire.DocumentValue(), Cells = { new Wire.DocumentQueryCell { Values = { CompleteExecutionWire.SparseDocumentValues } } } } },
        };
        foreach (var sample in new[] { CompleteQueryDocsWire.DocumentQuery, empty, sparse })
        {
            Assert.Equal(sample, Rendering.DocumentQueryResult(DocumentWire.Result(sample, Model.RunDocumentQueryMethod)));
        }
    }

    [Theory]
    [InlineData(DocumentForm.Markdown)]
    [InlineData(DocumentForm.Html)]
    [InlineData(DocumentForm.AsciiDoc)]
    public void ARenderedDocumentIsRenderedInTheFieldOfItsFormWithItsFileName(DocumentForm form)
    {
        var (sample, content) = form switch
        {
            DocumentForm.Html => (CompleteQueryDocsWire.Html, CompleteQueryDocsWire.Html.Html),
            DocumentForm.AsciiDoc => (CompleteQueryDocsWire.AsciiDoc, CompleteQueryDocsWire.AsciiDoc.Asciidoc),
            _ => (CompleteQueryDocsWire.Markdown, CompleteQueryDocsWire.Markdown.Markdown),
        };

        Assert.Equal(sample, Rendering.RenderedDocument(new RenderedDocument(form, content, sample.FileName)));
        Assert.Equal(new Wire.RenderDocumentResponse { Markdown = "# T\n" }, Rendering.RenderedDocument(new RenderedDocument(DocumentForm.Markdown, "# T\n", null)));
    }

    [Fact]
    public void AViewIsRenderedWithEveryFieldAndWithoutWhatItOmits()
    {
        foreach (var sample in new[] { CompleteQueryDocsWire.View, CompleteQueryDocsWire.SparseView, new Wire.RenderViewResponse() })
        {
            Assert.Equal(sample, Rendering.RenderedView(ViewWire.Rendered(sample, Model.RenderViewMethod)));
        }
    }
}
