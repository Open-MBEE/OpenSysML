//! Source-preserving authoring: an [`Editor`] collects edits of a loaded model and applies them at once.

use std::collections::BTreeSet;
use std::fmt;
use std::fs;
use std::path::Path;

use crate::capabilities::*;
use crate::connection::Connection;
use crate::domain::{Capabilities, Diagnostic, Symbol};
use crate::error::Error;
use crate::wire;
use crate::wire::edit_operation::Operation;

/// The order capabilities are reported in when the service refuses an edit request.
const EDIT_CAPABILITY_ORDER: &[&str] = &[
    CAPABILITY_APPLY_EDITS,
    CAPABILITY_AUTHORING,
    CAPABILITY_CONNECTION_AUTHORING,
    CAPABILITY_SATISFY_AUTHORING,
    CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
    CAPABILITY_TRANSITION_AUTHORING,
    CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
    CAPABILITY_METADATA_AUTHORING,
    CAPABILITY_METADATA_PREFIX_AUTHORING,
    CAPABILITY_SEQUENCE_AUTHORING,
    CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
    CAPABILITY_IMPORT_AUTHORING,
    CAPABILITY_DOCUMENTATION_AUTHORING,
    CAPABILITY_COMMENT_AUTHORING,
    CAPABILITY_MEMBER_MODIFIERS,
    CAPABILITY_IMPLICIT_PARAMETERS,
    CAPABILITY_CONSTRAINT_BODY_AUTHORING,
    CAPABILITY_STATE_ACTION_AUTHORING,
];

/// Capabilities an edit request needs as a whole, checked once every operation is read.
const EDIT_CAPABILITIES_CHECKED_LAST: &[&str] = &[
    CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING,
    CAPABILITY_MEMBER_MODIFIERS,
    CAPABILITY_IMPLICIT_PARAMETERS,
];

const ASSERTED_CONSTRAINT_KINDS: &[&str] = &[
    "assert",
    "assert not",
    "assert constraint",
    "assert not constraint",
];
const STATE_ACTION_KINDS: &[&str] = &[
    "exhibit state",
    "exhibit",
    "entry action",
    "do action",
    "exit action",
];
const ACTION_BODY_MEMBER_KINDS: &[&str] = &[
    "accept",
    "send",
    "assign",
    "if",
    "while",
    "loop",
    "for",
    "terminate",
];
const MAX_BODY_DEPTH: usize = 128;

/// Why the service refused an edit, as the wire's `EditFailure` names it.
#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub enum EditFailure {
    /// The service gave no reason.
    Unspecified,
    /// The request had no operations.
    NoOperations,
    /// The target is not declared in the model's own source.
    UnknownTarget,
    /// The target names more than one element.
    AmbiguousTarget,
    /// The target has no value to set.
    NotValued,
    /// The new value does not parse as one expression.
    InvalidValue,
    /// The new name does not lex as an identifier.
    InvalidName,
    /// The target has no declared name to rename.
    NotNamed,
    /// A rename would break references to the renamed element.
    RenameReferenced,
    /// Two operations edit the same bytes.
    OverlappingEdits,
    /// The edited notation could not be read back.
    ResultInvalid,
    /// An added member's owner is not declared.
    OwnerUnknown,
    /// An added member's owner cannot contain members.
    OwnerNotNamespace,
    /// A declaration kind is invalid for the source language.
    IllegalKind,
    /// The owner already declares the member name.
    MemberNameTaken,
    /// A referenced declaration was deleted without cascade.
    DeleteReferenced,
    /// A move's new owner is the moved declaration or inside it.
    OwnerInsideTarget,
    /// A move would leave a reference no spelling can restore.
    MoveReferenced,
    /// The element is referred to from a document the edit cannot rewrite.
    ReferencedElsewhere,
    /// A reason this client does not know, by its wire number.
    Other(i32),
}

impl EditFailure {
    fn from_wire(code: i32) -> Self {
        use wire::EditFailure as W;
        match W::try_from(code) {
            Ok(W::Unspecified) => Self::Unspecified,
            Ok(W::NoOperations) => Self::NoOperations,
            Ok(W::UnknownTarget) => Self::UnknownTarget,
            Ok(W::AmbiguousTarget) => Self::AmbiguousTarget,
            Ok(W::NotValued) => Self::NotValued,
            Ok(W::InvalidValue) => Self::InvalidValue,
            Ok(W::InvalidName) => Self::InvalidName,
            Ok(W::NotNamed) => Self::NotNamed,
            Ok(W::RenameReferenced) => Self::RenameReferenced,
            Ok(W::OverlappingEdits) => Self::OverlappingEdits,
            Ok(W::ResultInvalid) => Self::ResultInvalid,
            Ok(W::OwnerUnknown) => Self::OwnerUnknown,
            Ok(W::OwnerNotNamespace) => Self::OwnerNotNamespace,
            Ok(W::IllegalKind) => Self::IllegalKind,
            Ok(W::MemberNameTaken) => Self::MemberNameTaken,
            Ok(W::DeleteReferenced) => Self::DeleteReferenced,
            Ok(W::OwnerInsideTarget) => Self::OwnerInsideTarget,
            Ok(W::MoveReferenced) => Self::MoveReferenced,
            Ok(W::ReferencedElsewhere) => Self::ReferencedElsewhere,
            Err(_) => Self::Other(code),
        }
    }

    /// The wire enum's number for it.
    pub fn code(&self) -> i32 {
        use wire::EditFailure as W;
        let known = match self {
            Self::Unspecified => W::Unspecified,
            Self::NoOperations => W::NoOperations,
            Self::UnknownTarget => W::UnknownTarget,
            Self::AmbiguousTarget => W::AmbiguousTarget,
            Self::NotValued => W::NotValued,
            Self::InvalidValue => W::InvalidValue,
            Self::InvalidName => W::InvalidName,
            Self::NotNamed => W::NotNamed,
            Self::RenameReferenced => W::RenameReferenced,
            Self::OverlappingEdits => W::OverlappingEdits,
            Self::ResultInvalid => W::ResultInvalid,
            Self::OwnerUnknown => W::OwnerUnknown,
            Self::OwnerNotNamespace => W::OwnerNotNamespace,
            Self::IllegalKind => W::IllegalKind,
            Self::MemberNameTaken => W::MemberNameTaken,
            Self::DeleteReferenced => W::DeleteReferenced,
            Self::OwnerInsideTarget => W::OwnerInsideTarget,
            Self::MoveReferenced => W::MoveReferenced,
            Self::ReferencedElsewhere => W::ReferencedElsewhere,
            Self::Other(code) => return *code,
        };
        known as i32
    }

    /// The wire enum's name for it, such as `EDIT_FAILURE_UNKNOWN_TARGET`.
    pub fn name(&self) -> String {
        let known = match self {
            Self::Unspecified => "UNSPECIFIED",
            Self::NoOperations => "NO_OPERATIONS",
            Self::UnknownTarget => "UNKNOWN_TARGET",
            Self::AmbiguousTarget => "AMBIGUOUS_TARGET",
            Self::NotValued => "NOT_VALUED",
            Self::InvalidValue => "INVALID_VALUE",
            Self::InvalidName => "INVALID_NAME",
            Self::NotNamed => "NOT_NAMED",
            Self::RenameReferenced => "RENAME_REFERENCED",
            Self::OverlappingEdits => "OVERLAPPING_EDITS",
            Self::ResultInvalid => "RESULT_INVALID",
            Self::OwnerUnknown => "OWNER_UNKNOWN",
            Self::OwnerNotNamespace => "OWNER_NOT_NAMESPACE",
            Self::IllegalKind => "ILLEGAL_KIND",
            Self::MemberNameTaken => "MEMBER_NAME_TAKEN",
            Self::DeleteReferenced => "DELETE_REFERENCED",
            Self::OwnerInsideTarget => "OWNER_INSIDE_TARGET",
            Self::MoveReferenced => "MOVE_REFERENCED",
            Self::ReferencedElsewhere => "REFERENCED_ELSEWHERE",
            Self::Other(code) => return format!("EDIT_FAILURE_{code}"),
        };
        format!("EDIT_FAILURE_{known}")
    }

