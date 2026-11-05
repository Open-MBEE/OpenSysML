using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Writes the graph export, data import and result comparison requests and reads their answers into the client's
/// immutable records. Every field is read whatever the service advertises; a value is read by <see cref="ValueWire"/>,
/// naming where in the answer it is when it is malformed. A graph's content is kept as the service wrote it.
/// </summary>
internal static class GraphDataWire
{
    /// <summary>Refuses an import whose optional parts are empty text, which the service reads as none.</summary>
    /// <exception cref="InvalidRequestException">A format, a mapping or the import form is empty; nothing was sent.</exception>
    internal static void Validate(IReadOnlyList<DataImport> imports, ImportOptions options)
    {
        if (options.As is { Length: 0 })
        {
            throw new InvalidRequestException("ImportOptions.As is empty, which the service reads as none; leave it null for the default, values.");
        }

        for (var i = 0; i < imports.Count; i++)
        {
            if (imports[i].Format is { Length: 0 } || imports[i].Map is { Length: 0 })
            {
                throw new InvalidRequestException(
                    $"The data file at imports[{i}] sets its Format or Map to empty text, which the service reads as none; leave it null to read the content as its name and header say.");
            }
        }
    }

    /// <summary>Writes the data files of an import, in order.</summary>
    internal static IEnumerable<Wire.DataImport> ToWire(IEnumerable<DataImport> imports) =>
        imports.Select(file => new Wire.DataImport
        {
            Name = file.Name,
            Content = file.Content,
            ImportFormat = file.Format ?? string.Empty,
            ImportMap = file.Map ?? string.Empty,
        });

    /// <summary>Refuses comparison options naming empty text, which the service reads as none.</summary>
    /// <exception cref="InvalidRequestException">The draws, the engine or an observable's feature is empty; nothing was sent.</exception>
    internal static void Validate(CompareOptions options)
    {
        if (options.Draws is { Length: 0 } || options.Engine is { Length: 0 })
        {
            throw new InvalidRequestException("CompareOptions names an empty draw policy or engine; leave Draws or Engine null for the default.");
        }

        for (var i = 0; i < options.Observe.Count; i++)
        {
            if (options.Observe[i].Feature is { Length: 0 })
            {
                throw new InvalidRequestException(
                    $"The observable at CompareOptions.Observe[{i}] names an empty feature, which the service reads as none; leave it null for the target's feature of the observable's own name.");
            }
        }
    }

    /// <summary>Writes the observables a comparison reads, in order.</summary>
    internal static IEnumerable<Wire.ObservablePair> ToWire(IEnumerable<ObservablePair> observe) =>
        observe.Select(pair => new Wire.ObservablePair { Stored = pair.Stored, Feature = pair.Feature ?? string.Empty });

    /// <summary>Reads an exported graph, as the service wrote it.</summary>
    /// <exception cref="TransportException">The answer carries no form, no version or no subject.</exception>
    internal static Graphs Graphs(Wire.ExportGraphsResponse response, string method)
    {
        if (response.Content.Length == 0 || response.Version <= 0 || response.Subject.Length == 0)
        {
            throw Malformed(method, $"a graph form of version {response.Version} for the subject \"{response.Subject}\" in {response.Content.Length} characters, though every export is a form of a positive version naming its subject");
        }

        return new Graphs(response.Content, response.Version, response.Subject);
    }

    /// <summary>
    /// Reads an answer to an import of <paramref name="imports"/>: the refusal it reports, or what each file set and the
    /// model it made. The notation of a single-document model the import changed is answered in <c>content</c>, that of
    /// a model of several in <c>documents</c> alone; <paramref name="documentCount"/> says which when the model's
    /// documents are known.
    /// </summary>
    /// <param name="response">The answer.</param>
    /// <param name="method">The method answered.</param>
    /// <param name="imports">The data files sent, in order.</param>
    /// <param name="dryRun">Whether the import was a dry run, which changes nothing.</param>
    /// <param name="modelHash">The hash of the model imported into.</param>
    /// <param name="documentCount">How many documents the model has; null when the client does not know.</param>
    /// <param name="model">The handle of the model an answer names by hash.</param>
    /// <exception cref="ModelException">The answer reports that the model refused an import.</exception>
    /// <exception cref="TransportException">The answer is malformed.</exception>
    internal static ImportResult Import(
        Wire.ImportDataResponse response,
        string method,
        IReadOnlyList<DataImport> imports,
        bool dryRun,
        string modelHash,
        int? documentCount,
        Func<string, Model> model)
    {
        var diagnostics = ModelWire.Diagnostics(response.Diagnostics);
        if (response.Error.Length > 0)
        {
            if (response.Files.Count > 0 || response.Documents.Count > 0 || response.Content.Length > 0 || response.ModelHash.Length > 0)
            {
                throw Malformed(method, "a refused import beside what it set or the model it made, which a refusal never carries: the model is as it was");
            }

            throw new ModelException(response.Error, method, diagnostics);
        }

        if (response.Files.Count != imports.Count)
        {
            throw Malformed(method, $"{response.Files.Count} data file(s) for an import of {imports.Count}, which is answered one per file in import order");
        }

        var files = response.Files.Select((file, i) => File(file, i, imports[i], method)).ToList();
        if (dryRun)
        {
            if (response.ModelHash.Length > 0 || response.Content.Length > 0 || response.Documents.Count > 0)
            {
                throw Malformed(method, "a model, notation or documents for a dry run, which changes nothing");
            }

            return new ImportResult(files, null, [], null, diagnostics);
        }

        if (response.ModelHash.Length == 0)
        {
            throw Malformed(method, "neither the imported model's hash nor a refusal");
        }

        EditWire.DocumentNames(response.Documents, method);
        if (response.ModelHash == modelHash && (response.Content.Length > 0 || response.Documents.Count > 0))
        {
            throw Malformed(method, "edited notation for the model imported into, under its own hash, which an import that changed nothing answers");
        }

        if (response.Content.Length > 0)
        {
            if (documentCount is > 1)
            {
                throw Malformed(method, "notation in content for a model of several documents, whose notation is in documents alone");
            }

            if (response.Documents.Count > 1 || (response.Documents.Count == 1 && response.Documents[0].Content != response.Content))
            {
                throw Malformed(method, "documents other than the one document of a single-document model, whose notation content repeats");
            }
        }
        else if (documentCount == 1 && response.Documents.Count > 0)
        {
            throw Malformed(method, "the edited document of a single-document model without its notation in content");
        }

        return new ImportResult(
            files,
            response.Content.Length > 0 ? response.Content : null,
            RunsWire.Documents(response.Documents),
            model(response.ModelHash),
            diagnostics);
    }

