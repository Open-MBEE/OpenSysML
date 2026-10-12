//! Writing a model out as SysML notation, RDF Turtle or the SysML v2 API's JSON element form.

use std::ffi::OsStr;
use std::fmt;
use std::fs;
use std::path::{Path, PathBuf};

use crate::domain::Diagnostic;
use crate::error::Error;
use crate::wire;

/// SysML v2 textual notation.
pub const FORMAT_SYSML: &str = "sysml";
/// RDF Turtle.
pub const FORMAT_TURTLE: &str = "ttl";
/// The SysML v2 API's JSON element form.
pub const FORMAT_API_JSON: &str = "api-json";

/// What the service says of an experimental conversion when it says nothing itself.
pub const EXPERIMENTAL_NOTICE: &str = "RDF conversion — Turtle and the API's JSON element form \
alike — is experimental: the mapping covers model structure and the behavior its bodies state, \
refuses what it cannot write back, and its vocabulary may change without a compatibility path; \
see docs/reference/rdf-mapping.md § Status";

const TURTLE_NAMES: &[&str] = &["ttl", "turtle", "rdf"];
const API_JSON_NAMES: &[&str] = &["api-json", "json"];
const XMI_NAMES: &[&str] = &["xmi", "uml", "mdzip"];

/// Whether converting between these formats is experimental.
pub fn is_experimental(from_format: &str, to_format: &str) -> bool {
    TURTLE_NAMES.contains(&from_format)
        || TURTLE_NAMES.contains(&to_format)
        || API_JSON_NAMES.contains(&from_format)
        || API_JSON_NAMES.contains(&to_format)
        || XMI_NAMES.contains(&from_format)
}

/// The format a path's extension names: `.sysml`/`.kerml`, `.ttl`/`.turtle` or `.json`.
pub fn format_of_path(path: impl AsRef<Path>) -> Result<&'static str, Error> {
    let path = path.as_ref();
    let extension = path
        .extension()
        .and_then(OsStr::to_str)
        .map(str::to_ascii_lowercase)
        .unwrap_or_default();
    match extension.as_str() {
        "sysml" | "kerml" => Ok(FORMAT_SYSML),
        "ttl" | "turtle" => Ok(FORMAT_TURTLE),
        "json" => Ok(FORMAT_API_JSON),
        _ => Err(Error::InvalidRequest(format!(
            "cannot tell the format to write {} as: expected one of .json, .kerml, .sysml, \
             .ttl, .turtle, or pass the format explicitly",
            path.display()
        ))),
    }
}

/// What to convert: a file the service reads, inline content, or a model it has loaded.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ConvertSource {
    /// A file on the service's filesystem.
    File(PathBuf),
    /// Inline content.
    Content(String),
    /// A model the service has cached, by hash.
    Model(String),
}

/// How derived element ids are spelled when notation is written as Turtle or API JSON.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum IdForm {
    /// Each derived from its qualified name.
    #[default]
    Qualified,
    /// Name-based uuids under each root package, the library convention.
    Uuid,
}

impl IdForm {
    /// The spelling the wire and `sysml -id` name it by.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Qualified => "qualified",
            Self::Uuid => "uuid",
        }
    }
}

impl fmt::Display for IdForm {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// How to convert.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct ConvertOptions {
    /// The source's format; empty lets the service tell from the path or content.
    pub from_format: String,
    /// Write notation for a source with syntax errors instead of refusing it.
    pub tolerate_syntax_errors: bool,
    /// How derived ids are spelled; only notation written as Turtle or API JSON takes one.
    pub id_form: Option<IdForm>,
    /// For a model of several documents, the documents whose elements are written, by the
    /// names the parse gave them; references into the others link their ids. Empty writes
    /// every document. Needs the `convert_documents` capability.
    pub documents: Vec<String>,
}

/// A converted model.
#[derive(Clone, Debug)]
pub struct Conversion {
    /// The converted text.
    pub content: String,
    /// The format read.
    pub from_format: String,
    /// The format written.
    pub to_format: String,
    /// Diagnostics the service reported.
    pub diagnostics: Vec<Diagnostic>,
    /// Whether the conversion is experimental; [`Conversion::experimental_notice`] says why.
    pub experimental: bool,
    /// What makes the conversion experimental; empty when it is not.
    pub experimental_notice: String,
    wire: wire::ConvertResponse,
}

