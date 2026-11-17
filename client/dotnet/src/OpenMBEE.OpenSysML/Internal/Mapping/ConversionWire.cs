using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal.Mapping;

/// <summary>
/// Reads the conversion and migration answers into the client's immutable records, or raises the failure an answer
/// reports with everything it carried beside it. An answer of the wrong shape is a <see cref="TransportException"/>
/// with <see cref="StatusCode.Internal"/>.
/// </summary>
internal static class ConversionWire
{
    /// <summary>Reads a conversion.</summary>
    /// <exception cref="ConversionException">The answer reports that the conversion failed.</exception>
    /// <exception cref="TransportException">The answer is malformed.</exception>
    internal static Conversion Conversion(Wire.ConvertResponse response, string method)
    {
        Noticed(response.Experimental, response.ExperimentalNotice, method);
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            Unanswered(method, ("content", response.Content.Length > 0));
            throw new ConversionException(
                response.Error, method, diagnostics, response.FromFormat, response.ToFormat, response.Experimental, response.ExperimentalNotice);
        }

        if (response.FromFormat.Length == 0 || response.ToFormat.Length == 0)
        {
            throw TransportException.Malformed(method, "neither the formats of a conversion nor a failure");
        }

        return new Conversion(
            response.Content,
            response.FromFormat,
            response.ToFormat,
            diagnostics,
            response.Experimental,
            response.ExperimentalNotice,
            response.LibraryReferences.Select(reference => new LibraryReference(reference.Id, reference.QualifiedName, reference.Type, reference.LibraryPath)));
    }

    /// <summary>Reads a migration.</summary>
    /// <param name="response">The answer.</param>
    /// <param name="method">The method answered.</param>
    /// <param name="sourcePath">The full path of the v1 file migrated, when the source was one.</param>
    /// <exception cref="MigrationException">The answer reports that the migration failed.</exception>
    /// <exception cref="TransportException">The answer is malformed.</exception>
    internal static Migration Migration(Wire.MigrateResponse response, string method, string? sourcePath)
    {
        Noticed(response.Experimental, response.ExperimentalNotice, method);
        if (response.Error.Length > 0)
        {
            Unanswered(
                method,
                ("content", response.Content.Length > 0),
                ("report", response.Report is not null),
                ("results", response.Results.Length > 0),
                ("files", response.Files.Count > 0));
            throw new MigrationException(response.Error, method, response.FromFormat, response.ToFormat, response.Experimental, response.ExperimentalNotice);
        }

        if (response.Report is not { } report || response.FromFormat.Length == 0 || response.ToFormat.Length == 0)
        {
            throw TransportException.Malformed(method, "neither a migration with its formats and report nor a failure");
        }

        return new Migration(
            response.Content,
            response.FromFormat,
            response.ToFormat,
            Report(report),
            response.Results,
            Files(response.Files),
            response.Experimental,
            response.ExperimentalNotice,
            sourcePath);
    }

    /// <summary>Reads a migration's report.</summary>
    internal static MigrationReport Report(Wire.MigrationReport report) =>
        new(
            report.Source,
            report.Exporter,
            report.Summary,
            report.Mapped,
            report.Approximated,
            report.Unmapped,
            report.Skipped,
            report.Entries.Select(entry => new MigrationEntry(entry.Id, entry.Kind, entry.Name, entry.Target, entry.Verdict, entry.Note)),
            report.Text);

    /// <summary>Reads the files a migration refers to, in order.</summary>
    internal static IEnumerable<MigrationFile> Files(IEnumerable<Wire.MigrationFile> files) =>
        files.Select(file => new MigrationFile(file.Path, file.Content.Span));

    // The schema leaves the notice empty when the answer is not experimental; one that is not is malformed.
    private static void Noticed(bool experimental, string notice, string method)
    {
        if (!experimental && notice.Length > 0)
        {
            throw TransportException.Malformed(method, "an experimental notice for a result it did not mark experimental");
        }
    }

    // An answer reporting a failure carries none of a result's own fields; one that does is malformed.
    private static void Unanswered(string method, params (string Field, bool Set)[] answer)
    {
        foreach (var (field, set) in answer)
        {
            if (set)
            {
                throw TransportException.Malformed(method, $"a failure beside {field}, which only an answer reports");
            }
        }
    }
}