    /// Whether the element an edit names cannot carry that edit: unknown, ambiguous, unvalued or unnamed.
    pub fn is_target_error(&self) -> bool {
        matches!(
            self,
            Self::UnknownTarget | Self::AmbiguousTarget | Self::NotValued | Self::NotNamed
        )
    }

    /// Whether the new value, name or kind itself could not be read.
    pub fn is_invalid_edit(&self) -> bool {
        matches!(
            self,
            Self::InvalidValue | Self::InvalidName | Self::IllegalKind
        )
    }
}

impl fmt::Display for EditFailure {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.name())
    }
}

/// One declaration referring to the target of a refused rename, delete or move.
#[derive(Clone, Debug, Eq, Hash, PartialEq)]
pub struct Referrer {
    /// The referring declaration, as the notation names it.
    pub name: String,
    /// The document declaring it, as the parse named it.
    pub document: String,
}

/// An edit the service refused; nothing was changed.
#[derive(Clone, Debug)]
pub struct EditError {
    /// Why, in the service's wording.
    pub message: String,
    /// The refusal kind.
    pub failure: EditFailure,
    /// The parse errors of an unreadable new value, or the errors the edited notation was found to have.
    pub diagnostics: Vec<Diagnostic>,
    /// Where the references a refused rename, delete or move would break are made.
    pub referring_elements: Vec<String>,
    /// The same referrers, each with the document declaring it.
    pub referrers: Vec<Referrer>,
}

impl fmt::Display for EditError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "edit refused ({}): {}", self.failure, self.message)
    }
}

impl std::error::Error for EditError {}

/// The span one operation replaced.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AppliedEdit {
    /// Index of the operation in the request.
    pub operation_index: i32,
    /// The element the operation named.
    pub target: String,
    /// Byte offset of the replaced span in the original source.
    pub offset: i32,
    /// Byte length of the replaced span.
    pub length: i32,
    /// The text replaced.
    pub old_text: String,
    /// The text written in its place.
    pub new_text: String,
    /// The document edited; empty for the model's only one.
    pub document: String,
}

impl fmt::Display for AppliedEdit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{}: {:?} -> {:?}",
            self.target, self.old_text, self.new_text
        )
    }
}

/// One document of a model of several, as the edit left it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EditedDocument {
    /// The document's name, as the parse named it.
    pub name: String,
    /// Its edited content.
    pub content: String,
}

/// The edited notation and what each operation changed.
#[derive(Clone, Debug)]
pub struct EditResult {
    /// The edited notation of the model's only document; empty for a model of several.
    pub content: String,
    /// What each operation replaced.
    pub applied: Vec<AppliedEdit>,
    /// Every document of a model parsed from several, as the edit left it.
    pub documents: Vec<EditedDocument>,
    wire: wire::ApplyEditsResponse,
}

impl EditResult {
    /// The ApplyEdits response this was read from.
    pub fn wire(&self) -> &wire::ApplyEditsResponse {
        &self.wire
    }
    /// Write the edited notation to `path`, exactly as the service returned it; a model of
    /// several documents is refused, its documents being in [`EditResult::documents`].
    pub fn write(&self, path: impl AsRef<Path>) -> Result<(), Error> {
        if self.content.is_empty() && self.documents.len() > 1 {
            return Err(Error::InvalidRequest(
                "the edited model has several documents: write each of `documents` by name"
                    .to_owned(),
            ));
        }
        fs::write(path, self.content.as_bytes())?;
        Ok(())
    }
}

impl fmt::Display for EditResult {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.content)
    }
}

/// Options of an added member; every one defaults to absent.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct MemberOptions {
    /// The type, as notation.
    pub type_name: Option<String>,
    /// The multiplicity, as notation (`[2]`, `[0..*]`).
    pub multiplicity: Option<String>,
    /// The value expression, as notation.
    pub value: Option<String>,
    /// Types it specializes.
    pub specializes: Vec<String>,
    /// Whether it is declared `abstract`.
    pub is_abstract: bool,
    /// Features it redefines.
    pub redefines: Vec<String>,
    /// Whether the value is a `default`.
    pub is_default: bool,
    /// Its direction: `in`, `out` or `inout`.
    pub direction: Option<String>,
    /// Metadata prefixes (`#Safety`).
    pub metadata: Vec<String>,
    /// A constraint or calculation body expression.
    pub expression: Option<String>,
    /// Documentation text written in its body.
    pub doc: Option<String>,
}

impl MemberOptions {
    /// No options.
    pub fn new() -> Self {
        Self::default()
    }
    /// With a type.
    pub fn typed(mut self, type_name: impl Into<String>) -> Self {
        self.type_name = Some(type_name.into());
        self
    }
    /// With a multiplicity.
    pub fn multiplicity(mut self, multiplicity: impl Into<String>) -> Self {
        self.multiplicity = Some(multiplicity.into());
        self
    }
    /// With a value expression.
    pub fn value(mut self, value: impl Into<String>) -> Self {
        self.value = Some(value.into());
        self
    }
    /// Specializing one more type.
    pub fn specializes(mut self, general: impl Into<String>) -> Self {
        self.specializes.push(general.into());
        self
    }
    /// Declared `abstract`.
    pub fn abstract_(mut self) -> Self {
        self.is_abstract = true;
        self
    }
    /// Redefining one more feature.
    pub fn redefines(mut self, feature: impl Into<String>) -> Self {
        self.redefines.push(feature.into());
        self
    }
    /// With its value marked `default`.
    pub fn default_value(mut self) -> Self {
        self.is_default = true;
        self
    }
    /// With a direction.
    pub fn direction(mut self, direction: impl Into<String>) -> Self {
        self.direction = Some(direction.into());
        self
    }
    /// With one more metadata prefix.
    pub fn metadata(mut self, metadata_type: impl Into<String>) -> Self {
        self.metadata.push(metadata_type.into());
        self
    }
    /// With a body expression.
    pub fn expression(mut self, expression: impl Into<String>) -> Self {
        self.expression = Some(expression.into());
        self
    }
    /// With documentation.
    pub fn doc(mut self, doc: impl Into<String>) -> Self {
        self.doc = Some(doc.into());
        self
    }
}

/// Parameters and a result of an added calculation definition or usage.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CalcOptions {
    /// Input parameters as (name, type) pairs.
    pub inputs: Vec<(String, String)>,
    /// The return parameter's type.
    pub return_type: Option<String>,
    /// The return parameter's bound expression; requires `return_type`.
    pub return_expression: Option<String>,
    /// The result expression of the body; excludes `return_expression`.
    pub expression: Option<String>,
    /// The calculation's own member options.
    pub member: MemberOptions,
}

/// Parameters of an added action definition or usage.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct ActionOptions {
    /// Input parameters as (name, type) pairs.
    pub inputs: Vec<(String, String)>,
    /// Output parameters as (name, type) pairs.
    pub outputs: Vec<(String, String)>,
    /// The action's own member options.
    pub member: MemberOptions,
}

/// Options of an added metadata usage.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct MetadataOptions {
    /// Feature bindings as (feature, value notation) pairs.
    pub values: Vec<(String, String)>,
    /// Its name.
    pub name: Option<String>,
    /// The elements it is about.
    pub about: Vec<String>,
    /// Whether to write it in `#Type` shorthand.
    pub shorthand: bool,
}

/// Options of an added documentation comment.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct DocumentationOptions {
    /// Its name.
    pub name: Option<String>,
    /// Its locale.
    pub locale: Option<String>,
    /// Whether it replaces existing documentation rather than adding to it.
    pub replace: bool,
}

/// Options of an added comment.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct CommentOptions {
    /// Its name.
    pub name: Option<String>,
    /// The elements it is about.
    pub about: Vec<String>,
    /// Its locale.
    pub locale: Option<String>,
}

/// Options of an added satisfy relationship.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct SatisfyOptions {
    /// The feature satisfying the requirement.
    pub by: Option<String>,
    /// Whether it is written `assert satisfy`.
    pub asserted: bool,
    /// Whether it is written `not satisfy`.
    pub negated: bool,
}

/// Options of an added transition.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct TransitionOptions {
    /// Its name.
    pub name: Option<String>,
    /// Its `accept` trigger.
    pub trigger: Option<String>,
    /// Its `if` guard.
    pub guard: Option<String>,
    /// Its `do` effect.
    pub effect: Option<String>,
}

