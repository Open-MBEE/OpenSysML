//! Document queries and their typed rows, and the forms a document renders to.

use std::fmt;

use crate::domain::{quantity_from_wire, Quantity};
use crate::encode::quantity_to_wire;
use crate::error::Error;
use crate::wire;

/// A model element a document query names: its qualified name and the standard's type.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct ElementRef {
    /// Qualified name; empty for an element the query could not name.
    pub id: String,
    /// The standard's element type; empty when the service gave none.
    pub element_type: String,
}

impl ElementRef {
    /// A reference to the element named `id`.
    pub fn new(id: impl Into<String>) -> Self {
        Self {
            id: id.into(),
            element_type: String::new(),
        }
    }
}

impl fmt::Display for ElementRef {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.element_type.is_empty() {
            f.write_str(&self.id)
        } else {
            write!(f, "{} ({})", self.id, self.element_type)
        }
    }
}

/// An object of an instantiated subject, by runtime id or by path.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct ObjectRef {
    /// Runtime identity; 0 when bound by path.
    pub id: i64,
    /// Feature path from the subject, such as `vehicle.engine`.
    pub path: String,
    /// The element the object instantiates, when the service names it.
    pub element: Option<ElementRef>,
}

impl ObjectRef {
    /// The object at `path` from the subject.
    pub fn at(path: impl Into<String>) -> Self {
        Self {
            path: path.into(),
            ..Self::default()
        }
    }
}

impl fmt::Display for ObjectRef {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.path.is_empty() {
            write!(f, "#{}", self.id)
        } else {
            f.write_str(&self.path)
        }
    }
}

/// A verdict row: one assertion and how it stands on one object.
#[derive(Clone, Debug, PartialEq)]
pub struct DocumentVerdict {
    /// The assertion judged.
    pub assertion: ElementRef,
    /// What kind of assertion it is, such as `requirement`.
    pub kind: String,
    /// Its text.
    pub text: String,
    /// The object it was judged on.
    pub path: String,
    /// `pass`, `fail`, `inconclusive` or `error`.
    pub status: String,
    /// The condition evaluated.
    pub condition: String,
    /// Why it stands so.
    pub reason: String,
    /// The verification cases that verify it.
    pub verification: Vec<String>,
}

impl fmt::Display for DocumentVerdict {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.text)?;
        if !self.path.is_empty() {
            write!(f, " on {}", self.path)?;
        }
        write!(f, ": {}", self.status)
    }
}

/// A state row: the state one machine of one object rests in.
#[derive(Clone, Debug, PartialEq)]
pub struct DocumentState {
    /// The object whose machine it is.
    pub object: ObjectRef,
    /// The machine.
    pub machine: String,
    /// The state's short name.
    pub name: String,
    /// The state's path within the machine.
    pub path: String,
    /// The state's element, when the service names it.
    pub state: Option<ElementRef>,
    /// The region it is in; empty for a machine without regions.
    pub region: String,
    /// The states enclosing it, outermost first.
    pub enclosing: Vec<String>,
}

impl fmt::Display for DocumentState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}.{} in {}", self.object, self.machine, self.path)
    }
}

/// An event row: one thing a run did, at one simulated time.
#[derive(Clone, Debug, PartialEq)]
pub struct DocumentEvent {
    /// What happened, such as `transition` or `send`.
    pub kind: String,
    /// When, as a quantity or number.
    pub time: Box<DocumentValue>,
    /// What the run printed for it.
    pub text: String,
    /// The object it happened on.
    pub object: Option<ObjectRef>,
    /// The machine involved.
    pub machine: String,
    /// The state involved.
    pub state: String,
    /// The state a transition left.
    pub from_state: String,
    /// The state a transition entered.
    pub to_state: String,
    /// The object a send addressed.
    pub target: Option<ObjectRef>,
    /// The event's name.
    pub event: String,
    /// What a send carried.
    pub payload: Vec<String>,
    /// The choices a choice point offered.
    pub alternatives: Vec<String>,
    /// The choice it took.
    pub taken: String,
}

impl fmt::Display for DocumentEvent {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.time, self.text)
    }
}

