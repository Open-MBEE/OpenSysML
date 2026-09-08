use std::collections::HashMap;
use std::fmt;
use std::sync::Arc;

use crate::{error::Error, wire, Connection};

/// Language accepted by the parser.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Language {
    /// SysML v2 notation.
    Sysml,
    /// KerML notation.
    Kerml,
}

impl Language {
    pub(crate) fn as_str(self) -> &'static str {
        match self {
            Self::Sysml => "sysml",
            Self::Kerml => "kerml",
        }
    }
}

/// Options controlling a parse operation.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ParseOptions {
    /// Language of inline content.
    pub language: Language,
    /// Require strict SysML v2 conformance.
    pub strict_conformance: bool,
}

impl Default for ParseOptions {
    fn default() -> Self {
        Self {
            language: Language::Sysml,
            strict_conformance: false,
        }
    }
}

/// An exact service capability set.
#[derive(Clone, Debug)]
pub struct Capabilities {
    wire: wire::ServerInfoResponse,
}

impl Capabilities {
    pub(crate) fn new(wire: wire::ServerInfoResponse) -> Self {
        Self { wire }
    }

    /// Whether the service advertises `capability`.
    pub fn has(&self, capability: &str) -> bool {
        self.wire.capabilities.iter().any(|item| item == capability)
    }

    /// Require a capability, returning a legible remedy when it is absent.
    pub fn require(&self, capability: &str, remedy: impl Into<String>) -> Result<(), Error> {
        if self.has(capability) {
            Ok(())
        } else {
            Err(Error::MissingCapability {
                capability: capability.to_owned(),
                remedy: remedy.into(),
            })
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::ServerInfoResponse {
        &self.wire
    }
}

/// Information advertised by a connected service.
#[derive(Clone, Debug)]
pub struct ServerInfo {
    /// Build version reported by the service.
    pub version: String,
    /// Service capabilities.
    pub capabilities: Capabilities,
    wire: wire::ServerInfoResponse,
}

impl ServerInfo {
    pub(crate) fn from_wire(wire: wire::ServerInfoResponse) -> Self {
        let capabilities = Capabilities::new(wire.clone());
        Self {
            version: wire.version.clone(),
            capabilities,
            wire,
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::ServerInfoResponse {
        &self.wire
    }
}

/// A source span attached to a diagnostic.
#[derive(Clone, Debug)]
pub struct Span {
    /// Source file name.
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

impl From<wire::Span> for Span {
    fn from(value: wire::Span) -> Self {
        Self {
            file: value.file,
            start_line: value.start_line,
            start_col: value.start_col,
            end_line: value.end_line,
            end_col: value.end_col,
        }
    }
}

/// A parser or semantic diagnostic.
#[derive(Clone, Debug)]
pub struct Diagnostic {
    /// Severity such as `error`, `warning`, or `info`.
    pub severity: String,
    /// Human-readable diagnostic message.
    pub message: String,
    /// Optional source location.
    pub span: Option<Span>,
    wire: wire::Diagnostic,
}

impl From<wire::Diagnostic> for Diagnostic {
    fn from(wire: wire::Diagnostic) -> Self {
        let span = wire.span.clone().map(Span::from);
        Self {
            severity: wire.severity.clone(),
            message: wire.message.clone(),
            span,
            wire,
        }
    }
}

impl Diagnostic {
    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::Diagnostic {
        &self.wire
    }
}

/// An integer or real quantity magnitude.
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Magnitude {
    /// An exact integer magnitude.
    Integer(i64),
    /// A floating-point magnitude.
    Real(f64),
}

/// A reduced measurement unit.
#[derive(Clone, Debug, PartialEq)]
pub struct UnitTerm {
    /// Scale numerator.
    pub scale_num: f64,
    /// Scale denominator.
    pub scale_den: f64,
    /// Base-unit factors.
    pub factors: Vec<UnitFactor>,
}

/// One base unit raised to an exponent.
#[derive(Clone, Debug, PartialEq)]
pub struct UnitFactor {
    /// Fully qualified base-unit identifier.
    pub unit_id: String,
    /// Exponent of the base unit.
    pub exponent: f64,
}

/// A numeric magnitude and its measurement unit.
#[derive(Clone, Debug, PartialEq)]
pub struct Quantity {
    /// Magnitude, preserving integer versus real.
    pub magnitude: Magnitude,
    /// Unit as written by the model.
    pub unit: String,
    /// Reduced unit term, when supplied by the service.
    pub unit_term: Option<UnitTerm>,
}

/// A complex number in rectangular form: one value, never two reals.
///
/// A service advertising `complex_values` sends one as itself; an older one
/// sends an unsupported [`Value::Null`] in its place.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Complex {
    /// Real part.
    pub real: f64,
    /// Imaginary part.
    pub imaginary: f64,
}

impl fmt::Display for Complex {
    /// Writes `1.5 - 2.0i`; the sign between the parts is the imaginary part's.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let sign = if self.imaginary.is_sign_negative() {
            '-'
        } else {
            '+'
        };
        write!(f, "{:?} {sign} {:?}i", self.real, self.imaginary.abs())
    }
}

/// An enumeration literal value.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EnumLiteral {
    /// Fully qualified literal identity.
    pub literal_id: String,
    /// Fully qualified enumeration definition.
    pub enumeration_id: String,
    /// Reader-facing literal name.
    pub name: String,
}

/// A multidimensional array: its shape, and its elements flattened in
/// row-major order (last dimension varying fastest).
///
/// A rank-0 array holds exactly one element. An element is any [`Value`], a
/// nested array or a quantity included. A service advertising
/// `structured_values` sends one as itself; an older one sends an unsupported
/// [`Value::Null`] in its place.
#[derive(Clone, Debug, PartialEq)]
pub struct Array {
    dimensions: Vec<i64>,
    elements: Vec<Value>,
}

impl Array {
    /// Builds an array, checking that every dimension is positive and that
    /// the elements fill the dimensions exactly.
    pub fn new(dimensions: Vec<i64>, elements: Vec<Value>) -> Result<Self, Error> {
        let mut size: i64 = 1;
        for &extent in &dimensions {
            if extent <= 0 {
                return Err(Error::Decode(format!(
                    "array dimension is not positive: {extent}"
                )));
            }
            size = size.checked_mul(extent).ok_or_else(|| {
                Error::Decode(format!("array dimensions {dimensions:?} overflow"))
            })?;
        }
        if u64::try_from(size).ok() != u64::try_from(elements.len()).ok() {
            return Err(Error::Decode(format!(
                "array of dimensions {dimensions:?} holds {} element(s), want {size}",
                elements.len()
            )));
        }
        Ok(Self {
            dimensions,
            elements,
        })
    }

