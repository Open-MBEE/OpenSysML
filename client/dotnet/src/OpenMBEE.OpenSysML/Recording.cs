using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.RecordRunAsync"/> did: the run, sweep or sample it made, the record declarations it generated,
/// the notation they were merged into, and the <see cref="Model"/> holding them. The model the recording was asked of
/// is left as it was; nothing is written to disk.
/// </summary>
public sealed record Recording
{
    /// <summary>Creates a recording, copying its collections.</summary>
    /// <param name="run">The run of the case, when it was run once.</param>
    /// <param name="sweep">The sweep's table, when the case was run over the rows of a sweep.</param>
    /// <param name="sample">The sample's runs and conclusion, when the case was run as a Monte Carlo sample.</param>
    /// <param name="package">The qualified name of the package the records went into.</param>
    /// <param name="definition">The qualified name of the per-case record definition.</param>
    /// <param name="records">The qualified names of the records made, in order.</param>
    /// <param name="source">The record declarations generated: the package's nesting down to it, with the definition when new and every record.</param>
    /// <param name="content">The notation of the model's one document with the records merged in; empty when the model has several, or the records are a document of their own.</param>
    /// <param name="documents">The document the records went into.</param>
    /// <param name="skipped">The rows or runs that failed and were not recorded.</param>
    /// <param name="skipReasons">Why runs were not recorded, as <c>sysml -record-run</c> reports them.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <param name="model">The model with the records in it.</param>
    /// <exception cref="ArgumentNullException">A string, a collection or <paramref name="model"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null, or not exactly one of <paramref name="run"/>, <paramref name="sweep"/> and <paramref name="sample"/> is given.</exception>
    public Recording(
        Analysis? run,
        Sweep? sweep,
        Runs? sample,
        string package,
        string definition,
        IEnumerable<string> records,
        string source,
        string content,
        IEnumerable<EditedDocument> documents,
        long skipped,
        IEnumerable<string> skipReasons,
        IEnumerable<Diagnostic> diagnostics,
        Model model)
    {
        ArgumentNullException.ThrowIfNull(package);
        ArgumentNullException.ThrowIfNull(definition);
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(content);
        ArgumentNullException.ThrowIfNull(model);
        if ((run is null ? 0 : 1) + (sweep is null ? 0 : 1) + (sample is null ? 0 : 1) != 1)
        {
            throw new ArgumentException("A recording carries exactly one of the run, the sweep and the sample it made.", nameof(run));
        }

        Run = run;
        Sweep = sweep;
        Sample = sample;
        Package = package;
        Definition = definition;
        Records = new ValueList<string>(records);
        Source = source;
        Content = content;
        Documents = new ValueList<EditedDocument>(documents);
        Skipped = skipped;
        SkipReasons = new ValueList<string>(skipReasons);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Model = model;
    }

    /// <summary>The run of the case, when it was run once; null otherwise.</summary>
    public Analysis? Run { get; }

    /// <summary>The sweep's table, when the case was run over the rows of <see cref="RecordOptions.Ranges"/>; null otherwise.</summary>
    public Sweep? Sweep { get; }

    /// <summary>The sample's runs and conclusion, when the case was run <see cref="RecordOptions.Runs"/> times; null otherwise.</summary>
    public Runs? Sample { get; }

    /// <summary>The qualified name of the package the records went into.</summary>
    public string Package { get; }

    /// <summary>The qualified name of the per-case record definition.</summary>
    public string Definition { get; }

    /// <summary>The qualified names of the records made, in order: one per completed run, then a sample's own.</summary>
    public ValueList<string> Records { get; }

    /// <summary>The record declarations generated: the target package's nesting down to it, holding the record definition when new and every record.</summary>
    public string Source { get; }

    /// <summary>
    /// The notation of the model's one document with the records merged in, byte-identical to it outside the package
    /// they joined; empty when the model has several documents, or the records are a document of their own. Read
    /// <see cref="Documents"/> for every case.
    /// </summary>
    public string Content { get; }

    /// <summary>
    /// The document the records went into, named as the parse named it, or, when no document held the package's
    /// nesting, the document they make, named after their package (<c>Records.sysml</c>).
    /// </summary>
    public ValueList<EditedDocument> Documents { get; }

    /// <summary>The rows or runs that failed and were not recorded.</summary>
    public long Skipped { get; }

    /// <summary>Why runs were not recorded, as <c>sysml -record-run</c> reports them.</summary>
    public ValueList<string> SkipReasons { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>
    /// The model with the records in it, which the service holds under a hash of its own: a new handle borrowing the
    /// connection the recording was made over. It has no roots and no parse diagnostics, as a model named by hash alone.
    /// </summary>
    public Model Model { get; }
}
