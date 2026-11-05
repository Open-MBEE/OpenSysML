using System.Collections.Generic;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Requests and answers setting every field the graph export, data import and result comparison calls can reach.</summary>
internal static class CompleteGraphDataWire
{
    /// <summary>The model the requests name.</summary>
    internal const string Hash = "h";

    /// <summary>The model an import makes.</summary>
    internal const string ImportedHash = "imported";

    /// <summary>A form as the service writes one: characters outside ASCII, escapes, no spacing and one trailing newline.</summary>
    internal const string Form = "{\"version\":1,\"subject\":\"Démo::räce\",\"actions\":[{\"name\":\"Démo::räce\",\"span\":{\"document\":\"\\u003ccontent\\u003e\"},\"doc\":\"tab\\tquote\\\" ✓\"}]}\n";

    /// <summary>An export naming a subject outside ASCII.</summary>
    internal static Wire.ExportGraphsRequest ExportRequest => new() { ModelHash = Hash, Subject = "Démo::räce" };

    /// <summary>The form of the exported subject.</summary>
    internal static Wire.ExportGraphsResponse Graphs => new() { Content = Form, Version = 1, Subject = "Démo::räce" };

    /// <summary>Two data files: one read by its name and header, one read as a named format by a mapping.</summary>
    internal static IReadOnlyList<DataImport> Imports =>
    [
        new DataImport("fleet.csv", "element,count,label\nV::vehicle,7,fast\nV::car,9,\n"),
        new DataImport("fleet.txt", "{\"id\":\"car\",\"n\":3}\n") { Format = "jsonl", Map = "{\"element\":{\"path\":\"/id\",\"prefix\":\"V::\"},\"features\":{\"count\":{\"path\":\"/n\"}}}" },
    ];

    /// <summary>The options setting every field an import request has.</summary>
    internal static ImportOptions ImportOptions => new() { As = "values", AcceptDocuments = true };

    /// <summary>The request <see cref="Imports"/> and <see cref="ImportOptions"/> make.</summary>
    internal static Wire.ImportDataRequest ImportRequest => new()
    {
        ModelHash = Hash,
        Imports =
        {
            new Wire.DataImport { Name = "fleet.csv", Content = "element,count,label\nV::vehicle,7,fast\nV::car,9,\n" },
            new Wire.DataImport
            {
                Name = "fleet.txt",
                Content = "{\"id\":\"car\",\"n\":3}\n",
                ImportFormat = "jsonl",
                ImportMap = "{\"element\":{\"path\":\"/id\",\"prefix\":\"V::\"},\"features\":{\"count\":{\"path\":\"/n\"}}}",
            },
        },
        ImportAs = "values",
        AcceptDocuments = true,
    };

    /// <summary>The same request as a dry run.</summary>
    internal static Wire.ImportDataRequest DryRunRequest
    {
        get
        {
            var request = ImportRequest;
            request.ImportDryRun = true;
            return request;
        }
    }

    /// <summary>What both files set, each assignment's every fact, a redefinition reaching a nested feature setting no value.</summary>
    internal static IEnumerable<Wire.ImportedFile> Files =>
    [
        new Wire.ImportedFile
        {
            Name = "fleet.csv",
            Values = 3,
            Elements = 2,
            Assignments =
            {
                new Wire.ImportedValue
                {
                    Element = "V::vehicle",
                    Feature = "count",
                    Value = "7",
                    Redefines = "V::Vehicle::count",
                    Where = "fleet.csv line 2, column count",
                    Description = "V::vehicle::count = 7 (redefines V::Vehicle::count)",
                },
                new Wire.ImportedValue
                {
                    Element = "V::vehicle",
                    Feature = "label",
                    Value = "\"fast\"",
                    Redefines = "V::Vehicle::label",
                    Where = "fleet.csv line 2, column label",
                    Description = "V::vehicle::label = \"fast\" (redefines V::Vehicle::label)",
                },
                new Wire.ImportedValue { Element = "V::car", Feature = "engine", Redefines = "V::Vehicle::engine", Where = "fleet.csv line 3", Description = "V::car::engine (redefines V::Vehicle::engine)" },
                new Wire.ImportedValue { Element = "V::car", Feature = "count", Value = "9", Where = "fleet.csv line 3, column count", Description = "V::car::count = 9" },
            },
        },
        new Wire.ImportedFile { Name = "fleet.txt" },
    ];

