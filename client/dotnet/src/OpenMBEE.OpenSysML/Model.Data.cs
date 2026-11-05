using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

public sealed partial class Model
{
    internal const string ImportDataMethod = "sysml.SysMLService/ImportData";
    internal const string CompareResultsMethod = "sysml.SysMLService/CompareResults";

    /// <summary>
    /// Sets the feature values CSV, TSV, JSON or JSON Lines data assigns, one row per element, as <c>sysml -import</c>
    /// does: a feature the element declares has its value set, one it inherits is redefined in the element's body with
    /// the value. Each file is applied in order as one edit, all of them or none, and the edited model is parsed and held
    /// by the service under a hash of its own, which <see cref="ImportResult.Model"/> names; this model is left as it was,
    /// and nothing is written to disk.
    /// </summary>
    /// <remarks>
    /// A model of several documents (<see cref="Connection.ParseSourcesAsync"/>) is imported into as one, its edited
    /// notation in <see cref="ImportResult.Documents"/> alone. An empty list of files is sent too, and refused by the
    /// service with <see cref="StatusCode.InvalidArgument"/>. A lost answer does not prove the import was not made.
    /// </remarks>
    /// <param name="imports">The data files, imported in order.</param>
    /// <param name="options">What the rows are made into, whether the model is changed and whether its documents are accepted; null for <see cref="ImportOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>What each file set, the edited notation and the imported model; on a dry run, what each file would set alone.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="imports"/> is null.</exception>
    /// <exception cref="ArgumentException">A data file is null.</exception>
    /// <exception cref="InvalidRequestException">A data file's format or mapping, or the import form, is empty text; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.ImportData"/>; nothing was sent.</exception>
    /// <exception cref="ModelException">The model refused an import: a row naming no element or feature, a value that does not read, or values that make the model invalid, with the diagnostics they raised. Nothing was changed.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call: <see cref="StatusCode.InvalidArgument"/> for no files, a file without a name or an
    /// import form other than <c>values</c>; <see cref="StatusCode.FailedPrecondition"/> for a model of several documents
    /// when <see cref="ImportOptions.AcceptDocuments"/> is false or the service edits one alone.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<ImportResult> ImportDataAsync(IEnumerable<DataImport> imports, ImportOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(imports);
        var files = new ValueList<DataImport>(imports);
        Connection.ThrowIfDisposed();
        options ??= ImportOptions.Default;
        GraphDataWire.Validate(files, options);
        Connection.Capabilities.Require(Capabilities.ImportData);
        var request = new Wire.ImportDataRequest
        {
            ModelHash = Hash,
            Imports = { GraphDataWire.ToWire(files) },
            ImportAs = options.As ?? string.Empty,
            ImportDryRun = options.DryRun,
            AcceptDocuments = options.AcceptDocuments,
        };
        var response = await Connection.ModelCallAsync<Wire.ImportDataRequest, Wire.ImportDataResponse>(ImportDataMethod, request, cancellationToken).ConfigureAwait(false);
        return GraphDataWire.Import(
            response,
            ImportDataMethod,
            files,
            options.DryRun,
            Hash,
            DocumentCount,
            hash => new Model(Connection, hash, ImportDataMethod, default, default, default, null, default));
    }

    /// <summary>
    /// Runs every configuration a SysML v1 tool's results index names against this model, migrated from that tool's
    /// model, and tables the tool's min, mean, p50, p90 and max of each observable beside the runs' with their relative
    /// difference, as <c>sysml -compare-results</c> does. The results are the JSON <c>sysml -migration-results</c>
    /// writes. A configuration that cannot be run, or whose runs fail, is a comparison with that status, never an
    /// exception.
    /// </summary>
    /// <param name="results">The results index, as JSON.</param>
    /// <param name="options">The configurations, runs, seed, draws, clock step, observables and engine; null for <see cref="CompareOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>One comparison per configuration, in the order the results index them.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="results"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="results"/> is empty, which indexes nothing.</exception>
    /// <exception cref="InvalidRequestException">An option names an empty draw policy, engine or feature; nothing was sent.</exception>
    /// <exception cref="CapabilityException">The service lacks <see cref="Capabilities.CompareResults"/>, <see cref="Capabilities.Engines"/> for an engine, or <see cref="Capabilities.ScheduleExplore"/> for the explore engine, checked in that order; nothing was sent.</exception>
    /// <exception cref="CompareException">The service could compare nothing, saying why; it keeps what the answer reported beside.</exception>
    /// <exception cref="ModelNotFoundException">The service no longer holds the model.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    /// <exception cref="ServiceException">
    /// The service refused the call with <see cref="StatusCode.InvalidArgument"/>: results that are not the index's JSON,
    /// a negative number of runs, an unknown draw policy, a clock step no clock ticks by, or an engine it does not know.
    /// </exception>
    /// <exception cref="TransportException">The call failed in transport, or the answer was malformed.</exception>
    public async Task<Comparison> CompareResultsAsync(string results, CompareOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(results);
        Connection.ThrowIfDisposed();
        options ??= CompareOptions.Default;
        GraphDataWire.Validate(options);
        var capabilities = Connection.Capabilities;
        capabilities.Require(Capabilities.CompareResults);
        Engines.Require(options.Engine, capabilities);
        var request = new Wire.CompareResultsRequest
        {
            ModelHash = Hash,
            Results = results,
            Actions = { options.Actions },
            Runs = options.Runs,
            Draws = options.Draws ?? string.Empty,
            Observe = { GraphDataWire.ToWire(options.Observe) },
            Engine = options.Engine ?? string.Empty,
        };
        if (options.Seed is { } seed)
        {
            request.Seed = seed;
        }

        if (options.ClockStep is { } step)
        {
            request.ClockStep = step;
        }

        var response = await Connection.ModelCallAsync<Wire.CompareResultsRequest, Wire.CompareResultsResponse>(CompareResultsMethod, request, cancellationToken).ConfigureAwait(false);
        return GraphDataWire.Comparison(response, CompareResultsMethod);
    }
}
