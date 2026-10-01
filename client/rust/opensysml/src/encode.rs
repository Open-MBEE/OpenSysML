//! Request-side values: a [`Value`] as the wire carries it, checked against what the service reads.

use crate::capabilities::{
    upgrade_remedy, CAPABILITY_COMPLEX_VALUES, CAPABILITY_FUNCTION_VALUES,
    CAPABILITY_INFINITY_VALUE, CAPABILITY_MEASUREMENT_REFS, CAPABILITY_METAOBJECT_VALUES,
    CAPABILITY_SET_VALUES, CAPABILITY_STRUCTURED_VALUES, CAPABILITY_TENSOR_VALUES,
};
use crate::domain::{Capabilities, Magnitude, Quantity, UnitTerm, Value};
use crate::error::Error;
use crate::wire;

/// Deepest nesting a sent value may have, so a cyclic-looking builder cannot exhaust the stack.
const MAX_DEPTH: usize = 128;

/// Encode `value` for a request, refusing one the service would misread or not read at all.
pub(crate) fn value_to_wire(
    value: &Value,
    capabilities: &Capabilities,
) -> Result<wire::Value, Error> {
    encode(value, capabilities, 0)
}

fn kind(kind: wire::value::Kind) -> wire::Value {
    wire::Value { kind: Some(kind) }
}

fn require(capabilities: &Capabilities, capability: &str) -> Result<(), Error> {
    capabilities.require(capability, upgrade_remedy(capability))
}

fn encode(value: &Value, capabilities: &Capabilities, depth: usize) -> Result<wire::Value, Error> {
    use wire::value::Kind;
    if depth > MAX_DEPTH {
        return Err(Error::UnsupportedValue(format!(
            "value nests deeper than {MAX_DEPTH} levels"
        )));
    }
    let nested = |item: &Value| encode(item, capabilities, depth + 1);
    Ok(match value {
        Value::Integer(v) => kind(Kind::IntValue(*v)),
        Value::Real(v) => kind(Kind::RealValue(*v)),
        Value::Boolean(v) => kind(Kind::BoolValue(*v)),
        Value::Text(v) => kind(Kind::StringValue(v.clone())),
        Value::InstanceRef(v) => kind(Kind::InstanceId(*v)),
        Value::Null => kind(Kind::Null(String::new())),
        Value::Sequence(elements) => kind(Kind::Sequence(wire::ValueSequence {
            elements: elements.iter().map(nested).collect::<Result<_, _>>()?,
        })),
        Value::Quantity(q) => kind(Kind::Quantity(quantity_to_wire(q)?)),
        Value::EnumLiteral(literal) => {
            if literal.literal_id.is_empty() {
                return Err(Error::UnsupportedValue(
                    "enumeration literal naming no literal".to_owned(),
                ));
            }
            kind(Kind::EnumLiteral(Box::new(wire::EnumLiteral {
                literal_id: literal.literal_id.clone(),
                enumeration_id: literal.enumeration_id.clone(),
                name: literal.name.clone(),
                value: match &literal.value {
                    Some(scalar) => Some(Box::new(nested(scalar)?)),
                    None => None,
                },
            })))
        }
        Value::Complex(c) => {
            require(capabilities, CAPABILITY_COMPLEX_VALUES)?;
            kind(Kind::Complex(wire::Complex {
                real: c.real,
                imaginary: c.imaginary,
            }))
        }
        Value::Array(array) => {
            require(capabilities, CAPABILITY_STRUCTURED_VALUES)?;
            kind(Kind::Array(wire::Array {
                dimensions: array.dimensions().to_vec(),
                elements: array
                    .elements()
                    .iter()
                    .map(nested)
                    .collect::<Result<_, _>>()?,
            }))
        }
        Value::Vector(vector) => {
            require(capabilities, CAPABILITY_STRUCTURED_VALUES)?;
            if vector.components.is_empty() {
                return Err(Error::UnsupportedValue(
                    "vector has no components".to_owned(),
                ));
            }
            kind(Kind::Vector(wire::Vector {
                components: vector
                    .components
                    .iter()
                    .map(|component| {
                        kind(match *component {
                            Magnitude::Integer(v) => Kind::IntValue(v),
                            Magnitude::Real(v) => Kind::RealValue(v),
                        })
                    })
                    .collect(),
            }))
        }
        Value::VectorQuantity(vector) => {
            require(capabilities, CAPABILITY_STRUCTURED_VALUES)?;
            kind(Kind::VectorQuantity(wire::VectorQuantity {
                components: vector
                    .components()
                    .iter()
                    .map(quantity_to_wire)
                    .collect::<Result<_, _>>()?,
            }))
        }
        Value::MeasurementRef(reference) => {
            require(capabilities, CAPABILITY_MEASUREMENT_REFS)?;
            check_scale(&reference.unit_term, || {
                format!("measurement reference {}", unit_label(&reference.unit))
            })?;
            kind(Kind::MeasurementRef(wire::MeasurementRef {
                unit: reference.unit.clone(),
                unit_term: Some(unit_term_to_wire(&reference.unit_term)),
                unit_id: reference.unit_id.clone().unwrap_or_default(),
            }))
        }
        Value::Function(function) => {
            require(capabilities, CAPABILITY_FUNCTION_VALUES)?;
            if function.calc_id.is_empty() {
                return Err(Error::UnsupportedValue(
                    "function naming no calc".to_owned(),
                ));
            }
            if function.self_id == Some(0) {
                return Err(Error::UnsupportedValue(
                    "function closing over object 0, which the wire reads as no object".to_owned(),
                ));
            }
            kind(Kind::Function(wire::Function {
                calc_id: function.calc_id.clone(),
                self_id: function.self_id.unwrap_or(0),
            }))
        }
        Value::Set(set) => {
            require(capabilities, CAPABILITY_SET_VALUES)?;
            kind(Kind::Set(wire::ValueSet {
                elements: set
                    .elements()
                    .iter()
                    .map(nested)
                    .collect::<Result<_, _>>()?,
            }))
        }
        Value::TensorQuantity(tensor) => {
            require(capabilities, CAPABILITY_TENSOR_VALUES)?;
            kind(Kind::TensorQuantity(wire::TensorQuantity {
                dimensions: tensor.dimensions().to_vec(),
                components: tensor
                    .components()
                    .iter()
                    .map(quantity_to_wire)
                    .collect::<Result<_, _>>()?,
            }))
        }
        Value::Metaobject(metaobject) => {
            require(capabilities, CAPABILITY_METAOBJECT_VALUES)?;
            if metaobject.element_id.is_empty() {
                return Err(Error::UnsupportedValue(
                    "metaobject naming no element".to_owned(),
                ));
            }
            kind(Kind::Metaobject(wire::Metaobject {
                element_id: metaobject.element_id.clone(),
                metaclass_id: metaobject.metaclass_id.clone(),
            }))
        }
        Value::Infinity => {
            require(capabilities, CAPABILITY_INFINITY_VALUE)?;
            kind(Kind::Infinity(true))
        }
        Value::Unset => {
            return Err(Error::UnsupportedValue(
                "unset is what a feature with no value reads as, not an argument: \
                 leave the argument out instead"
                    .to_owned(),
            ))
        }
        Value::Undetermined(_) => {
            return Err(Error::UnsupportedValue(
                "undetermined is an answer the model leaves open, not an argument".to_owned(),
            ))
        }
    })
}

