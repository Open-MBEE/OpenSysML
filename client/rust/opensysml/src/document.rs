//! Document queries and their typed rows, and the forms a document renders to.

use std::fmt;

use crate::domain::{quantity_from_wire, BigInteger, Quantity};
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

pub(crate) fn document_event_from_wire(event: wire::DocumentEvent) -> Result<DocumentEvent, Error> {
    let time = match event.time {
        Some(time) => value_of(*time)?,
        None => return Err(Error::Decode("document event carries no time".to_owned())),
    };
    Ok(DocumentEvent {
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

/// Encode a typed state-run event for a service response.
pub fn document_event_to_wire(event: &DocumentEvent) -> Result<wire::DocumentEvent, Error> {
    Ok(wire::DocumentEvent {
        kind: event.kind.clone(),
        time: Some(Box::new(bound_value("trace time", &event.time)?)),
        text: event.text.clone(),
        object: event
            .object
            .as_ref()
            .map(|object| Box::new(object_to_wire(object))),
        machine: event.machine.clone(),
        state: event.state.clone(),
        from: event.from_state.clone(),
        to: event.to_state.clone(),
        target: event
            .target
            .as_ref()
            .map(|target| Box::new(object_to_wire(target))),
        event: event.event.clone(),
        payload: event.payload.clone(),
        alternatives: event.alternatives.clone(),
        taken: event.taken.clone(),
    })
}

fn object_to_wire(object: &ObjectRef) -> wire::DocumentObject {
    use wire::document_value::Kind;
    wire::DocumentObject {
        instance_id: object.id,
        path: object.path.clone(),
        element: object.element.as_ref().map(|element| {
            Box::new(wire::DocumentValue {
                element_type: element.element_type.clone(),
                kind: Some(Kind::ElementId(element.id.clone())),
            })
        }),
    }
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
    /// An Integer beyond `i64`.
    BigInteger(BigInteger),
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
            Self::BigInteger(v) => v.fmt(f),
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

/// Which ports a rendered view includes.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub enum RenderViewPorts {
    /// Ports used by rendered edges.
    #[default]
    Minimal,
    /// All declared ports.
    Full,
}

/// A source location carried by a rendered node, edge, or row.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct RenderSpan {
    /// Source file path or name.
    pub file: String,
    /// One-based starting line.
    pub start_line: i32,
    /// One-based starting column.
    pub start_col: i32,
    /// One-based ending line.
    pub end_line: i32,
    /// One-based ending column.
    pub end_col: i32,
}

/// One feature rendered on a node boundary.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct RenderPort {
    /// Stable identifier within the rendered view.
    pub id: String,
    /// Port name.
    pub name: String,
    /// Port type name.
    pub port_type: String,
    /// Port direction.
    pub direction: String,
}

/// Node position and size, when stated.
#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct RenderGeometry {
    /// Horizontal position.
    pub x: f64,
    /// Vertical position.
    pub y: f64,
    /// Width when size is stated.
    pub width: f64,
    /// Height when size is stated.
    pub height: f64,
    /// Whether width and height are present.
    pub has_size: bool,
    /// Whether the node is collapsed.
    pub collapsed: bool,
}

/// Visual properties shared by nodes and edges.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderStyle {
    /// Fill color or pattern.
    pub fill: String,
    /// Line color or pattern.
    pub line: String,
    /// Text color or pattern.
    pub text: String,
    /// Font family.
    pub font: String,
    /// Font size.
    pub font_size: f64,
    /// Whether text is bold.
    pub bold: bool,
    /// Whether text is italic.
    pub italic: bool,
}

/// A routed edge point.
#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct RenderPoint {
    /// Horizontal coordinate.
    pub x: f64,
    /// Vertical coordinate.
    pub y: f64,
}