/// One value a document query answers or is bound to.
#[derive(Clone, Debug, PartialEq)]
pub enum DocumentValue {
    /// A model element.
    Element(ElementRef),
    /// An object of the subject.
    Object(ObjectRef),
    /// Text.
    Text(String),
    /// An integer.
    Integer(i64),
    /// A real number.
    Real(f64),
    /// A boolean.
    Boolean(bool),
    /// A quantity with its unit.
    Quantity(Quantity),
    /// The unbounded value `*`; answered, never bound.
    Infinity,
    /// A verdict; answered, never bound.
    Verdict(DocumentVerdict),
    /// A state; answered, never bound.
    State(DocumentState),
    /// An event; answered, never bound.
    Event(DocumentEvent),
}

impl fmt::Display for DocumentValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Element(v) => v.fmt(f),
            Self::Object(v) => v.fmt(f),
            Self::Text(v) => f.write_str(v),
            Self::Integer(v) => v.fmt(f),
            Self::Real(v) => v.fmt(f),
            Self::Boolean(v) => v.fmt(f),
            Self::Quantity(v) => v.fmt(f),
            Self::Infinity => f.write_str("*"),
            Self::Verdict(v) => v.fmt(f),
            Self::State(v) => v.fmt(f),
            Self::Event(v) => v.fmt(f),
        }
    }
}

/// One row of a document query: the element it is about and one cell per column.
#[derive(Clone, Debug, PartialEq)]
pub struct DocumentRow {
    /// The element the row is about.
    pub element: ElementRef,
    /// The cells, each holding the values of one column.
    pub cells: Vec<Vec<DocumentValue>>,
    /// The verdict a verdict row is.
    pub verdict: Option<DocumentVerdict>,
    /// The object an object, state or event row is about.
    pub object: Option<ObjectRef>,
    /// The state a state row is.
    pub state: Option<DocumentState>,
    /// The event an event row is.
    pub event: Option<DocumentEvent>,
}

/// What a document query answered: its columns and rows, in the service's order.
#[derive(Clone, Debug, PartialEq)]
pub struct DocumentQueryResult {
    /// Column names.
    pub columns: Vec<String>,
    /// Rows.
    pub rows: Vec<DocumentRow>,
    wire: wire::RunDocumentQueryResponse,
}

impl DocumentQueryResult {
    /// The RunDocumentQuery response this was read from.
    pub fn wire(&self) -> &wire::RunDocumentQueryResponse {
        &self.wire
    }
}

/// The form a document renders to.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub enum DocumentForm {
    /// Markdown.
    #[default]
    Markdown,
    /// A self-contained HTML page.
    Html,
}

pub(crate) fn bindings_to_wire<K: AsRef<str>>(
    bindings: &[(K, Vec<DocumentValue>)],
) -> Result<Vec<wire::DocumentQueryBinding>, Error> {
    bindings
        .iter()
        .map(|(parameter, values)| {
            let parameter = parameter.as_ref();
            Ok(wire::DocumentQueryBinding {
                parameter: parameter.to_owned(),
                values: values
                    .iter()
                    .map(|value| bound_value(parameter, value))
                    .collect::<Result<_, _>>()?,
            })
        })
        .collect()
}

fn bound_value(parameter: &str, value: &DocumentValue) -> Result<wire::DocumentValue, Error> {
    use wire::document_value::Kind;
    let refuse = |why: &str| {
        Error::InvalidRequest(format!("binding {parameter:?} cannot carry {value}: {why}"))
    };
    let kind = match value {
        DocumentValue::Element(element) => Kind::ElementId(element.id.clone()),
        DocumentValue::Object(object) => {
            if object.id == 0 && object.path.is_empty() {
                return Err(refuse(
                    "an object is bound by id or by path; neither was given",
                ));
            }
            Kind::Object(Box::new(wire::DocumentObject {
                instance_id: object.id,
                path: object.path.clone(),
                element: None,
            }))
        }
        DocumentValue::Text(text) => Kind::StringValue(text.clone()),
        DocumentValue::Integer(v) => Kind::IntValue(*v),
        DocumentValue::Real(v) => Kind::RealValue(*v),
        DocumentValue::Boolean(v) => Kind::BoolValue(*v),
        DocumentValue::Quantity(quantity) => {
            Kind::Quantity(quantity_to_wire(quantity).map_err(|error| refuse(&error.to_string()))?)
        }
        DocumentValue::Infinity => {
            return Err(refuse(
                "the unbounded value is answered by queries, not bound to them",
            ))
        }
        DocumentValue::Verdict(_) => {
            return Err(refuse(
                "a verdict is answered by queries, not bound to them",
            ))
        }
        DocumentValue::State(_) => {
            return Err(refuse(
                "a state row is answered by queries, not bound to them",
            ))
        }
        DocumentValue::Event(_) => {
            return Err(refuse(
                "an event row is answered by queries, not bound to them",
            ))
        }
    };
    Ok(wire::DocumentValue {
        element_type: String::new(),
        kind: Some(kind),
    })
}

