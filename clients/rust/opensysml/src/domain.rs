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
    /// What was found, stable across message wording: `syntax`, a validation
    /// code, `choice-point` or `guard-unevaluable`; empty when the service assigned none.
    pub code: String,
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
            code: wire.code.clone(),
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
        let size = shape_size("array", &dimensions)?;
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
        row_major(&self.dimensions, index).and_then(|flat| self.elements.get(flat))
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

/// A unique, unordered collection: a `Collections::Set`'s elements.
///
/// The service sends the members in its canonical order (numbers ascending,
/// then strings, and so on), each exactly once; two sets are equal when they
/// hold the same members whatever the order, judged by
/// [`Value::same_value`]. A service advertising `set_values` sends one as
/// itself; an older one sends an unsupported [`Value::Null`] in its place.
#[derive(Clone, Debug)]
pub struct Set {
    elements: Vec<Value>,
}

impl Set {
    /// Builds a set, refusing one that lists a member twice by
    /// [`Value::same_value`].
    pub fn new(elements: Vec<Value>) -> Result<Self, Error> {
        for (i, element) in elements.iter().enumerate() {
            if elements[..i].iter().any(|e| e.same_value(element)) {
                return Err(Error::Decode(format!(
                    "set lists a member twice: {element:?}"
                )));
            }
        }
        Ok(Self { elements })
    }

    /// The members, each once, in the order the service sent them.
    pub fn elements(&self) -> &[Value] {
        &self.elements
    }

    /// Number of members.
    pub fn len(&self) -> usize {
        self.elements.len()
    }

    /// Whether the set has no members.
    pub fn is_empty(&self) -> bool {
        self.elements.is_empty()
    }

    /// Whether `value` is a member, by [`Value::same_value`].
    pub fn contains(&self, value: &Value) -> bool {
        self.elements.iter().any(|e| e.same_value(value))
    }
}

impl PartialEq for Set {
    /// Order-insensitive: the same members in any order are the same set.
    fn eq(&self, other: &Self) -> bool {
        self.len() == other.len() && self.elements.iter().all(|e| other.contains(e))
    }
}

/// A tensor of quantities of any rank: its shape, and its components
/// flattened in row-major order, each a [`Quantity`] with its own unit.
///
/// A rank-one tensor stays a tensor, distinct from a [`VectorQuantity`]. A
/// service advertising `tensor_values` sends one as itself; an older one
/// sends an unsupported [`Value::Null`] in its place.
#[derive(Clone, Debug, PartialEq)]
pub struct TensorQuantity {
    dimensions: Vec<i64>,
    components: Vec<Quantity>,
}