/// Options of an added import.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct ImportOptions {
    /// Its visibility: `public`, `private` or `protected`.
    pub visibility: Option<String>,
    /// Whether it imports recursively (`::**`).
    pub recursive: bool,
    /// Whether it is `import all`.
    pub all: bool,
    /// Filter expressions.
    pub filters: Vec<String>,
}

/// Options of an added connection-like usage.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct ConnectionOptions {
    /// Its name.
    pub name: Option<String>,
    /// Its type.
    pub type_name: Option<String>,
}

/// How an action-body statement is joined to what precedes it.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct StatementOptions {
    /// Whether it is written as a `then` succession; `None` takes the default of the editor or body.
    pub then: Option<bool>,
    /// The succession's source multiplicity; requires `then`.
    pub multiplicity: Option<String>,
    /// The member it is inserted after; only on an [`Editor`].
    pub after: Option<String>,
}

/// What a `then` succession leads to.
#[derive(Clone, Debug, PartialEq)]
pub enum ThenStep {
    /// A reference to an action already declared.
    Ref(String),
    /// A newly declared action usage.
    Action {
        /// Its name.
        name: String,
        /// Its type.
        type_name: Option<String>,
        /// Its declaration keyword; `action` when absent.
        kind: Option<String>,
    },
}

/// The kind of a state's entry, do or exit action.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum StateActionKind {
    /// `entry action`.
    Entry,
    /// `do action`.
    Do,
    /// `exit action`.
    Exit,
}

impl StateActionKind {
    fn keyword(self) -> &'static str {
        match self {
            Self::Entry => "entry action",
            Self::Do => "do action",
            Self::Exit => "exit action",
        }
    }
}

fn sequence(owner: &str, keyword: &str, member_kind: &str) -> wire::AddSequenceEdit {
    wire::AddSequenceEdit {
        owner: owner.to_owned(),
        keyword: keyword.to_owned(),
        member_kind: member_kind.to_owned(),
        ..Default::default()
    }
}

fn opt(value: Option<&str>) -> String {
    value.unwrap_or_default().to_owned()
}

fn joined_keyword(then: bool, options: &StatementOptions) -> Result<&'static str, Error> {
    if options.multiplicity.is_some() && !then {
        return Err(Error::InvalidRequest(
            "multiplicity requires then=true".to_owned(),
        ));
    }
    Ok(if then { "then" } else { "" })
}

/// A nested action body: the statements of an `if`, `while`, `loop` or `for`.
///
/// A statement's `then` defaults to joining it to the one before; the first is unjoined.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Body {
    operations: Vec<wire::AddSequenceEdit>,
}

impl Body {
    /// An empty body.
    pub fn new() -> Self {
        Self::default()
    }

    /// The statements collected, as the wire carries them.
    pub fn operations(&self) -> &[wire::AddSequenceEdit] {
        &self.operations
    }

    fn keyword(&self, options: &StatementOptions) -> Result<&'static str, Error> {
        if options.after.is_some() {
            return Err(Error::InvalidRequest(
                "a body statement takes no after: its place is its order in the body".to_owned(),
            ));
        }
        let then = options.then.unwrap_or(!self.operations.is_empty());
        joined_keyword(then, options)
    }

    fn statement(
        &mut self,
        kind: &str,
        options: &StatementOptions,
        fill: impl FnOnce(&mut wire::AddSequenceEdit),
    ) -> Result<&mut Self, Error> {
        let keyword = self.keyword(options)?;
        let mut op = sequence("", keyword, kind);
        op.multiplicity = opt(options.multiplicity.as_deref());
        fill(&mut op);
        self.operations.push(op);
        Ok(self)
    }

    /// `first <ref>;`
    pub fn add_first(&mut self, reference: &str) -> &mut Self {
        let mut op = sequence("", "first", "");
        op.r#ref = reference.to_owned();
        self.operations.push(op);
        self
    }

    /// `then <ref>;` or `then action <name> : <type>;`
    pub fn add_then(&mut self, step: ThenStep, multiplicity: Option<&str>) -> &mut Self {
        let mut op = then_step("", step);
        op.multiplicity = opt(multiplicity);
        self.operations.push(op);
        self
    }

    /// A bare action usage in the body.
    pub fn add_action(
        &mut self,
        name: Option<&str>,
        type_name: Option<&str>,
        kind: Option<&str>,
    ) -> &mut Self {
        let mut op = sequence("", "", kind.unwrap_or("action"));
        op.member_name = opt(name);
        op.r#type = opt(type_name);
        self.operations.push(op);
        self
    }

    /// `accept <payload> : <type> via <port>;`
    pub fn add_accept(
        &mut self,
        payload: &str,
        type_name: Option<&str>,
        via: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("accept", options, |op| {
            op.parameter = payload.to_owned();
            op.r#type = opt(type_name);
            op.via = opt(via);
        })
    }

    /// `send <payload> to <target> via <port>;`
    pub fn add_send(
        &mut self,
        payload: &str,
        to: Option<&str>,
        via: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("send", options, |op| {
            op.value = payload.to_owned();
            op.target = opt(to);
            op.via = opt(via);
        })
    }

    /// `assign <target> := <value>;`
    pub fn add_assign(
        &mut self,
        target: &str,
        value: &str,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("assign", options, |op| {
            op.target = target.to_owned();
            op.value = value.to_owned();
        })
    }

    /// `if <condition> { ... } else { ... }`
    pub fn add_if(
        &mut self,
        condition: &str,
        body: &Body,
        else_body: Option<&Body>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("if", options, |op| {
            op.condition = condition.to_owned();
            op.body = body.operations.clone();
            op.else_body = else_body.map(|b| b.operations.clone()).unwrap_or_default();
        })
    }

    /// `while <condition> { ... } until <until>;`
    pub fn add_while(
        &mut self,
        condition: &str,
        body: &Body,
        until: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("while", options, |op| {
            op.condition = condition.to_owned();
            op.until = opt(until);
            op.body = body.operations.clone();
        })
    }

    /// `loop { ... } until <until>;`
    pub fn add_loop(
        &mut self,
        body: &Body,
        until: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("loop", options, |op| {
            op.until = opt(until);
            op.body = body.operations.clone();
        })
    }

    /// `for <variable> : <type> in <collection> { ... }`
    pub fn add_for(
        &mut self,
        variable: &str,
        collection: &str,
        body: &Body,
        type_name: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("for", options, |op| {
            op.parameter = variable.to_owned();
            op.value = collection.to_owned();
            op.r#type = opt(type_name);
            op.body = body.operations.clone();
        })
    }

    /// `terminate <occurrence>;`
    pub fn add_terminate(
        &mut self,
        occurrence: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement("terminate", options, |op| op.value = opt(occurrence))
    }

    /// `if <guard> then <ref>;`, one branch of a decision.
    pub fn add_guarded_then(&mut self, guard: &str, reference: &str) -> &mut Self {
        let mut op = sequence("", "if", "");
        op.condition = guard.to_owned();
        op.r#ref = reference.to_owned();
        self.operations.push(op);
        self
    }

    /// `else <ref>;`, a decision's default branch.
    pub fn add_else(&mut self, reference: &str) -> &mut Self {
        let mut op = sequence("", "else", "");
        op.r#ref = reference.to_owned();
        self.operations.push(op);
        self
    }
}

fn then_step(owner: &str, step: ThenStep) -> wire::AddSequenceEdit {
    match step {
        ThenStep::Ref(reference) => {
            let mut op = sequence(owner, "then", "");
            op.r#ref = reference;
            op
        }
        ThenStep::Action {
            name,
            type_name,
            kind,
        } => {
            let mut op = sequence(owner, "then", kind.as_deref().unwrap_or("action"));
            op.member_name = name;
            op.r#type = type_name.unwrap_or_default();
            op
        }
    }
}

/// Collects source-preserving edits of one loaded model and applies them in one request.
///
/// The service edits the source it parsed, so everything an operation does not touch, comments
/// and layout included, comes back byte for byte. [`Editor::apply`] consumes the editor: an editor
/// describes an edit of the model it was made from, so edit further from the model it returns.
#[derive(Debug)]
pub struct Editor {
    model_hash: String,
    connection: Connection,
    document: String,
    several_documents: bool,
    operations: Vec<wire::EditOperation>,
}