pub(crate) fn result_of(
    response: wire::RunDocumentQueryResponse,
) -> Result<DocumentQueryResult, Error> {
    let wire = response.clone();
    Ok(DocumentQueryResult {
        columns: response.columns.into_iter().map(|c| c.name).collect(),
        rows: response
            .rows
            .into_iter()
            .map(row_of)
            .collect::<Result<_, _>>()?,
        wire,
    })
}

fn row_of(row: wire::DocumentQueryRow) -> Result<DocumentRow, Error> {
    let cells = row
        .cells
        .into_iter()
        .map(|cell| cell.values.into_iter().map(value_of).collect())
        .collect::<Result<_, Error>>()?;
    let element = row.element.unwrap_or_default();
    let mut out = DocumentRow {
        element: ElementRef::default(),
        cells,
        verdict: None,
        object: None,
        state: None,
        event: None,
    };
    match element.kind {
        Some(wire::document_value::Kind::Verdict(_))
        | Some(wire::document_value::Kind::Object(_))
        | Some(wire::document_value::Kind::State(_))
        | Some(wire::document_value::Kind::Event(_)) => match value_of(element)? {
            DocumentValue::Verdict(verdict) => {
                out.element = verdict.assertion.clone();
                out.verdict = Some(verdict);
            }
            DocumentValue::Object(object) => {
                out.element = object.element.clone().unwrap_or_default();
                out.object = Some(object);
            }
            DocumentValue::State(state) => {
                out.element = state.object.element.clone().unwrap_or_default();
                out.object = Some(state.object.clone());
                out.state = Some(state);
            }
            DocumentValue::Event(event) => {
                out.element = event
                    .object
                    .as_ref()
                    .and_then(|object| object.element.clone())
                    .unwrap_or_default();
                out.object = event.object.clone();
                out.event = Some(event);
            }
            _ => unreachable!("the arms matched above decode to themselves"),
        },
        _ => out.element = element_of(&element),
    }
    Ok(out)
}

fn element_of(value: &wire::DocumentValue) -> ElementRef {
    match &value.kind {
        Some(wire::document_value::Kind::ElementId(id)) => ElementRef {
            id: id.clone(),
            element_type: value.element_type.clone(),
        },
        _ => ElementRef {
            id: String::new(),
            element_type: value.element_type.clone(),
        },
    }
}

fn boxed_element(value: Option<Box<wire::DocumentValue>>) -> ElementRef {
    value.map(|v| element_of(&v)).unwrap_or_default()
}

fn object_of(object: wire::DocumentObject) -> ObjectRef {
    ObjectRef {
        id: object.instance_id,
        path: object.path,
        element: Some(boxed_element(object.element)),
    }
}

impl TryFrom<wire::DocumentValue> for DocumentValue {
    type Error = Error;

    fn try_from(value: wire::DocumentValue) -> Result<Self, Error> {
        value_of(value)
    }
}