/// One flattened rendered node; parents precede children.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderNode {
    /// Stable identifier within the rendered view.
    pub id: String,
    /// Rendered node kind.
    pub kind: String,
    /// Display name.
    pub name: String,
    /// Whether the name was synthesized.
    pub name_synthesized: bool,
    /// Type name.
    pub node_type: String,
    /// Additional node detail.
    pub detail: String,
    /// Rendered node text.
    pub text: String,
    /// Whether this node stands in for an omitted element.
    pub stand_in: bool,
    /// Parent node identifier.
    pub parent: String,
    /// Ports included for this node.
    pub ports: Vec<RenderPort>,
    /// Source location, when available.
    pub origin: Option<RenderSpan>,
    /// Position and size, when stated.
    pub geometry: Option<RenderGeometry>,
    /// Visual properties, when stated.
    pub style: Option<RenderStyle>,
}

/// A rendered edge between two nodes.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderEdge {
    /// Source node identifier.
    pub from: String,
    /// Target node identifier.
    pub to: String,
    /// Source port identifier.
    pub from_port: String,
    /// Target port identifier.
    pub to_port: String,
    /// Display label.
    pub label: String,
    /// Edge name.
    pub name: String,
    /// Rendered edge kind.
    pub kind: String,
    /// Source location, when available.
    pub origin: Option<RenderSpan>,
    /// Routed edge points.
    pub route: Vec<RenderPoint>,
    /// Visual properties, when stated.
    pub style: Option<RenderStyle>,
}

/// The drawing surface, when stated by the view.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderCanvas {
    /// Coordinate unit.
    pub unit: String,
    /// Canvas width.
    pub width: f64,
    /// Canvas height.
    pub height: f64,
    /// Whether width and height are present.
    pub has_size: bool,
}

/// A rendered table row.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderRow {
    /// Rendered cell values.
    pub cells: Vec<String>,
    /// Source location, when available.
    pub origin: Option<RenderSpan>,
}

/// A rendered note and its optional node or edge anchor.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderNote {
    /// Note text.
    pub text: String,
    /// Node identifier this note is anchored to.
    pub anchor: String,
    /// Source node identifier for an edge anchor.
    pub edge_from: String,
    /// Target node identifier for an edge anchor.
    pub edge_to: String,
    /// Horizontal position.
    pub x: f64,
    /// Vertical position.
    pub y: f64,
    /// Width when size is stated.
    pub width: f64,
    /// Height when size is stated.
    pub height: f64,
    /// Whether width and height are present.
    pub has_size: bool,
    /// Source location, when available.
    pub origin: Option<RenderSpan>,
}

/// The complete machine-readable rendering of a named or targeted pseudo-view.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct RenderedView {
    /// Qualified view name or pseudo-view name.
    pub view: String,
    /// Rendered view kind.
    pub kind: String,
    /// Declared rendering statement.
    pub stated: String,
    /// Rendered nodes in parent-before-child order.
    pub nodes: Vec<RenderNode>,
    /// Rendered edges.
    pub edges: Vec<RenderEdge>,
    /// Table column names.
    pub columns: Vec<String>,
    /// Rendered table rows.
    pub rows: Vec<RenderRow>,
    /// Canvas, when stated.
    pub canvas: Option<RenderCanvas>,
    /// Rendered notes.
    pub notes: Vec<RenderNote>,
    /// Renderer notices.
    pub notices: Vec<String>,
    wire: wire::RenderViewResponse,
}

impl RenderedView {
    /// The RenderView response this value was decoded from.
    pub fn wire(&self) -> &wire::RenderViewResponse {
        &self.wire
    }
}