impl Editor {
    pub(crate) fn new(model_hash: String, connection: Connection, several_documents: bool) -> Self {
        Self {
            model_hash,
            connection,
            document: String::new(),
            several_documents,
            operations: Vec::new(),
        }
    }

    /// Target the declarations of `document`, named as the parse named it; by default the
    /// operations target the model's first document.
    pub fn in_document(&mut self, document: impl Into<String>) -> &mut Self {
        self.document = document.into();
        self
    }

    /// The document the operations target; empty is the model's first.
    pub fn document(&self) -> &str {
        &self.document
    }

    /// Collect an operation as the wire carries it, for one no builder method spells; the
    /// capabilities it needs are still required before the edit is sent.
    pub fn add_operation(&mut self, operation: wire::EditOperation) -> &mut Self {
        self.operations.push(operation);
        self
    }

    /// The operations collected, as the wire carries them.
    pub fn operations(&self) -> &[wire::EditOperation] {
        &self.operations
    }

    /// How many operations are collected.
    pub fn len(&self) -> usize {
        self.operations.len()
    }

    /// Whether no operation is collected.
    pub fn is_empty(&self) -> bool {
        self.operations.is_empty()
    }

    fn push(&mut self, operation: Operation) -> &mut Self {
        self.operations.push(wire::EditOperation {
            operation: Some(operation),
        });
        self
    }

    /// Replace the value of `target` with `value`, SysML notation as it should read in the file.
    pub fn set_value(&mut self, target: impl AsRef<str>, value: &str) -> &mut Self {
        self.push(Operation::SetValue(wire::SetValueEdit {
            target: target.as_ref().to_owned(),
            value: value.to_owned(),
        }))
    }

    /// Rename the declaration `target`; refused while anything refers to it.
    pub fn rename(&mut self, target: impl AsRef<str>, new_name: &str) -> &mut Self {
        self.push(Operation::Rename(wire::RenameEdit {
            target: target.as_ref().to_owned(),
            new_name: new_name.to_owned(),
        }))
    }

    /// Delete the declaration `target`; with `cascade`, also the references to it.
    pub fn delete(&mut self, target: impl AsRef<str>, cascade: bool) -> &mut Self {
        self.push(Operation::Delete(wire::DeleteEdit {
            target: target.as_ref().to_owned(),
            cascade,
        }))
    }

    /// Move the declaration `target` into `owner`, respelling the references to it.
    pub fn move_to(&mut self, target: impl AsRef<str>, owner: impl AsRef<str>) -> &mut Self {
        self.push(Operation::Move(wire::MoveEdit {
            target: target.as_ref().to_owned(),
            owner: owner.as_ref().to_owned(),
        }))
    }

    /// Add a member of declaration keyword `kind` (`part`, `attribute def`, …) to `owner`.
    pub fn add_member(
        &mut self,
        owner: impl AsRef<str>,
        kind: &str,
        name: &str,
        options: MemberOptions,
    ) -> &mut Self {
        self.push(Operation::AddMember(wire::AddMemberEdit {
            owner: owner.as_ref().to_owned(),
            kind: kind.to_owned(),
            name: name.to_owned(),
            r#type: options.type_name.unwrap_or_default(),
            multiplicity: options.multiplicity.unwrap_or_default(),
            value: options.value.unwrap_or_default(),
            specializes: options.specializes,
            is_abstract: options.is_abstract,
            redefines: options.redefines,
            is_default: options.is_default,
            direction: options.direction.unwrap_or_default(),
            metadata_prefixes: options.metadata,
            doc: options.doc.unwrap_or_default(),
            body_expression: options.expression.unwrap_or_default(),
        }))
    }

    /// Add a verification case's or requirement's objective; unnamed when `name` is `None`.
    pub fn add_objective(
        &mut self,
        owner: impl AsRef<str>,
        name: Option<&str>,
        type_name: Option<&str>,
    ) -> &mut Self {
        let options = MemberOptions {
            type_name: type_name.map(str::to_owned),
            ..Default::default()
        };
        self.add_member(owner, "objective", name.unwrap_or_default(), options)
    }

    /// Add `verify <requirement>;` to a verification case's objective.
    pub fn add_verify(&mut self, owner: impl AsRef<str>, requirement: &str) -> &mut Self {
        self.push(Operation::AddVerify(wire::AddVerifyEdit {
            owner: owner.as_ref().to_owned(),
            requirement: requirement.to_owned(),
        }))
    }

    /// Add a metadata usage of `metadata_type` to `owner`.
    pub fn add_metadata(
        &mut self,
        owner: impl AsRef<str>,
        metadata_type: &str,
        options: MetadataOptions,
    ) -> &mut Self {
        self.push(Operation::AddMetadata(wire::AddMetadataEdit {
            owner: owner.as_ref().to_owned(),
            metadata_type: metadata_type.to_owned(),
            name: options.name.unwrap_or_default(),
            about: options.about,
            values: options
                .values
                .into_iter()
                .map(|(feature, value)| wire::MetadataFeatureValue { feature, value })
                .collect(),
            shorthand: options.shorthand,
        }))
    }

    /// Prefix the declaration `target` with `#<metadata_type>`.
    pub fn add_metadata_prefix(
        &mut self,
        target: impl AsRef<str>,
        metadata_type: &str,
    ) -> &mut Self {
        self.push(Operation::AddMetadataPrefix(wire::AddMetadataPrefixEdit {
            target: target.as_ref().to_owned(),
            metadata_type: metadata_type.to_owned(),
        }))
    }

    /// Add a `doc` comment to `target`.
    pub fn add_documentation(
        &mut self,
        target: impl AsRef<str>,
        body: &str,
        options: DocumentationOptions,
    ) -> &mut Self {
        self.push(Operation::AddDocumentation(wire::AddDocumentationEdit {
            target: target.as_ref().to_owned(),
            body: body.to_owned(),
            name: options.name.unwrap_or_default(),
            locale: options.locale.unwrap_or_default(),
            replace: options.replace,
        }))
    }

    /// Add a `comment` to `owner`.
    pub fn add_comment(
        &mut self,
        owner: impl AsRef<str>,
        body: &str,
        options: CommentOptions,
    ) -> &mut Self {
        self.push(Operation::AddComment(wire::AddCommentEdit {
            owner: owner.as_ref().to_owned(),
            body: body.to_owned(),
            name: options.name.unwrap_or_default(),
            about: options.about,
            locale: options.locale.unwrap_or_default(),
        }))
    }

    /// Add a one-line `//*` note to `target`; refused when `text` holds a line break.
    pub fn add_note(&mut self, target: impl AsRef<str>, text: &str) -> Result<&mut Self, Error> {
        if text.contains(['\n', '\r']) {
            return Err(Error::InvalidRequest(
                "a note is one line: its text may not contain a line break".to_owned(),
            ));
        }
        Ok(self.push(Operation::AddNote(wire::AddNoteEdit {
            target: target.as_ref().to_owned(),
            text: text.to_owned(),
        })))
    }

    /// Add `satisfy <requirement> by <feature>;` to `owner`.
    pub fn add_satisfy(
        &mut self,
        owner: impl AsRef<str>,
        requirement: &str,
        options: SatisfyOptions,
    ) -> &mut Self {
        self.push(Operation::AddSatisfy(wire::AddSatisfyEdit {
            owner: owner.as_ref().to_owned(),
            requirement: requirement.to_owned(),
            satisfying_feature: options.by.unwrap_or_default(),
            is_asserted: options.asserted,
            is_negated: options.negated,
        }))
    }

    /// Add a `require` or `assume` constraint to a requirement.
    pub fn add_requirement_constraint(
        &mut self,
        owner: impl AsRef<str>,
        kind: &str,
        expression: &str,
        name: Option<&str>,
    ) -> &mut Self {
        self.push(Operation::AddRequirementConstraint(
            wire::AddRequirementConstraintEdit {
                owner: owner.as_ref().to_owned(),
                kind: kind.to_owned(),
                expression: expression.to_owned(),
                name: opt(name),
            },
        ))
    }