    /// Extent of each dimension, all positive.
    pub fn dimensions(&self) -> &[i64] {
        &self.dimensions
    }

    /// Number of dimensions.
    pub fn rank(&self) -> usize {
        self.dimensions.len()
    }

    /// The elements in row-major order.
    pub fn elements(&self) -> &[Value] {
        &self.elements
    }

    /// The element at a multi-index, one coordinate per dimension; `None` when
    /// the index has the wrong rank or a coordinate is outside its dimension.
    pub fn get(&self, index: &[i64]) -> Option<&Value> {
        if index.len() != self.dimensions.len() {
            return None;
        }
        let mut flat: i64 = 0;
        for (&coordinate, &extent) in index.iter().zip(&self.dimensions) {
            if coordinate < 0 || coordinate >= extent {
                return None;
            }
            flat = flat * extent + coordinate;
        }
        self.elements.get(usize::try_from(flat).ok()?)
    }
}

/// A vector of numbers, each kept as the [`Magnitude`] the model computed:
/// one value, never a sequence of numbers.
///
/// A service advertising `structured_values` sends one as itself; an older
/// one sends an unsupported [`Value::Null`] in its place.
#[derive(Clone, Debug, PartialEq)]
pub struct Vector {
    /// The components, in order.
    pub components: Vec<Magnitude>,
}

impl Vector {
    /// Number of components.
    pub fn dimension(&self) -> usize {
        self.components.len()
    }
}

/// A vector whose components are quantities, each with its own unit:
/// `VectorOf((3.0, 4.0)) [m]` holds two metres. The units usually agree but
/// need not.
///
/// A service advertising `structured_values` sends one as itself; an older
/// one sends an unsupported [`Value::Null`] in its place.
#[derive(Clone, Debug, PartialEq)]
pub struct VectorQuantity {
    components: Vec<Quantity>,
}

impl VectorQuantity {
    /// Builds a vector quantity, refusing one with no components.
    pub fn new(components: Vec<Quantity>) -> Result<Self, Error> {
        if components.is_empty() {
            return Err(Error::Decode(
                "vector quantity has no components".to_owned(),
            ));
        }
        Ok(Self { components })
    }

    /// The components, at least one, in order.
    pub fn components(&self) -> &[Quantity] {
        &self.components
    }

    /// Number of components.
    pub fn dimension(&self) -> usize {
        self.components.len()
    }

