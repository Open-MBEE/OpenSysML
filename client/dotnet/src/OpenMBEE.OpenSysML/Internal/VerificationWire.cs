using System.Collections.Generic;
using System.Linq;
using Google.Protobuf.Collections;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Reads the verification, validation, calculation, analysis, sweep and engine answers into the client's immutable
/// records. Every field is read whatever the service advertises; a value is read by <see cref="ValueWire"/> and an
/// object by <see cref="ModelWire"/>, each naming where in the answer it is when it is malformed. The failure an
/// answer reports is the caller's to raise.
/// </summary>
internal static class VerificationWire
{
    /// <summary>Reads a verdict.</summary>
    internal static Verdict Verdict(Wire.Verdict verdict, string method, string path) =>
        new(
            verdict.Kind,
            verdict.ElementId,
            verdict.Element,
            verdict.Holds,
            verdict.Condition,
            verdict.InstanceId,
            verdict.InstanceTypeId,
            verdict.Error,
            Reason(verdict.FailureReason),
            verdict.RequirementId,
            ExecutionWire.StandingOf(verdict.Engine, verdict.Strength, verdict.Bounds),
            verdict.InstancePath,
            verdict.Question,
            verdict.Status,
            verdict.Witness.Select((assignment, i) => new WitnessAssignment(
                assignment.Feature,
                assignment.Value is { } value ? ValueWire.FromWire(value, method, $"{path}.witness[{i}].value") : null,
                assignment.Unit,
                assignment.Exact)),
            verdict.Notes.Select(note => new VerdictNote(note.Kind, note.Subject, note.Text)));

    /// <summary>Reads the verdicts of a repeated field, in order.</summary>
    internal static List<Verdict> Verdicts(RepeatedField<Wire.Verdict> verdicts, string method, string path) =>
        [.. verdicts.Select((verdict, i) => Verdict(verdict, method, $"{path}[{i}]"))];

    /// <summary>Reads what verification cases' bodies answered, in order.</summary>
    internal static List<VerificationVerdict> Verifications(RepeatedField<Wire.VerificationVerdict> verdicts) =>
        [.. verdicts.Select(verdict => new VerificationVerdict(
            verdict.CaseId, verdict.Kind, verdict.Detail, verdict.Subcase, verdict.RequirementId, verdict.InstanceId, verdict.InstanceTypeId))];

    /// <summary>Reads the objects an answer reports, in order.</summary>
    internal static List<Instance> Instances(RepeatedField<Wire.Instance> instances, string method, string path) =>
        [.. instances.Select((instance, i) => ModelWire.Instance(instance, method, $"{path}[{i}]"))];

    /// <summary>Reads named values, in order, refusing one that names no value.</summary>
    /// <exception cref="TransportException">An output carries no value.</exception>
    internal static List<CalcOutput> Outputs(RepeatedField<Wire.CalcOutput> outputs, string method, string path) =>
        [.. outputs.Select((output, i) => new CalcOutput(
            output.Name,
            output.Value is { } value
                ? ValueWire.FromWire(value, method, $"{path}[{i}].value")
                : throw ValueWire.MalformedAnswer(method, $"{path}[{i}]", $"the output {ValueText.Quoted(output.Name)} carries no value")))];

    /// <summary>Reads the applications a case made of its own calcs, in order.</summary>
    internal static List<CaseEvaluation> Evaluations(RepeatedField<Wire.CaseEvaluation> evaluations, string method, string path) =>
        [.. evaluations.Select((evaluation, i) => new CaseEvaluation(
            evaluation.FunctionId,
            ValueWire.FromWire(evaluation.Arguments, method, $"{path}[{i}].arguments"),
            evaluation.Result is { } result ? ValueWire.FromWire(result, method, $"{path}[{i}].result") : null,
            evaluation.Error,
            evaluation.Selected,
            evaluation.Tied))];