impl TensorQuantity {
    /// Builds a tensor, checking that every dimension is positive and that
    /// the components fill the dimensions exactly.
    pub fn new(dimensions: Vec<i64>, components: Vec<Quantity>) -> Result<Self, Error> {
        let size = shape_size("tensor quantity", &dimensions)?;
        if u64::try_from(size).ok() != u64::try_from(components.len()).ok() {
            return Err(Error::Decode(format!(
                "tensor quantity of dimensions {dimensions:?} holds {} component(s), want {size}",
                components.len()
            )));
        }
        Ok(Self {
            dimensions,
            components,
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

    /// The components in row-major order.
    pub fn components(&self) -> &[Quantity] {
        &self.components
    }

    /// The component at a multi-index, one coordinate per dimension; `None`
    /// when the index has the wrong rank or a coordinate is outside its
    /// dimension.
    pub fn get(&self, index: &[i64]) -> Option<&Quantity> {
        row_major(&self.dimensions, index).and_then(|flat| self.components.get(flat))
    }

    /// The one unit every component is written in, or `None` when they differ
    /// (a rank-0 tensor has exactly one component, so always its unit).
    pub fn unit(&self) -> Option<&str> {
        let first = &self.components.first()?.unit;
        self.components
            .iter()
            .all(|component| component.unit == *first)
            .then_some(first.as_str())
    }
}

/// The element count a shape describes, refusing a non-positive extent or
/// one that overflows.
fn shape_size(what: &str, dimensions: &[i64]) -> Result<i64, Error> {
    let mut size: i64 = 1;
    for &extent in dimensions {
        if extent <= 0 {
            return Err(Error::Decode(format!(
                "{what} dimension is not positive: {extent}"
            )));
        }
        size = size
            .checked_mul(extent)
            .ok_or_else(|| Error::Decode(format!("{what} dimensions {dimensions:?} overflow")))?;
    }
    Ok(size)
}

/// The row-major offset of a multi-index, `None` when it is out of shape.
fn row_major(dimensions: &[i64], index: &[i64]) -> Option<usize> {
    if index.len() != dimensions.len() {
        return None;
    }
    let mut flat: i64 = 0;
    for (&coordinate, &extent) in index.iter().zip(dimensions) {
        if coordinate < 0 || coordinate >= extent {
            return None;
        }
        flat = flat * extent + coordinate;
    }
    usize::try_from(flat).ok()
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

/// A calc held as a value: a calc definition, or a calc usage with an input no
/// read could supply, as `Sq` in `Fn(Sq, 3.0)` or the `f` of `in calc f {...}`.
///
/// It is the declaration it is a value of, which is its identity: two functions
/// are equal exactly when both fields are. A function closing over the bindings
/// of the behavior body it is declared in has no wire form; the service sends
/// it as an unsupported [`Value::Null`], as does a service without
/// `function_values` for every function.
#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct Function {
    /// FQN of the calc declaration (`Analysis::Sq`).
    pub calc_id: String,
    /// ID of the object the calc's feature names resolve against, for a calc
    /// usage read off a part (`holder.scale`); `None` for a function closing
    /// over no object.
    pub self_id: Option<i64>,
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
    /// A calc held as a value.
    Function(Function),
    /// A unique, unordered collection.
    Set(Set),
    /// A tensor of quantities of any rank.
    TensorQuantity(TensorQuantity),
    /// Explicit null value.
    Null,
    /// A materialized feature with no value.
    Unset,
    /// The unbounded value `*`, ordered above every finite magnitude.
    Infinity,
}

impl Value {
    /// Whether two values are the same value to the model, as the service
    /// judges a set's membership: numbers by value, so a whole [`Value::Real`]
    /// is the [`Value::Integer`] of its value and a [`Value::Complex`] on the
    /// real axis is its real part, exactly across the whole `i64` range; a
    /// sequence's order counts and a set's does not; a quantity is compared
    /// over its base units, so `1 [m]` is `100 [cm]` — exactly while integer
    /// magnitudes scale by whole factors — and one lacking a reduction is
    /// compared in its unit as written; a measurement reference is one
    /// reduction at one scale however spelt, except that a named unit of
    /// dimension one is only its own declaration (`rad` is not `sr`); an
    /// enumeration literal is its `literal_id`, whatever else describes it.
    /// Every other arm compares as `==` does.
    pub fn same_value(&self, other: &Value) -> bool {
        match (self, other) {
            (Value::Integer(_) | Value::Real(_) | Value::Complex(_), _) => {
                numbers_equal(self, other)
            }
            (Value::Sequence(a), Value::Sequence(b)) => sequences_equal(a, b),
            (Value::Quantity(a), Value::Quantity(b)) => quantities_equal(a, b),
            (Value::Array(a), Value::Array(b)) => {
                a.dimensions == b.dimensions && sequences_equal(&a.elements, &b.elements)
            }
            (Value::Vector(a), Value::Vector(b)) => {
                a.components.len() == b.components.len()
                    && a.components
                        .iter()
                        .zip(&b.components)
                        .all(|(m, n)| magnitudes_equal(*m, *n))
            }
            (Value::VectorQuantity(a), Value::VectorQuantity(b)) => {
                components_equal(&a.components, &b.components)
            }
            (Value::TensorQuantity(a), Value::TensorQuantity(b)) => {
                a.dimensions == b.dimensions && components_equal(&a.components, &b.components)
            }
            (Value::MeasurementRef(a), Value::MeasurementRef(b)) => measurement_refs_equal(a, b),
            (Value::EnumLiteral(a), Value::EnumLiteral(b)) => a.literal_id == b.literal_id,
            _ => self == other,
        }
    }
}

// One reduction at one scale (`SI::'m/s'` is `m/s`, `km/m` is `m/mm`); a named
// unit reducing to nothing is only the declaration it names.
fn measurement_refs_equal(a: &MeasurementRef, b: &MeasurementRef) -> bool {
    if !same_reduction(&a.unit_term, &b.unit_term) {
        return false;
    }
    if exponents(&a.unit_term).is_empty() && (a.unit_id.is_some() || b.unit_id.is_some()) {
        return a.unit_id == b.unit_id;
    }
    true
}

// One reduction: commensurable at one scale, however the ratio is written.
fn same_reduction(a: &UnitTerm, b: &UnitTerm) -> bool {
    commensurable(a, b)
        && !zero_scale(a)
        && !zero_scale(b)
        && a.scale_num * b.scale_den == b.scale_num * a.scale_den
}

fn numbers_equal(a: &Value, b: &Value) -> bool {
    let on_axis = |v: &Value| match *v {
        Value::Complex(z) if z.imaginary == 0.0 => Some(Magnitude::Real(z.real)),
        Value::Integer(n) => Some(Magnitude::Integer(n)),
        Value::Real(r) => Some(Magnitude::Real(r)),
        _ => None,
    };
    match (on_axis(a), on_axis(b)) {
        (Some(x), Some(y)) => magnitudes_equal(x, y),
        _ => a == b,
    }
}

fn magnitudes_equal(a: Magnitude, b: Magnitude) -> bool {
    match (a, b) {
        (Magnitude::Integer(x), Magnitude::Integer(y)) => x == y,
        (Magnitude::Real(x), Magnitude::Real(y)) => x == y,
        (Magnitude::Integer(n), Magnitude::Real(r))
        | (Magnitude::Real(r), Magnitude::Integer(n)) => real_is_int(r, n),
    }
}

// Whether `r` is exactly the integer `n`, never rounding `n`.
fn real_is_int(r: f64, n: i64) -> bool {
    r.fract() == 0.0
        && (-9_223_372_036_854_775_808.0..9_223_372_036_854_775_808.0).contains(&r)
        && r as i64 == n
}

fn sequences_equal(a: &[Value], b: &[Value]) -> bool {
    a.len() == b.len() && a.iter().zip(b).all(|(x, y)| x.same_value(y))
}

// As the service judges it: over the base units when both carry a reduction,
// exactly while integer magnitudes scale by whole factors.
fn quantities_equal(a: &Quantity, b: &Quantity) -> bool {
    let (Some(x), Some(y)) = (&a.unit_term, &b.unit_term) else {
        return magnitudes_equal(a.magnitude, b.magnitude)
            && a.unit == b.unit
            && a.unit_term == b.unit_term;
    };
    if !commensurable(x, y) || zero_scale(x) || zero_scale(y) {
        return false;
    }
    if let (Some(m), Some(n)) = (exact_base_magnitude(a), exact_base_magnitude(b)) {
        return m == n;
    }
    base_magnitude(a.magnitude, x) == base_magnitude(b.magnitude, y)
}

// The base unit exponents, repeated units summed and cancelled ones dropped.
fn exponents(term: &UnitTerm) -> HashMap<&str, f64> {
    let mut totals: HashMap<&str, f64> = HashMap::new();
    for factor in &term.factors {
        *totals.entry(factor.unit_id.as_str()).or_insert(0.0) += factor.exponent;
    }
    totals.retain(|_, exponent| *exponent != 0.0);
    totals
}

fn commensurable(a: &UnitTerm, b: &UnitTerm) -> bool {
    exponents(a) == exponents(b)
}

fn zero_scale(term: &UnitTerm) -> bool {
    term.scale_num == 0.0 || term.scale_den == 0.0
}

fn base_magnitude(magnitude: Magnitude, term: &UnitTerm) -> f64 {
    let m = match magnitude {
        Magnitude::Integer(n) => n as f64,
        Magnitude::Real(r) => r,
    };
    m * term.scale_num / term.scale_den
}

// The base magnitude as an exact rational (numerator, denominator), while an
// integer magnitude scales by whole factors that fit.
fn exact_base_magnitude(q: &Quantity) -> Option<ExactRational> {
    let Magnitude::Integer(n) = q.magnitude else {
        return None;
    };
    let term = q.unit_term.as_ref()?;
    let num = i128::from(whole(term.scale_num)?);
    let den = i128::from(whole(term.scale_den)?);
    Some(ExactRational::new(i128::from(n).checked_mul(num)?, den))
}

fn whole(scale: f64) -> Option<i64> {
    (scale.fract() == 0.0
        && (-9_223_372_036_854_775_808.0..9_223_372_036_854_775_808.0).contains(&scale))
    .then_some(scale as i64)
}

#[derive(Clone, Copy, Debug, PartialEq)]
struct ExactRational {
    num: i128,
    den: i128,
}

impl ExactRational {
    fn new(num: i128, den: i128) -> Self {
        let g = gcd(num.unsigned_abs(), den.unsigned_abs()) as i128;
        let sign = if den < 0 { -1 } else { 1 };
        Self {
            num: sign * num / g,
            den: sign * den / g,
        }
    }
}

fn gcd(mut a: u128, mut b: u128) -> u128 {
    while b != 0 {
        (a, b) = (b, a % b);
    }
    a.max(1)
}

fn components_equal(a: &[Quantity], b: &[Quantity]) -> bool {
    a.len() == b.len() && a.iter().zip(b).all(|(x, y)| quantities_equal(x, y))
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
        wire::value::Kind::Function(v) => {
            if v.calc_id.is_empty() {
                return Err(Error::Decode("a function names no calc".to_owned()));
            }
            Ok(Value::Function(Function {
                calc_id: v.calc_id,
                self_id: (v.self_id != 0).then_some(v.self_id),
            }))
        }
        wire::value::Kind::Set(v) => Ok(Value::Set(Set::new(
            v.elements
                .into_iter()
                .map(value_from_wire)
                .collect::<Result<_, _>>()?,
        )?)),
        wire::value::Kind::TensorQuantity(v) => Ok(Value::TensorQuantity(TensorQuantity::new(
            v.dimensions,
            v.components
                .into_iter()
                .map(quantity_from_wire)
                .collect::<Result<_, _>>()?,
        )?)),
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
        wire::value::Kind::Function(_) => "function",
        wire::value::Kind::Set(_) => "set",
        wire::value::Kind::TensorQuantity(_) => "tensor_quantity",
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
    fn a_diagnostic_carries_its_code() {
        let coded = Diagnostic::from(wire::Diagnostic {
            severity: "info".to_owned(),
            message: "choice point: 2 steppable tokens".to_owned(),
            code: "choice-point".to_owned(),
            span: None,
        });
        assert_eq!(coded.code, "choice-point");
        assert_eq!(coded.wire().code, "choice-point");

        let uncoded = Diagnostic::from(wire::Diagnostic {
            severity: "error".to_owned(),
            message: "expected '}'".to_owned(),
            ..Default::default()
        });
        assert_eq!(uncoded.code, "");
    }

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

    fn set(elements: Vec<wire::Value>) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::Set(wire::ValueSet { elements })),
        }
    }