    /// The one unit every component is written in, or `None` when they differ.
    pub fn unit(&self) -> Option<&str> {
        let first = &self.components[0].unit;
        self.components
            .iter()
            .all(|component| component.unit == *first)
            .then_some(first.as_str())
    }
}

/// A measurement unit held as a value by itself, with no magnitude: `SI::m`,
/// or `m / s` as an operation composed it.
///
/// A service advertising `measurement_refs` sends one as itself; an older one
/// sends an unsupported [`Value::Null`] in its place.
#[derive(Clone, Debug, PartialEq)]
pub struct MeasurementRef {
    /// Unit as written by the model (`km`), empty for one never written down.
    pub unit: String,
    /// What the unit reduces to; the service always sends it.
    pub unit_term: UnitTerm,
    /// FQN of the one declaration the unit names (`SI::kilometre`); `None` for
    /// a unit an operation composed.
    pub unit_id: Option<String>,
}

/// A runtime value returned by the service.
#[derive(Clone, Debug, PartialEq)]
pub enum Value {
    /// Integer value.
    Integer(i64),
    /// Real value.
    Real(f64),
    /// Complex value.
    Complex(Complex),
    /// Boolean value.
    Boolean(bool),
    /// Text value.
    Text(String),
    /// Instance reference.
    InstanceRef(i64),
    /// Sequence value.
    Sequence(Vec<Value>),
    /// Quantity value.
    Quantity(Quantity),
    /// Enumeration literal value.
    EnumLiteral(EnumLiteral),
    /// Multidimensional array value.
    Array(Array),
    /// Numeric vector value.
    Vector(Vector),
    /// Vector of quantities.
    VectorQuantity(VectorQuantity),
    /// A bare measurement unit.
    MeasurementRef(MeasurementRef),
    /// Explicit null value.
    Null,
    /// A materialized feature with no value.
    Unset,
    /// The unbounded value `*`, ordered above every finite magnitude.
    Infinity,
}

pub(crate) fn value_from_wire(value: wire::Value) -> Result<Value, Error> {
    let Some(kind) = value.kind else {
        return Err(Error::Decode("Value has no kind".to_owned()));
    };
    match kind {
        wire::value::Kind::IntValue(v) => Ok(Value::Integer(v)),
        wire::value::Kind::RealValue(v) => Ok(Value::Real(v)),
        wire::value::Kind::Complex(v) => Ok(Value::Complex(Complex {
            real: v.real,
            imaginary: v.imaginary,
        })),
        wire::value::Kind::BoolValue(v) => Ok(Value::Boolean(v)),
        wire::value::Kind::StringValue(v) => Ok(Value::Text(v)),
        wire::value::Kind::InstanceId(v) => Ok(Value::InstanceRef(v)),
        wire::value::Kind::Sequence(v) => Ok(Value::Sequence(
            v.elements
                .into_iter()
                .map(value_from_wire)
                .collect::<Result<_, _>>()?,
        )),
        wire::value::Kind::Null(_) => Ok(Value::Null),
        wire::value::Kind::Quantity(v) => Ok(Value::Quantity(quantity_from_wire(v)?)),
        wire::value::Kind::Array(v) => Ok(Value::Array(Array::new(
            v.dimensions,
            v.elements
                .into_iter()
                .map(value_from_wire)
                .collect::<Result<_, _>>()?,
        )?)),
        wire::value::Kind::Vector(v) => Ok(Value::Vector(Vector {
            components: v
                .components
                .into_iter()
                .map(|component| match component.kind {
                    Some(wire::value::Kind::IntValue(value)) => Ok(Magnitude::Integer(value)),
                    Some(wire::value::Kind::RealValue(value)) => Ok(Magnitude::Real(value)),
                    other => Err(Error::Decode(format!(
                        "vector component is not a number: {}",
                        other.as_ref().map_or("no kind", kind_name)
                    ))),
                })
                .collect::<Result<_, _>>()?,
        })),
        wire::value::Kind::VectorQuantity(v) => Ok(Value::VectorQuantity(VectorQuantity::new(
            v.components
                .into_iter()
                .map(quantity_from_wire)
                .collect::<Result<_, _>>()?,
        )?)),
        wire::value::Kind::MeasurementRef(v) => {
            Ok(Value::MeasurementRef(measurement_ref_from_wire(v)?))
        }
        wire::value::Kind::EnumLiteral(v) => Ok(Value::EnumLiteral(EnumLiteral {
            literal_id: v.literal_id,
            enumeration_id: v.enumeration_id,
            name: v.name,
        })),
        wire::value::Kind::Unset(_) => Ok(Value::Unset),
        // Only an asserted arm carries the unbounded value.
        wire::value::Kind::Infinity(asserted) => {
            if !asserted {
                return Err(Error::Decode(
                    "the infinity arm states no value unless it is true".to_owned(),
                ));
            }
            Ok(Value::Infinity)
        }
    }
}

/// The wire field name of a `Value` arm, for messages about a misplaced one.
fn kind_name(kind: &wire::value::Kind) -> &'static str {
    match kind {
        wire::value::Kind::IntValue(_) => "int_value",
        wire::value::Kind::RealValue(_) => "real_value",
        wire::value::Kind::BoolValue(_) => "bool_value",
        wire::value::Kind::StringValue(_) => "string_value",
        wire::value::Kind::InstanceId(_) => "instance_id",
        wire::value::Kind::Sequence(_) => "sequence",
        wire::value::Kind::Null(_) => "null",
        wire::value::Kind::Quantity(_) => "quantity",
        wire::value::Kind::EnumLiteral(_) => "enum_literal",
        wire::value::Kind::Unset(_) => "unset",
        wire::value::Kind::Infinity(_) => "infinity",
        wire::value::Kind::Complex(_) => "complex",
        wire::value::Kind::Array(_) => "array",
        wire::value::Kind::Vector(_) => "vector",
        wire::value::Kind::VectorQuantity(_) => "vector_quantity",
        wire::value::Kind::MeasurementRef(_) => "measurement_ref",
    }
}

