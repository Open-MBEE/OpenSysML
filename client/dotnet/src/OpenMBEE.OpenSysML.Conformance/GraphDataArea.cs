using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Exporting a behavior's lowered graph through <see cref="Model.ExportGraphsAsync"/>, importing data through
/// <see cref="Model.ImportDataAsync"/> and comparing a tool's results through <see cref="Model.CompareResultsAsync"/>.
/// An import the model refused, or results nothing of which could be compared, reach the caller as an exception keeping
/// what the answer reported, which is rendered back in band.
/// </summary>
internal static class GraphDataArea
{
    /// <summary>The area's adapters.</summary>
    internal static IEnumerable<RpcAdapter> Adapters { get; } =
    [
        RpcAdapter.Of<Wire.ExportGraphsRequest>("ExportGraphs", ["model_hash", "subject"], ExportGraphsAsync),
        RpcAdapter.Of<Wire.ImportDataRequest>("ImportData", ["model_hash", "imports", "import_as", "import_dry_run", "accept_documents"], ImportDataAsync),
        RpcAdapter.Of<Wire.CompareResultsRequest>(
            "CompareResults",
            ["model_hash", "results", "actions", "runs", "seed", "draws", "clock_step", "observe", "engine"],
            CompareResultsAsync),
    ];

    private static async Task<ScenarioAnswer> ExportGraphsAsync(Connection connection, Wire.ExportGraphsRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (request.Subject.Length == 0)
        {
            throw new UnexpressibleRequestException("the public API names a behavior by a qualified name that is not empty");
        }

        return new ScenarioAnswer.Answered(Rendering.Graphs(await model.ExportGraphsAsync(request.Subject, cancellationToken).ConfigureAwait(false)));
    }

    private static async Task<ScenarioAnswer> ImportDataAsync(Connection connection, Wire.ImportDataRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        var imports = request.Imports.Select((file, i) => Import(file, $"imports[{i}]")).ToList();
        var options = new ImportOptions { As = Named(request.ImportAs), DryRun = request.ImportDryRun, AcceptDocuments = request.AcceptDocuments };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.ImportResult(await model.ImportDataAsync(imports, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (ModelException refused) when (refused.GetType() == typeof(ModelException))
        {
            return new ScenarioAnswer.Answered(Rendering.ImportResult(refused));
        }
    }

    private static async Task<ScenarioAnswer> CompareResultsAsync(Connection connection, Wire.CompareResultsRequest request, CancellationToken cancellationToken)
    {
        var model = ApiAdapters.ModelOf(connection, request.ModelHash);
        if (request.Results.Length == 0)
        {
            throw new UnexpressibleRequestException("the public API always names the results compared, so it cannot send a request naming none");
        }

        var options = new CompareOptions
        {
            Actions = request.Actions,
            Runs = request.Runs,
            Seed = request.HasSeed ? request.Seed : null,
            Draws = Named(request.Draws),
            ClockStep = request.HasClockStep ? request.ClockStep : null,
            Observe = [.. request.Observe.Select((pair, i) => Observable(pair, $"observe[{i}]"))],
            Engine = Named(request.Engine),
        };
        try
        {
            return new ScenarioAnswer.Answered(Rendering.Comparison(await model.CompareResultsAsync(request.Results, options, cancellationToken).ConfigureAwait(false)));
        }
        catch (CompareException failure)
        {
            return new ScenarioAnswer.Answered(Rendering.Comparison(failure));
        }
    }

    private static DataImport Import(Wire.DataImport file, string path)
    {
        if (file.Name.Length == 0)
        {
            throw new UnexpressibleRequestException($"the public API names a data file by a name that is not empty, which {path} does not");
        }

        return new DataImport(file.Name, file.Content) { Format = Named(file.ImportFormat), Map = Named(file.ImportMap) };
    }

    private static ObservablePair Observable(Wire.ObservablePair pair, string path) =>
        string.IsNullOrWhiteSpace(pair.Stored)
            ? throw new UnexpressibleRequestException($"the public API names a stored observable by a name that is not blank, which {path} does not")
            : new ObservablePair(pair.Stored, Named(pair.Feature));

    // A field the request leaves empty is an optional part the API leaves null.
    private static string? Named(string value) => value.Length > 0 ? value : null;
}