pub(crate) fn rendered_view_of(response: wire::RenderViewResponse) -> RenderedView {
    let wire = response.clone();
    let span = |span: wire::Span| RenderSpan {
        file: span.file,
        start_line: span.start_line,
        start_col: span.start_col,
        end_line: span.end_line,
        end_col: span.end_col,
    };
    let style = |style: wire::RenderStyle| RenderStyle {
        fill: style.fill,
        line: style.line,
        text: style.text,
        font: style.font,
        font_size: style.font_size,
        bold: style.bold,
        italic: style.italic,
    };
    RenderedView {
        view: response.view,
        kind: response.kind,
        stated: response.stated,
        nodes: response
            .nodes
            .into_iter()
            .map(|node| RenderNode {
                id: node.id,
                kind: node.kind,
                name: node.name,
                name_synthesized: node.name_synthesized,
                node_type: node.r#type,
                detail: node.detail,
                text: node.text,
                stand_in: node.stand_in,
                parent: node.parent,
                ports: node
                    .ports
                    .into_iter()
                    .map(|port| RenderPort {
                        id: port.id,
                        name: port.name,
                        port_type: port.r#type,
                        direction: port.direction,
                    })
                    .collect(),
                origin: node.origin.map(&span),
                geometry: node.geometry.map(|geometry| RenderGeometry {
                    x: geometry.x,
                    y: geometry.y,
                    width: geometry.width,
                    height: geometry.height,
                    has_size: geometry.has_size,
                    collapsed: geometry.collapsed,
                }),
                style: node.style.map(&style),
            })
            .collect(),
        edges: response
            .edges
            .into_iter()
            .map(|edge| RenderEdge {
                from: edge.from,
                to: edge.to,
                from_port: edge.from_port,
                to_port: edge.to_port,
                label: edge.label,
                name: edge.name,
                kind: edge.kind,
                origin: edge.origin.map(&span),
                route: edge
                    .route
                    .into_iter()
                    .map(|point| RenderPoint {
                        x: point.x,
                        y: point.y,
                    })
                    .collect(),
                style: edge.style.map(&style),
            })
            .collect(),
        columns: response.columns,
        rows: response
            .rows
            .into_iter()
            .map(|row| RenderRow {
                cells: row.cells,
                origin: row.origin.map(&span),
            })
            .collect(),
        canvas: response.canvas.map(|canvas| RenderCanvas {
            unit: canvas.unit,
            width: canvas.width,
            height: canvas.height,
            has_size: canvas.has_size,
        }),
        notes: response
            .notes
            .into_iter()
            .map(|note| RenderNote {
                text: note.text,
                anchor: note.anchor,
                edge_from: note.edge_from,
                edge_to: note.edge_to,
                x: note.x,
                y: note.y,
                width: note.width,
                height: note.height,
                has_size: note.has_size,
                origin: note.origin.map(&span),
            })
            .collect(),
        notices: response.notices,
        wire,
    }
}

#[cfg(test)]
mod render_view_tests {
    use super::*;