fn measurement_ref_from_wire(v: wire::MeasurementRef) -> Result<MeasurementRef, Error> {
    let Some(term) = v.unit_term else {
        if v.unit.is_empty() && v.unit_id.is_empty() {
            return Err(Error::Decode(
                "a measurement reference names no unit".to_owned(),
            ));
        }
        let named = if v.unit.is_empty() {
            &v.unit_id
        } else {
            &v.unit
        };
        return Err(Error::Decode(format!(
            "a measurement reference {named} has no reduction to base units"
        )));
    };
    Ok(MeasurementRef {
        unit: v.unit,
        unit_term: unit_term_from_wire(term),
        unit_id: (!v.unit_id.is_empty()).then_some(v.unit_id),
    })
}

fn unit_term_from_wire(term: wire::UnitTerm) -> UnitTerm {
    UnitTerm {
        scale_num: term.scale_num,
        scale_den: term.scale_den,
        factors: term
            .factors
            .into_iter()
            .map(|factor| UnitFactor {
                unit_id: factor.unit_id,
                exponent: factor.exponent,
            })
            .collect(),
    }
}

fn quantity_from_wire(v: wire::Quantity) -> Result<Quantity, Error> {
    let magnitude = match v.magnitude {
        Some(wire::quantity::Magnitude::IntMagnitude(value)) => Magnitude::Integer(value),
        Some(wire::quantity::Magnitude::RealMagnitude(value)) => Magnitude::Real(value),
        None => return Err(Error::Decode("Quantity has no magnitude".to_owned())),
    };
    Ok(Quantity {
        magnitude,
        unit: v.unit,
        unit_term: v.unit_term.map(unit_term_from_wire),
    })
}

/// A symbol in a parsed model.
#[derive(Clone, Debug)]
pub struct Symbol {
    wire: wire::SymbolInfo,
    connection: Arc<crate::connection::ConnectionInner>,
    model_hash: String,
}

