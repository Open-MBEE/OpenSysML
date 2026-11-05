namespace OpenMBEE.OpenSysML;

/// <summary>
/// What kind of failure an answer's error reports, so a caller acts on the kind rather than on the
/// message. A number this client does not name, from a newer service, is kept as that number over the
/// protobuf encodings; over Connect JSON an unknown name reads as <see cref="Unspecified"/>.
/// </summary>
public enum FailureReason
{
    /// <summary>No failure, or one the service did not classify.</summary>
    Unspecified = 0,

    /// <summary>The condition or calculation could not be evaluated: an unbound feature, incommensurable units, an exhausted step budget.</summary>
    Evaluation = 1,

    /// <summary>The symbol named declares something else, which is a wrong request rather than an undecided answer about the model.</summary>
    WrongKind = 2,

    /// <summary>Several objects carry the element; name one as the subject.</summary>
    AmbiguousSubject = 3,

    /// <summary>An engine was asked but decided nothing.</summary>
    Undecided = 4,
}
