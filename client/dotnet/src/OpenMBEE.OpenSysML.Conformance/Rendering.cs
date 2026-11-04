using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Numerics;
using Google.Protobuf.Collections;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Writes what the client's public API returned back as the protocol messages a scenario is stated
/// against, so the comparison judges what a caller was given. It reads only public domain types:
/// a fact the API does not expose cannot be rendered, and a scenario asserting it fails.
/// </summary>
internal static class Rendering
{
    /// <summary>Diagnostics, in the order the API reports them.</summary>
    internal static IEnumerable<Wire.Diagnostic> Diagnostics(IEnumerable<Diagnostic> diagnostics)
    {
        foreach (var diagnostic in diagnostics)
        {
            yield return Diagnostic(diagnostic);
        }
    }

    /// <summary>One diagnostic, its severity as the service spelled it.</summary>
    internal static Wire.Diagnostic Diagnostic(Diagnostic diagnostic)
    {
        ArgumentNullException.ThrowIfNull(diagnostic);
        var rendered = new Wire.Diagnostic
        {
            Severity = diagnostic.SeverityName,
            Message = diagnostic.Message,
            Code = diagnostic.Code,
            OrderDependent = { diagnostic.OrderDependent },
            Independent = diagnostic.Independent,
        };
        if (diagnostic.Span is { } span)
        {
            rendered.Span = new Wire.Span
            {
                File = span.File,
                StartLine = span.StartLine,
                StartCol = span.StartColumn,
                EndLine = span.EndLine,
                EndCol = span.EndColumn,
            };
        }

        return rendered;
    }

    /// <summary>A symbol with every fact the API reports of it.</summary>
    internal static Wire.SymbolInfo Symbol(Symbol symbol)
    {
        ArgumentNullException.ThrowIfNull(symbol);
        var rendered = new Wire.SymbolInfo
        {
            Id = symbol.Id,
            Name = symbol.Name,
            Kind = symbol.Kind,
            ChildIds = { symbol.ChildIds },
            WithheldLibraryAttributes = symbol.WithheldLibraryAttributes,
        };
        foreach (var (name, fact) in symbol.Metadata)
        {
            rendered.Metadata.Add(name, fact);
        }

        foreach (var attribute in symbol.Attributes)
        {
            var info = new Wire.AttributeInfo { Name = attribute.Name, Type = attribute.Type, Unit = attribute.Unit };
            if (attribute.Value is { } value)
            {
                info.Value = Value(value);
            }

            rendered.Attributes.Add(info);
        }

        if (symbol.TypeFacts is { } type)
        {
            rendered.TypeInfo = new Wire.TypeInfo
            {
                Declared = type.Declared,
                ResolvedId = type.ResolvedId,
                ResolvedKind = type.ResolvedKind,
                Primitive = type.Primitive,
                PrimitiveSource = type.PrimitiveSource,
                Quantity = type.IsQuantity,
                Unit = type.Unit,
            };
        }

        if (symbol.Multiplicity is { } multiplicity)
        {
            rendered.Multiplicity = new Wire.MultiplicityInfo { Lower = multiplicity.Lower, Upper = multiplicity.Upper };
        }

        foreach (var edge in symbol.Specializations)
        {
            rendered.Specializations.Add(new Wire.Specialization
            {
                Kind = edge.Kind,
                Declared = edge.Declared,
                TargetId = edge.TargetId,
                TargetKind = edge.TargetKind,
            });
        }

        return rendered;
    }

    /// <summary>An object the service built, with what each of its features holds.</summary>
    internal static Wire.Instance Instance(Instance instance)
    {
        ArgumentNullException.ThrowIfNull(instance);
        var rendered = new Wire.Instance { Id = instance.Id, TypeSymbolId = instance.TypeSymbolId };
        foreach (var (name, feature) in instance.FeatureValues)
        {
            var held = new Wire.FeatureValue
            {
                FeatureName = feature.FeatureName,
                Materialized = feature.IsMaterialized,
                Error = feature.Error,
                OrderDependent = feature.OrderDependent,
            };
            if (feature.Value is { } value)
            {
                held.Value = Value(value);
            }

            foreach (var each in feature.Values)
            {
                held.Values.Add(Value(each));
            }

            rendered.FeatureValues.Add(name, held);
        }

        return rendered;
    }

    /// <summary>One run of an action, with everything the API reports of it.</summary>
    internal static Wire.ExecuteActionResponse ActionRun(ActionRun run)
    {
        ArgumentNullException.ThrowIfNull(run);
        var rendered = new Wire.ExecuteActionResponse { FinalTime = run.FinalTime, TraceDropped = run.TraceDropped };
        Values(rendered.Outputs, run.Outputs);
        rendered.Diagnostics.AddRange(Diagnostics(run.Diagnostics));
        Values(rendered.PerformerAttributes, run.PerformerAttributes);
        rendered.OrderDependent.Add(run.OrderDependent.ToDictionary(StringComparer.Ordinal));
        rendered.Trace.AddRange(Trace(run.Trace));
        return rendered;
    }

    /// <summary>One run of a state machine, with its trace and everything else the API reports of it.</summary>
    internal static Wire.ExecuteStateResponse StateRun(StateRun run)
    {
        ArgumentNullException.ThrowIfNull(run);
        var rendered = new Wire.ExecuteStateResponse { StatesVisited = { run.StatesVisited }, FinalTime = run.FinalTime, TraceDropped = run.TraceDropped };
        Values(rendered.FinalContext, run.FinalContext);
        rendered.Diagnostics.AddRange(Diagnostics(run.Diagnostics));
        rendered.Trace.AddRange(Trace(run.Trace));
        rendered.OrderDependent.Add(run.OrderDependent.ToDictionary(StringComparer.Ordinal));
        return rendered;
    }

    /// <summary>An exploration's outcomes, in the order the API reports them.</summary>
    internal static IEnumerable<Wire.Outcome> Outcomes(Exploration exploration)
    {
        ArgumentNullException.ThrowIfNull(exploration);
        foreach (var outcome in exploration.Outcomes)
        {
            var rendered = new Wire.Outcome
            {
                FinalState = outcome.FinalState,
                StatesVisited = { outcome.StatesVisited },
                Error = outcome.Error,
                Linearizations = outcome.Linearizations,
                Witness = { outcome.Witness },
                Probability = outcome.Probability,
            };
            Values(rendered.Outputs, outcome.Outputs);
            rendered.Diagnostics.AddRange(Diagnostics(outcome.Diagnostics));
            if (outcome.ProbabilityRange is { } range)
            {
                rendered.ProbabilityRange = new Wire.ProbabilityRange { Min = range.Min, Max = range.Max };
            }

            yield return rendered;
        }
    }