impl Symbol {
    pub(crate) fn new(
        wire: wire::SymbolInfo,
        connection: Arc<crate::connection::ConnectionInner>,
        model_hash: String,
    ) -> Self {
        Self {
            wire,
            connection,
            model_hash,
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::SymbolInfo {
        &self.wire
    }
    /// Fully qualified identifier.
    pub fn id(&self) -> &str {
        &self.wire.id
    }
    /// Short name.
    pub fn name(&self) -> &str {
        &self.wire.name
    }
    /// Service symbol kind.
    pub fn kind(&self) -> &str {
        &self.wire.kind
    }
    /// Child symbols, fetched lazily from the service.
    pub fn children(&self) -> Result<Vec<Symbol>, Error> {
        self.wire
            .child_ids
            .iter()
            .map(|id| {
                Connection {
                    inner: self.connection.clone(),
                }
                .get_symbol(&self.model_hash, id)
            })
            .collect()
    }
}

/// An instantiated model object.
#[derive(Clone, Debug)]
pub struct Instance {
    wire: wire::Instance,
    features: HashMap<String, FeatureValue>,
}

impl Instance {
    pub(crate) fn from_wire(wire: wire::Instance) -> Result<Self, Error> {
        let features = wire
            .feature_values
            .iter()
            .map(|(name, value)| {
                FeatureValue::from_wire(value.clone()).map(|item| (name.clone(), item))
            })
            .collect::<Result<_, _>>()?;
        Ok(Self { wire, features })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::Instance {
        &self.wire
    }
    /// Runtime identity.
    pub fn id(&self) -> i64 {
        self.wire.id
    }
    /// Fully qualified type identifier.
    pub fn type_symbol_id(&self) -> &str {
        &self.wire.type_symbol_id
    }
    /// Feature values keyed by feature name.
    pub fn feature_values(&self) -> &HashMap<String, FeatureValue> {
        &self.features
    }
    /// One feature value, if present.
    pub fn feature(&self, name: &str) -> Option<&FeatureValue> {
        self.features.get(name)
    }
}

/// A value held by one instance feature.
#[derive(Clone, Debug)]
pub struct FeatureValue {
    wire: wire::FeatureValue,
    value: Option<Value>,
    values: Vec<Value>,
}

impl FeatureValue {
    fn from_wire(wire: wire::FeatureValue) -> Result<Self, Error> {
        let value = wire.value.clone().map(value_from_wire).transpose()?;
        let values = wire
            .values
            .iter()
            .cloned()
            .map(value_from_wire)
            .collect::<Result<_, _>>()?;
        Ok(Self {
            wire,
            value,
            values,
        })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::FeatureValue {
        &self.wire
    }
    /// Feature name.
    pub fn name(&self) -> &str {
        &self.wire.feature_name
    }
    /// Single-valued feature value.
    pub fn value(&self) -> Option<&Value> {
        self.value.as_ref()
    }
    /// Multi-valued feature values.
    pub fn values(&self) -> &[Value] {
        &self.values
    }
    /// Whether the feature was materialized.
    pub fn materialized(&self) -> bool {
        self.wire.materialized
    }
    /// In-band evaluation error for this feature.
    pub fn error(&self) -> Option<&str> {
        (!self.wire.error.is_empty()).then_some(self.wire.error.as_str())
    }
}

/// Options for evaluating an expression.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct EvalOptions {
    /// Optional context symbol identifier.
    pub context: Option<String>,
    /// Optional subject symbol identifier.
    pub subject: Option<String>,
}

/// An evaluated expression and its wire response.
#[derive(Clone, Debug)]
pub struct Evaluation {
    /// Decoded expression result.
    pub result: Value,
    wire: wire::EvaluateResponse,
}

impl Evaluation {
    pub(crate) fn new(result: Value, wire: wire::EvaluateResponse) -> Self {
        Self { result, wire }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::EvaluateResponse {
        &self.wire
    }
}

/// A parsed model.
#[derive(Clone, Debug)]
pub struct Model {
    wire: wire::ParseFileResponse,
    root: Option<Symbol>,
    diagnostics: Vec<Diagnostic>,
    connection: Connection,
}

impl Model {
    pub(crate) fn from_wire(
        wire: wire::ParseFileResponse,
        connection: Connection,
    ) -> Result<Self, Error> {
        let root = wire.root.clone().map(|root_wire| {
            Symbol::new(root_wire, connection.inner.clone(), wire.model_hash.clone())
        });
        let diagnostics = wire
            .diagnostics
            .iter()
            .cloned()
            .map(Diagnostic::from)
            .collect();
        Ok(Self {
            wire,
            root,
            diagnostics,
            connection,
        })
    }

    pub(crate) fn from_hash(hash: &str, connection: Connection) -> Self {
        Self {
            wire: wire::ParseFileResponse {
                model_hash: hash.to_owned(),
                ..Default::default()
            },
            root: None,
            diagnostics: Vec::new(),
            connection,
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::ParseFileResponse {
        &self.wire
    }
    /// Content hash used by subsequent service requests.
    pub fn hash(&self) -> &str {
        &self.wire.model_hash
    }
    /// Diagnostics in service order.
    pub fn diagnostics(&self) -> &[Diagnostic] {
        &self.diagnostics
    }
    /// Root namespace symbol, if this handle includes one.
    pub fn root(&self) -> Option<&Symbol> {
        self.root.as_ref()
    }
    /// Evaluate an expression and return its domain value.
    pub fn eval(&self, expr: &str) -> Result<Value, Error> {
        Ok(self.evaluate(expr, &EvalOptions::default())?.result)
    }
    /// Evaluate an expression with optional context and subject.
    pub fn evaluate(&self, expr: &str, options: &EvalOptions) -> Result<Evaluation, Error> {
        self.connection.evaluate(self.hash(), expr, options)
    }
    /// Look up a fully qualified symbol.
    pub fn symbol(&self, fqn: &str) -> Result<Symbol, Error> {
        self.connection.get_symbol(self.hash(), fqn)
    }
    /// Instantiate a part or usage.
    pub fn instantiate(&self, fqn: &str) -> Result<Instantiation, Error> {
        self.connection.instantiate(self.hash(), fqn)
    }
}

/// An instantiation response and its primary instance.
#[derive(Clone, Debug)]
pub struct Instantiation {
    /// Primary instantiated object.
    pub instance: Instance,
    instances: Vec<Instance>,
    wire: wire::InstantiateResponse,
}

impl Instantiation {
    pub(crate) fn from_wire(wire: wire::InstantiateResponse) -> Result<Self, Error> {
        let instance_wire = wire
            .instance
            .clone()
            .ok_or_else(|| Error::Decode("instantiate response has no instance".to_owned()))?;
        let instance = Instance::from_wire(instance_wire)?;
        let instances = wire
            .instances
            .iter()
            .cloned()
            .map(Instance::from_wire)
            .collect::<Result<_, _>>()?;
        Ok(Self {
            instance,
            instances,
            wire,
        })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::InstantiateResponse {
        &self.wire
    }
    /// All reachable instances included by the service.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn value_arms_are_decoded_without_loss() {
        let sequence = wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![
                    wire::Value {
                        kind: Some(wire::value::Kind::IntValue(7)),
                    },
                    wire::Value {
                        kind: Some(wire::value::Kind::RealValue(2.5)),
                    },
                ],
            })),
        };
        let result = value_from_wire(sequence);
        assert_eq!(
            result.ok(),
            Some(Value::Sequence(vec![Value::Integer(7), Value::Real(2.5)]))
        );
    }

    #[test]
    fn a_complex_number_is_one_value_with_both_parts() {
        let complex = |real, imaginary| wire::Value {
            kind: Some(wire::value::Kind::Complex(wire::Complex {
                real,
                imaginary,
            })),
        };
        assert_eq!(
            value_from_wire(complex(1.5, -2.0)).ok(),
            Some(Value::Complex(Complex {
                real: 1.5,
                imaginary: -2.0
            }))
        );
        // Proto3 defaults are zero, so an empty Complex message is 0 + 0i.
        assert_eq!(
            value_from_wire(wire::Value {
                kind: Some(wire::value::Kind::Complex(wire::Complex::default())),
            })
            .ok(),
            Some(Value::Complex(Complex {
                real: 0.0,
                imaginary: 0.0
            }))
        );
        let sequence = wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![complex(1.0, 2.0), complex(3.0, 4.0)],
            })),
        };
        assert_eq!(
            value_from_wire(sequence).ok(),
            Some(Value::Sequence(vec![
                Value::Complex(Complex {
                    real: 1.0,
                    imaginary: 2.0
                }),
                Value::Complex(Complex {
                    real: 3.0,
                    imaginary: 4.0
                }),
            ]))
        );
        assert_ne!(
            Value::Complex(Complex {
                real: 1.5,
                imaginary: 0.0
            }),
            Value::Real(1.5)
        );
    }

    #[test]
    fn only_an_asserted_infinity_arm_is_the_unbounded_value() {
        let arm = |asserted| wire::Value {
            kind: Some(wire::value::Kind::Infinity(asserted)),
        };
        assert_eq!(value_from_wire(arm(true)).ok(), Some(Value::Infinity));
        assert!(matches!(value_from_wire(arm(false)), Err(Error::Decode(_))));
        let nested = wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![arm(false)],
            })),
        };
        assert!(matches!(value_from_wire(nested), Err(Error::Decode(_))));
    }

    #[test]
    fn a_complex_number_prints_in_rectangular_form() {
        let print = |real, imaginary| Complex { real, imaginary }.to_string();
        assert_eq!(print(1.5, -2.0), "1.5 - 2.0i");
        assert_eq!(print(1.0, 2.0), "1.0 + 2.0i");
        assert_eq!(print(0.0, 0.0), "0.0 + 0.0i");
        assert_eq!(print(-3.25, -0.0), "-3.25 - 0.0i");
    }

    fn int(value: i64) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::IntValue(value)),
        }
    }

    fn real(value: f64) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::RealValue(value)),
        }
    }

    fn metres(magnitude: f64) -> wire::Quantity {
        wire::Quantity {
            magnitude: Some(wire::quantity::Magnitude::RealMagnitude(magnitude)),
            unit: "m".to_owned(),
            unit_term: Some(wire::UnitTerm {
                scale_num: 1.0,
                scale_den: 1.0,
                factors: vec![wire::UnitFactor {
                    unit_id: "SI::metre".to_owned(),
                    exponent: 1.0,
                }],
            }),
        }
    }

    fn metre_term() -> UnitTerm {
        UnitTerm {
            scale_num: 1.0,
            scale_den: 1.0,
            factors: vec![UnitFactor {
                unit_id: "SI::metre".to_owned(),
                exponent: 1.0,
            }],
        }
    }

    fn array(dimensions: Vec<i64>, elements: Vec<wire::Value>) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::Array(wire::Array {
                dimensions,
                elements,
            })),
        }
    }

    fn vector(components: Vec<wire::Value>) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::Vector(wire::Vector { components })),
        }
    }

    fn vector_quantity(components: Vec<wire::Quantity>) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::VectorQuantity(wire::VectorQuantity {
                components,
            })),
        }
    }

    #[test]
    fn an_array_keeps_its_shape_and_row_major_elements() {
        let Ok(Value::Array(grid)) = value_from_wire(array(
            vec![2, 3],
            vec![int(1), int(2), int(3), int(4), int(5), int(6)],
        )) else {
            panic!("a (2, 3) array should decode");
        };
        assert_eq!(grid.dimensions(), [2, 3]);
        assert_eq!(grid.rank(), 2);
        assert_eq!(grid.get(&[1, 2]), Some(&Value::Integer(6)));
        assert_eq!(grid.get(&[0, 1]), Some(&Value::Integer(2)));
        assert_eq!(grid.get(&[2, 0]), None);
        assert_eq!(grid.get(&[0]), None);
        assert_eq!(grid.elements().len(), 6);

        // Rank 0 holds exactly one element; rank 1 and 3 keep every extent.
        let Ok(Value::Array(scalar)) = value_from_wire(array(vec![], vec![real(7.0)])) else {
            panic!("a rank-0 array should decode");
        };
        assert_eq!(scalar.rank(), 0);
        assert_eq!(scalar.get(&[]), Some(&Value::Real(7.0)));
        let Ok(Value::Array(cube)) =
            value_from_wire(array(vec![2, 2, 2], (0..8).map(int).collect()))
        else {
            panic!("a (2, 2, 2) array should decode");
        };
        assert_eq!(cube.get(&[1, 0, 1]), Some(&Value::Integer(5)));

        // An element is any value: a nested array of a quantity, or a vector.
        let nested = value_from_wire(array(
            vec![2],
            vec![
                array(
                    vec![1],
                    vec![wire::Value {
                        kind: Some(wire::value::Kind::Quantity(metres(3.0))),
                    }],
                ),
                vector(vec![real(1.0), real(2.0)]),
            ],
        ));
        let inner = Array::new(
            vec![1],
            vec![Value::Quantity(Quantity {
                magnitude: Magnitude::Real(3.0),
                unit: "m".to_owned(),
                unit_term: Some(metre_term()),
            })],
        )
        .ok();
        assert_eq!(
            nested.ok(),
            Some(Value::Array(
                Array::new(
                    vec![2],
                    vec![
                        Value::Array(inner.expect("inner array is well formed")),
                        Value::Vector(Vector {
                            components: vec![Magnitude::Real(1.0), Magnitude::Real(2.0)],
                        }),
                    ],
                )
                .expect("outer array is well formed")
            ))
        );
    }

    #[test]
    fn a_malformed_array_is_refused() {
        let short = value_from_wire(array(vec![2, 3], vec![int(1), int(2)]));
        assert!(
            matches!(&short, Err(Error::Decode(message)) if message.contains("want 6")),
            "{short:?}"
        );
        assert!(matches!(
            value_from_wire(array(vec![0], vec![])),
            Err(Error::Decode(message)) if message.contains("not positive")
        ));
        assert!(matches!(
            value_from_wire(array(vec![-1], vec![int(1)])),
            Err(Error::Decode(_))
        ));
        assert!(matches!(
            value_from_wire(array(vec![i64::MAX, 2], vec![])),
            Err(Error::Decode(message)) if message.contains("overflow")
        ));
    }

    #[test]
    fn a_vector_keeps_integer_and_real_components_apart() {
        assert_eq!(
            value_from_wire(vector(vec![real(3.0), real(4.0)])).ok(),
            Some(Value::Vector(Vector {
                components: vec![Magnitude::Real(3.0), Magnitude::Real(4.0)],
            }))
        );
        assert_eq!(
            value_from_wire(vector(vec![int(1), real(2.5)])).ok(),
            Some(Value::Vector(Vector {
                components: vec![Magnitude::Integer(1), Magnitude::Real(2.5)],
            }))
        );
        assert_eq!(
            value_from_wire(vector(vec![])).ok(),
            Some(Value::Vector(Vector { components: vec![] }))
        );
        assert_ne!(
            value_from_wire(vector(vec![real(3.0)])).ok(),
            Some(Value::Sequence(vec![Value::Real(3.0)]))
        );

        let text = value_from_wire(vector(vec![
            real(1.0),
            wire::Value {
                kind: Some(wire::value::Kind::StringValue("two".to_owned())),
            },
        ]));
        assert!(
            matches!(&text, Err(Error::Decode(message)) if message.contains("string_value")),
            "{text:?}"
        );
        assert!(matches!(
            value_from_wire(vector(vec![wire::Value { kind: None }])),
            Err(Error::Decode(message)) if message.contains("no kind")
        ));
    }

    #[test]
    fn a_vector_quantity_keeps_one_quantity_per_component() {
        let Ok(Value::VectorQuantity(position)) =
            value_from_wire(vector_quantity(vec![metres(3.0), metres(4.0)]))
        else {
            panic!("a vector quantity should decode");
        };
        assert_eq!(position.dimension(), 2);
        assert_eq!(position.unit(), Some("m"));
        assert_eq!(
            position.components(),
            [
                Quantity {
                    magnitude: Magnitude::Real(3.0),
                    unit: "m".to_owned(),
                    unit_term: Some(metre_term()),
                },
                Quantity {
                    magnitude: Magnitude::Real(4.0),
                    unit: "m".to_owned(),
                    unit_term: Some(metre_term()),
                },
            ]
        );

        // Units may differ per component; a composed unit keeps its reduction.
        let speed = wire::Quantity {
            magnitude: Some(wire::quantity::Magnitude::RealMagnitude(5.0)),
            unit: "m/s".to_owned(),
            unit_term: Some(wire::UnitTerm {
                scale_num: 1.0,
                scale_den: 1.0,
                factors: vec![
                    wire::UnitFactor {
                        unit_id: "SI::metre".to_owned(),
                        exponent: 1.0,
                    },
                    wire::UnitFactor {
                        unit_id: "SI::second".to_owned(),
                        exponent: -1.0,
                    },
                ],
            }),
        };
        let Ok(Value::VectorQuantity(mixed)) =
            value_from_wire(vector_quantity(vec![metres(1.0), speed]))
        else {
            panic!("a mixed vector quantity should decode");
        };
        assert_eq!(mixed.unit(), None);
        assert_eq!(mixed.components()[1].unit, "m/s");
        assert_eq!(
            mixed.components()[1]
                .unit_term
                .as_ref()
                .map(|term| term.factors.len()),
            Some(2)
        );

        assert!(matches!(
            value_from_wire(vector_quantity(vec![])),
            Err(Error::Decode(message)) if message.contains("no components")
        ));
        assert!(matches!(
            value_from_wire(vector_quantity(vec![wire::Quantity::default()])),
            Err(Error::Decode(message)) if message.contains("no magnitude")
        ));
    }

    fn measurement_ref(
        unit: &str,
        unit_id: &str,
        unit_term: Option<wire::UnitTerm>,
    ) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::MeasurementRef(wire::MeasurementRef {
                unit: unit.to_owned(),
                unit_term,
                unit_id: unit_id.to_owned(),
            })),
        }
    }

    fn wire_term(scale_num: f64, factors: &[(&str, f64)]) -> wire::UnitTerm {
        wire::UnitTerm {
            scale_num,
            scale_den: 1.0,
            factors: factors
                .iter()
                .map(|(unit_id, exponent)| wire::UnitFactor {
                    unit_id: (*unit_id).to_owned(),
                    exponent: *exponent,
                })
                .collect(),
        }
    }

    #[test]
    fn a_measurement_reference_keeps_its_unit_reduction_and_declaration() {
        let km = wire_term(1000.0, &[("SI::metre", 1.0)]);
        assert_eq!(
            value_from_wire(measurement_ref("km", "SI::kilometre", Some(km))).ok(),
            Some(Value::MeasurementRef(MeasurementRef {
                unit: "km".to_owned(),
                unit_term: UnitTerm {
                    scale_num: 1000.0,
                    scale_den: 1.0,
                    factors: vec![UnitFactor {
                        unit_id: "SI::metre".to_owned(),
                        exponent: 1.0,
                    }],
                },
                unit_id: Some("SI::kilometre".to_owned()),
            }))
        );

        // A unit an operation composed names no declaration, and none is invented.
        let speed = wire_term(1.0, &[("SI::metre", 1.0), ("SI::second", -1.0)]);
        let Ok(Value::MeasurementRef(composed)) =
            value_from_wire(measurement_ref("m/s", "", Some(speed)))
        else {
            panic!("a composed unit should decode");
        };
        assert_eq!(composed.unit_id, None);
        assert_eq!(composed.unit_term.factors.len(), 2);

        // Naming no unit, or a unit without its reduction, is malformed at any depth.
        assert!(matches!(
            value_from_wire(measurement_ref("", "", None)),
            Err(Error::Decode(message)) if message.contains("names no unit")
        ));
        assert!(matches!(
            value_from_wire(measurement_ref("km", "", None)),
            Err(Error::Decode(message)) if message.contains("km has no reduction")
        ));
        let nested = wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![measurement_ref("", "SI::kilometre", None)],
            })),
        };
        assert!(matches!(
            value_from_wire(nested),
            Err(Error::Decode(message)) if message.contains("SI::kilometre has no reduction")
        ));
    }

    #[test]
    fn a_structured_value_survives_the_wire_bytes() {
        use prost::Message;
        for value in [
            array(
                vec![2, 3],
                vec![int(1), int(2), int(3), int(4), int(5), int(6)],
            ),
            vector(vec![real(3.0), int(4)]),
            vector_quantity(vec![metres(3.0), metres(4.0)]),
            measurement_ref(
                "km",
                "SI::kilometre",
                Some(wire_term(1000.0, &[("SI::metre", 1.0)])),
            ),
        ] {
            let again = wire::Value::decode(value.encode_to_vec().as_slice())
                .expect("a value encodes to decodable bytes");
            assert_eq!(
                value_from_wire(value.clone()).ok(),
                value_from_wire(again).ok()
            );
        }
    }

    #[test]
    fn value_arms_are_decoded_without_capability_gates() {
        let result = value_from_wire(wire::Value {
            kind: Some(wire::value::Kind::Unset(true)),
        });
        assert_eq!(result.ok(), Some(Value::Unset));
    }
}