    /// <summary>The notation the import made of a single-document model.</summary>
    internal const string Notation = "package V {\n\tpart car : Vehicle {\n\t\tattribute :>> count = 9;\n\t}\n}\n";

    /// <summary>A single-document model imported into: its notation in content and in its one document.</summary>
    internal static Wire.ImportDataResponse Imported => new()
    {
        Files = { Files },
        Documents = { new Wire.EditedDocument { Name = "fleet.sysml", Content = Notation } },
        Content = Notation,
        ModelHash = ImportedHash,
        Diagnostics = { CompleteWire.Diagnostic },
    };

    /// <summary>A model of several documents imported into: each changed document, no content.</summary>
    internal static Wire.ImportDataResponse ImportedSeveral => new()
    {
        Files = { Files },
        Documents =
        {
            new Wire.EditedDocument { Name = "vehicles.sysml", Content = Notation },
            new Wire.EditedDocument { Name = "fleet.sysml", Content = "package F { import V::*; }\n" },
        },
        ModelHash = ImportedHash,
    };

    /// <summary>A dry run: what each file would set, nothing made.</summary>
    internal static Wire.ImportDataResponse DryRun => new() { Files = { Files }, Diagnostics = { CompleteWire.BareDiagnostic } };

    /// <summary>An import the model refused, with the diagnostics the values raised.</summary>
    internal static Wire.ImportDataResponse Refusal => new()
    {
        Error = "the imported values make the model invalid: count is not an Integer",
        Diagnostics = { CompleteWire.Diagnostic },
    };

    /// <summary>The options setting every field a comparison request has.</summary>
    internal static CompareOptions CompareOptions => new()
    {
        Actions = ["Group 1", "Cfg::Nothing"],
        Runs = 4,
        Seed = ulong.MaxValue,
        Draws = "average",
        ClockStep = 0.5,
        Observe = [new ObservablePair("total", "target.total"), new ObservablePair("count")],
        Engine = "sweep",
    };

    /// <summary>The results a comparison is asked of.</summary>
    internal const string Results = "{\"source\":\"probe.xmi\",\"configurations\":[]}";

    /// <summary>The request <see cref="CompareOptions"/> makes.</summary>
    internal static Wire.CompareResultsRequest CompareRequest => new()
    {
        ModelHash = Hash,
        Results = Results,
        Actions = { "Group 1", "Cfg::Nothing" },
        Runs = 4,
        Seed = ulong.MaxValue,
        Draws = "average",
        ClockStep = 0.5,
        Observe = { new Wire.ObservablePair { Stored = "total", Feature = "target.total" }, new Wire.ObservablePair { Stored = "count" } },
        Engine = "sweep",
    };

