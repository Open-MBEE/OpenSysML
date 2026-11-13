using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service answered a batch of edits but refused it: the request was well formed, and this is why the model was not
/// edited. Nothing of the batch was applied, so a refusal never leaves a partial edit. <see cref="Failure"/> is the kind
/// a caller acts on; <see cref="ModelException.Diagnostics"/> holds the parse errors of an unreadable new value or the
/// errors the edited source was found to have, and <see cref="Referrers"/> the declarations whose references a rename,
/// delete or move would break.
/// </summary>
public class EditException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The refusal, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="diagnostics">The diagnostics the answer carried.</param>
    /// <param name="failure">What kind of refusal it is.</param>
    /// <param name="referringElements">Where the references to the refused target are made, as the service spelled them.</param>
    /// <param name="referrers">The referring declarations, each with its document.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A diagnostic, referring element or referrer is null.</exception>
    public EditException(
        string message,
        string method,
        IEnumerable<Diagnostic> diagnostics,
        EditFailure failure,
        IEnumerable<string> referringElements,
        IEnumerable<Referrer> referrers)
        : base(message, method, diagnostics)
    {
        Failure = failure;
        ReferringElements = new ValueList<string>(referringElements);
        Referrers = new ValueList<Referrer>(referrers);
    }

    /// <summary>What kind of refusal this is; a kind this release does not name keeps its number.</summary>
    public EditFailure Failure { get; }

    /// <summary>
    /// Where the references to the target of a refused rename, delete or move are made: the qualified name of each
    /// referring namespace, suffixed with its document in parentheses when that is not the document edited. Empty for a
    /// refusal that names none.
    /// </summary>
    public ValueList<string> ReferringElements { get; }

    /// <summary>
    /// The declarations referring to the refused target, each with the document declaring it, in document then name
    /// order; empty for a refusal that names none, and from a service without <see cref="Capabilities.EditDocuments"/>,
    /// which names <see cref="ReferringElements"/> alone.
    /// </summary>
    public ValueList<Referrer> Referrers { get; }
}