    /// <summary>How an exploration ended.</summary>
    internal static Wire.ExplorationStatus ExplorationStatus(Exploration exploration)
    {
        ArgumentNullException.ThrowIfNull(exploration);
        return new Wire.ExplorationStatus
        {
            Complete = exploration.Complete,
            Runs = exploration.Runs,
            BudgetsHit = { exploration.BudgetsHit },
            RunsBudget = exploration.RunsBudget,
            DepthBudget = exploration.DepthBudget,
            ProbabilitiesLowerBound = exploration.ProbabilitiesLowerBound,
            FailedLinearizations = exploration.FailedLinearizations,
            Unenumerated = { exploration.Unenumerated },
        };
    }

    /// <summary>What a check found, with every witness the API reports.</summary>
    internal static Wire.CheckBehaviorResponse CheckResult(CheckResult result)
    {
        ArgumentNullException.ThrowIfNull(result);
        var rendered = new Wire.CheckBehaviorResponse
        {
            Verdict = result.Verdict,
            Lines = { result.Lines },
            Claim = result.Claim,
            FreeInputs = { result.FreeInputs },
            Assumptions = { result.Assumptions },
            Witness = Witness(result.Witness),
            Contrast = Witness(result.Contrast),
            Engine = result.Standing.Engine,
            Strength = result.Standing.Strength,
            Reason = result.Reason,
        };
        if (result.Search is { } search)
        {
            rendered.Search = new Wire.CheckSearch
            {
                States = search.States,
                Moves = search.Moves,
                MaxDepth = search.MaxDepth,
                BoundsHit = { search.BoundsHit },
                Horizon = search.Horizon,
                HorizonSet = search.HorizonSet,
                MassBounded = search.MassBounded,
                Notes = { search.Notes },
                Scope = { search.Scope },
                Status = search.Status,
            };
        }

        foreach (var violation in result.Violations)
        {
            rendered.Violations.Add(new Wire.CheckViolation
            {
                Kind = violation.Kind,
                Property = violation.Property,
                Error = violation.Error,
                Depth = violation.Depth,
                Probability = violation.Probability,
                Witness = Witness(violation.Witness),
                Text = violation.Text,
            });
        }

        foreach (var divergence in result.Divergences)
        {
            var rendering = new Wire.CheckDivergence { Feature = divergence.Feature };
            foreach (var value in divergence.Values)
            {
                rendering.Values.Add(new Wire.CheckDivergentValue { Value = value.Value, Witness = Witness(value.Witness) });
            }

            rendered.Divergences.Add(rendering);
        }

        foreach (var final in result.Finals)
        {
            var rendering = new Wire.CheckFinal { Outcome = final.Outcome, Witness = Witness(final.Witness) };
            rendering.Values.Add(final.Values.ToDictionary(StringComparer.Ordinal));
            rendered.Finals.Add(rendering);
        }

        foreach (var bound in result.Standing.Bounds)
        {
            rendered.Bounds.Add(new Wire.Bound { Name = bound.Name, Limit = bound.Limit, Reached = bound.Reached });
        }

        return rendered;
    }

    /// <summary>A verdict, with its standing and witness.</summary>
    internal static Wire.Verdict Verdict(Verdict verdict)
    {
        ArgumentNullException.ThrowIfNull(verdict);
        var rendered = new Wire.Verdict
        {
            Kind = verdict.Kind,
            ElementId = verdict.ElementId,
            Element = verdict.Element,
            Holds = verdict.Holds,
            Condition = verdict.Condition,
            InstanceId = verdict.InstanceId,
            InstanceTypeId = verdict.InstanceTypeId,
            Error = verdict.Error,
            FailureReason = Reason(verdict.FailureReason),
            RequirementId = verdict.RequirementId,
            Engine = verdict.Standing.Engine,
            Strength = verdict.Standing.Strength,
            Bounds = { Bounds(verdict.Standing) },
            InstancePath = verdict.InstancePath,
            Question = verdict.Question,
            Status = verdict.Status,
            Notes = { verdict.Notes.Select(note => new Wire.VerdictNote { Kind = note.Kind, Subject = note.Subject, Text = note.Text }) },
        };
        foreach (var assignment in verdict.Witness)
        {
            var witness = new Wire.WitnessAssignment { Feature = assignment.Feature, Unit = assignment.Unit, Exact = assignment.Exact };
            if (assignment.Value is { } value)
            {
                witness.Value = Value(value);
            }

            rendered.Witness.Add(witness);
        }

        return rendered;
    }

    /// <summary>Verdicts, in the order the API reports them.</summary>
    internal static IEnumerable<Wire.Verdict> Verdicts(IEnumerable<Verdict> verdicts) => verdicts.Select(Verdict);

    /// <summary>What verification cases' bodies answered, in the order the API reports them.</summary>
    internal static IEnumerable<Wire.VerificationVerdict> Verifications(IEnumerable<VerificationVerdict> verdicts) =>
        verdicts.Select(verdict => new Wire.VerificationVerdict
        {
            CaseId = verdict.CaseId,
            Kind = verdict.Kind,
            Detail = verdict.Detail,
            Subcase = verdict.Subcase,
            RequirementId = verdict.RequirementId,
            InstanceId = verdict.InstanceId,
            InstanceTypeId = verdict.InstanceTypeId,
        });

    /// <summary>Objects, in the order the API reports them.</summary>
    internal static IEnumerable<Wire.Instance> Instances(IEnumerable<Instance> instances) => instances.Select(Instance);

    /// <summary>Named values, in the order the API reports them.</summary>
    internal static IEnumerable<Wire.CalcOutput> Outputs(IEnumerable<CalcOutput> outputs) =>
        outputs.Select(output => new Wire.CalcOutput { Name = output.Name, Value = Value(output.Value) });

    /// <summary>A failure's kind, as the number the API kept.</summary>
    internal static Wire.FailureReason Reason(FailureReason reason) => (Wire.FailureReason)(int)reason;

