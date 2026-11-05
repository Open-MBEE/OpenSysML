using System;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Connection
{
    internal const string ConvertMethod = "sysml.SysMLService/Convert";
    internal const string MigrateMethod = "sysml.SysMLService/Migrate";

    /// <summary>
    /// Writes a model out in another format, as <c>sysml -convert</c> does: SysML or KerML notation, RDF Turtle, or the
    /// SysML v2 API's JSON element form, from a file, inline content, or a model the service parsed. Which directions
    /// and options apply is the service's to judge, as is a SysML v1 source, which it refuses with
    /// <see cref="StatusCode.InvalidArgument"/> pointing at <see cref="MigrateAsync"/>: a v1 model is migrated, element
    /// by element and reported, not converted.
    /// </summary>
    /// <param name="source">The model to convert.</param>
    /// <param name="toFormat">The format to write, named as <see cref="ConversionFormat"/> names one; the service refuses an empty or unknown one, or a v1 format.</param>
    /// <param name="options">The source's format, how tolerant to be, how ids are spelled, the documents written, the compact form, the check's diagnostics and the library stubs; null for <see cref="ConvertOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The converted model, its formats, its diagnostics, the library elements it refers to, and whether the mapping it went through is experimental.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="source"/> or <paramref name="toFormat"/> is null.</exception>
    /// <exception cref="InvalidRequestException"><see cref="ConvertOptions.FromFormat"/> is empty, or <see cref="ConvertOptions.IdForm"/> is not an <see cref="IdForm"/>; nothing was sent.</exception>
    /// <exception cref="CapabilityException">
    /// The service lacks <see cref="Capabilities.Convert"/>, or <see cref="Capabilities.ConvertDocuments"/>,
    /// <see cref="Capabilities.ConvertLibraryStubs"/>, <see cref="Capabilities.ConvertCompact"/> or
    /// <see cref="Capabilities.ConvertCheckDiagnostics"/> for an option that needs it, checked in that order; nothing was sent.
    /// </exception>
    /// <exception cref="ConversionException">The source would not parse, or the mapping refused what it cannot write; the exception keeps the diagnostics and formats.</exception>
    /// <exception cref="ModelFileNotFoundException">The service could not read the file.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a format it cannot read or write, a v1 source, or an option given where it does not apply.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Conversion> ConvertAsync(ConvertSource source, string toFormat, ConvertOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(toFormat);
        ThrowIfDisposed();
        options ??= ConvertOptions.Default;
        if (options.FromFormat is { Length: 0 })
        {
            throw new InvalidRequestException("ConvertOptions.FromFormat is empty; leave it null to infer the source's format.");
        }

        if (options.IdForm is { } form && !Enum.IsDefined(form))
        {
            throw new InvalidRequestException($"ConvertOptions.IdForm is {form}, which is not an IdForm.");
        }

        // The order the service checks them in, so the first refusal is the one it would make.
        var capabilities = Capabilities;
        capabilities.Require(OpenSysML.Capabilities.Convert);
        if (options.Documents.Count > 0)
        {
            capabilities.Require(OpenSysML.Capabilities.ConvertDocuments);
        }

        if (options.IncludeLibraryStubs)
        {
            capabilities.Require(OpenSysML.Capabilities.ConvertLibraryStubs);
        }

        if (options.AsksCompact)
        {
            capabilities.Require(OpenSysML.Capabilities.ConvertCompact);
        }

        if (options.IncludeCheckDiagnostics)
        {
            capabilities.Require(OpenSysML.Capabilities.ConvertCheckDiagnostics);
        }

        var request = new Wire.ConvertRequest
        {
            FromFormat = options.FromFormat ?? string.Empty,
            ToFormat = toFormat,
            TolerateSyntaxErrors = options.TolerateSyntaxErrors,
            IdForm = options.IdForm switch
            {
                IdForm.Qualified => "qualified",
                IdForm.Uuid => "uuid",
                _ => string.Empty,
            },
            Documents = { options.Documents },
            Compact = options.Compact,
            OmitDerived = options.OmitDerived,
            KeepDerived = { options.KeepDerived },
            IncludeCheckDiagnostics = options.IncludeCheckDiagnostics,
            IncludeLibraryStubs = options.IncludeLibraryStubs,
        };
        Wire.ConvertResponse response;
        if (source.FilePath is { } file)
        {
            request.FilePath = file;
            response = await FileCallAsync<Wire.ConvertRequest, Wire.ConvertResponse>(ConvertMethod, request, cancellationToken).ConfigureAwait(false);
        }
        else if (source.ModelHash is { } hash)
        {
            request.ModelHash = hash;
            response = await ModelCallAsync<Wire.ConvertRequest, Wire.ConvertResponse>(ConvertMethod, request, cancellationToken).ConfigureAwait(false);
        }
        else
        {
            request.Content = source.Content!;
            response = await UnaryAsync<Wire.ConvertRequest, Wire.ConvertResponse>(ConvertMethod, request, cancellationToken).ConfigureAwait(false);
        }

        return ConversionWire.Conversion(response, ConvertMethod);
    }

    /// <summary>
    /// Migrates a SysML v1 model to v2, as <c>sysml -migrate</c> does: UML XMI with the SysML profile applied, an
    /// Eclipse UML2 file or a Cameo/MagicDraw archive, written in one of the formats <see cref="ConvertAsync"/> writes.
    /// Migration is ledgered, not lossless: every element lands in <see cref="Migration.Report"/> as mapped,
    /// approximated, unmapped or skipped, and an element without a v2 form is reported, never raised. A v2 source is
    /// the service's to refuse with <see cref="StatusCode.InvalidArgument"/>, pointing at <see cref="ConvertAsync"/>.
    /// Nothing is written to disk: <see cref="Migration.WriteAsync"/> writes the model and its files.
    /// </summary>
    /// <param name="source">The v1 model.</param>
    /// <param name="toFormat">The format to write, named as <see cref="ConversionFormat"/> names one; the service refuses an empty or unknown one, or a v1 format.</param>
    /// <param name="options">The v1 form, the full report, the results, a layout, an image base URL and strictness; null for <see cref="MigrateOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The migrated model, its report, and the results and files asked for.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="source"/> or <paramref name="toFormat"/> is null.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty string, or both a layout path and layout content; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.Migrate"/>; nothing was sent.</exception>
    /// <exception cref="MigrationException">The v1 model could not be migrated at all.</exception>
    /// <exception cref="ModelFileNotFoundException">The service could not read the file or the layout.</exception>
    /// <exception cref="ServiceException">The service refused the call, as it does a v2 source, inline content naming no form, a format it cannot write, or a layout it cannot read.</exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Migration> MigrateAsync(MigrateSource source, string toFormat, MigrateOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(toFormat);
        ThrowIfDisposed();
        options ??= MigrateOptions.Default;
        if (options.FromFormat is { Length: 0 } || options.LayoutPath is { Length: 0 } || options.LayoutContent is { Length: 0 } || options.ImageBaseUrl is { Length: 0 })
        {
            throw new InvalidRequestException(
                "MigrateOptions names an empty form, layout or image base URL; leave FromFormat, LayoutPath, LayoutContent or ImageBaseUrl null for none.");
        }

        if (options.LayoutPath is not null && options.LayoutContent is not null)
        {
            throw new InvalidRequestException("MigrateOptions names both a LayoutPath and LayoutContent; a migration is laid out by one MTIP export.");
        }

        Capabilities.Require(OpenSysML.Capabilities.Migrate);
        var request = new Wire.MigrateRequest
        {
            FromFormat = options.FromFormat ?? string.Empty,
            ToFormat = toFormat,
            Report = options.Report,
            Results = options.Results,
            ImageBaseUrl = options.ImageBaseUrl ?? string.Empty,
            Strict = options.Strict,
        };
        if (source.FilePath is { } file)
        {
            request.FilePath = file;
        }
        else
        {
            request.Content = ByteString.CopyFrom(source.Content!.Value.Span);
        }

        if (options.LayoutPath is { } layoutPath)
        {
            request.LayoutPath = layoutPath;
        }
        else if (options.LayoutContent is { } layoutContent)
        {
            request.LayoutContent = layoutContent;
        }

        var response = await FileCallAsync<Wire.MigrateRequest, Wire.MigrateResponse>(MigrateMethod, request, cancellationToken).ConfigureAwait(false);
        return ConversionWire.Migration(response, MigrateMethod, source.FilePath);
    }
}
