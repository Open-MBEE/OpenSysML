//! The documents one model is parsed from.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use crate::domain::Language;
use crate::error::Error;
use crate::wire;

/// One document of a model: a file the service reads, or named inline content.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum SourceDocument {
    /// A file, named by its path; its extension says which language it is.
    File(PathBuf),
    /// Inline content, named for diagnostics.
    Inline {
        /// The name diagnostics report it under.
        name: String,
        /// The notation.
        content: String,
        /// Its language; `None` lets the service read it as SysML.
        language: Option<Language>,
    },
}

impl SourceDocument {
    /// The file at `path`.
    pub fn file(path: impl AsRef<Path>) -> Self {
        Self::File(path.as_ref().to_path_buf())
    }
    /// SysML content named `name`.
    pub fn inline(name: impl Into<String>, content: impl Into<String>) -> Self {
        Self::Inline {
            name: name.into(),
            content: content.into(),
            language: None,
        }
    }
    /// Content named `name` in `language`.
    pub fn inline_in(
        name: impl Into<String>,
        content: impl Into<String>,
        language: Language,
    ) -> Self {
        Self::Inline {
            name: name.into(),
            content: content.into(),
            language: Some(language),
        }
    }
    /// The name diagnostics report the document under: its path, or its given name.
    pub fn document_name(&self) -> String {
        match self {
            Self::File(path) => path.to_string_lossy().into_owned(),
            Self::Inline { name, .. } => name.clone(),
        }
    }
    pub(crate) fn has_language(&self) -> bool {
        matches!(
            self,
            Self::Inline {
                language: Some(_),
                ..
            }
        )
    }
    fn to_wire(&self) -> wire::SourceDocument {
        use wire::source_document::Source;
        match self {
            Self::File(path) => wire::SourceDocument {
                source: Some(Source::FilePath(path.to_string_lossy().into_owned())),
                ..Default::default()
            },
            Self::Inline {
                name,
                content,
                language,
            } => wire::SourceDocument {
                language: language
                    .map(Language::as_str)
                    .unwrap_or_default()
                    .to_owned(),
                name: name.clone(),
                source: Some(Source::Content(content.clone())),
            },
        }
    }
}

/// Check `documents` name one model unambiguously and encode them.
pub(crate) fn documents_to_wire(
    documents: &[SourceDocument],
) -> Result<Vec<wire::SourceDocument>, Error> {
    if documents.is_empty() {
        return Err(Error::InvalidRequest(
            "parse_sources needs at least one document".to_owned(),
        ));
    }
    let mut seen = BTreeSet::new();
    for document in documents {
        match document {
            SourceDocument::File(path) if path.as_os_str().is_empty() => {
                return Err(Error::InvalidRequest(
                    "a file document needs a path".to_owned(),
                ))
            }
            SourceDocument::Inline { name, .. } if name.is_empty() => {
                return Err(Error::InvalidRequest(
                    "inline content needs a name; diagnostics report it under that name".to_owned(),
                ))
            }
            _ => {}
        }
        let name = document.document_name();
        if !seen.insert(name.clone()) {
            return Err(Error::InvalidRequest(format!(
                "two documents are named {name:?}"
            )));
        }
    }
    Ok(documents.iter().map(SourceDocument::to_wire).collect())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn documents_encode_their_source() {
        let encoded = documents_to_wire(&[
            SourceDocument::file("a.sysml"),
            SourceDocument::inline_in("b", "package B;", Language::Kerml),
        ])
        .unwrap();
        assert!(matches!(
            &encoded[0].source,
            Some(wire::source_document::Source::FilePath(p)) if p == "a.sysml"
        ));
        assert_eq!(encoded[1].name, "b");
        assert_eq!(encoded[1].language, "kerml");
    }

    #[test]
    fn ambiguous_or_empty_documents_are_refused() {
        for documents in [
            vec![],
            vec![SourceDocument::inline("", "x")],
            vec![SourceDocument::file("")],
            vec![SourceDocument::inline("a", "x"), SourceDocument::file("a")],
        ] {
            assert!(matches!(
                documents_to_wire(&documents),
                Err(Error::InvalidRequest(_))
            ));
        }
    }
}