    /// <summary>The bounds of a standing.</summary>
    internal static IEnumerable<Wire.Bound> Bounds(Standing standing) =>
        standing.Bounds.Select(bound => new Wire.Bound { Name = bound.Name, Limit = bound.Limit, Reached = bound.Reached });

    /// <summary>A calculation's answer.</summary>
    internal static Wire.EvaluateCalcResponse Calculation(Calculation calculation)
    {
        ArgumentNullException.ThrowIfNull(calculation);
        var rendered = new Wire.EvaluateCalcResponse
        {
            Outputs = { Outputs(calculation.Outputs) },
            Diagnostics = { Diagnostics(calculation.Diagnostics) },
            Engine = calculation.Standing.Engine,
            Strength = calculation.Standing.Strength,
            Bounds = { Bounds(calculation.Standing) },
        };
        if (calculation.Result is { } result)
        {
            rendered.Result = Value(result);
        }

        return rendered;
    }

    /// <summary>One run of an analysis case, with everything the API reports of it.</summary>
    internal static Wire.RunAnalysisResponse Analysis(Analysis analysis)
    {
        ArgumentNullException.ThrowIfNull(analysis);
        var rendered = new Wire.RunAnalysisResponse
        {
            Outputs = { Outputs(analysis.Outputs) },
            Verdicts = { Verdicts(analysis.Verdicts) },
            Instances = { Instances(analysis.Instances) },
            Diagnostics = { Diagnostics(analysis.Diagnostics) },
            VerificationVerdicts = { Verifications(analysis.Verifications) },
            Evaluations = { Evaluations(analysis.Evaluations) },
            Engine = analysis.Standing.Engine,
            Strength = analysis.Standing.Strength,
            Bounds = { Bounds(analysis.Standing) },
        };
        if (analysis.ToolDryRun is { } preview)
        {
            rendered.ToolDryRun = ToolDryRun(preview);
        }

        return rendered;
    }

    /// <summary>A sweep's table, with every row's inputs, outputs, verdicts and failure.</summary>
    internal static Wire.RunSweepResponse Sweep(Sweep sweep)
    {
        ArgumentNullException.ThrowIfNull(sweep);
        return new Wire.RunSweepResponse
        {
            Parameters = { sweep.Parameters },
            Sampled = sweep.Sampled,
            Seed = sweep.Seed,
            Instances = { Instances(sweep.Instances) },
            Diagnostics = { Diagnostics(sweep.Diagnostics) },
            Engine = sweep.Standing.Engine,
            Strength = sweep.Standing.Strength,
            Bounds = { Bounds(sweep.Standing) },
            Rows = { Rows(sweep.Rows) },
        };
    }

    /// <summary>The runs' table, with every row, each observable's distribution and a case's conclusion.</summary>
    internal static Wire.RunRunsResponse Runs(Runs runs)
    {
        ArgumentNullException.ThrowIfNull(runs);
        var rendered = new Wire.RunRunsResponse
        {
            Rows = { Rows(runs.Rows) },
            Observables = { runs.Observables },
            Distributions = { runs.Distributions.Select(Distribution) },
            Lines = { runs.Lines },
            Outputs = { Outputs(runs.Outputs) },
            Verdicts = { Verdicts(runs.Verdicts) },
            Unconcluded = runs.Unconcluded,
            Diagnostics = { Diagnostics(runs.Diagnostics) },
            Instances = { Instances(runs.Instances) },
            Engine = runs.Standing.Engine,
            Strength = runs.Standing.Strength,
            Bounds = { Bounds(runs.Standing) },
        };
        if (runs.Seed is { } seed)
        {
            rendered.Seed = seed;
        }

        return rendered;
    }

    /// <summary>What a self-check found, with every finding and count.</summary>
    internal static Wire.SelfCheckResponse SelfCheck(SelfCheck check)
    {
        ArgumentNullException.ThrowIfNull(check);
        return new Wire.SelfCheckResponse
        {
            Status = check.Status,
            Findings =
            {
                check.Findings.Select(finding => new Wire.SelfCheckFinding
                {
                    Status = finding.Status,
                    Subject = finding.Subject,
                    Location = finding.Location,
                    Constraint = finding.Constraint,
                    Element = finding.Element,
                    Message = finding.Message,
                    Line = finding.Line,
                }),
            },
            Elements = check.Elements,
            Checks = check.Checks,
            Violations = check.Violations,
            Unevaluated = check.Unevaluated,
            EvaluationErrors = check.EvaluationErrors,
            Applications = { check.Applications.Select(application => new Wire.SelfCheckApplication { Constraint = application.Constraint, Count = application.Count }) },
            Summary = check.Summary,
            Diagnostics = { Diagnostics(check.Diagnostics) },
        };
    }

    /// <summary>A recording, with the runs it made, the records it generated and the model it made.</summary>
    internal static Wire.RecordRunResponse Recording(Recording recording)
    {
        ArgumentNullException.ThrowIfNull(recording);
        var rendered = Ran(recording.Run, recording.Sweep, recording.Sample);
        rendered.Package = recording.Package;
        rendered.Definition = recording.Definition;
        rendered.Records.Add(recording.Records);
        rendered.Source = recording.Source;
        rendered.Content = recording.Content;
        rendered.Documents.Add(recording.Documents.Select(document => new Wire.EditedDocument { Name = document.Name, Content = document.Content }));
        rendered.Skipped = recording.Skipped;
        rendered.Reasons.Add(recording.SkipReasons);
        rendered.Diagnostics.Add(Diagnostics(recording.Diagnostics));
        rendered.ModelHash = recording.Model.Hash;
        return rendered;
    }

    /// <summary>A recording the service did not make, with what it kept: the runs, and the failure where they reported it.</summary>
    internal static Wire.RecordRunResponse Recording(RecordException failure)
    {
        ArgumentNullException.ThrowIfNull(failure);
        var rendered = Ran(failure.Run, failure.Sweep, failure.Sample);
        rendered.Package = failure.Package;
        rendered.Definition = failure.Definition;
        rendered.Records.Add(failure.Records);
        rendered.Source = failure.GeneratedSource;
        rendered.Skipped = failure.Skipped;
        rendered.Reasons.Add(failure.SkipReasons);
        rendered.Error = failure.Message;
        rendered.FailureReason = Reason(failure.Reason);
        rendered.Diagnostics.Add(Diagnostics(failure.Diagnostics));
        if (failure.RunFailed)
        {
            switch (rendered)
            {
                case { Run: { } run }:
                    (run.Error, run.FailureReason) = (failure.Message, rendered.FailureReason);
                    break;
                case { Sweep: { } sweep }:
                    (sweep.Error, sweep.FailureReason) = (failure.Message, rendered.FailureReason);
                    break;
                case { Sample: { } sample }:
                    (sample.Error, sample.FailureReason) = (failure.Message, rendered.FailureReason);
                    break;
            }
        }

        return rendered;
    }

