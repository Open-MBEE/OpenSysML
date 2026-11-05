using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Reads the runs, self-check and recording answers into the client's immutable records. Every field is read whatever
/// the service advertises; a value is read by <see cref="ValueWire"/>, naming where in the answer it is when it is
/// malformed. The failure an answer reports is the caller's to raise.
/// </summary>
internal static class RunsWire
{
    /// <summary>Reads the runs' table: everything the answer reports beside its error.</summary>
    /// <param name="response">The answer.</param>
    /// <param name="method">The method that answered it.</param>
    /// <param name="path">Where in the method's answer it is: empty for the answer itself, <c>sample</c> for a recording's.</param>
    /// <exception cref="TransportException">A value the answer must hold is absent or malformed.</exception>
    internal static Runs Runs(Wire.RunRunsResponse response, string method, string path = "") =>
        new(
            VerificationWire.Rows(response.Rows, method, VerificationWire.At(path, "rows")),
            response.Observables,
            response.Distributions.Select((distribution, i) => Distribution(distribution, method, $"{VerificationWire.At(path, "distributions")}[{i}]")),
            response.Lines,
            VerificationWire.Outputs(response.Outputs, method, VerificationWire.At(path, "outputs")),
            VerificationWire.Verdicts(response.Verdicts, method, VerificationWire.At(path, "verdicts")),
            response.Unconcluded,
            response.HasSeed ? response.Seed : null,
            VerificationWire.Instances(response.Instances, method, VerificationWire.At(path, "instances")),
            ModelWire.Diagnostics(response.Diagnostics),
            ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds));

    /// <summary>Reads what a self-check found: everything the answer reports beside its error.</summary>
    internal static SelfCheck SelfCheck(Wire.SelfCheckResponse response) =>
        new(
            response.Status,
            response.Findings.Select(finding => new SelfCheckFinding(
                finding.Status, finding.Subject, finding.Location, finding.Constraint, finding.Element, finding.Message, finding.Line)),
            response.Elements,
            response.Checks,
            response.Violations,
            response.Unevaluated,
            response.EvaluationErrors,
            response.Applications.Select(application => new SelfCheckApplication(application.Constraint, application.Count)),
            response.Summary,
            ModelWire.Diagnostics(response.Diagnostics));

    /// <summary>Reads the documents an answer carries, in order.</summary>
    internal static IEnumerable<EditedDocument> Documents(IEnumerable<Wire.EditedDocument> documents) =>
        documents.Select(document => new EditedDocument(document.Name, document.Content));

    private static RunDistribution Distribution(Wire.RunDistribution distribution, string method, string path) =>
        new(
            distribution.Name,
            distribution.Unit,
            distribution.Count,
            distribution.Numeric,
            distribution.Min is { } min ? ValueWire.FromWire(min, method, path + ".min") : null,
            distribution.Max is { } max ? ValueWire.FromWire(max, method, path + ".max") : null,
            distribution.Mean is { } mean ? ValueWire.FromWire(mean, method, path + ".mean") : null,
            distribution.Deviation,
            distribution.Percentiles.Select((percentile, i) =>
                new RunPercentile(percentile.Percentile, Required(percentile.Value, method, $"{path}.percentiles[{i}]", "value"))),
            distribution.Histogram.Select((bin, i) => new RunHistogramBin(
                Required(bin.Low, method, $"{path}.histogram[{i}]", "low"),
                Required(bin.High, method, $"{path}.histogram[{i}]", "high"),
                bin.Count)),
            distribution.Values.Select((counted, i) => new RunCountedValue(Required(counted.Value, method, $"{path}.values[{i}]", "value"), counted.Count)));

    // A value the schema states every entry carries; an entry without it is malformed.
    private static Value Required(Wire.Value? value, string method, string path, string field) =>
        value is null
            ? throw ValueWire.MalformedAnswer(method, path, $"the entry carries no {field}")
            : ValueWire.FromWire(value, method, $"{path}.{field}");
}
