using System;
using System.Linq;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance adapters render what the conversion and migration calls returned as wire messages, reading only
/// public types. Every field those responses can reach is set in a sample, and each sample, read by the client and
/// rendered back, comes out as it went in: a format, a diagnostic, a verdict, a file or a failure's notice the client
/// dropped, or one the rendering forgot, fails here.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceConversionRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheConversionAndMigrationResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(Wire.ConvertResponse.Descriptor, Wire.MigrateResponse.Descriptor);
        IMessage[] samples =
        [
            CompleteConversionWire.Conversion,
            CompleteConversionWire.ApiJsonConversion,
            CompleteConversionWire.FailedConversion,
            CompleteConversionWire.Migration,
            CompleteConversionWire.FailedMigration,
            CompleteWire.Diagnostic,
        ];

        Assert.Equal([], reachable.Except(CompleteWire.SetFields(samples), StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void AConversionIsRenderedWithItsFormatsDiagnosticsNoticeAndLibraryReferences()
    {
        var notation = new Wire.ConvertResponse { Content = "package P;\n", FromFormat = "sysml", ToFormat = "sysml" };
        foreach (var sample in new[] { CompleteConversionWire.Conversion, CompleteConversionWire.ApiJsonConversion, notation, new Wire.ConvertResponse { FromFormat = "sysml", ToFormat = "sysml" } })
        {
            Assert.Equal(sample, Rendering.Conversion(ConversionWire.Conversion(sample, Connection.ConvertMethod)));
        }
    }

    [Fact]
    public void AConversionNotMadeIsRenderedWithWhatItsFailureKept()
    {
        var unreadable = new Wire.ConvertResponse { Error = "1 syntax error", FromFormat = "sysml", ToFormat = "sysml", Diagnostics = { CompleteWire.BareDiagnostic } };
        foreach (var sample in new[] { CompleteConversionWire.FailedConversion, unreadable, new Wire.ConvertResponse { Error = "e" } })
        {
            var failure = Assert.Throws<ConversionException>(() => ConversionWire.Conversion(sample, Connection.ConvertMethod));
            Assert.Equal(sample, Rendering.Conversion(failure));
        }
    }

    [Fact]
    public void AMigrationIsRenderedWithItsReportResultsAndFiles()
    {
        var summary = new Wire.MigrateResponse
        {
            Content = "package P;\n",
            FromFormat = "xmi",
            ToFormat = "sysml",
            Experimental = true,
            ExperimentalNotice = "n",
            Report = new Wire.MigrationReport { Source = "<content>", Summary = "migrated 0 element(s)" },
        };
        foreach (var sample in new[] { CompleteConversionWire.Migration, summary })
        {
            Assert.Equal(sample, Rendering.Migration(ConversionWire.Migration(sample, Connection.MigrateMethod, null)));
        }
    }

    [Fact]
    public void AMigrationNotMadeIsRenderedWithWhatItsFailureKept()
    {
        foreach (var sample in new[] { CompleteConversionWire.FailedMigration, new Wire.MigrateResponse { Error = "e" } })
        {
            var failure = Assert.Throws<MigrationException>(() => ConversionWire.Migration(sample, Connection.MigrateMethod, null));
            Assert.Equal(sample, Rendering.Migration(failure));
        }
    }
}