    /// Add `require constraint { <expression> }` to a requirement.
    pub fn add_require_constraint(
        &mut self,
        owner: impl AsRef<str>,
        expression: &str,
        name: Option<&str>,
    ) -> &mut Self {
        self.add_requirement_constraint(owner, "require", expression, name)
    }

    /// Add `assume constraint { <expression> }` to a requirement.
    pub fn add_assume_constraint(
        &mut self,
        owner: impl AsRef<str>,
        expression: &str,
        name: Option<&str>,
    ) -> &mut Self {
        self.add_requirement_constraint(owner, "assume", expression, name)
    }

    /// Add a transition from `source` to `target` to a state.
    pub fn add_transition(
        &mut self,
        owner: impl AsRef<str>,
        source: &str,
        target: &str,
        options: TransitionOptions,
    ) -> &mut Self {
        self.push(Operation::AddTransition(wire::AddTransitionEdit {
            owner: owner.as_ref().to_owned(),
            name: options.name.unwrap_or_default(),
            source: source.to_owned(),
            target: target.to_owned(),
            trigger: options.trigger.unwrap_or_default(),
            guard: options.guard.unwrap_or_default(),
            effect: options.effect.unwrap_or_default(),
            initial: false,
        }))
    }

    /// Add `entry; then <target>;`, a state's initial transition.
    pub fn add_entry_transition(&mut self, owner: impl AsRef<str>, target: &str) -> &mut Self {
        self.push(Operation::AddTransition(wire::AddTransitionEdit {
            owner: owner.as_ref().to_owned(),
            target: target.to_owned(),
            initial: true,
            ..Default::default()
        }))
    }

    fn add_sequence(&mut self, op: wire::AddSequenceEdit) -> &mut Self {
        self.push(Operation::AddSequence(op))
    }

    /// `first <ref>;` in an action's body.
    pub fn add_first(
        &mut self,
        owner: impl AsRef<str>,
        reference: &str,
        after: Option<&str>,
    ) -> &mut Self {
        let mut op = sequence(owner.as_ref(), "first", "");
        op.r#ref = reference.to_owned();
        op.after = opt(after);
        self.add_sequence(op)
    }

    /// `then <ref>;` or `then action <name> : <type>;` in an action's body.
    pub fn add_then(
        &mut self,
        owner: impl AsRef<str>,
        step: ThenStep,
        after: Option<&str>,
        multiplicity: Option<&str>,
    ) -> &mut Self {
        let mut op = then_step(owner.as_ref(), step);
        op.after = opt(after);
        op.multiplicity = opt(multiplicity);
        self.add_sequence(op)
    }

    fn statement(
        &mut self,
        owner: &str,
        kind: &str,
        options: &StatementOptions,
        fill: impl FnOnce(&mut wire::AddSequenceEdit),
    ) -> Result<&mut Self, Error> {
        let keyword = joined_keyword(options.then.unwrap_or(true), options)?;
        let mut op = sequence(owner, keyword, kind);
        op.after = opt(options.after.as_deref());
        op.multiplicity = opt(options.multiplicity.as_deref());
        fill(&mut op);
        Ok(self.add_sequence(op))
    }

