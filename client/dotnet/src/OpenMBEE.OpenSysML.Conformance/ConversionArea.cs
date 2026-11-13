using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Converting and migrating through <see cref="Connection"/>'s calls, and through <see cref="Model"/>'s for a model
/// named by hash. A conversion or migration the service answered but could not make reaches the caller as a
/// <see cref="ConversionException"/> or <see cref="MigrationException"/> keeping the formats and diagnostics, which are
/// rendered back in band.
/// </summary>
internal static class ConversionArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.ConvertRequest>(
            "Convert",
            [
                "file_path", "content", "model_hash", "from_format", "to_format", "tolerate_syntax_errors", "id_form", "documents", "compact", "omit_derived",
                "keep_derived", "include_check_diagnostics", "include_library_stubs",
            ],
            ConvertAsync),
        RpcAdapter.Of<Wire.MigrateRequest>(
            "Migrate",
            ["file_path", "content", "from_format", "to_format", "report", "results", "layout_path", "layout_content", "image_base_url", "strict"],
            MigrateAsync),
    ];

    private static async Task<ScenarioAnswer> ConvertAsync(Connection connection, Wire.ConvertRequest request, CancellationToken cancellationToken)
    {
        var options = new ConvertOptions
        {
            FromFormat = Named(request.FromFormat),
            TolerateSyntaxErrors = request.TolerateSyntaxErrors,
            IdForm = request.IdForm switch
            {
                "" => null,
                "qualified" => IdForm.Qualified,
                "uuid" => IdForm.Uuid,
                _ => throw new UnexpressibleRequestException($"the public API names an id form as an IdForm, which has none spelled \"{request.IdForm}\""),
            },
            Documents = request.Documents,
            Compact = request.Compact,
            OmitDerived = request.OmitDerived,
            KeepDerived = request.KeepDerived,
            IncludeCheckDiagnostics = request.IncludeCheckDiagnostics,
            IncludeLibraryStubs = request.IncludeLibraryStubs,
        };
        Task<Conversion> converting = request.SourceCase switch
        {
            Wire.ConvertRequest.SourceOneofCase.FilePath when request.FilePath.Length == 0 =>
                throw new UnexpressibleRequestException("the public API names a file by a path that is not empty"),
            Wire.ConvertRequest.SourceOneofCase.FilePath => connection.ConvertAsync(ConvertSource.FromFile(request.FilePath), request.ToFormat, options, cancellationToken),
            Wire.ConvertRequest.SourceOneofCase.Content => connection.ConvertAsync(ConvertSource.Inline(request.Content), request.ToFormat, options, cancellationToken),
            Wire.ConvertRequest.SourceOneofCase.ModelHash => ApiAdapters.ModelOf(connection, request.ModelHash).ConvertAsync(request.ToFormat, options, cancellationToken),
            _ => throw new UnexpressibleRequestException("the public API always names a source, so it cannot send a request naming none"),
        };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.Conversion(await converting.ConfigureAwait(false)));
        }
        catch (ConversionException failure)
        {
            return new ScenarioAnswer.Answered(Rendering.Conversion(failure));
        }
    }

    private static async Task<ScenarioAnswer> MigrateAsync(Connection connection, Wire.MigrateRequest request, CancellationToken cancellationToken)
    {
        var options = new MigrateOptions
        {
            FromFormat = Named(request.FromFormat),
            Report = request.Report,
            Results = request.Results,
            LayoutPath = request.LayoutCase == Wire.MigrateRequest.LayoutOneofCase.LayoutPath ? request.LayoutPath : null,
            LayoutContent = request.LayoutCase == Wire.MigrateRequest.LayoutOneofCase.LayoutContent ? request.LayoutContent : null,
            ImageBaseUrl = Named(request.ImageBaseUrl),
            Strict = request.Strict,
        };
        var source = request.SourceCase switch
        {
            Wire.MigrateRequest.SourceOneofCase.FilePath when request.FilePath.Length == 0 =>
                throw new UnexpressibleRequestException("the public API names a file by a path that is not empty"),
            Wire.MigrateRequest.SourceOneofCase.FilePath => FileSource(request.FilePath),
            Wire.MigrateRequest.SourceOneofCase.Content => MigrateSource.Inline(request.Content.Span),
            _ => throw new UnexpressibleRequestException("the public API always names a source, so it cannot send a request naming none"),
        };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.Migration(await connection.MigrateAsync(source, request.ToFormat, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (MigrationException failure)
        {
            return new ScenarioAnswer.Answered(Rendering.Migration(failure));
        }
    }

    private static MigrateSource FileSource(string path)
    {
        MigrateSource source;
        try
        {
            source = MigrateSource.FromFile(path);
        }
        catch (ArgumentException)
        {
            throw new UnexpressibleRequestException("the public API requires a fully-qualified, valid migration file path");
        }

        return source.FilePath == path
            ? source
            : throw new UnexpressibleRequestException("the public API normalizes a migration file path before sending it");
    }

    // A field the request leaves empty is an option the API leaves null.
    private static string? Named(string value) => value.Length > 0 ? value : null;
}