    // The rows of a table, each with its inputs, outputs, verdicts, timing and failure.
    private static IEnumerable<Wire.SweepRow> Rows(IEnumerable<SweepRow> rows) =>
        rows.Select(row => new Wire.SweepRow
        {
            Inputs = { Outputs(row.Inputs) },
            Outputs = { Outputs(row.Outputs) },
            Verdicts = { Verdicts(row.Verdicts) },
            ElapsedMicros = row.ElapsedMicroseconds,
            Error = row.Error,
            FailureReason = Reason(row.FailureReason),
            Evaluations = { Evaluations(row.Evaluations) },
        });

    private static Wire.RunDistribution Distribution(RunDistribution distribution)
    {
        var rendered = new Wire.RunDistribution
        {
            Name = distribution.Name,
            Unit = distribution.Unit,
            Count = distribution.Count,
            Numeric = distribution.Numeric,
            Deviation = distribution.Deviation,
            Percentiles = { distribution.Percentiles.Select(percentile => new Wire.RunPercentile { Percentile = percentile.Percentile, Value = Value(percentile.Value) }) },
            Histogram = { distribution.Histogram.Select(bin => new Wire.RunHistogramBin { Low = Value(bin.Low), High = Value(bin.High), Count = bin.Count }) },
            Values = { distribution.Values.Select(counted => new Wire.RunCountedValue { Value = Value(counted.Value), Count = counted.Count }) },
        };
        rendered.Min = distribution.Min is { } min ? Value(min) : null;
        rendered.Max = distribution.Max is { } max ? Value(max) : null;
        rendered.Mean = distribution.Mean is { } mean ? Value(mean) : null;
        return rendered;
    }

    // A recording's answer holding the one run, sweep or sample it made, or none.
    private static Wire.RecordRunResponse Ran(Analysis? run, Sweep? sweep, Runs? sample) => new()
    {
        Run = run is null ? null : Analysis(run),
        Sweep = sweep is null ? null : Sweep(sweep),
        Sample = sample is null ? null : Runs(sample),
    };

    /// <summary>A conversion, with its formats, diagnostics, library references and whether it is experimental.</summary>
    internal static Wire.ConvertResponse Conversion(Conversion conversion)
    {
        ArgumentNullException.ThrowIfNull(conversion);
        return new Wire.ConvertResponse
        {
            Content = conversion.Content,
            FromFormat = conversion.FromFormat,
            ToFormat = conversion.ToFormat,
            Diagnostics = { Diagnostics(conversion.Diagnostics) },
            Experimental = conversion.Experimental,
            ExperimentalNotice = conversion.ExperimentalNotice,
            LibraryReferences =
            {
                conversion.LibraryReferences.Select(reference => new Wire.LibraryReference { Id = reference.Id, QualifiedName = reference.QualifiedName, Type = reference.Type }),
            },
        };
    }

    /// <summary>A conversion not made, in band, with the formats and diagnostics the failure kept.</summary>
    internal static Wire.ConvertResponse Conversion(ConversionException failure)
    {
        ArgumentNullException.ThrowIfNull(failure);
        return new Wire.ConvertResponse
        {
            FromFormat = failure.FromFormat,
            ToFormat = failure.ToFormat,
            Error = failure.Message,
            Diagnostics = { Diagnostics(failure.Diagnostics) },
            Experimental = failure.Experimental,
            ExperimentalNotice = failure.ExperimentalNotice,
        };
    }

    /// <summary>A migration, with its report, results and files.</summary>
    internal static Wire.MigrateResponse Migration(Migration migration)
    {
        ArgumentNullException.ThrowIfNull(migration);
        var report = migration.Report;
        return new Wire.MigrateResponse
        {
            Content = migration.Content,
            FromFormat = migration.FromFormat,
            ToFormat = migration.ToFormat,
            Experimental = migration.Experimental,
            ExperimentalNotice = migration.ExperimentalNotice,
            Report = new Wire.MigrationReport
            {
                Source = report.Source,
                Exporter = report.Exporter,
                Summary = report.Summary,
                Mapped = report.Mapped,
                Approximated = report.Approximated,
                Unmapped = report.Unmapped,
                Skipped = report.Skipped,
                Entries =
                {
                    report.Entries.Select(entry => new Wire.MigrationEntry
                    {
                        Id = entry.Id,
                        Kind = entry.Kind,
                        Name = entry.Name,
                        Target = entry.Target,
                        Verdict = entry.Verdict,
                        Note = entry.Note,
                    }),
                },
                Text = report.Text,
            },
            Results = migration.Results,
            Files = { migration.Files.Select(file => new Wire.MigrationFile { Path = file.Path, Content = Google.Protobuf.ByteString.CopyFrom(file.Content.Span) }) },
        };
    }

    /// <summary>A migration not made, in band, with the formats the failure kept.</summary>
    internal static Wire.MigrateResponse Migration(MigrationException failure)
    {
        ArgumentNullException.ThrowIfNull(failure);
        return new Wire.MigrateResponse
        {
            FromFormat = failure.FromFormat,
            ToFormat = failure.ToFormat,
            Error = failure.Message,
            Experimental = failure.Experimental,
            ExperimentalNotice = failure.ExperimentalNotice,
        };
    }

    /// <summary>
    /// The notation a batch of edits produced, with what each edit changed and every document it rewrote; a model of
    /// several documents answers no content.
    /// </summary>
    internal static Wire.ApplyEditsResponse EditResult(EditResult result)
    {
        ArgumentNullException.ThrowIfNull(result);
        return new Wire.ApplyEditsResponse
        {
            Content = result.Content ?? string.Empty,
            Applied =
            {
                result.Applied.Select(applied => new Wire.AppliedEdit
                {
                    OperationIndex = applied.OperationIndex,
                    Target = applied.Target,
                    Offset = applied.Offset,
                    Length = applied.Length,
                    OldText = applied.OldText,
                    NewText = applied.NewText,
                    Document = applied.Document ?? string.Empty,
                }),
            },
            Diagnostics = { Diagnostics(result.Diagnostics) },
            Documents = { result.Documents.Select(document => new Wire.EditedDocument { Name = document.Name, Content = document.Content }) },
        };
    }

