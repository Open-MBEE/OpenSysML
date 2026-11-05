namespace OpenMBEE.OpenSysML;

/// <summary>Where <see cref="Model.EvalAsync"/> and <see cref="Model.EvaluateAsync"/> evaluate an expression.</summary>
public sealed class EvalOptions
{
    /// <summary>The service's defaults: the model's first document's root scope, and no subject.</summary>
    public static EvalOptions Default { get; } = new();

    /// <summary>
    /// The qualified name of the symbol whose scope the expression's names resolve in, so a feature
    /// reads its declared default; null for the root scope.
    /// </summary>
    public string? ContextSymbolId { get; init; }

    /// <summary>
    /// The qualified name of a definition or usage to instantiate and evaluate against, so a feature
    /// reads that object's value rather than the declared default; null for none. Without a context
    /// the subject also names the scope. Needs the <see cref="Capabilities.EvaluateSubject"/> capability.
    /// </summary>
    public string? SubjectSymbolId { get; init; }
}