    /// `accept <payload> : <type> via <port>;` in an action's body.
    pub fn add_accept(
        &mut self,
        owner: impl AsRef<str>,
        payload: &str,
        type_name: Option<&str>,
        via: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "accept", options, |op| {
            op.parameter = payload.to_owned();
            op.r#type = opt(type_name);
            op.via = opt(via);
        })
    }

    /// `send <payload> to <target> via <port>;` in an action's body.
    pub fn add_send(
        &mut self,
        owner: impl AsRef<str>,
        payload: &str,
        to: Option<&str>,
        via: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "send", options, |op| {
            op.value = payload.to_owned();
            op.target = opt(to);
            op.via = opt(via);
        })
    }

    /// `assign <target> := <value>;` in an action's body.
    pub fn add_assign(
        &mut self,
        owner: impl AsRef<str>,
        target: &str,
        value: &str,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "assign", options, |op| {
            op.target = target.to_owned();
            op.value = value.to_owned();
        })
    }

    /// `if <condition> { ... } else { ... }` in an action's body.
    pub fn add_if(
        &mut self,
        owner: impl AsRef<str>,
        condition: &str,
        body: &Body,
        else_body: Option<&Body>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "if", options, |op| {
            op.condition = condition.to_owned();
            op.body = body.operations.clone();
            op.else_body = else_body.map(|b| b.operations.clone()).unwrap_or_default();
        })
    }

    /// `while <condition> { ... } until <until>;` in an action's body.
    pub fn add_while(
        &mut self,
        owner: impl AsRef<str>,
        condition: &str,
        body: &Body,
        until: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "while", options, |op| {
            op.condition = condition.to_owned();
            op.until = opt(until);
            op.body = body.operations.clone();
        })
    }

    /// `loop { ... } until <until>;` in an action's body.
    pub fn add_loop(
        &mut self,
        owner: impl AsRef<str>,
        body: &Body,
        until: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "loop", options, |op| {
            op.until = opt(until);
            op.body = body.operations.clone();
        })
    }

    /// `for <variable> : <type> in <collection> { ... }` in an action's body.
    pub fn add_for(
        &mut self,
        owner: impl AsRef<str>,
        variable: &str,
        collection: &str,
        body: &Body,
        type_name: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "for", options, |op| {
            op.parameter = variable.to_owned();
            op.value = collection.to_owned();
            op.r#type = opt(type_name);
            op.body = body.operations.clone();
        })
    }

    /// `terminate <occurrence>;` in an action's body.
    pub fn add_terminate(
        &mut self,
        owner: impl AsRef<str>,
        occurrence: Option<&str>,
        options: &StatementOptions,
    ) -> Result<&mut Self, Error> {
        self.statement(owner.as_ref(), "terminate", options, |op| {
            op.value = opt(occurrence)
        })
    }

    /// `if <guard> then <ref>;`, one branch of a decision.
    pub fn add_guarded_then(
        &mut self,
        owner: impl AsRef<str>,
        guard: &str,
        reference: &str,
        after: Option<&str>,
    ) -> &mut Self {
        let mut op = sequence(owner.as_ref(), "if", "");
        op.condition = guard.to_owned();
        op.r#ref = reference.to_owned();
        op.after = opt(after);
        self.add_sequence(op)
    }

    /// `else <ref>;`, a decision's default branch.
    pub fn add_else(
        &mut self,
        owner: impl AsRef<str>,
        reference: &str,
        after: Option<&str>,
    ) -> &mut Self {
        let mut op = sequence(owner.as_ref(), "else", "");
        op.r#ref = reference.to_owned();
        op.after = opt(after);
        self.add_sequence(op)
    }

    /// Add an import of `target` to `owner`.
    pub fn add_import(
        &mut self,
        owner: impl AsRef<str>,
        target: &str,
        options: ImportOptions,
    ) -> &mut Self {
        self.push(Operation::AddImport(wire::AddImportEdit {
            owner: owner.as_ref().to_owned(),
            visibility: options.visibility.unwrap_or_default(),
            target: target.to_owned(),
            is_recursive: options.recursive,
            is_import_all: options.all,
            filters: options.filters,
        }))
    }

    /// Add a connection-like usage of keyword `kind` (`connection`, `flow`, …) between two ends.
    pub fn add_connection(
        &mut self,
        owner: impl AsRef<str>,
        kind: &str,
        from: &str,
        to: &str,
        options: ConnectionOptions,
    ) -> &mut Self {
        self.push(Operation::AddConnection(wire::AddConnectionEdit {
            owner: owner.as_ref().to_owned(),
            kind: kind.to_owned(),
            from_end: from.to_owned(),
            to_end: to.to_owned(),
            name: options.name.unwrap_or_default(),
            r#type: options.type_name.unwrap_or_default(),
        }))
    }

    /// Add an `allocation` between two ends.
    pub fn add_allocation(
        &mut self,
        owner: impl AsRef<str>,
        from: &str,
        to: &str,
        options: ConnectionOptions,
    ) -> &mut Self {
        self.add_connection(owner, "allocation", from, to, options)
    }

    /// Add a `flow` between two ends.
    pub fn add_flow(
        &mut self,
        owner: impl AsRef<str>,
        from: &str,
        to: &str,
        options: ConnectionOptions,
    ) -> &mut Self {
        self.add_connection(owner, "flow", from, to, options)
    }

    /// Add a `succession` between two ends.
    pub fn add_succession(
        &mut self,
        owner: impl AsRef<str>,
        from: &str,
        to: &str,
        options: ConnectionOptions,
    ) -> &mut Self {
        self.add_connection(owner, "succession", from, to, options)
    }

    fn with_parameters(
        &mut self,
        owner: &str,
        kind: &str,
        name: &str,
        member: MemberOptions,
        parameters: &[(&str, &[(String, String)])],
    ) -> &mut Self {
        self.add_member(owner, kind, name, member);
        let qualified = if owner.is_empty() {
            name.to_owned()
        } else {
            format!("{owner}::{name}")
        };
        for (direction, pairs) in parameters {
            for (parameter, type_name) in *pairs {
                self.add_parameter(
                    &qualified,
                    direction,
                    parameter,
                    None,
                    MemberOptions::new().typed(type_name),
                );
            }
        }
        self
    }

    fn calc(
        &mut self,
        owner: &str,
        kind: &str,
        name: &str,
        options: CalcOptions,
    ) -> Result<&mut Self, Error> {
        if options.expression.is_some() && options.return_expression.is_some() {
            return Err(Error::InvalidRequest(
                "expression and return_expression both bind the result; give one".to_owned(),
            ));
        }
        if options.return_expression.is_some()
            && options.return_type.as_deref().unwrap_or("").is_empty()
        {
            return Err(Error::InvalidRequest(
                "return_expression requires return_type".to_owned(),
            ));
        }
        let mut member = options.member;
        if options.expression.is_some() {
            member.expression = options.expression;
        }
        self.with_parameters(owner, kind, name, member, &[("in", &options.inputs)]);
        if options.return_type.is_some() || options.return_expression.is_some() {
            let qualified = if owner.is_empty() {
                name.to_owned()
            } else {
                format!("{owner}::{name}")
            };
            let mut result = MemberOptions::new();
            result.type_name = options.return_type;
            result.value = options.return_expression;
            self.add_return(qualified, "", result);
        }
        Ok(self)
    }

    /// Add a `calc def` with its input parameters and return.
    pub fn add_calc_def(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        options: CalcOptions,
    ) -> Result<&mut Self, Error> {
        self.calc(owner.as_ref(), "calc def", name, options)
    }

    /// Add a `calc` usage with its input parameters and return.
    pub fn add_calc(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        options: CalcOptions,
    ) -> Result<&mut Self, Error> {
        self.calc(owner.as_ref(), "calc", name, options)
    }

    /// Add a directed parameter; `kind` is its declaration keyword, absent for an implicit one.
    pub fn add_parameter(
        &mut self,
        owner: impl AsRef<str>,
        direction: &str,
        name: &str,
        kind: Option<&str>,
        mut options: MemberOptions,
    ) -> &mut Self {
        options.direction = Some(direction.to_owned());
        self.add_member(owner, kind.unwrap_or_default(), name, options)
    }

    /// Add a `return` parameter; `name` may be empty.
    pub fn add_return(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        options: MemberOptions,
    ) -> &mut Self {
        self.add_member(owner, "return", name, options)
    }

    /// Add an `action def` with its parameters.
    pub fn add_action_def(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        options: ActionOptions,
    ) -> &mut Self {
        self.with_parameters(
            owner.as_ref(),
            "action def",
            name,
            options.member,
            &[("in", &options.inputs), ("out", &options.outputs)],
        )
    }

    /// Add an `action` usage with its parameters.
    pub fn add_action(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        options: ActionOptions,
    ) -> &mut Self {
        self.with_parameters(
            owner.as_ref(),
            "action",
            name,
            options.member,
            &[("in", &options.inputs), ("out", &options.outputs)],
        )
    }

    /// Add `perform action <name> : <type>;`.
    pub fn add_perform_action(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        options: MemberOptions,
    ) -> &mut Self {
        self.add_member(owner, "perform action", name, options)
    }

    /// Add `perform <action>;`.
    pub fn add_perform(
        &mut self,
        owner: impl AsRef<str>,
        action: &str,
        doc: Option<&str>,
    ) -> &mut Self {
        let options = MemberOptions {
            doc: doc.map(str::to_owned),
            ..Default::default()
        };
        self.add_member(owner, "perform", action, options)
    }

    /// Add `exhibit state <name> : <type>;`.
    pub fn add_exhibit_state(
        &mut self,
        owner: impl AsRef<str>,
        name: &str,
        type_name: Option<&str>,
    ) -> &mut Self {
        let options = MemberOptions {
            type_name: type_name.map(str::to_owned),
            ..Default::default()
        };
        self.add_member(owner, "exhibit state", name, options)
    }

    /// Add `exhibit <state>;`.
    pub fn add_exhibit(&mut self, owner: impl AsRef<str>, state: &str) -> &mut Self {
        self.add_member(owner, "exhibit", state, MemberOptions::new())
    }

    /// Add a state's `entry`, `do` or `exit` action.
    pub fn add_state_action(
        &mut self,
        owner: impl AsRef<str>,
        kind: StateActionKind,
        name: &str,
        type_name: Option<&str>,
    ) -> &mut Self {
        let options = MemberOptions {
            type_name: type_name.map(str::to_owned),
            ..Default::default()
        };
        self.add_member(owner, kind.keyword(), name, options)
    }

    /// Add `assert constraint` (or `assert not constraint`), unnamed when `name` is `None`.
    pub fn add_assert_constraint(
        &mut self,
        owner: impl AsRef<str>,
        name: Option<&str>,
        type_name: Option<&str>,
        expression: Option<&str>,
        negated: bool,
    ) -> &mut Self {
        let options = MemberOptions {
            type_name: type_name.map(str::to_owned),
            expression: expression.map(str::to_owned),
            ..Default::default()
        };
        let kind = if negated {
            "assert not constraint"
        } else {
            "assert constraint"
        };
        self.add_member(owner, kind, name.unwrap_or_default(), options)
    }

    /// Add `assert <ref>;` (or `assert not <ref>;`).
    pub fn add_assert(
        &mut self,
        owner: impl AsRef<str>,
        reference: &str,
        negated: bool,
    ) -> &mut Self {
        let kind = if negated { "assert not" } else { "assert" };
        self.add_member(owner, kind, reference, MemberOptions::new())
    }

    /// Apply every collected operation in one request, consuming the editor.
    ///
    /// Each capability an operation needs is required of the service before anything is sent.
    pub fn apply(self) -> Result<EditResult, Error> {
        if self.operations.is_empty() {
            return Err(Error::Edit(Box::new(EditError {
                message: "this editor has no operations: add an edit before applying it".to_owned(),
                failure: EditFailure::NoOperations,
                diagnostics: Vec::new(),
                referring_elements: Vec::new(),
                referrers: Vec::new(),
            })));
        }
        if self.several_documents {
            self.connection.capabilities().require(
                CAPABILITY_EDIT_DOCUMENTS,
                upgrade_remedy(CAPABILITY_EDIT_DOCUMENTS),
            )?;
        }
        self.connection
            .apply_edits(&self.model_hash, &self.document, self.operations)
    }
}