    #[test]
    fn rendered_view_decoder_preserves_fields_and_optional_messages() {
        let response = wire::RenderViewResponse {
            view: "Demo::view".to_owned(),
            kind: "interconnection".to_owned(),
            stated: "rendered".to_owned(),
            nodes: vec![wire::RenderNode {
                id: "n0".to_owned(),
                kind: "part".to_owned(),
                name: "root".to_owned(),
                name_synthesized: true,
                r#type: "Demo::Part".to_owned(),
                detail: "detail".to_owned(),
                text: "text".to_owned(),
                stand_in: true,
                ports: vec![wire::RenderPort {
                    id: "n0.0".to_owned(),
                    name: "api".to_owned(),
                    r#type: "Demo::API".to_owned(),
                    direction: "inout".to_owned(),
                }],
                origin: Some(wire::Span {
                    file: "views.sysml".to_owned(),
                    start_line: 4,
                    ..Default::default()
                }),
                geometry: Some(wire::RenderGeometry {
                    x: 1.0,
                    y: 2.0,
                    width: 3.0,
                    height: 4.0,
                    has_size: true,
                    collapsed: true,
                }),
                style: Some(wire::RenderStyle {
                    fill: "#fff".to_owned(),
                    font_size: 12.0,
                    bold: true,
                    italic: true,
                    ..Default::default()
                }),
                ..Default::default()
            }],
            edges: vec![wire::RenderEdge {
                from: "n0".to_owned(),
                to: "n1".to_owned(),
                from_port: "n0.0".to_owned(),
                to_port: "n1.0".to_owned(),
                label: "wire".to_owned(),
                name: "wire".to_owned(),
                kind: "connection".to_owned(),
                route: vec![wire::RenderPoint { x: 2.0, y: 3.0 }],
                ..Default::default()
            }],
            columns: vec!["a".to_owned()],
            rows: vec![wire::RenderRow {
                cells: vec!["x".to_owned()],
                ..Default::default()
            }],
            canvas: Some(wire::RenderCanvas {
                unit: "px".to_owned(),
                width: 800.0,
                height: 400.0,
                has_size: true,
            }),
            notes: vec![wire::RenderNote {
                text: "note".to_owned(),
                anchor: "n0".to_owned(),
                edge_from: "n0".to_owned(),
                edge_to: "n1".to_owned(),
                has_size: true,
                ..Default::default()
            }],
            notices: vec!["notice".to_owned()],
        };

        let rendered = rendered_view_of(response);
        assert_eq!(rendered.view, "Demo::view");
        assert!(rendered.nodes[0].name_synthesized);
        assert_eq!(rendered.nodes[0].ports[0].direction, "inout");
        assert_eq!(rendered.nodes[0].origin.as_ref().unwrap().start_line, 4);
        assert!(rendered.nodes[0].geometry.as_ref().unwrap().collapsed);
        assert_eq!(rendered.nodes[0].style.as_ref().unwrap().font_size, 12.0);
        assert_eq!(rendered.edges[0].from_port, "n0.0");
        assert_eq!(rendered.edges[0].route[0].x, 2.0);
        assert_eq!(rendered.rows[0].cells, vec!["x".to_owned()]);
        assert!(rendered.canvas.as_ref().unwrap().has_size);
        assert_eq!(rendered.notes[0].edge_to, "n1");
        assert_eq!(rendered.notices, vec!["notice".to_owned()]);

        let empty = rendered_view_of(wire::RenderViewResponse::default());
        assert!(empty.canvas.is_none());
        assert!(empty.nodes.is_empty());
    }
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

/// Whether a wire binding sends an Integer beyond int64, which needs `big_int_values`.
pub(crate) fn binding_holds_big_int(binding: &wire::DocumentQueryBinding) -> bool {
    use wire::document_value::Kind;
    binding.values.iter().any(|value| match &value.kind {
        Some(Kind::BigIntValue(_)) => true,
        Some(Kind::Quantity(quantity)) => matches!(
            quantity.magnitude,
            Some(wire::quantity::Magnitude::BigIntMagnitude(_))
        ),
        _ => false,
    })
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
        DocumentValue::BigInteger(v) => Kind::BigIntValue(v.as_str().to_owned()),
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
        element: object.element.map(|v| element_of(&v)),
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
        Some(Kind::BigIntValue(v)) => DocumentValue::BigInteger(BigInteger::parse(&v)?),
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
    fn a_wide_integer_crosses_as_big_int_value() {
        use wire::document_value::Kind;
        let wide = BigInteger::parse("1180591620717411303424").unwrap();
        let bound =
            bindings_to_wire(&[("n", vec![DocumentValue::BigInteger(wide.clone())])]).unwrap();
        assert_eq!(
            bound[0].values[0].kind,
            Some(Kind::BigIntValue("1180591620717411303424".to_owned()))
        );
        assert!(binding_holds_big_int(&bound[0]));
        let read = DocumentValue::try_from(bound[0].values[0].clone()).unwrap();
        assert_eq!(read, DocumentValue::BigInteger(wide.clone()));
        assert_eq!(read.to_string(), "1180591620717411303424");

        let magnitude = bindings_to_wire(&[(
            "m",
            vec![DocumentValue::Quantity(Quantity {
                magnitude: Magnitude::BigInteger(wide),
                unit: String::new(),
                unit_term: None,
            })],
        )])
        .unwrap();
        assert!(binding_holds_big_int(&magnitude[0]));
        let narrow = bindings_to_wire(&[("n", vec![DocumentValue::Integer(i64::MAX)])]).unwrap();
        assert!(!binding_holds_big_int(&narrow[0]));
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
    fn an_object_naming_no_element_has_none() {
        let value = DocumentValue::try_from(wire::DocumentValue {
            element_type: String::new(),
            kind: Some(wire::document_value::Kind::Object(Box::new(
                wire::DocumentObject {
                    instance_id: 7,
                    path: "car.engine".to_owned(),
                    element: None,
                },
            ))),
        })
        .unwrap();
        match value {
            DocumentValue::Object(object) => assert_eq!(object.element, None),
            other => panic!("expected an object, got {other:?}"),
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
