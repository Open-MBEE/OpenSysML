//! A blocking Rust client for the OpenSysML `sysml-grpc` service.
#![deny(unsafe_code)]
#![warn(missing_docs)]

mod binary;
mod connection;
mod domain;
mod error;

/// Generated protobuf protocol types; this is the protocol layer, not the ergonomic surface.
#[allow(missing_docs)]
pub mod wire {
    include!("proto/sysml/sysml.rs");
}

pub use connection::Connection;
pub use domain::{
    Array, Capabilities, Complex, Diagnostic, EnumLiteral, EvalOptions, Evaluation, FeatureValue,
    Function, Instance, Instantiation, Language, Magnitude, MeasurementRef, Model, ParseOptions,
    Quantity, ServerInfo, Span, Symbol, UnitFactor, UnitTerm, Value, Vector, VectorQuantity,
};
pub use error::{Error, Status};

/// Parse a SysML file using a private or externally selected service.
pub fn load(path: impl AsRef<std::path::Path>) -> Result<Model, Error> {
    Connection::connect()?.parse_file(path.as_ref(), &ParseOptions::default())
}

/// Parse inline SysML content using a private or externally selected service.
pub fn loads(content: &str) -> Result<Model, Error> {
    Connection::connect()?.parse_content(content, &ParseOptions::default())
}

/// Parse a file with explicit options.
pub fn load_with(
    path: impl AsRef<std::path::Path>,
    options: &ParseOptions,
) -> Result<Model, Error> {
    Connection::connect()?.parse_file(path.as_ref(), options)
}

/// Parse inline content with explicit options.
pub fn loads_with(content: &str, options: &ParseOptions) -> Result<Model, Error> {
    Connection::connect()?.parse_content(content, options)
}