    /// <summary>A refused batch of edits, in band: why, its kind, its diagnostics and the declarations referring to its target.</summary>
    internal static Wire.ApplyEditsResponse EditResult(EditException failure)
    {
        ArgumentNullException.ThrowIfNull(failure);
        return new Wire.ApplyEditsResponse
        {
            Error = failure.Message,
            Failure = (Wire.EditFailure)(int)failure.Failure,
            Diagnostics = { Diagnostics(failure.Diagnostics) },
            ReferringElements = { failure.ReferringElements },
            Referrers = { failure.Referrers.Select(referrer => new Wire.Referrer { Name = referrer.Name, Document = referrer.Document }) },
        };
    }

    /// <summary>An exported graph form, its version and the subject as resolved.</summary>
    internal static Wire.ExportGraphsResponse Graphs(Graphs graphs)
    {
        ArgumentNullException.ThrowIfNull(graphs);
        return new Wire.ExportGraphsResponse { Content = graphs.Content, Version = graphs.Version, Subject = graphs.Subject };
    }

    /// <summary>What an import set, file by file and assignment by assignment, with the notation and model it made; a dry run answers neither.</summary>
    internal static Wire.ImportDataResponse ImportResult(ImportResult result)
    {
        ArgumentNullException.ThrowIfNull(result);
        return new Wire.ImportDataResponse
        {
            Files =
            {
                result.Files.Select(file => new Wire.ImportedFile
                {
                    Name = file.Name,
                    Values = file.Values,
                    Elements = file.Elements,
                    Assignments =
                    {
                        file.Assignments.Select(value => new Wire.ImportedValue
                        {
                            Element = value.Element,
                            Feature = value.Feature,
                            Value = value.Value,
                            Redefines = value.Redefines,
                            Where = value.Where,
                            Description = value.Description,
                        }),
                    },
                }),
            },
            Documents = { result.Documents.Select(document => new Wire.EditedDocument { Name = document.Name, Content = document.Content }) },
            Content = result.Content ?? string.Empty,
            ModelHash = result.Model?.Hash ?? string.Empty,
            Diagnostics = { Diagnostics(result.Diagnostics) },
        };
    }

    /// <summary>An import the model refused, in band: why, and the diagnostics behind it.</summary>
    internal static Wire.ImportDataResponse ImportResult(ModelException refused)
    {
        ArgumentNullException.ThrowIfNull(refused);
        return new Wire.ImportDataResponse { Error = refused.Message, Diagnostics = { Diagnostics(refused.Diagnostics) } };
    }

    /// <summary>A comparison, every configuration with its runs, objects and observables, and the engine that ran them.</summary>
    internal static Wire.CompareResultsResponse Comparison(Comparison comparison)
    {
        ArgumentNullException.ThrowIfNull(comparison);
        return new Wire.CompareResultsResponse
        {
            Configurations = { comparison.Configurations.Select(Configuration) },
            Source = comparison.Source,
            Diagnostics = { Diagnostics(comparison.Diagnostics) },
            Engine = comparison.Standing.Engine,
            Strength = comparison.Standing.Strength,
            Bounds = { Bounds(comparison.Standing) },
        };
    }

    /// <summary>Results nothing of which was compared, in band: why, its kind, and what the answer reported beside.</summary>
    internal static Wire.CompareResultsResponse Comparison(CompareException failure)
    {
        ArgumentNullException.ThrowIfNull(failure);
        var rendered = Comparison(failure.Partial);
        rendered.Error = failure.Message;
        rendered.FailureReason = Reason(failure.Reason);
        return rendered;
    }

    private static Wire.ConfigurationComparison Configuration(ConfigurationComparison configuration)
    {
        var rendered = new Wire.ConfigurationComparison
        {
            Id = configuration.Id,
            Name = configuration.Name,
            Behavior = configuration.Behavior,
            Target = configuration.Target,
            ResultLocation = configuration.ResultLocation,
            Status = configuration.Status,
            Unresolved = configuration.Unresolved,
            StoredRuns = configuration.StoredRuns,
            StoredSnapshots = configuration.StoredSnapshots,
            Runs = configuration.Runs,
            Completed = configuration.Completed,
            Draws = configuration.Draws,
            ClockStep = configuration.ClockStep,
            Rows = { Rows(configuration.Rows) },
            Instances = { Instances(configuration.Instances) },
            Observables = { configuration.Observables.Select(Observable) },
            Errors = { configuration.Errors },
            Notes = { configuration.Notes },
            Lines = { configuration.Lines },
        };
        if (configuration.Seed is { } seed)
        {
            rendered.Seed = seed;
        }

        return rendered;
    }

    private static Wire.ObservableComparison Observable(ObservableComparison observable)
    {
        var rendered = new Wire.ObservableComparison
        {
            Stored = observable.Stored,
            Feature = observable.Feature,
            Unit = observable.Unit,
            Tool = Distribution(observable.Tool),
            Pooled = observable.Pooled,
            Unstored = observable.Unstored,
            Run = Distribution(observable.Run),
            Produced = observable.Produced,
            Compared = observable.Compared,
            Notes = { observable.Notes },
        };
        if (observable.Difference is { } difference)
        {
            rendered.Difference = new Wire.StatisticDifference { Min = difference.Min, Mean = difference.Mean, P50 = difference.P50, P90 = difference.P90, Max = difference.Max };
        }

        if (observable.Declared is { } declared)
        {
            rendered.Declared = new Wire.CaseStatistics
            {
                Analysis = declared.Analysis,
                Rows = { declared.Rows.Select(row => new Wire.CaseStatistic { Statistic = row.Statistic, Of = row.Of, Tool = row.Tool, Run = row.Run, Difference = row.Difference }) },
                Notes = { declared.Notes },
            };
        }

        return rendered;
    }

