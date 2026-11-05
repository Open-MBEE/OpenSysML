namespace OpenMBEE.OpenSysML;

/// <summary>
/// Why the service refused a batch of edits, so a caller acts on the kind rather than on the message. Every refusal
/// is one of these: an edit is never silently dropped. A number this client does not name, from a newer service, is
/// kept as that number over the protobuf encodings; over Connect JSON an unknown name reads as <see cref="Unspecified"/>.
/// </summary>
public enum EditFailure
{
    /// <summary>No failure, or one the service did not classify.</summary>
    Unspecified = 0,

    /// <summary>The batch named no edit.</summary>
    NoOperations = 1,

    /// <summary>No element of the name a target or owner gave is in the model, or it is declared in another document than the one edited.</summary>
    UnknownTarget = 2,

    /// <summary>The name denotes several declarations.</summary>
    AmbiguousTarget = 3,

    /// <summary>The element a value was set on can carry no value.</summary>
    NotValued = 4,

    /// <summary>The new value does not parse as an expression; the diagnostics say where.</summary>
    InvalidValue = 5,

    /// <summary>The new name does not lex as an identifier.</summary>
    InvalidName = 6,

    /// <summary>The element renamed declares no name to rewrite.</summary>
    NotNamed = 7,

    /// <summary>A rename would break references to the element.</summary>
    RenameReferenced = 8,

    /// <summary>Two edits cover the same bytes.</summary>
    OverlappingEdits = 9,

    /// <summary>The edited source has errors the original had not; the diagnostics name them.</summary>
    ResultInvalid = 10,

    /// <summary>The namespace a member was added to does not exist.</summary>
    OwnerUnknown = 11,

    /// <summary>The owner named cannot contain members.</summary>
    OwnerNotNamespace = 12,

    /// <summary>The declaration kind is not one the document's language has.</summary>
    IllegalKind = 13,

    /// <summary>The owner already declares the name.</summary>
    MemberNameTaken = 14,

    /// <summary>A delete would leave references to the element; the referrers name them.</summary>
    DeleteReferenced = 15,

    /// <summary>A move's owner is its target or inside it.</summary>
    OwnerInsideTarget = 16,

    /// <summary>A move would leave a reference no spelling restores; the referrers name them.</summary>
    MoveReferenced = 17,

    /// <summary>
    /// The target is referred to from a document the edit cannot rewrite: a library document, or, for a move, any
    /// document but the target's own.
    /// </summary>
    ReferencedElsewhere = 18,
}
