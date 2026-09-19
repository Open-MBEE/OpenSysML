use std::str::FromStr;
use thiserror::Error;

/// Canonical status names used by gRPC and Connect.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Status {
    /// The operation completed successfully.
    Ok,
    /// The request was cancelled.
    Cancelled,
    /// An unknown service error occurred.
    Unknown,
    /// The caller supplied an invalid argument.
    InvalidArgument,
    /// The deadline expired before completion.
    DeadlineExceeded,
    /// The requested resource was not found.
    NotFound,
    /// The requested operation is not available.
    Unimplemented,
    /// The service is unavailable.
    Unavailable,
    /// The request is unauthenticated.
    Unauthenticated,
    /// The caller is not authorized.
    PermissionDenied,
    /// The resource already exists.
    AlreadyExists,
    /// The operation was refused because the resource is exhausted.
    ResourceExhausted,
    /// The service refused the operation because of preconditions.
    FailedPrecondition,
    /// The operation was aborted.
    Aborted,
    /// The caller attempted an operation outside its range.
    OutOfRange,
    /// The service encountered an internal error.
    Internal,
    /// The service cannot provide the requested data.
    DataLoss,
}

impl Status {
    /// Return the canonical gRPC status name.
    pub fn canonical_name(&self) -> &'static str {
        match self {
            Self::Ok => "OK",
            Self::Cancelled => "CANCELLED",
            Self::Unknown => "UNKNOWN",
            Self::InvalidArgument => "INVALID_ARGUMENT",
            Self::DeadlineExceeded => "DEADLINE_EXCEEDED",
            Self::NotFound => "NOT_FOUND",
            Self::Unimplemented => "UNIMPLEMENTED",
            Self::Unavailable => "UNAVAILABLE",
            Self::Unauthenticated => "UNAUTHENTICATED",
            Self::PermissionDenied => "PERMISSION_DENIED",
            Self::AlreadyExists => "ALREADY_EXISTS",
            Self::ResourceExhausted => "RESOURCE_EXHAUSTED",
            Self::FailedPrecondition => "FAILED_PRECONDITION",
            Self::Aborted => "ABORTED",
            Self::OutOfRange => "OUT_OF_RANGE",
            Self::Internal => "INTERNAL",
            Self::DataLoss => "DATA_LOSS",
        }
    }

    /// Parse a canonical gRPC status name.
    pub fn from_canonical_name(name: &str) -> Option<Self> {
        Some(match name {
            "OK" => Self::Ok,
            "CANCELLED" => Self::Cancelled,
            "UNKNOWN" => Self::Unknown,
            "INVALID_ARGUMENT" => Self::InvalidArgument,
            "DEADLINE_EXCEEDED" => Self::DeadlineExceeded,
            "NOT_FOUND" => Self::NotFound,
            "UNIMPLEMENTED" => Self::Unimplemented,
            "UNAVAILABLE" => Self::Unavailable,
            "UNAUTHENTICATED" => Self::Unauthenticated,
            "PERMISSION_DENIED" => Self::PermissionDenied,
            "ALREADY_EXISTS" => Self::AlreadyExists,
            "RESOURCE_EXHAUSTED" => Self::ResourceExhausted,
            "FAILED_PRECONDITION" => Self::FailedPrecondition,
            "ABORTED" => Self::Aborted,
            "OUT_OF_RANGE" => Self::OutOfRange,
            "INTERNAL" => Self::Internal,
            "DATA_LOSS" => Self::DataLoss,
            _ => return None,
        })
    }

    pub(crate) fn from_connect_code(code: &str) -> Self {
        match code.to_ascii_lowercase().as_str() {
            "ok" => Self::Ok,
            "cancelled" => Self::Cancelled,
            "invalid_argument" => Self::InvalidArgument,
            "deadline_exceeded" => Self::DeadlineExceeded,
            "not_found" => Self::NotFound,
            "unimplemented" => Self::Unimplemented,
            "unavailable" => Self::Unavailable,
            "unauthenticated" => Self::Unauthenticated,
            "permission_denied" => Self::PermissionDenied,
            "already_exists" => Self::AlreadyExists,
            "resource_exhausted" => Self::ResourceExhausted,
            "failed_precondition" => Self::FailedPrecondition,
            "aborted" => Self::Aborted,
            "out_of_range" => Self::OutOfRange,
            "internal" => Self::Internal,
            "data_loss" => Self::DataLoss,
            _ => Self::Unknown,
        }
    }
}

impl FromStr for Status {
    type Err = ();

    fn from_str(value: &str) -> Result<Self, Self::Err> {
        Self::from_canonical_name(value).ok_or(())
    }
}

/// Errors returned by the blocking OpenSysML client.
#[derive(Debug, Error)]
pub enum Error {
    /// The service refused an RPC at the transport layer.
    #[error("service returned {status:?}: {message}")]
    Service {
        /// Canonical gRPC status.
        status: Status,
        /// Message supplied by the service.
        message: String,
    },
    /// The service does not advertise an operation's required capability.
    #[error("missing capability {capability:?}: {remedy}")]
    MissingCapability {
        /// Capability that was required.
        capability: String,
        /// Suggested way to obtain the capability.
        remedy: String,
    },
    /// A private service could not be started or stopped cleanly.
    #[error("service start failed: {0}")]
    ServiceStart(String),
    /// No sysml-grpc binary could be resolved.
    #[error("sysml-grpc binary not found; looked in: {looked_in:?}")]
    BinaryNotFound {
        /// Locations searched.
        looked_in: Vec<String>,
    },
    /// No sysml-grpc release asset is published for this platform.
    #[error("no sysml-grpc release asset is published for {os}/{arch}")]
    UnsupportedPlatform {
        /// Operating system, as `std::env::consts::OS` names it.
        os: String,
        /// Architecture, as `std::env::consts::ARCH` names it.
        arch: String,
    },
    /// A release binary could not be downloaded or installed.
    #[error("sysml-grpc download failed: {0}")]
    BinaryDownload(String),
    /// A download did not match the digest expected for it.
    #[error("sysml-grpc checksum mismatch: {0}")]
    ChecksumMismatch(String),
    /// This client pins no digest for the release, so it cannot verify it.
    #[error("unpinned sysml-grpc release: {0}")]
    UnpinnedRelease(String),
    /// The HTTP transport failed before a response was decoded.
    #[error("transport error: {0}")]
    Transport(String),
    /// A successful HTTP response was not valid protobuf.
    #[error("protobuf decode error: {0}")]
    Decode(String),
    /// The service answered successfully but reported an in-band model error.
    #[error("model error: {0}")]
    Model(String),
    /// Local filesystem or process I/O failed.
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
}

#[cfg(test)]
mod tests {
    use super::Status;
    use std::str::FromStr;

    #[test]
    fn canonical_names_round_trip() {
        let statuses = [
            Status::Ok,
            Status::Cancelled,
            Status::Unknown,
            Status::InvalidArgument,
            Status::DeadlineExceeded,
            Status::NotFound,
            Status::Unimplemented,
            Status::Unavailable,
            Status::Unauthenticated,
            Status::PermissionDenied,
            Status::AlreadyExists,
            Status::ResourceExhausted,
            Status::FailedPrecondition,
            Status::Aborted,
            Status::OutOfRange,
            Status::Internal,
            Status::DataLoss,
        ];
        for status in statuses {
            assert_eq!(Status::from_str(status.canonical_name()), Ok(status));
        }
        assert_eq!(Status::from_canonical_name("not_found"), None);
    }
}