impl Conversion {
    /// The Convert response this was read from.
    pub fn wire(&self) -> &wire::ConvertResponse {
        &self.wire
    }
    /// Write the content to `path` byte for byte, returning the path.
    pub fn write(&self, path: impl AsRef<Path>) -> Result<PathBuf, Error> {
        let path = path.as_ref();
        fs::write(path, self.content.as_bytes())?;
        Ok(path.to_path_buf())
    }
}

impl fmt::Display for Conversion {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.content)
    }
}

pub(crate) fn request_of(
    to_format: &str,
    source: &ConvertSource,
    options: &ConvertOptions,
) -> wire::ConvertRequest {
    use wire::convert_request::Source;
    wire::ConvertRequest {
        from_format: options.from_format.clone(),
        to_format: to_format.to_owned(),
        tolerate_syntax_errors: options.tolerate_syntax_errors,
        id_form: options
            .id_form
            .map(|form| form.as_str().to_owned())
            .unwrap_or_default(),
        documents: options.documents.clone(),
        compact: false,
        omit_derived: false,
        keep_derived: Vec::new(),
        source: Some(match source {
            ConvertSource::File(path) => Source::FilePath(path.to_string_lossy().into_owned()),
            ConvertSource::Content(content) => Source::Content(content.clone()),
            ConvertSource::Model(hash) => Source::ModelHash(hash.clone()),
        }),
    }
}

pub(crate) fn conversion_of(response: wire::ConvertResponse) -> Result<Conversion, Error> {
    let wire = response.clone();
    let diagnostics: Vec<Diagnostic> = response
        .diagnostics
        .into_iter()
        .map(Diagnostic::from)
        .collect();
    if !response.error.is_empty() {
        return Err(Error::Conversion {
            message: response.error,
            diagnostics,
        });
    }
    let experimental =
        response.experimental || is_experimental(&response.from_format, &response.to_format);
    let experimental_notice = if !response.experimental_notice.is_empty() {
        response.experimental_notice
    } else if experimental {
        EXPERIMENTAL_NOTICE.to_owned()
    } else {
        String::new()
    };
    Ok(Conversion {
        content: response.content,
        from_format: response.from_format,
        to_format: response.to_format,
        diagnostics,
        experimental,
        experimental_notice,
        wire,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extensions_name_formats() {
        assert_eq!(format_of_path("a.SysML").unwrap(), FORMAT_SYSML);
        assert_eq!(format_of_path("a.kerml").unwrap(), FORMAT_SYSML);
        assert_eq!(format_of_path("a.turtle").unwrap(), FORMAT_TURTLE);
        assert_eq!(format_of_path("dir/a.json").unwrap(), FORMAT_API_JSON);
        assert!(matches!(
            format_of_path("a.txt"),
            Err(Error::InvalidRequest(_))
        ));
        assert!(matches!(
            format_of_path("noext"),
            Err(Error::InvalidRequest(_))
        ));
    }

    #[test]
    fn rdf_and_xmi_are_experimental() {
        assert!(is_experimental("sysml", "ttl"));
        assert!(is_experimental("json", "sysml"));
        assert!(is_experimental("xmi", "sysml"));
        assert!(!is_experimental("sysml", "sysml"));
    }

    #[test]
    fn a_refusal_keeps_its_diagnostics_and_a_notice_is_filled_in() {
        let refused = conversion_of(wire::ConvertResponse {
            error: "cannot".to_owned(),
            diagnostics: vec![wire::Diagnostic::default()],
            ..Default::default()
        })
        .unwrap_err();
        assert!(
            matches!(&refused, Error::Conversion { diagnostics, .. } if diagnostics.len() == 1)
        );
        let converted = conversion_of(wire::ConvertResponse {
            content: "x".to_owned(),
            from_format: "sysml".to_owned(),
            to_format: "ttl".to_owned(),
            ..Default::default()
        })
        .unwrap();
        assert!(converted.experimental);
        assert_eq!(converted.experimental_notice, EXPERIMENTAL_NOTICE);
    }
}