    private static Wire.ComparedDistribution? Distribution(ComparedDistribution? distribution) => distribution is null
        ? null
        : new Wire.ComparedDistribution
        {
            Count = distribution.Count,
            Min = distribution.Min,
            Mean = distribution.Mean,
            P50 = distribution.P50,
            P90 = distribution.P90,
            Max = distribution.Max,
            Deviation = distribution.Deviation,
        };

    /// <summary>What a query selected, every element with its id, type and reported properties.</summary>
    internal static Wire.QueryResponse QueryResult(QueryResult result)
    {
        ArgumentNullException.ThrowIfNull(result);
        return new Wire.QueryResponse
        {
            Elements =
            {
                result.Elements.Select(element => new Wire.QueryResultElement
                {
                    Id = element.Id,
                    Type = element.Type,
                    Properties = { element.Properties.ToDictionary(StringComparer.Ordinal) },
                }),
            },
        };
    }

    /// <summary>A document query's columns and rows, each value in the arm of its kind.</summary>
    internal static Wire.RunDocumentQueryResponse DocumentQueryResult(DocumentQueryResult result)
    {
        ArgumentNullException.ThrowIfNull(result);
        return new Wire.RunDocumentQueryResponse
        {
            Columns = { result.Columns.Select(column => new Wire.DocumentQueryColumn { Name = column }) },
            Rows =
            {
                result.Rows.Select(row => new Wire.DocumentQueryRow
                {
                    Element = DocumentValue(row.Element),
                    Cells = { row.Cells.Select(cell => new Wire.DocumentQueryCell { Values = { cell.Select(DocumentValue) } }) },
                }),
            },
        };
    }

    /// <summary>A rendered document, in the field of its form, and the file a rendered set writes it to.</summary>
    internal static Wire.RenderDocumentResponse RenderedDocument(RenderedDocument document)
    {
        ArgumentNullException.ThrowIfNull(document);
        var rendered = new Wire.RenderDocumentResponse { FileName = document.FileName ?? string.Empty };
        switch (document.Form)
        {
            case DocumentForm.Html:
                rendered.Html = document.Content;
                break;
            case DocumentForm.AsciiDoc:
                rendered.Asciidoc = document.Content;
                break;
            default:
                rendered.Markdown = document.Content;
                break;
        }

        return rendered;
    }

    /// <summary>A view rendering, with every node, edge, row, note and notice, and absent what the API reports absent.</summary>
    internal static Wire.RenderViewResponse RenderedView(RenderedView view)
    {
        ArgumentNullException.ThrowIfNull(view);
        return new Wire.RenderViewResponse
        {
            View = view.View,
            Kind = view.Kind,
            Stated = view.Stated,
            Nodes =
            {
                view.Nodes.Select(node => new Wire.RenderNode
                {
                    Id = node.Id,
                    Kind = node.Kind,
                    Name = node.Name,
                    NameSynthesized = node.NameSynthesized,
                    Type = node.Type,
                    Detail = node.Detail,
                    Text = node.Text,
                    StandIn = node.StandIn,
                    Parent = node.Parent,
                    Ports = { node.Ports.Select(port => new Wire.RenderPort { Id = port.Id, Name = port.Name, Type = port.Type, Direction = port.Direction, Inherited = port.Inherited }) },
                    Origin = Span(node.Origin),
                    Geometry = node.Geometry is { } geometry
                        ? new Wire.RenderGeometry
                        {
                            X = geometry.X,
                            Y = geometry.Y,
                            Width = geometry.Width,
                            Height = geometry.Height,
                            HasSize = geometry.HasSize,
                            Collapsed = geometry.Collapsed,
                            Unplaced = geometry.Unplaced,
                        }
                        : null,
                    Style = Style(node.Style),
                    Compartment = node.Compartment,
                    Inherited = node.Inherited,
                }),
            },
            Edges =
            {
                view.Edges.Select(edge => new Wire.RenderEdge
                {
                    From = edge.From,
                    To = edge.To,
                    FromPort = edge.FromPort,
                    ToPort = edge.ToPort,
                    Label = edge.Label,
                    Name = edge.Name,
                    Kind = edge.Kind,
                    Origin = Span(edge.Origin),
                    Route = { edge.Route.Select(point => new Wire.RenderPoint { X = point.X, Y = point.Y }) },
                    Style = Style(edge.Style),
                }),
            },
            Columns = { view.Columns },
            Rows = { view.Rows.Select(row => new Wire.RenderRow { Cells = { row.Cells }, Origin = Span(row.Origin) }) },
            Canvas = view.Canvas is { } canvas ? new Wire.RenderCanvas { Unit = canvas.Unit, Width = canvas.Width, Height = canvas.Height, HasSize = canvas.HasSize } : null,
            Notes =
            {
                view.Notes.Select(note => new Wire.RenderNote
                {
                    Text = note.Text,
                    Anchor = note.Anchor,
                    EdgeFrom = note.EdgeFrom,
                    EdgeTo = note.EdgeTo,
                    X = note.X,
                    Y = note.Y,
                    Width = note.Width,
                    Height = note.Height,
                    HasSize = note.HasSize,
                    Origin = Span(note.Origin),
                }),
            },
            Notices = { view.Notices },
            Shown = { view.Shown },
        };
    }

    /// <summary>One engine's listing.</summary>
    internal static Wire.EngineInfo Engine(EngineInfo engine)
    {
        ArgumentNullException.ThrowIfNull(engine);
        return new Wire.EngineInfo
        {
            Name = engine.Name,
            Authority = engine.Authority,
            Answers = { engine.Answers },
            Bounds = { engine.Bounds },
            Process = engine.Process,
            ProcessFound = engine.ProcessFound,
            Ready = engine.Ready,
            Unavailable = engine.Unavailable,
            Kind = engine.Kind,
            Protocol = engine.Protocol,
            Source = engine.Source,
            Command = engine.Command,
            Version = engine.Version,
            Served = engine.Served,
        };
    }

    /// <summary>A run's trace, in the order the API reports it.</summary>
    internal static IEnumerable<Wire.DocumentEvent> Trace(IEnumerable<DocumentEvent> trace)
    {
        foreach (var record in trace)
        {
            yield return Event(record);
        }
    }

