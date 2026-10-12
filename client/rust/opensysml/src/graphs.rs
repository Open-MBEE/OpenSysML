use crate::wire;

/// The lowered graph of an action or state machine and of every behavior it
/// performs, in the canonical `graphs:<version>` JSON form an external analysis
/// engine is sent.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Graphs {
    /// The form as canonical JSON, ending in one newline.
    pub content: String,
    /// The version of the form, the `version` field of the JSON.
    pub version: i32,
    /// The qualified name of the behavior as resolved.
    pub subject: String,
}

impl Graphs {
    pub(crate) fn from_wire(response: wire::ExportGraphsResponse) -> Self {
        Self {
            content: response.content,
            version: response.version,
            subject: response.subject,
        }
    }

    /// The protocol message this was decoded from.
    pub fn wire(&self) -> wire::ExportGraphsResponse {
        wire::ExportGraphsResponse {
            content: self.content.clone(),
            version: self.version,
            subject: self.subject.clone(),
        }
    }
}