    /// <summary>
    /// A comparison of three configurations: one that ran, its observables stored, pooled with the case's statistics,
    /// unstored and not compared; one whose runs drew nothing; and one that could not be run.
    /// </summary>
    internal static Wire.CompareResultsResponse Comparison => new()
    {
        Configurations =
        {
            new Wire.ConfigurationComparison
            {
                Id = "_c1",
                Name = "Cfg::'Group 1'",
                Behavior = "run",
                Target = "target",
                ResultLocation = "Results",
                Status = "fails",
                StoredRuns = 10,
                StoredSnapshots = 3,
                Runs = 4,
                Completed = 2,
                Draws = "average",
                Seed = ulong.MaxValue,
                ClockStep = 0.5,
                Rows = { CompleteVerificationWire.Sweep.Rows },
                Instances = { CompleteWire.Instance },
                Observables =
                {
                    new Wire.ObservableComparison
                    {
                        Stored = "total",
                        Feature = "target.total",
                        Unit = "kg",
                        Tool = new Wire.ComparedDistribution { Count = 2, Min = 4, Mean = 5, P50 = 4.5, P90 = 5.9, Max = 6, Deviation = 1.4142135623730951 },
                        Run = new Wire.ComparedDistribution { Count = 2, Min = 4, Mean = 4, P50 = 4, P90 = 4, Max = 4 },
                        Produced = 2,
                        Compared = true,
                        Difference = new Wire.StatisticDifference { Min = "+0.0%", Mean = "-20.0%", P50 = "-11.1%", P90 = "-32.2%", Max = "-33.3%" },
                    },
                    new Wire.ObservableComparison
                    {
                        Stored = "mass",
                        Feature = "target.mass",
                        Tool = new Wire.ComparedDistribution { Count = 10, Mean = 5 },
                        Pooled = true,
                        Run = new Wire.ComparedDistribution { Count = 2, Min = double.NegativeInfinity, Mean = 1.5, P50 = double.NaN, P90 = 1e-300, Max = double.MaxValue },
                        Produced = 2,
                        Compared = true,
                        Difference = new Wire.StatisticDifference { Mean = "-20.0%" },
                        Declared = new Wire.CaseStatistics
                        {
                            Analysis = "Cfg::MassCase",
                            Rows =
                            {
                                new Wire.CaseStatistic { Statistic = "N", Of = "return N", Tool = "10", Run = "2" },
                                new Wire.CaseStatistic { Statistic = "Mean", Of = "return Mean", Tool = "5.0", Run = "4.0", Difference = "-20.0%" },
                            },
                            Notes = { "the case declares no OutOfSpec" },
                        },
                        Notes = { "note: \"summary\" summarises 10 run(s) of mass: mean 5.0" },
                    },
                    new Wire.ObservableComparison { Stored = "count", Feature = "target.count", Unstored = true, Compared = true, Notes = { "note: no completed run produced target.count" } },
                    new Wire.ObservableComparison { Stored = "speed", Feature = "target.speed", Notes = { "note: the tool stored no observable named speed" } },
                },
                Errors = { "division by zero" },
                Notes = { "the configuration states no numberOfRuns" },
                Lines = { "compare Cfg::'Group 1' — 10 stored run(s)", "error: division by zero" },
            },
            new Wire.ConfigurationComparison { Id = "_c2", Name = "Cfg::'Group 2'", Behavior = "run", Status = "holds", Runs = 1, Completed = 1, Draws = "min" },
            new Wire.ConfigurationComparison
            {
                Name = "Cfg::Nothing",
                Status = "unresolved",
                Unresolved = "no configuration is named Cfg::Nothing",
                Lines = { "error: no configuration is named Cfg::Nothing" },
            },
        },
        Source = "probe.xmi",
        Diagnostics = { CompleteWire.Diagnostic },
        Engine = "sweep",
        Strength = "observed",
        Bounds = { CompleteVerificationWire.Bounds },
    };

    /// <summary>Results nothing of which was compared, with the failure's kind and what the answer reported beside.</summary>
    internal static Wire.CompareResultsResponse CompareRefusal => new()
    {
        Source = "probe.xmi",
        Error = "the results index no configuration this model declares",
        FailureReason = Wire.FailureReason.Evaluation,
        Diagnostics = { CompleteWire.Diagnostic },
        Engine = "sweep",
        Strength = "not covered",
        Bounds = { CompleteVerificationWire.Bounds },
    };

    /// <summary>Every answer above.</summary>
    internal static IEnumerable<IMessage> All => [Graphs, Imported, ImportedSeveral, DryRun, Refusal, Comparison, CompareRefusal];
}