fn value_of(value: wire::DocumentValue) -> Result<DocumentValue, Error> {
    use wire::document_value::Kind;
    let element_type = value.element_type;
    Ok(match value.kind {
        Some(Kind::ElementId(id)) => DocumentValue::Element(ElementRef { id, element_type }),
        Some(Kind::StringValue(v)) => DocumentValue::Text(v),
        Some(Kind::IntValue(v)) => DocumentValue::Integer(v),
        Some(Kind::RealValue(v)) => DocumentValue::Real(v),
        Some(Kind::BoolValue(v)) => DocumentValue::Boolean(v),
        Some(Kind::Infinity(true)) => DocumentValue::Infinity,
        Some(Kind::Infinity(false)) => {
            return Err(Error::Decode(
                "the infinity arm states no value unless it is true".to_owned(),
            ))
        }
        Some(Kind::Quantity(q)) => DocumentValue::Quantity(quantity_from_wire(q)?),
        Some(Kind::Object(object)) => DocumentValue::Object(object_of(*object)),
        Some(Kind::Verdict(verdict)) => {
            let verdict = *verdict;
            DocumentValue::Verdict(DocumentVerdict {
                assertion: boxed_element(verdict.assertion),
                kind: verdict.kind,
                text: verdict.text,
                path: verdict.path,
                status: verdict.verdict,
                condition: verdict.condition,
                reason: verdict.reason,
                verification: verdict.verification,
            })
        }
        Some(Kind::State(state)) => {
            let state = *state;
            DocumentValue::State(DocumentState {
                object: object_of(state.object.map(|o| *o).unwrap_or_default()),
                machine: state.machine,
                name: state.name,
                path: state.state_path,
                state: state.state.map(|s| element_of(&s)),
                region: state.region,
                enclosing: state.enclosing,
            })
        }
        Some(Kind::Event(event)) => {
            let event = *event;
            let time = match event.time {
                Some(time) => value_of(*time)?,
                None => return Err(Error::Decode("document event carries no time".to_owned())),
            };
            DocumentValue::Event(DocumentEvent {
                kind: event.kind,
                time: Box::new(time),
                text: event.text,
                object: event.object.map(|o| object_of(*o)),
                machine: event.machine,
                state: event.state,
                from_state: event.from,
                to_state: event.to,
                target: event.target.map(|o| object_of(*o)),
                event: event.event,
                payload: event.payload,
                alternatives: event.alternatives,
                taken: event.taken,
            })
        }
        None => {
            return Err(Error::Decode(
                "the service answered a document value with no kind".to_owned(),
            ))
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::Magnitude;

    #[test]
    fn bindings_carry_what_queries_read() {
        let bound = bindings_to_wire(&[(
            "x",
            vec![
                DocumentValue::Element(ElementRef::new("P::a")),
                DocumentValue::Object(ObjectRef::at("v.e")),
                DocumentValue::Integer(3),
                DocumentValue::Quantity(Quantity {
                    magnitude: Magnitude::Integer(2),
                    unit: String::new(),
                    unit_term: None,
                }),
            ],
        )])
        .unwrap();
        assert_eq!(bound[0].parameter, "x");
        assert_eq!(bound[0].values.len(), 4);
    }

    #[test]
    fn answers_are_refused_as_bindings() {
        for value in [
            DocumentValue::Infinity,
            DocumentValue::Object(ObjectRef::default()),
            DocumentValue::Quantity(Quantity {
                magnitude: Magnitude::Real(1.0),
                unit: "kg".to_owned(),
                unit_term: None,
            }),
        ] {
            let error = bindings_to_wire(&[("p", vec![value])]).unwrap_err();
            assert!(matches!(error, Error::InvalidRequest(_)), "{error}");
        }
    }

    #[test]
    fn verdict_rows_name_their_assertion() {
        use wire::document_value::Kind;
        let response = wire::RunDocumentQueryResponse {
            columns: vec![wire::DocumentQueryColumn {
                name: "status".to_owned(),
            }],
            rows: vec![wire::DocumentQueryRow {
                element: Some(wire::DocumentValue {
                    element_type: String::new(),
                    kind: Some(Kind::Verdict(Box::new(wire::DocumentVerdict {
                        assertion: Some(Box::new(wire::DocumentValue {
                            element_type: "RequirementUsage".to_owned(),
                            kind: Some(Kind::ElementId("P::r".to_owned())),
                        })),
                        verdict: "pass".to_owned(),
                        text: "r".to_owned(),
                        ..Default::default()
                    }))),
                }),
                cells: vec![wire::DocumentQueryCell {
                    values: vec![wire::DocumentValue {
                        element_type: String::new(),
                        kind: Some(Kind::StringValue("pass".to_owned())),
                    }],
                }],
            }],
        };
        let result = result_of(response).unwrap();
        assert_eq!(result.columns, ["status"]);
        let row = &result.rows[0];
        assert_eq!(row.element.id, "P::r");
        assert_eq!(row.verdict.as_ref().unwrap().status, "pass");
        assert_eq!(row.cells[0], [DocumentValue::Text("pass".to_owned())]);
    }
}