macro_rules! member_kinds {
    ($($(#[$doc:meta])* $method:ident => $kind:literal;)*) => {
        impl Editor {
            $(
                $(#[$doc])*
                pub fn $method(&mut self, owner: impl AsRef<str>, name: &str, options: MemberOptions) -> &mut Self {
                    self.add_member(owner, $kind, name, options)
                }
            )*
        }
    };
}

member_kinds! {
    /// Add a `package`.
    add_package => "package";
    /// Add a `part def`.
    add_part_def => "part def";
    /// Add a `part`.
    add_part => "part";
    /// Add an `attribute def`.
    add_attribute_def => "attribute def";
    /// Add an `attribute`.
    add_attribute => "attribute";
    /// Add an `item def`.
    add_item_def => "item def";
    /// Add an `item`.
    add_item => "item";
    /// Add a `port def`.
    add_port_def => "port def";
    /// Add a `port`.
    add_port => "port";
    /// Add a KerML `class`.
    add_class => "class";
    /// Add a KerML `struct`.
    add_struct => "struct";
    /// Add a KerML `datatype`.
    add_datatype => "datatype";
    /// Add a KerML `classifier`.
    add_classifier => "classifier";
    /// Add a KerML `feature`.
    add_feature => "feature";
    /// Add a KerML `assoc`.
    add_assoc => "assoc";
    /// Add a KerML `behavior`.
    add_behavior => "behavior";
    /// Add a KerML `function`.
    add_function => "function";
    /// Add a KerML `predicate`.
    add_predicate => "predicate";
    /// Add a KerML `interaction`.
    add_interaction => "interaction";
    /// Add a KerML `metaclass`.
    add_metaclass => "metaclass";
    /// Add a `state def`.
    add_state_def => "state def";
    /// Add a `state`.
    add_state => "state";
    /// Add a `constraint def`; its body is `options.expression`.
    add_constraint_def => "constraint def";
    /// Add a `constraint`; its body is `options.expression`.
    add_constraint => "constraint";
    /// Add a `requirement def`.
    add_requirement_def => "requirement def";
    /// Add a `requirement`.
    add_requirement => "requirement";
}

impl AsRef<str> for Symbol {
    fn as_ref(&self) -> &str {
        self.id()
    }
}

/// Reads an edit request the way the service will, requiring each capability as it is met.
pub(crate) struct EditCapabilities<'a> {
    capabilities: &'a Capabilities,
    requested: BTreeSet<&'static str>,
}

impl<'a> EditCapabilities<'a> {
    pub(crate) fn new(capabilities: &'a Capabilities) -> Result<Self, Error> {
        let mut this = Self {
            capabilities,
            requested: BTreeSet::new(),
        };
        this.require(&[CAPABILITY_APPLY_EDITS])?;
        Ok(this)
    }

    fn require(&mut self, capabilities: &[&'static str]) -> Result<(), Error> {
        for capability in capabilities {
            self.capabilities
                .require(capability, upgrade_remedy(capability))?;
            self.requested.insert(capability);
        }
        Ok(())
    }

    fn note(&mut self, capability: &'static str, needed: bool) {
        if needed {
            self.requested.insert(capability);
        }
    }

    pub(crate) fn read(&mut self, operation: &wire::EditOperation) -> Result<(), Error> {
        let Some(operation) = &operation.operation else {
            return Err(Error::InvalidRequest(
                "edit operation names no edit".to_owned(),
            ));
        };
        match operation {
            Operation::SetValue(_) | Operation::Rename(_) => Ok(()),
            Operation::Delete(_) | Operation::Move(_) => self.require(&[CAPABILITY_AUTHORING]),
            Operation::AddMember(add) => self.member(add),
            Operation::AddConnection(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_CONNECTION_AUTHORING])
            }
            Operation::AddSatisfy(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_SATISFY_AUTHORING])
            }
            Operation::AddRequirementConstraint(_) => self.require(&[
                CAPABILITY_AUTHORING,
                CAPABILITY_REQUIREMENT_CONSTRAINT_AUTHORING,
            ]),
            Operation::AddTransition(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_TRANSITION_AUTHORING])
            }
            Operation::AddVerify(_) => self.require(&[
                CAPABILITY_AUTHORING,
                CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING,
            ]),
            Operation::AddMetadata(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_METADATA_AUTHORING])
            }
            Operation::AddMetadataPrefix(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_METADATA_PREFIX_AUTHORING])
            }
            Operation::AddSequence(add) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_SEQUENCE_AUTHORING])?;
                let extended = extended_sequence(add, 0)?;
                self.note(CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING, extended);
                Ok(())
            }
            Operation::AddImport(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_IMPORT_AUTHORING])
            }
            Operation::AddDocumentation(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_DOCUMENTATION_AUTHORING])
            }
            Operation::AddComment(_) | Operation::AddNote(_) => {
                self.require(&[CAPABILITY_AUTHORING, CAPABILITY_COMMENT_AUTHORING])
            }
        }
    }

    fn member(&mut self, add: &wire::AddMemberEdit) -> Result<(), Error> {
        self.require(&[CAPABILITY_AUTHORING])?;
        self.note(CAPABILITY_IMPLICIT_PARAMETERS, add.kind.is_empty());
        if add.kind == "objective" && add.name.is_empty() {
            self.require(&[CAPABILITY_VERIFICATION_OBJECTIVE_AUTHORING])?;
        }
        if !add.doc.is_empty() {
            self.require(&[CAPABILITY_DOCUMENTATION_AUTHORING])?;
        }
        self.note(
            CAPABILITY_MEMBER_MODIFIERS,
            add.is_abstract
                || !add.redefines.is_empty()
                || add.is_default
                || !add.direction.is_empty()
                || add.kind == "ref"
                || add.kind == "return",
        );
        if !add.metadata_prefixes.is_empty() {
            self.require(&[CAPABILITY_METADATA_AUTHORING])?;
        }
        if !add.body_expression.is_empty() || ASSERTED_CONSTRAINT_KINDS.contains(&add.kind.as_str())
        {
            self.require(&[CAPABILITY_CONSTRAINT_BODY_AUTHORING])?;
        }
        if STATE_ACTION_KINDS.contains(&add.kind.as_str()) {
            self.require(&[CAPABILITY_STATE_ACTION_AUTHORING])?;
        }
        Ok(())
    }

    /// Require the capabilities judged over the whole request; answer every one requested, in report order.
    pub(crate) fn finish(mut self) -> Result<Vec<&'static str>, Error> {
        for capability in EDIT_CAPABILITIES_CHECKED_LAST {
            if self.requested.contains(capability) {
                self.require(&[capability])?;
            }
        }
        Ok(EDIT_CAPABILITY_ORDER
            .iter()
            .copied()
            .filter(|c| self.requested.contains(c))
            .collect())
    }
}

fn extended_sequence(add: &wire::AddSequenceEdit, depth: usize) -> Result<bool, Error> {
    if depth > MAX_BODY_DEPTH {
        return Err(Error::InvalidRequest(
            "nested action-body items exceed the maximum depth".to_owned(),
        ));
    }
    let mut extended = add.keyword == "if"
        || add.keyword == "else"
        || (add.keyword.is_empty() && !add.member_kind.is_empty())
        || ACTION_BODY_MEMBER_KINDS.contains(&add.member_kind.as_str())
        || [
            &add.condition,
            &add.value,
            &add.target,
            &add.via,
            &add.until,
            &add.multiplicity,
            &add.parameter,
        ]
        .iter()
        .any(|field| !field.is_empty())
        || !add.body.is_empty()
        || !add.else_body.is_empty();
    for child in add.body.iter().chain(&add.else_body) {
        extended = extended_sequence(child, depth + 1)? || extended;
    }
    Ok(extended)
}

