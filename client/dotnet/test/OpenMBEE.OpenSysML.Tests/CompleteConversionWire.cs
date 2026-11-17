using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Answers that set every field the conversion and migration responses can reach, and each failure in band.</summary>
internal static class CompleteConversionWire
{
    /// <summary>The bytes of a picture a migrated model refers to.</summary>
    internal static readonly byte[] Picture = [0x89, (byte)'P', (byte)'N', (byte)'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff];

    /// <summary>Notation written as Turtle, tolerated and checked, through the experimental mapping.</summary>
    internal static Wire.ConvertResponse Conversion => new()
    {
        Content = "@prefix sysml: <https://www.omg.org/spec/SysML/20250201/> .\n",
        FromFormat = "sysml",
        ToFormat = "ttl",
        Diagnostics = { CompleteWire.Diagnostic, CompleteWire.BareDiagnostic },
        Experimental = true,
        ExperimentalNotice = "RDF conversion is experimental",
    };

    /// <summary>Notation written as the API's JSON, with the standard-library elements it refers to listed beside it.</summary>
    internal static Wire.ConvertResponse ApiJsonConversion => new()
    {
        Content = "[{\"@id\":\"P\",\"@type\":\"Package\"}]",
        FromFormat = "sysml",
        ToFormat = "api-json",
        Experimental = true,
        ExperimentalNotice = "API JSON conversion is experimental",
        LibraryReferences =
        {
            new Wire.LibraryReference { Id = "0c539f7c-20a1-520d-bae0-f3f855517aea", QualifiedName = "SI::second", Type = "AttributeUsage" },
            new Wire.LibraryReference { Id = "2a413ac9-972d-5182-afd3-46cdde4e0eba", QualifiedName = "SI", Type = "LibraryPackage" },
            new Wire.LibraryReference { Id = "1e506913-dbf3-55da-a841-56afa1d5e151", Type = "BindingConnector", LibraryPath = "Occurrences::Occurrence/BindingConnector[0]" },
        },
    };

    /// <summary>A conversion the mapping refused, keeping its formats, diagnostics and notice.</summary>
    internal static Wire.ConvertResponse FailedConversion => new()
    {
        FromFormat = "sysml",
        ToFormat = "api-json",
        Error = "cannot write feature declaration",
        Diagnostics = { CompleteWire.Diagnostic },
        Experimental = true,
        ExperimentalNotice = "RDF conversion is experimental",
    };

    /// <summary>A migration with its full report, results and two files.</summary>
    internal static Wire.MigrateResponse Migration => new()
    {
        Content = "package Vehicle {\n    part def Vehicle;\n}\n",
        FromFormat = "xmi",
        ToFormat = "sysml",
        Experimental = true,
        ExperimentalNotice = "SysML v1 migration is experimental",
        Report = new Wire.MigrationReport
        {
            Source = "<content>",
            Exporter = "MagicDraw Clean XMI Exporter",
            Summary = "migrated 4 element(s): 1 mapped, 1 approximated, 1 unmapped (1 skipped)",
            Mapped = 1,
            Approximated = 1,
            Unmapped = 1,
            Skipped = 1,
            Entries =
            {
                new Wire.MigrationEntry { Id = "_b1", Kind = "«Block»", Name = "Vehicle", Target = "part def Vehicle::Vehicle", Verdict = "mapped" },
                new Wire.MigrationEntry { Id = "_p1", Kind = "«ValueProperty»", Name = "Vehicle::mass", Target = "attribute Vehicle::Vehicle::mass", Verdict = "approximated", Note = "unit restated" },
                new Wire.MigrationEntry { Id = "_d1", Kind = "«DeferredEvent»", Name = "Vehicle::wait", Verdict = "unmapped", Note = "no v2 form" },
                new Wire.MigrationEntry { Id = "_s1", Kind = "Profile", Name = "SysML", Verdict = "skipped", Note = "profile" },
            },
            Text = "# SysML v1 to v2 migration report\n",
        },
        Results = "{\n  \"runs\": []\n}\n",
        Files =
        {
            new Wire.MigrationFile { Path = "images/fleet.png", Content = ByteString.CopyFrom(Picture) },
            new Wire.MigrationFile { Path = "images/empty.png" },
        },
    };

    /// <summary>A migration that could not read the model, keeping its formats and notice.</summary>
    internal static Wire.MigrateResponse FailedMigration => new()
    {
        FromFormat = "xmi",
        ToFormat = "ttl",
        Error = "<content>: not an XMI document",
        Experimental = true,
        ExperimentalNotice = "SysML v1 migration is experimental",
    };
}