fn unit_label(unit: &str) -> &str {
    if unit.is_empty() {
        "of dimension one"
    } else {
        unit
    }
}

fn check_scale(term: &UnitTerm, what: impl Fn() -> String) -> Result<(), Error> {
    if !term.scale_num.is_finite() || !term.scale_den.is_finite() || term.scale_den == 0.0 {
        return Err(Error::UnsupportedValue(format!(
            "{} has a scale of {}/{}, which reduces to no base unit",
            what(),
            term.scale_num,
            term.scale_den
        )));
    }
    Ok(())
}

pub(crate) fn quantity_to_wire(q: &Quantity) -> Result<wire::Quantity, Error> {
    let unit_term = match &q.unit_term {
        Some(term) => {
            check_scale(term, || format!("quantity unit {}", unit_label(&q.unit)))?;
            Some(unit_term_to_wire(term))
        }
        None if q.unit.is_empty() => Some(wire::UnitTerm {
            scale_num: 1.0,
            scale_den: 1.0,
            factors: Vec::new(),
        }),
        None => {
            return Err(Error::UnsupportedValue(format!(
                "quantity unit {} carries no reduction to base units, so the service cannot \
                 tell what it measures: build it from a unit the service sent, or give its unit_term",
                q.unit
            )))
        }
    };
    Ok(wire::Quantity {
        unit: q.unit.clone(),
        unit_term,
        magnitude: Some(match q.magnitude {
            Magnitude::Integer(v) => wire::quantity::Magnitude::IntMagnitude(v),
            Magnitude::Real(v) => wire::quantity::Magnitude::RealMagnitude(v),
        }),
    })
}

fn unit_term_to_wire(term: &UnitTerm) -> wire::UnitTerm {
    wire::UnitTerm {
        scale_num: term.scale_num,
        scale_den: term.scale_den,
        factors: term
            .factors
            .iter()
            .map(|factor| wire::UnitFactor {
                unit_id: factor.unit_id.clone(),
                exponent: factor.exponent,
            })
            .collect(),
    }
}