    /// <summary>Reads a comparison: one per configuration, with its rows, objects and observables.</summary>
    /// <exception cref="CompareException">The answer reports that nothing could be compared, keeping what it reported beside.</exception>
    /// <exception cref="TransportException">The answer is malformed.</exception>
    /// <exception cref="UnsupportedValueException">A value answered is of a kind this client does not know.</exception>
    internal static Comparison Comparison(Wire.CompareResultsResponse response, string method)
    {
        var read = new Comparison(
            response.Source,
            response.Configurations.Select((configuration, i) => Configuration(configuration, method, $"configurations[{i}]")),
            ModelWire.Diagnostics(response.Diagnostics),
            ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds));
        if (response.Error.Length > 0)
        {
            throw new CompareException(response.Error, method, VerificationWire.Reason(response.FailureReason), read);
        }

        if (response.FailureReason != Wire.FailureReason.Unspecified)
        {
            throw Malformed(method, $"the failure kind {response.FailureReason} without the failure it names");
        }

        return read;
    }

    private static ImportedFile File(Wire.ImportedFile file, int i, DataImport sent, string method)
    {
        if (file.Name != sent.Name)
        {
            throw Malformed(method, $"files[{i}] for \"{file.Name}\", though the data file imports[{i}] sent is \"{sent.Name}\"");
        }

        if (file.Values < 0 || file.Elements < 0)
        {
            throw Malformed(method, $"files[{i}] setting {file.Values} value(s) on {file.Elements} element(s), which no import counts");
        }

        return new ImportedFile(
            file.Name,
            file.Values,
            file.Elements,
            file.Assignments.Select(value => new ImportedValue(value.Element, value.Feature, value.Value, value.Redefines, value.Where, value.Description)));
    }

    private static ConfigurationComparison Configuration(Wire.ConfigurationComparison configuration, string method, string path) =>
        new(
            configuration.Id,
            configuration.Name,
            configuration.Behavior,
            configuration.Target,
            configuration.ResultLocation,
            configuration.Status,
            configuration.Unresolved,
            configuration.StoredRuns,
            configuration.StoredSnapshots,
            configuration.Runs,
            configuration.Completed,
            configuration.Draws,
            configuration.HasSeed ? configuration.Seed : null,
            configuration.ClockStep,
            VerificationWire.Rows(configuration.Rows, method, path + ".rows"),
            VerificationWire.Instances(configuration.Instances, method, path + ".instances"),
            configuration.Observables.Select(Observable),
            configuration.Errors,
            configuration.Notes,
            configuration.Lines);

    private static ObservableComparison Observable(Wire.ObservableComparison observable) =>
        new(
            observable.Stored,
            observable.Feature,
            observable.Unit,
            Distribution(observable.Tool),
            observable.Pooled,
            observable.Unstored,
            Distribution(observable.Run),
            observable.Produced,
            observable.Compared,
            observable.Difference is { } difference
                ? new StatisticDifference(difference.Min, difference.Mean, difference.P50, difference.P90, difference.Max)
                : null,
            observable.Declared is { } declared
                ? new CaseStatistics(
                    declared.Analysis,
                    declared.Rows.Select(row => new CaseStatistic(row.Statistic, row.Of, row.Tool, row.Run, row.Difference)),
                    declared.Notes)
                : null,
            observable.Notes);

    private static ComparedDistribution? Distribution(Wire.ComparedDistribution? distribution) =>
        distribution is null
            ? null
            : new(distribution.Count, distribution.Min, distribution.Mean, distribution.P50, distribution.P90, distribution.Max, distribution.Deviation);

    private static TransportException Malformed(string method, string what) =>
        new(StatusCode.Internal, $"{method} answered {what}.", method);
}
