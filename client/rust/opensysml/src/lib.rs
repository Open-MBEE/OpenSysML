//! A blocking Rust client for the OpenSysML `sysml-grpc` service.
#![deny(unsafe_code)]
#![warn(missing_docs)]

mod binary;
mod capabilities;
mod connection;
mod conversion;
mod document;
mod domain;
mod edit;
mod encode;
mod error;
mod migration;
mod model;
mod operations;
mod query;
mod results;
mod sources;
mod typefacts;

/// Generated protobuf protocol types; this is the protocol layer, not the ergonomic surface.
#[allow(missing_docs)]
pub mod wire {
    include!("proto/sysml/sysml.rs");
}

pub use connection::Connection;
pub use domain::{
    Array, Capabilities, Complex, Diagnostic, EnumLiteral, EvalOptions, Evaluation, FeatureValue,
    Function, Instance, Instantiation, Language, Magnitude, MeasurementRef, Metaobject, Model,
    ModelResponse, ParseOptions, Quantity, ServerInfo, Set, Span, Symbol, TensorQuantity,
    Undetermined, UnitFactor, UnitTerm, Value, Vector, VectorQuantity,
};
pub use error::{Error, Status};
pub use wire::FailureReason;

pub use capabilities::*;
pub use conversion::{
    format_of_path, is_experimental, Conversion, ConvertOptions, ConvertSource, IdForm,
    EXPERIMENTAL_NOTICE, FORMAT_API_JSON, FORMAT_SYSML, FORMAT_TURTLE,
};
pub use document::{
    DocumentEvent, DocumentForm, DocumentQueryResult, DocumentRow, DocumentState, DocumentValue,
    DocumentVerdict, ElementRef, ObjectRef,
};
pub use edit::{
    ActionOptions, AppliedEdit, Body, CalcOptions, CommentOptions, ConnectionOptions,
    DocumentationOptions, EditError, EditFailure, EditResult, EditedDocument, Editor,
    ImportOptions, MemberOptions, MetadataOptions, Referrer, SatisfyOptions, StateActionKind,
    StatementOptions, ThenStep, TransitionOptions,
};
pub use migration::{
    is_v1, path_is_v1, Layout, MigrateOptions, MigrateSource, Migration, MigrationEntry,
    MigrationReport, MIGRATED_NOT_CONVERTED, MIGRATION_NOTICE, V1_FORMATS, VERDICT_APPROXIMATED,
    VERDICT_MAPPED, VERDICT_SKIPPED, VERDICT_UNMAPPED,
};
pub use operations::{
    AnalysisOptions, RunOptions, SourcesOptions, SweepOptions, VerifyOptions, SCHEDULE_EXPLORE,
};
pub use query::{
    CompositeOperator, Constraint, PrimitiveOperator, Query, QueryElement,
    TYPE_COMPOSITE_CONSTRAINT, TYPE_PRIMITIVE_CONSTRAINT, TYPE_QUERY,
};
pub use results::*;
pub use sources::SourceDocument;
pub use typefacts::{AttributeFacts, Multiplicity, Specialization, SymbolFacts, TypeFacts};

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