    fn tensor(dimensions: Vec<i64>, components: Vec<wire::Quantity>) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::TensorQuantity(wire::TensorQuantity {
                dimensions,
                components,
            })),
        }
    }

    #[test]
    fn a_set_holds_each_member_once_and_compares_in_any_order() {
        let Ok(Value::Set(members)) = value_from_wire(set(vec![int(1), int(2), int(3)])) else {
            panic!("a set should decode");
        };
        assert_eq!(members.len(), 3);
        assert!(!members.is_empty());
        assert_eq!(
            members.elements(),
            [Value::Integer(1), Value::Integer(2), Value::Integer(3)]
        );
        assert!(members.contains(&Value::Integer(2)));
        assert!(!members.contains(&Value::Integer(4)));
        assert!(members.contains(&Value::Real(2.0)));
        assert!(!members.contains(&Value::Real(2.5)));

        // The same members in another order are the same set; a sequence is not.
        let reordered = Set::new(vec![
            Value::Integer(3),
            Value::Integer(1),
            Value::Integer(2),
        ])
        .expect("three distinct members");
        assert_eq!(members, reordered);
        assert_ne!(
            Value::Set(members.clone()),
            Value::Sequence(vec![
                Value::Integer(1),
                Value::Integer(2),
                Value::Integer(3)
            ])
        );
        assert_ne!(
            members,
            Set::new(vec![Value::Integer(1), Value::Integer(2)]).expect("two members")
        );

        // An empty set is a set; a set nests.
        let Ok(Value::Set(empty)) = value_from_wire(set(vec![])) else {
            panic!("an empty set should decode");
        };
        assert!(empty.is_empty());
        let Ok(Value::Set(nested)) = value_from_wire(set(vec![set(vec![int(1)]), set(vec![])]))
        else {
            panic!("a set of sets should decode");
        };
        assert_eq!(nested.len(), 2);
        assert!(nested.contains(&Value::Set(empty)));
        let Ok(Value::Sequence(holding)) = value_from_wire(wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![set(vec![int(1)]), int(2)],
            })),
        }) else {
            panic!("a sequence holding a set should decode");
        };
        assert!(matches!(holding[0], Value::Set(_)));

        // A member listed twice is not a set, judged as the model does: an
        // Integer and the whole Real of its value are one member.
        for twice in [
            set(vec![int(1), int(1)]),
            set(vec![int(1), real(1.0)]),
            set(vec![real(1.5), complex(1.5, 0.0)]),
            set(vec![int(2), complex(2.0, 0.0)]),
        ] {
            let twice = value_from_wire(twice);
            assert!(
                matches!(&twice, Err(Error::Decode(message)) if message.contains("twice")),
                "{twice:?}"
            );
        }
        for alike in [
            set(vec![int(1), real(1.5)]),
            set(vec![int((1 << 53) + 1), real(9_007_199_254_740_992.0)]),
            set(vec![real(1.0), complex(1.0, 1.0)]),
            set(vec![int(1), bool(true)]),
        ] {
            let Ok(Value::Set(two)) = value_from_wire(alike) else {
                panic!("members that only look alike should decode");
            };
            assert_eq!(two.len(), 2);
        }
        assert!(matches!(
            value_from_wire(set(vec![wire::Value { kind: None }])),
            Err(Error::Decode(message)) if message.contains("no kind")
        ));
    }

    fn complex(real: f64, imaginary: f64) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::Complex(wire::Complex {
                real,
                imaginary,
            })),
        }
    }

    fn bool(value: bool) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::BoolValue(value)),
        }
    }

    /// `same_value` judges numbers as the service does: by value across
    /// Integer, Real and a Complex on the real axis, exactly, inside
    /// quantities, vectors and sets; `==` stays structural.
    #[test]
    fn same_value_judges_numbers_by_value() {
        let z = |real, imaginary| Value::Complex(Complex { real, imaginary });
        let metre = |magnitude| {
            Value::Quantity(Quantity {
                magnitude,
                unit: "m".to_owned(),
                unit_term: None,
            })
        };
        let cases = [
            (Value::Integer(1), Value::Real(1.0), true),
            (Value::Integer(1), Value::Real(1.5), false),
            (
                Value::Integer((1 << 53) + 1),
                Value::Real(9_007_199_254_740_992.0),
                false,
            ),
            (
                Value::Integer(1 << 53),
                Value::Real(9_007_199_254_740_992.0),
                true,
            ),
            (
                Value::Integer(i64::MAX),
                Value::Real(9_223_372_036_854_775_808.0),
                false,
            ),
            (
                Value::Integer(i64::MIN),
                Value::Real(-9_223_372_036_854_775_808.0),
                true,
            ),
            (Value::Integer(0), Value::Real(f64::INFINITY), false),
            (Value::Integer(0), Value::Real(-0.0), true),
            (Value::Real(2.5), z(2.5, 0.0), true),
            (Value::Integer(2), z(2.0, 0.0), true),
            (Value::Integer(2), z(2.0, 1.0), false),
            (z(2.0, 1.0), z(2.0, 1.0), true),
            (Value::Integer(1), Value::Boolean(true), false),
            (Value::Integer(1), Value::Text("1".to_owned()), false),
            (
                metre(Magnitude::Integer(1)),
                metre(Magnitude::Real(1.0)),
                true,
            ),
            (
                metre(Magnitude::Integer(1)),
                metre(Magnitude::Real(2.0)),
                false,
            ),
            (
                Value::Vector(Vector {
                    components: vec![Magnitude::Integer(1), Magnitude::Real(2.0)],
                }),
                Value::Vector(Vector {
                    components: vec![Magnitude::Real(1.0), Magnitude::Integer(2)],
                }),
                true,
            ),
            (
                Value::Sequence(vec![Value::Integer(1), Value::Integer(2)]),
                Value::Sequence(vec![Value::Real(1.0), Value::Real(2.0)]),
                true,
            ),
            (
                Value::Sequence(vec![Value::Integer(1), Value::Integer(2)]),
                Value::Sequence(vec![Value::Integer(2), Value::Integer(1)]),
                false,
            ),
            (
                Value::Set(Set::new(vec![Value::Integer(1), Value::Real(2.5)]).unwrap()),
                Value::Set(Set::new(vec![Value::Real(2.5), Value::Real(1.0)]).unwrap()),
                true,
            ),
            (
                Value::Set(Set::new(vec![Value::Integer((1 << 53) + 1)]).unwrap()),
                Value::Set(Set::new(vec![Value::Real(9_007_199_254_740_992.0)]).unwrap()),
                false,
            ),
            (
                Value::Set(Set::new(vec![Value::Integer(1), Value::Integer(2)]).unwrap()),
                Value::Sequence(vec![Value::Integer(1), Value::Integer(2)]),
                false,
            ),
        ];
        for (a, b, want) in cases {
            assert_eq!(a.same_value(&b), want, "{a:?} vs {b:?}");
            assert_eq!(b.same_value(&a), want, "{b:?} vs {a:?}");
        }
        assert_ne!(Value::Integer(1), Value::Real(1.0));
        assert_eq!(
            Set::new(vec![Value::Integer(1)]).unwrap(),
            Set::new(vec![Value::Real(1.0)]).unwrap()
        );
    }

    /// `same_value` judges quantities over their base units, as the service
    /// does, so equivalent quantities are one set member however written.
    #[test]
    fn same_value_judges_quantities_across_units() {
        let term = |scale_num, scale_den, factors: &[(&str, f64)]| {
            Some(UnitTerm {
                scale_num,
                scale_den,
                factors: factors
                    .iter()
                    .map(|(unit_id, exponent)| UnitFactor {
                        unit_id: (*unit_id).to_owned(),
                        exponent: *exponent,
                    })
                    .collect(),
            })
        };
        let quantity = |magnitude, unit: &str, unit_term| {
            Value::Quantity(Quantity {
                magnitude,
                unit: unit.to_owned(),
                unit_term,
            })
        };
        let metre = &[("SI::metre", 1.0)][..];
        let speed = &[("SI::metre", 1.0), ("SI::second", -1.0)][..];
        let m = |n| quantity(Magnitude::Integer(n), "m", term(1.0, 1.0, metre));
        let cm = |n| quantity(Magnitude::Integer(n), "cm", term(1.0, 100.0, metre));
        let km = |n| quantity(Magnitude::Integer(n), "km", term(1000.0, 1.0, metre));
        let huge = (1 << 53) + 1;
        let cases = [
            (m(1), cm(100), true),
            (
                m(1),
                quantity(Magnitude::Real(100.0), "cm", term(0.01, 1.0, metre)),
                true,
            ),
            (
                quantity(Magnitude::Real(1.0), "m", term(1.0, 1.0, metre)),
                cm(100),
                true,
            ),
            (m(1), cm(1), false),
            (m(1000), km(1), true),
            (m(1001), km(1), false),
            (m(1000 * huge), km(huge), true),
            (m(1000 * huge + 1), km(huge), false),
            (
                m(1),
                quantity(
                    Magnitude::Integer(1),
                    "s",
                    term(1.0, 1.0, &[("SI::second", 1.0)]),
                ),
                false,
            ),
            (
                quantity(Magnitude::Real(5.4), "km/h", term(1000.0, 3600.0, speed)),
                quantity(
                    Magnitude::Real(1.5),
                    "m/s",
                    term(1.0, 1.0, &[("SI::second", -1.0), ("SI::metre", 1.0)]),
                ),
                true,
            ),
            (
                quantity(Magnitude::Integer(36), "km/h", term(1000.0, 3600.0, speed)),
                quantity(Magnitude::Integer(10), "m/s", term(1.0, 1.0, speed)),
                true,
            ),
            (
                quantity(Magnitude::Integer(36), "km/h", term(1000.0, 3600.0, speed)),
                quantity(Magnitude::Integer(11), "m/s", term(1.0, 1.0, speed)),
                false,
            ),
            (
                m(1),
                quantity(
                    Magnitude::Integer(1),
                    "m·s/s",
                    term(
                        1.0,
                        1.0,
                        &[
                            ("SI::metre", 1.0),
                            ("SI::second", -1.0),
                            ("SI::second", 1.0),
                        ],
                    ),
                ),
                true,
            ),
            (
                m(0),
                quantity(Magnitude::Integer(0), "x", term(0.0, 1.0, metre)),
                false,
            ),
            (
                quantity(Magnitude::Integer(0), "x", term(0.0, 1.0, metre)),
                quantity(Magnitude::Integer(0), "x", term(0.0, 1.0, metre)),
                false,
            ),
            // Without a reduction, the unit as written is all there is to compare.
            (
                quantity(Magnitude::Integer(1), "m", None),
                quantity(Magnitude::Real(1.0), "m", None),
                true,
            ),
            (
                quantity(Magnitude::Integer(1), "m", None),
                quantity(Magnitude::Integer(100), "cm", None),
                false,
            ),
            (
                quantity(Magnitude::Integer(1), "m", None),
                quantity(Magnitude::Integer(1), "s", None),
                false,
            ),
            (quantity(Magnitude::Integer(1), "m", None), m(1), false),
            (
                Value::Set(Set::new(vec![m(1), km(2)]).unwrap()),
                Value::Set(Set::new(vec![m(2000), cm(100)]).unwrap()),
                true,
            ),
            (
                Value::Set(Set::new(vec![m(1), km(2)]).unwrap()),
                Value::Set(Set::new(vec![m(2000), cm(1)]).unwrap()),
                false,
            ),
        ];
        for (a, b, want) in cases {
            assert_eq!(a.same_value(&b), want, "{a:?} vs {b:?}");
            assert_eq!(b.same_value(&a), want, "{b:?} vs {a:?}");
        }

        let component = |v| match v {
            Value::Quantity(q) => q,
            other => panic!("not a quantity: {other:?}"),
        };
        let lengths = Set::new(vec![m(1), m(2)]).unwrap();
        assert!(lengths.contains(&cm(100)));
        assert!(!lengths.contains(&cm(1)));
        assert!(Set::new(vec![m(1), cm(100)]).is_err());
        assert_eq!(Set::new(vec![m(1), cm(1)]).unwrap().len(), 2);
        assert!(Value::VectorQuantity(
            VectorQuantity::new(vec![component(m(1)), component(km(1))]).unwrap()
        )
        .same_value(&Value::VectorQuantity(
            VectorQuantity::new(vec![component(cm(100)), component(m(1000))]).unwrap()
        )));
        assert!(Value::TensorQuantity(
            TensorQuantity::new(vec![1, 1], vec![component(m(1))]).unwrap()
        )
        .same_value(&Value::TensorQuantity(
            TensorQuantity::new(vec![1, 1], vec![component(cm(100))]).unwrap()
        )));
    }

    #[test]
    fn same_value_judges_measurement_refs_by_reduction() {
        let reference =
            |unit: &str, unit_id: Option<&str>, scale_num, scale_den, factors: &[(&str, f64)]| {
                Value::MeasurementRef(MeasurementRef {
                    unit: unit.to_owned(),
                    unit_id: unit_id.map(str::to_owned),
                    unit_term: UnitTerm {
                        scale_num,
                        scale_den,
                        factors: factors
                            .iter()
                            .map(|(unit_id, exponent)| UnitFactor {
                                unit_id: (*unit_id).to_owned(),
                                exponent: *exponent,
                            })
                            .collect(),
                    },
                })
            };
        let metre = &[("SI::metre", 1.0)][..];
        let ratio_factors = &[("SI::metre", 1.0), ("SI::metre", -1.0)][..];
        let named_speed = || {
            reference(
                "SI::'m/s'",
                Some("SI::'m/s'"),
                1.0,
                1.0,
                &[("SI::metre", 1.0), ("SI::second", -1.0)],
            )
        };
        let composed_speed = || {
            reference(
                "m / s",
                None,
                1.0,
                1.0,
                &[("SI::second", -1.0), ("SI::metre", 1.0)],
            )
        };
        let km = || reference("km", Some("SI::kilometre"), 1000.0, 1.0, metre);
        let km_alias = || reference("km", Some("SI::km"), 2000.0, 2.0, metre);
        let rad = || reference("rad", Some("SI::radian"), 1.0, 1.0, &[]);
        let sr = || reference("sr", Some("SI::steradian"), 1.0, 1.0, &[]);
        let ratio = || reference("m / m", None, 1.0, 1.0, ratio_factors);
        let set = |members| Value::Set(Set::new(members).unwrap());
        let cases = [
            (named_speed(), composed_speed(), true),
            (km(), km_alias(), true),
            (
                reference("km/m", None, 1000.0, 1.0, &[]),
                reference("m/mm", None, 1.0, 0.001, &[]),
                true,
            ),
            (
                km(),
                reference("m", Some("SI::metre"), 1.0, 1.0, metre),
                false,
            ),
            (
                reference("m", Some("SI::metre"), 1.0, 1.0, metre),
                reference("s", Some("SI::second"), 1.0, 1.0, &[("SI::second", 1.0)]),
                false,
            ),
            (
                reference("x", None, 0.0, 1.0, metre),
                reference("x", None, 0.0, 1.0, metre),
                false,
            ),
            (rad(), sr(), false),
            (
                rad(),
                reference("SI::rad", Some("SI::radian"), 1.0, 1.0, ratio_factors),
                true,
            ),
            (rad(), ratio(), false),
            (ratio(), reference("", None, 1.0, 1.0, &[]), true),
            (
                set(vec![named_speed(), rad()]),
                set(vec![rad(), composed_speed()]),
                true,
            ),
            (
                set(vec![named_speed(), rad()]),
                set(vec![sr(), composed_speed()]),
                false,
            ),
        ];
        for (a, b, want) in cases {
            assert_eq!(a.same_value(&b), want, "{a:?} vs {b:?}");
            assert_eq!(b.same_value(&a), want, "{b:?} vs {a:?}");
        }

        assert!(Set::new(vec![named_speed()])
            .unwrap()
            .contains(&composed_speed()));
        assert!(!Set::new(vec![rad()]).unwrap().contains(&sr()));
        assert!(Set::new(vec![named_speed(), composed_speed()]).is_err());
        assert!(Set::new(vec![km(), km_alias()]).is_err());
        assert_eq!(Set::new(vec![rad(), sr()]).unwrap().len(), 2);
    }

    #[test]
    fn same_value_judges_enum_literals_by_literal_id() {
        let literal = |literal_id: &str, enumeration_id: &str, name: &str| {
            Value::EnumLiteral(EnumLiteral {
                literal_id: literal_id.to_owned(),
                enumeration_id: enumeration_id.to_owned(),
                name: name.to_owned(),
            })
        };
        let red = || literal("D::Color::red", "D::Color", "Color::red");
        let same = || literal("D::Color::red", "E::Palette", "red");
        let green = || literal("D::Color::green", "D::Color", "Color::red");
        assert!(red().same_value(&same()));
        assert!(!red().same_value(&green()));
        assert!(Set::new(vec![red()])
            .unwrap()
            .contains(&literal("D::Color::red", "", "")));
        assert!(
            Value::Set(Set::new(vec![red(), green()]).unwrap()).same_value(&Value::Set(
                Set::new(vec![literal("D::Color::green", "", ""), same()]).unwrap()
            ))
        );
        assert!(Set::new(vec![red(), same()]).is_err());
        assert_eq!(Set::new(vec![red(), green()]).unwrap().len(), 2);
    }

    #[test]
    fn a_tensor_quantity_keeps_its_rank_shape_and_row_major_components() {
        let Ok(Value::TensorQuantity(cube)) = value_from_wire(tensor(
            vec![2, 2, 2],
            (1..=8).map(|i| metres(f64::from(i))).collect(),
        )) else {
            panic!("a (2, 2, 2) tensor should decode");
        };
        assert_eq!(cube.rank(), 3);
        assert_eq!(cube.dimensions(), [2, 2, 2]);
        assert_eq!(cube.components().len(), 8);
        assert_eq!(cube.unit(), Some("m"));
        assert_eq!(
            cube.get(&[1, 0, 1]).map(|q| q.magnitude),
            Some(Magnitude::Real(6.0))
        );
        assert_eq!(
            cube.get(&[0, 0, 0]).map(|q| q.magnitude),
            Some(Magnitude::Real(1.0))
        );
        assert_eq!(
            cube.get(&[1, 1, 1]).map(|q| q.magnitude),
            Some(Magnitude::Real(8.0))
        );
        assert_eq!(cube.get(&[1, 1]), None);
        assert_eq!(cube.get(&[1, 1, 1, 0]), None);
        assert_eq!(cube.get(&[2, 0, 0]), None);
        assert_eq!(cube.get(&[0, -1, 0]), None);

        // A rank-one tensor stays a tensor, never a vector quantity.
        let Ok(Value::TensorQuantity(line)) =
            value_from_wire(tensor(vec![2], vec![metres(1.0), metres(2.0)]))
        else {
            panic!("a rank-1 tensor should decode");
        };
        assert_eq!(line.rank(), 1);
        assert_ne!(
            Value::TensorQuantity(line),
            Value::VectorQuantity(
                VectorQuantity::new(vec![
                    Quantity {
                        magnitude: Magnitude::Real(1.0),
                        unit: "m".to_owned(),
                        unit_term: Some(metre_term()),
                    },
                    Quantity {
                        magnitude: Magnitude::Real(2.0),
                        unit: "m".to_owned(),
                        unit_term: Some(metre_term()),
                    },
                ])
                .expect("two components")
            )
        );

        // Components with differing units report no shared one.
        let speed = wire::Quantity {
            magnitude: Some(wire::quantity::Magnitude::IntMagnitude(5)),
            unit: "m/s".to_owned(),
            unit_term: None,
        };
        let Ok(Value::TensorQuantity(mixed)) =
            value_from_wire(tensor(vec![1, 2], vec![metres(1.0), speed]))
        else {
            panic!("a mixed tensor should decode");
        };
        assert_eq!(mixed.unit(), None);
        assert_eq!(
            mixed.get(&[0, 1]).map(|q| q.magnitude),
            Some(Magnitude::Integer(5))
        );
    }

    #[test]
    fn a_malformed_tensor_quantity_is_refused() {
        let short = value_from_wire(tensor(
            vec![2, 2],
            vec![metres(1.0), metres(2.0), metres(3.0)],
        ));
        assert!(
            matches!(&short, Err(Error::Decode(message)) if message.contains("want 4")),
            "{short:?}"
        );
        assert!(matches!(
            value_from_wire(tensor(vec![], vec![metres(1.0), metres(2.0)])),
            Err(Error::Decode(message)) if message.contains("want 1")
        ));
        assert!(matches!(
            value_from_wire(tensor(vec![0], vec![])),
            Err(Error::Decode(message)) if message.contains("not positive")
        ));
        assert!(matches!(
            value_from_wire(tensor(vec![-1], vec![metres(1.0)])),
            Err(Error::Decode(message)) if message.contains("not positive")
        ));
        assert!(matches!(
            value_from_wire(tensor(vec![i64::MAX, 2], vec![])),
            Err(Error::Decode(message)) if message.contains("overflow")
        ));
        assert!(matches!(
            value_from_wire(tensor(vec![1], vec![wire::Quantity::default()])),
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

    fn function(calc_id: &str, self_id: i64) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::Function(wire::Function {
                calc_id: calc_id.to_owned(),
                self_id,
            })),
        }
    }

    #[test]
    fn a_function_is_the_calc_it_names_read_against_an_object_or_none() {
        assert_eq!(
            value_from_wire(function("Demo::Sq", 0)).ok(),
            Some(Value::Function(Function {
                calc_id: "Demo::Sq".to_owned(),
                self_id: None,
            }))
        );
        assert_eq!(
            value_from_wire(function("Demo::Scaler::scale", 7)).ok(),
            Some(Value::Function(Function {
                calc_id: "Demo::Scaler::scale".to_owned(),
                self_id: Some(7),
            }))
        );

        // Naming no calc is malformed at any depth.
        assert!(matches!(
            value_from_wire(function("", 0)),
            Err(Error::Decode(message)) if message.contains("names no calc")
        ));
        let nested = wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![function("", 3)],
            })),
        };
        assert!(matches!(
            value_from_wire(nested),
            Err(Error::Decode(message)) if message.contains("names no calc")
        ));
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