    /// <summary>A document value, in the arm of its kind.</summary>
    /// <exception cref="InvalidOperationException">The value is of a kind this renderer does not know.</exception>
    internal static Wire.DocumentValue DocumentValue(DocumentValue value)
    {
        ArgumentNullException.ThrowIfNull(value);
        return value switch
        {
            OpenSysML.DocumentValue.ElementValue element => Element(element),
            OpenSysML.DocumentValue.ObjectValue held => new() { Object = Object(held.Value) },
            OpenSysML.DocumentValue.TextValue text => new() { StringValue = text.Text },
            OpenSysML.DocumentValue.NumberValue number => number.Number switch
            {
                OpenSysML.Value.IntegerValue integer => new() { IntValue = integer.Value },
                OpenSysML.Value.BigIntegerValue big => new() { BigIntValue = Digits(big.Value) },
                OpenSysML.Value.RationalValue rational => new() { RationalValue = Terms(rational.Value) },
                OpenSysML.Value.RealValue real => new() { RealValue = real.Value },
                var other => throw new InvalidOperationException($"no rendering of a document number that is a {other.GetType().FullName}"),
            },
            OpenSysML.DocumentValue.BooleanValue boolean => new() { BoolValue = boolean.Value },
            OpenSysML.DocumentValue.InfinityValue => new() { Infinity = true },
            OpenSysML.DocumentValue.QuantityValue quantity => new() { Quantity = Quantity(quantity.Quantity) },
            OpenSysML.DocumentValue.VerdictValue verdict => new() { Verdict = Verdict(verdict.Verdict) },
            OpenSysML.DocumentValue.StateValue state => new() { State = State(state.State) },
            OpenSysML.DocumentValue.EventValue record => new() { Event = Event(record.Event) },
            _ => throw new InvalidOperationException($"no rendering of a {value.GetType().FullName}"),
        };
    }

    /// <summary>A value, in the arm of its kind.</summary>
    /// <exception cref="InvalidOperationException">The value is of a kind this renderer does not know.</exception>
    internal static Wire.Value Value(Value value)
    {
        ArgumentNullException.ThrowIfNull(value);
        return value switch
        {
            Value.NumberValue number => Number(number),
            Value.ComplexValue complex => new() { Complex = new Wire.Complex { Real = complex.Real, Imaginary = complex.Imaginary } },
            Value.BooleanValue boolean => new() { BoolValue = boolean.Value },
            Value.TextValue text => new() { StringValue = text.Value },
            Value.InstanceRefValue reference => new() { InstanceId = reference.InstanceId },
            Value.EnumLiteralValue literal => new() { EnumLiteral = EnumLiteral(literal) },
            Value.SequenceValue sequence => new() { Sequence = new Wire.ValueSequence { Elements = { Values(sequence.Elements) } } },
            Value.SetValue set => new() { Set = new Wire.ValueSet { Elements = { Values(set.Elements) } } },
            Value.QuantityValue quantity => new() { Quantity = Quantity(quantity.Quantity) },
            Value.ArrayValue array => new() { Array = new Wire.Array { Dimensions = { array.Dimensions }, Elements = { Values(array.Elements) } } },
            Value.VectorValue vector => new() { Vector = new Wire.Vector { Components = { Values(vector.Components) } } },
            Value.VectorQuantityValue vector => new() { VectorQuantity = new Wire.VectorQuantity { Components = { Quantities(vector.Components) } } },
            Value.TensorQuantityValue tensor => new()
            {
                TensorQuantity = new Wire.TensorQuantity { Dimensions = { tensor.Dimensions }, Components = { Quantities(tensor.Components) } },
            },
            Value.MeasurementRefValue reference => new()
            {
                MeasurementRef = new Wire.MeasurementRef { Unit = reference.Unit, UnitTerm = Term(reference.Reduction), UnitId = reference.UnitId ?? string.Empty },
            },
            Value.FunctionValue function => new() { Function = new Wire.Function { CalcId = function.CalcId, SelfId = function.SelfId ?? 0 } },
            Value.MetaobjectValue metaobject => new() { Metaobject = new Wire.Metaobject { ElementId = metaobject.ElementId, MetaclassId = metaobject.MetaclassId } },
            Value.NullValue nothing => new() { Null = nothing.Unsupported },
            OpenSysML.Value.UnsetValue => new() { Unset = true },
            Value.UndeterminedValue undetermined => new() { Undetermined = Undetermined(undetermined) },
            OpenSysML.Value.InfinityValue => new() { Infinity = true },
            _ => throw new InvalidOperationException($"no rendering of a {value.GetType().FullName}, a value of kind {value.Kind}"),
        };
    }

    private static IEnumerable<Wire.Value> Values<T>(IEnumerable<T> values)
        where T : OpenSysML.Value
    {
        foreach (var value in values)
        {
            yield return Value(value);
        }
    }

    private static IEnumerable<Wire.Quantity> Quantities(IEnumerable<Quantity> quantities)
    {
        foreach (var quantity in quantities)
        {
            yield return Quantity(quantity);
        }
    }

    private static Wire.Value Number(OpenSysML.Value.NumberValue number) => number switch
    {
        OpenSysML.Value.IntegerValue integer => new() { IntValue = integer.Value },
        OpenSysML.Value.BigIntegerValue big => new() { BigIntValue = Digits(big.Value) },
        OpenSysML.Value.RationalValue rational => new() { RationalValue = Terms(rational.Value) },
        OpenSysML.Value.RealValue real => new() { RealValue = real.Value },
        _ => throw new InvalidOperationException($"no rendering of a {number.GetType().FullName}, a number of kind {number.Kind}"),
    };

    private static Wire.EnumLiteral EnumLiteral(Value.EnumLiteralValue literal)
    {
        var rendered = new Wire.EnumLiteral { LiteralId = literal.LiteralId, EnumerationId = literal.EnumerationId, Name = literal.Name };
        if (literal.Scalar is { } scalar)
        {
            rendered.Value = Value(scalar);
        }

        return rendered;
    }

    private static Wire.Quantity Quantity(Quantity quantity)
    {
        var rendered = new Wire.Quantity { Unit = quantity.Unit };
        switch (quantity.Magnitude)
        {
            case Value.IntegerValue integer:
                rendered.IntMagnitude = integer.Value;
                break;
            case Value.BigIntegerValue big:
                rendered.BigIntMagnitude = Digits(big.Value);
                break;
            case Value.RationalValue rational:
                rendered.RationalMagnitude = Terms(rational.Value);
                break;
            case Value.RealValue real:
                rendered.RealMagnitude = real.Value;
                break;
            default:
                throw new InvalidOperationException($"no rendering of a quantity whose magnitude is a {quantity.Magnitude.GetType().FullName}");
        }

        if (quantity.Reduction is { } reduction)
        {
            rendered.UnitTerm = Term(reduction);
        }

        return rendered;
    }

