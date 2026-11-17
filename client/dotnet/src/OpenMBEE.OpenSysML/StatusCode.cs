namespace OpenMBEE.OpenSysML;

/// <summary>
/// The canonical status of a call, as gRPC numbers it and the Connect protocol names it. A call
/// the service answered has <see cref="Ok"/> even when the answer reports a failure in band.
/// </summary>
public enum StatusCode
{
    /// <summary>The call was answered.</summary>
    Ok = 0,

    /// <summary>The call was cancelled.</summary>
    Cancelled = 1,

    /// <summary>A failure nobody classified, or a status this client does not know.</summary>
    Unknown = 2,

    /// <summary>The request was wrong.</summary>
    InvalidArgument = 3,

    /// <summary>The call outlived its deadline.</summary>
    DeadlineExceeded = 4,

    /// <summary>What the request named does not exist.</summary>
    NotFound = 5,

    /// <summary>What the request would create exists already.</summary>
    AlreadyExists = 6,

    /// <summary>The caller may not do this.</summary>
    PermissionDenied = 7,

    /// <summary>A resource, such as a message size limit, is exhausted.</summary>
    ResourceExhausted = 8,

    /// <summary>The service is not in a state where this call can succeed.</summary>
    FailedPrecondition = 9,

    /// <summary>The call was aborted.</summary>
    Aborted = 10,

    /// <summary>An argument is outside the valid range.</summary>
    OutOfRange = 11,

    /// <summary>The call is not implemented, by this build of the service or by what answered.</summary>
    Unimplemented = 12,

    /// <summary>An internal failure, or an answer that broke the protocol.</summary>
    Internal = 13,

    /// <summary>The service could not be reached, or is not serving.</summary>
    Unavailable = 14,

    /// <summary>Data was lost or corrupted.</summary>
    DataLoss = 15,

    /// <summary>The caller is not authenticated.</summary>
    Unauthenticated = 16,
}