    /// <summary>Reads one run of an analysis case: everything the answer reports beside its error.</summary>
    /// <param name="response">The answer.</param>
    /// <param name="method">The method that answered it.</param>
    /// <param name="path">Where in the method's answer it is: empty for the answer itself, <c>run</c> for a recording's.</param>
    internal static Analysis Analysis(Wire.RunAnalysisResponse response, string method, string path = "") =>
        new(
            Outputs(response.Outputs, method, At(path, "outputs")),
            Verdicts(response.Verdicts, method, At(path, "verdicts")),
            Verifications(response.VerificationVerdicts),
            Evaluations(response.Evaluations, method, At(path, "evaluations")),
            Instances(response.Instances, method, At(path, "instances")),
            ModelWire.Diagnostics(response.Diagnostics),
            ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds),
            response.ToolDryRun is { } preview ? ToolDryRun(preview) : null);

    /// <summary>Reads a sweep's table.</summary>
    /// <param name="response">The answer.</param>
    /// <param name="method">The method that answered it.</param>
    /// <param name="path">Where in the method's answer it is: empty for the answer itself, <c>sweep</c> for a recording's.</param>
    internal static Sweep Sweep(Wire.RunSweepResponse response, string method, string path = "") =>
        new(
            Rows(response.Rows, method, At(path, "rows")),
            response.Parameters,
            response.Sampled,
            response.Seed,
            Instances(response.Instances, method, At(path, "instances")),
            ModelWire.Diagnostics(response.Diagnostics),
            ExecutionWire.StandingOf(response.Engine, response.Strength, response.Bounds));

    /// <summary>Reads the rows of a table, one per run, in order, a failed run's beside what it computed first.</summary>
    internal static List<SweepRow> Rows(RepeatedField<Wire.SweepRow> rows, string method, string path) =>
        [.. rows.Select((row, i) => new SweepRow(
            Outputs(row.Inputs, method, $"{path}[{i}].inputs"),
            Outputs(row.Outputs, method, $"{path}[{i}].outputs"),
            Verdicts(row.Verdicts, method, $"{path}[{i}].verdicts"),
            Evaluations(row.Evaluations, method, $"{path}[{i}].evaluations"),
            row.ElapsedMicros,
            row.Error,
            Reason(row.FailureReason)))];

    /// <summary>A field's path within a part of an answer, the part's own path empty for the answer itself.</summary>
    internal static string At(string path, string field) => path.Length == 0 ? field : $"{path}.{field}";

    /// <summary>Reads one engine's listing.</summary>
    internal static EngineInfo Engine(Wire.EngineInfo engine) =>
        new(
            engine.Name,
            engine.Authority,
            engine.Answers,
            engine.Bounds,
            engine.Process,
            engine.ProcessFound,
            engine.Ready,
            engine.Unavailable,
            engine.Kind,
            engine.Protocol,
            engine.Source,
            engine.Command,
            engine.Version,
            engine.Served);

    /// <summary>Reads a failure's kind, keeping a number this client does not name.</summary>
    internal static FailureReason Reason(Wire.FailureReason reason) => (FailureReason)(int)reason;

    private static ToolDryRun ToolDryRun(Wire.ToolDryRun preview) =>
        new(
            preview.Action,
            preview.Tool,
            preview.Version,
            preview.Manifest,
            preview.Executable,
            preview.Protocol,
            preview.Args,
            preview.Env,
            preview.Cwd,
            preview.Stdin,
            preview.StdinData,
            preview.InputFile is { } file ? new ToolDryRunFile(file.Name, file.Format, file.Data) : null,
            preview.OutputDir,
            preview.Inputs.Select(input => new ToolDryRunInput(input.Variable, input.Parameter, input.Value, input.Unit)),
            preview.Outputs.Select(output => new ToolDryRunOutput(output.Variable, output.Parameter)),
            preview.ReplySource,
            preview.Lines);
}