pub(crate) fn edit_result_of(response: wire::ApplyEditsResponse) -> Result<EditResult, Error> {
    let wire = response.clone();
    if !response.error.is_empty() {
        return Err(Error::Edit(Box::new(EditError {
            message: response.error,
            failure: EditFailure::from_wire(response.failure),
            diagnostics: response
                .diagnostics
                .into_iter()
                .map(Diagnostic::from)
                .collect(),
            referring_elements: response.referring_elements,
            referrers: response
                .referrers
                .into_iter()
                .map(|r| Referrer {
                    name: r.name,
                    document: r.document,
                })
                .collect(),
        })));
    }
    Ok(EditResult {
        content: response.content,
        applied: response
            .applied
            .into_iter()
            .map(|a| AppliedEdit {
                operation_index: a.operation_index,
                target: a.target,
                offset: a.offset,
                length: a.length,
                old_text: a.old_text,
                new_text: a.new_text,
                document: a.document,
            })
            .collect(),
        documents: response
            .documents
            .into_iter()
            .map(|d| EditedDocument {
                name: d.name,
                content: d.content,
            })
            .collect(),
        wire,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn capabilities(names: &[&str]) -> Capabilities {
        Capabilities::new(wire::ServerInfoResponse {
            capabilities: names.iter().map(|name| (*name).to_owned()).collect(),
            ..Default::default()
        })
    }

    fn operation(operation: Operation) -> wire::EditOperation {
        wire::EditOperation {
            operation: Some(operation),
        }
    }

    fn missing(names: &[&str], operations: &[wire::EditOperation]) -> String {
        let capabilities = capabilities(names);
        let result = EditCapabilities::new(&capabilities).and_then(|mut reader| {
            for op in operations {
                reader.read(op)?;
            }
            reader.finish()
        });
        match result {
            Err(Error::MissingCapability { capability, remedy }) => {
                assert!(!remedy.is_empty());
                capability
            }
            other => panic!("expected a missing capability, got {other:?}"),
        }
    }

    #[test]
    fn apply_edits_is_required_before_anything_else() {
        assert_eq!(
            missing(&[], &[operation(Operation::Delete(Default::default()))]),
            CAPABILITY_APPLY_EDITS
        );
    }

    #[test]
    fn each_operation_requires_its_authoring_capability() {
        let base = [CAPABILITY_APPLY_EDITS, CAPABILITY_AUTHORING];
        let cases = [
            (
                Operation::AddConnection(Default::default()),
                CAPABILITY_CONNECTION_AUTHORING,
            ),
            (
                Operation::AddSatisfy(Default::default()),
                CAPABILITY_SATISFY_AUTHORING,
            ),
            (
                Operation::AddTransition(Default::default()),
                CAPABILITY_TRANSITION_AUTHORING,
            ),
            (
                Operation::AddImport(Default::default()),
                CAPABILITY_IMPORT_AUTHORING,
            ),
            (
                Operation::AddComment(Default::default()),
                CAPABILITY_COMMENT_AUTHORING,
            ),
            (
                Operation::AddMetadataPrefix(Default::default()),
                CAPABILITY_METADATA_PREFIX_AUTHORING,
            ),
        ];
        for (op, expected) in cases {
            assert_eq!(missing(&base, &[operation(op)]), expected);
        }
    }

    #[test]
    fn whole_request_capabilities_are_judged_after_every_operation() {
        let caps = [
            CAPABILITY_APPLY_EDITS,
            CAPABILITY_AUTHORING,
            CAPABILITY_IMPORT_AUTHORING,
        ];
        let modified = operation(Operation::AddMember(wire::AddMemberEdit {
            kind: "part".to_owned(),
            name: "p".to_owned(),
            is_abstract: true,
            ..Default::default()
        }));
        let import = operation(Operation::AddImport(Default::default()));
        assert_eq!(
            missing(&caps, &[modified, import]),
            CAPABILITY_MEMBER_MODIFIERS
        );
    }

    #[test]
    fn a_granted_request_reports_its_capabilities_in_order() {
        let caps = capabilities(EDIT_CAPABILITY_ORDER);
        let mut reader = EditCapabilities::new(&caps).unwrap();
        reader
            .read(&operation(Operation::AddImport(Default::default())))
            .unwrap();
        reader
            .read(&operation(Operation::AddConnection(Default::default())))
            .unwrap();
        assert_eq!(
            reader.finish().unwrap(),
            vec![
                CAPABILITY_APPLY_EDITS,
                CAPABILITY_AUTHORING,
                CAPABILITY_CONNECTION_AUTHORING,
                CAPABILITY_IMPORT_AUTHORING,
            ]
        );
    }

    #[test]
    fn an_operation_naming_no_edit_is_invalid() {
        let caps = capabilities(&[CAPABILITY_APPLY_EDITS]);
        let mut reader = EditCapabilities::new(&caps).unwrap();
        assert!(matches!(
            reader.read(&wire::EditOperation { operation: None }),
            Err(Error::InvalidRequest(_))
        ));
    }

    #[test]
    fn a_nested_statement_needs_action_body_authoring() {
        let mut inner = Body::new();
        inner
            .add_assign("x", "1", &StatementOptions::default())
            .unwrap();
        let mut outer = Body::new();
        outer
            .add_loop(&inner, None, &StatementOptions::default())
            .unwrap();
        let mut nested = outer.operations()[0].clone();
        nested.keyword = String::new();
        nested.member_kind = String::new();
        nested.until = String::new();
        let op = operation(Operation::AddSequence(nested));
        assert_eq!(
            missing(
                &[
                    CAPABILITY_APPLY_EDITS,
                    CAPABILITY_AUTHORING,
                    CAPABILITY_SEQUENCE_AUTHORING
                ],
                &[op]
            ),
            CAPABILITY_ACTION_BODY_STATEMENT_AUTHORING
        );
    }

    #[test]
    fn a_body_nested_too_deep_is_refused() {
        let mut body = Body::new();
        body.add_first("a");
        for _ in 0..=MAX_BODY_DEPTH + 1 {
            let mut outer = Body::new();
            outer
                .add_loop(&body, None, &StatementOptions::default())
                .unwrap();
            body = outer;
        }
        assert!(matches!(
            extended_sequence(&body.operations()[0], 0),
            Err(Error::InvalidRequest(_))
        ));
    }

    #[test]
    fn a_body_statement_takes_no_after_and_joins_by_default() {
        let mut body = Body::new();
        let after = StatementOptions {
            after: Some("a".to_owned()),
            ..Default::default()
        };
        assert!(matches!(
            body.add_terminate(None, &after),
            Err(Error::InvalidRequest(_))
        ));
        body.add_terminate(None, &StatementOptions::default())
            .unwrap();
        body.add_terminate(None, &StatementOptions::default())
            .unwrap();
        let keywords: Vec<_> = body
            .operations()
            .iter()
            .map(|op| op.keyword.as_str())
            .collect();
        assert_eq!(keywords, vec!["", "then"]);
    }

    #[test]
    fn a_refused_edit_carries_its_typed_failure_and_referrers() {
        let error = edit_result_of(wire::ApplyEditsResponse {
            error: "P::a is referred to".to_owned(),
            failure: wire::EditFailure::ReferencedElsewhere as i32,
            referring_elements: vec!["Q::b".to_owned()],
            referrers: vec![wire::Referrer {
                name: "Q::b".to_owned(),
                document: "q.sysml".to_owned(),
            }],
            ..Default::default()
        })
        .unwrap_err();
        let Error::Edit(error) = error else {
            panic!("not an edit error: {error:?}");
        };
        assert_eq!(error.failure, EditFailure::ReferencedElsewhere);
        assert!(!error.failure.is_target_error() && !error.failure.is_invalid_edit());
        assert_eq!(error.referrers[0].document, "q.sysml");
        assert_eq!(error.referring_elements, vec!["Q::b".to_owned()]);
    }

    #[test]
    fn an_unknown_failure_keeps_its_wire_number() {
        let failure = EditFailure::from_wire(9999);
        assert_eq!(failure, EditFailure::Other(9999));
        assert_eq!(failure.code(), 9999);
    }

    #[test]
    fn an_edit_result_keeps_every_document_and_applied_span() {
        let result = edit_result_of(wire::ApplyEditsResponse {
            applied: vec![wire::AppliedEdit {
                operation_index: 0,
                target: "P::a".to_owned(),
                offset: 3,
                length: 1,
                old_text: "a".to_owned(),
                new_text: "b".to_owned(),
                document: "p.sysml".to_owned(),
            }],
            documents: vec![
                wire::EditedDocument {
                    name: "p.sysml".to_owned(),
                    content: "package P { part b; }".to_owned(),
                },
                wire::EditedDocument {
                    name: "q.sysml".to_owned(),
                    content: "package Q;".to_owned(),
                },
            ],
            ..Default::default()
        })
        .unwrap();
        assert_eq!(result.applied[0].document, "p.sysml");
        assert_eq!(result.documents.len(), 2);
        assert_eq!(result.wire().documents.len(), 2);
    }

    #[test]
    fn a_result_of_several_documents_is_not_written_as_one() {
        let result = edit_result_of(wire::ApplyEditsResponse {
            documents: vec![
                wire::EditedDocument {
                    name: "p.sysml".to_owned(),
                    content: "package P;".to_owned(),
                },
                wire::EditedDocument {
                    name: "q.sysml".to_owned(),
                    content: "package Q;".to_owned(),
                },
            ],
            ..Default::default()
        })
        .unwrap();
        let path = std::env::temp_dir().join("opensysml-several-documents.sysml");
        fs::write(&path, "package Kept;").unwrap();
        assert!(matches!(result.write(&path), Err(Error::InvalidRequest(_))));
        assert_eq!(fs::read_to_string(&path).unwrap(), "package Kept;");
        fs::remove_file(path).unwrap();
    }
}