    private static Wire.UnitTerm Term(UnitTerm term)
    {
        var rendered = new Wire.UnitTerm { ScaleNum = term.ScaleNumerator, ScaleDen = term.ScaleDenominator };
        foreach (var factor in term.Factors)
        {
            rendered.Factors.Add(new Wire.UnitFactor { UnitId = factor.UnitId, Exponent = factor.Exponent });
        }

        return rendered;
    }

    // The client reads an absent count as empty bounds, so only a bound it reports brings the count back.
    private static Wire.Undetermined Undetermined(Value.UndeterminedValue undetermined)
    {
        var rendered = new Wire.Undetermined { Reason = undetermined.Reason };
        if (undetermined.CountLower.Length > 0 || undetermined.CountUpper.Length > 0)
        {
            rendered.Count = new Wire.MultiplicityInfo { Lower = undetermined.CountLower, Upper = undetermined.CountUpper };
        }

        return rendered;
    }

    private static string Digits(BigInteger integer) => integer.ToString(CultureInfo.InvariantCulture);

    private static Wire.Rational Terms(Rational rational) =>
        new() { Numerator = Digits(rational.Numerator), Denominator = Digits(rational.Denominator) };

    private static void Values(MapField<string, Wire.Value> field, ValueDictionary<Value> values)
    {
        foreach (var (name, value) in values)
        {
            field.Add(name, Value(value));
        }
    }

    // An element named by no id is the value of no arm, which is how the service sends one.
    private static Wire.DocumentValue Element(DocumentValue.ElementValue element) =>
        element.Id is { } id ? new() { ElementId = id, ElementType = element.ElementType } : new() { ElementType = element.ElementType };

    private static Wire.Span? Span(SourceSpan? span) => span is null
        ? null
        : new Wire.Span { File = span.File, StartLine = span.StartLine, StartCol = span.StartColumn, EndLine = span.EndLine, EndCol = span.EndColumn };

    private static Wire.RenderStyle? Style(ViewStyle? style) => style is null
        ? null
        : new Wire.RenderStyle { Fill = style.Fill, Line = style.Line, Text = style.Text, Font = style.Font, FontSize = style.FontSize, Bold = style.Bold, Italic = style.Italic };

    private static Wire.DocumentValue? OptionalElement(DocumentValue.ElementValue? element) => element is null ? null : Element(element);

    private static Wire.DocumentObject Object(DocumentObject held) =>
        new() { InstanceId = held.InstanceId, Path = held.Path, Element = OptionalElement(held.Element) };

    private static Wire.DocumentVerdict Verdict(DocumentVerdict verdict) =>
        new()
        {
            Assertion = OptionalElement(verdict.Assertion),
            Kind = verdict.Kind,
            Text = verdict.Text,
            Path = verdict.Path,
            Verdict = verdict.Verdict,
            Condition = verdict.Condition,
            Reason = verdict.Reason,
            Verification = { verdict.Verification },
        };

    private static Wire.DocumentState State(DocumentState state) =>
        new()
        {
            Object = state.Subject is { } subject ? Object(subject) : null,
            Machine = state.Machine,
            Name = state.Name,
            StatePath = state.StatePath,
            State = OptionalElement(state.State),
            Region = state.Region,
            Enclosing = { state.Enclosing },
        };

    private static Wire.DocumentEvent Event(DocumentEvent record) =>
        new()
        {
            Kind = record.Kind,
            Time = record.Time is { } time ? DocumentValue(time) : null,
            Object = record.Subject is { } subject ? Object(subject) : null,
            Machine = record.Machine,
            State = record.State,
            From = record.From,
            To = record.To,
            Target = record.Target is { } target ? Object(target) : null,
            Event = record.Event,
            Payload = { record.Payload },
            Alternatives = { record.Alternatives },
            Taken = record.Taken,
            Text = record.Text,
            Token = record.Token,
            Step = record.Step,
            Feature = record.Feature,
            Value = record.Value is { } value ? Value(value) : null,
        };

    private static IEnumerable<Wire.CaseEvaluation> Evaluations(IEnumerable<CaseEvaluation> evaluations)
    {
        foreach (var evaluation in evaluations)
        {
            var rendered = new Wire.CaseEvaluation
            {
                FunctionId = evaluation.FunctionId,
                Arguments = { Values(evaluation.Arguments) },
                Error = evaluation.Error,
                Selected = evaluation.Selected,
                Tied = evaluation.Tied,
            };
            if (evaluation.Result is { } result)
            {
                rendered.Result = Value(result);
            }

            yield return rendered;
        }
    }

    private static Wire.ToolDryRun ToolDryRun(ToolDryRun preview)
    {
        var rendered = new Wire.ToolDryRun
        {
            Action = preview.Action,
            Tool = preview.Tool,
            Version = preview.Version,
            Manifest = preview.Manifest,
            Executable = preview.Executable,
            Protocol = preview.Protocol,
            Args = { preview.Args },
            Env = { preview.Environment },
            Cwd = preview.WorkingDirectory,
            Stdin = preview.Stdin,
            StdinData = preview.StdinData,
            OutputDir = preview.OutputDirectory,
            Inputs = { preview.Inputs.Select(input => new Wire.ToolDryRunInput { Variable = input.Variable, Parameter = input.Parameter, Value = input.Value, Unit = input.Unit }) },
            Outputs = { preview.Outputs.Select(output => new Wire.ToolDryRunOutput { Variable = output.Variable, Parameter = output.Parameter }) },
            ReplySource = preview.ReplySource,
            Lines = { preview.Lines },
        };
        if (preview.InputFile is { } file)
        {
            rendered.InputFile = new Wire.ToolDryRunFile { Name = file.Name, Format = file.Format, Data = file.Data };
        }

        return rendered;
    }

    private static Wire.CheckWitness? Witness(CheckWitness? witness) =>
        witness is null
            ? null
            : new()
            {
                Schedule = witness.Schedule,
                Draws = { witness.Draws },
                Choices = { witness.Choices },
                Inputs = { witness.Inputs },
                FileName = witness.FileName,
                FileContent = witness.FileContent,
            };
}
