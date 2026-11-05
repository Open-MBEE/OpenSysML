using System;
using System.Linq;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A model the service has parsed, named by the <see cref="Hash"/> every later call carries. It is
/// immutable and safe to share between threads: it holds the hash, what the parse reported, and the
/// connection it is read over, nothing that a call changes.
/// </summary>
/// <remarks>
/// <para>
/// A model from <see cref="Connection.LoadAsync"/>, <see cref="Connection.LoadContentAsync"/>,
/// <see cref="Connection.ParseSourcesAsync"/> or <see cref="Connection.ModelByHash"/> borrows its
/// connection: disposing it does nothing, and the connection's owner closes the connection. A model
/// from <see cref="OpenSysMLClient"/> owns a connection of its own, which disposing the model closes.
/// </para>
/// <para>
/// Every call answers what the service reported or throws: <see cref="ModelException"/> when the
/// service answered but reported a failure about the model, <see cref="ModelNotFoundException"/>
/// when it no longer holds the model, <see cref="ServiceException"/> when it refused the call, and
/// <see cref="CapabilityException"/>, before anything is sent, when it does not advertise what the
/// call needs.
/// </para>
/// </remarks>
public sealed partial class Model : IAsyncDisposable
{
    private readonly string parseMethod;
    private readonly bool ownsConnection;

    internal Model(
        Connection connection,
        string hash,
        string parseMethod,
        ValueList<Symbol> roots,
        ValueList<Diagnostic> parseDiagnostics,
        ValueList<string> documents,
        string? baseModelHash,
        ValueList<string> affected,
        bool ownsConnection = false)
    {
        Connection = connection;
        Hash = hash;
        this.parseMethod = parseMethod;
        Roots = roots;
        ParseDiagnostics = parseDiagnostics;
        Documents = documents;
        BaseModelHash = baseModelHash;
        Affected = affected;
        Errors = new ValueList<Diagnostic>(parseDiagnostics.Where(diagnostic => diagnostic.Severity == DiagnosticSeverity.Error));
        this.ownsConnection = ownsConnection;
    }

    /// <summary>The connection the model is read over.</summary>
    public Connection Connection { get; }

    /// <summary>The hash the service knows the model by.</summary>
    public string Hash { get; }

    /// <summary>
    /// The model's outermost elements, in document order: one per document of a
    /// <see cref="Connection.ParseSourcesAsync"/> model, the one root of a loaded model, and none for
    /// a model named by hash alone.
    /// </summary>
    public ValueList<Symbol> Roots { get; }

    /// <summary>The first of <see cref="Roots"/>, the only one of a loaded model; null when there is none.</summary>
    public Symbol? Root => Roots.IsEmpty ? null : Roots[0];

    /// <summary>
    /// What the parse that produced the model reported, in report order; empty for a model named by
    /// hash alone. <see cref="GetDiagnosticsAsync"/> asks the service again, under a lint selection.
    /// </summary>
    public ValueList<Diagnostic> ParseDiagnostics { get; }

    /// <summary>
    /// The name of each document, in document order, as diagnostics report it: each path or name a
    /// <see cref="Connection.ParseSourcesAsync"/> model was given (empty for an inline document named
    /// by position), the path a loaded file was read from, and none for inline notation or a model
    /// named by hash alone.
    /// </summary>
    public ValueList<string> Documents { get; }

    /// <summary>The <see cref="SourcesOptions.BaseModelHash"/> the parse related its answer to; null when it named none.</summary>
    public string? BaseModelHash { get; }

    /// <summary>
    /// The documents, by name and in document order, whose results may differ from those of the
    /// <see cref="BaseModelHash"/> model; every other document's are that model's. Meaningful only when
    /// <see cref="BaseModelHash"/> is not null, and then every document when the service cannot relate
    /// the two models.
    /// </summary>
    public ValueList<string> Affected { get; }

    /// <summary>The error-severity diagnostics among <see cref="ParseDiagnostics"/>, which make a model unusable.</summary>
    public ValueList<Diagnostic> Errors { get; }

    /// <summary>
    /// Whether the parse reported errors. A model with errors is still navigable, but its symbols may be
    /// missing or unresolved; check this, or call <see cref="RequireNoErrors"/>, before treating it as
    /// the model that was written.
    /// </summary>
    public bool HasErrors => !Errors.IsEmpty;

    /// <summary>Whether disposing this model closes its connection, as for a model from <see cref="OpenSysMLClient"/>.</summary>
    public bool OwnsConnection => ownsConnection;

    /// <summary>How many documents the model has, as its parse said; null for a model named by hash or answered by a call that does not say.</summary>
    internal int? DocumentCount => parseMethod switch
    {
        Connection.ParseFileMethod => 1,
        Connection.ParseSourcesMethod => Documents.Count,
        _ => null,
    };

    /// <summary>Requires a parse without errors, for chaining onto a load.</summary>
    /// <returns>This model.</returns>
    /// <exception cref="ModelErrorsException">The parse reported errors, which the exception carries.</exception>
    public Model RequireNoErrors()
    {
        if (Errors.IsEmpty)
        {
            return this;
        }

        var where = Documents.Count == 1 && Documents[0].Length > 0 ? Documents[0] : "the model";
        var summary = string.Join("; ", Errors.Take(3).Select(error => error.Message));
        if (Errors.Count > 3)
        {
            summary += $"; ... and {Errors.Count - 3} more";
        }

        throw new ModelErrorsException($"{where} has {Errors.Count} error(s): {summary}", parseMethod, Errors);
    }

    /// <summary>
    /// Closes the model's connection when the model owns it, once however often it is called; for a
    /// model that borrows its connection, does nothing. Concurrent calls wait for the same close.
    /// Never throws: a failure to close is logged.
    /// </summary>
    /// <returns>When the connection is closed.</returns>
    public ValueTask DisposeAsync() => ownsConnection ? Connection.DisposeAsync() : ValueTask.CompletedTask;

    /// <summary>The model, as <c>model 3fa2… (2 roots, 1 error)</c>.</summary>
    /// <returns>The model's text.</returns>
    public override string ToString() =>
        $"model {Hash} ({Roots.Count} root{(Roots.Count == 1 ? string.Empty : "s")}, {Errors.Count} error{(Errors.Count == 1 ? string.Empty : "s")})";

    /// <summary>The same model, owning its connection, which disposing it then closes.</summary>
    internal Model Owning() =>
        new(Connection, Hash, parseMethod, Roots, ParseDiagnostics, Documents, BaseModelHash, Affected, ownsConnection: true);
}
