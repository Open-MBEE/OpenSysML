//! Request-side values: a [`Value`] as the wire carries it, checked against what the service reads.

use crate::capabilities::{
    upgrade_remedy, CAPABILITY_BIG_INT_VALUES, CAPABILITY_COMPLEX_VALUES, CAPABILITY_ENUM_VALUES,
    CAPABILITY_FUNCTION_VALUES, CAPABILITY_INFINITY_VALUE, CAPABILITY_MEASUREMENT_REFS,
    CAPABILITY_METAOBJECT_VALUES, CAPABILITY_SET_VALUES, CAPABILITY_STRUCTURED_VALUES,
    CAPABILITY_TENSOR_VALUES,
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

/// Refuse an Integer beyond int64 to a service that would read its arm as null.
fn require_magnitudes<'a>(
    capabilities: &Capabilities,
    mut magnitudes: impl Iterator<Item = &'a Magnitude>,
) -> Result<(), Error> {
    if magnitudes.any(|magnitude| matches!(magnitude, Magnitude::BigInteger(_))) {
        require(capabilities, CAPABILITY_BIG_INT_VALUES)?;
    }
    Ok(())
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
        Value::BigInteger(v) => {
            require(capabilities, CAPABILITY_BIG_INT_VALUES)?;
            kind(Kind::BigIntValue(v.as_str().to_owned()))
        }
        Value::Real(v) => kind(Kind::RealValue(*v)),
        Value::Boolean(v) => kind(Kind::BoolValue(*v)),
        Value::Text(v) => kind(Kind::StringValue(v.clone())),
        Value::InstanceRef(v) => kind(Kind::InstanceId(*v)),
        Value::Null => kind(Kind::Null(String::new())),
        Value::Sequence(elements) => kind(Kind::Sequence(wire::ValueSequence {
            elements: elements.iter().map(nested).collect::<Result<_, _>>()?,
        })),
        Value::Quantity(q) => {
            require_magnitudes(capabilities, std::iter::once(&q.magnitude))?;
            kind(Kind::Quantity(quantity_to_wire(q)?))
        }
        Value::EnumLiteral(literal) => {
            require(capabilities, CAPABILITY_ENUM_VALUES)?;
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
            require_magnitudes(capabilities, vector.components.iter())?;
            kind(Kind::Vector(wire::Vector {
                components: vector
                    .components
                    .iter()
                    .map(|component| {
                        kind(match component {
                            Magnitude::Integer(v) => Kind::IntValue(*v),
                            Magnitude::BigInteger(v) => Kind::BigIntValue(v.as_str().to_owned()),
                            Magnitude::Real(v) => Kind::RealValue(*v),
                        })
                    })
                    .collect(),
            }))
        }
        Value::VectorQuantity(vector) => {
            require(capabilities, CAPABILITY_STRUCTURED_VALUES)?;
            require_magnitudes(
                capabilities,
                vector.components().iter().map(|q| &q.magnitude),
            )?;
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
            require_magnitudes(
                capabilities,
                tensor.components().iter().map(|q| &q.magnitude),
            )?;
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
        magnitude: Some(match &q.magnitude {
            Magnitude::Integer(v) => wire::quantity::Magnitude::IntMagnitude(*v),
            Magnitude::BigInteger(v) => {
                wire::quantity::Magnitude::BigIntMagnitude(v.as_str().to_owned())
            }
            Magnitude::Real(v) => wire::quantity::Magnitude::RealMagnitude(*v),
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::capabilities::CAPABILITY_COMPLEX_VALUES;
    use crate::domain::{
        Array, BigInteger, Complex, EnumLiteral, Function, MeasurementRef, Metaobject, Set,
        TensorQuantity, Undetermined, UnitFactor, Vector, VectorQuantity,
    };
    use wire::value::Kind;

    const ALL: &[&str] = &[
        CAPABILITY_ENUM_VALUES,
        CAPABILITY_COMPLEX_VALUES,
        CAPABILITY_STRUCTURED_VALUES,
        CAPABILITY_MEASUREMENT_REFS,
        CAPABILITY_FUNCTION_VALUES,
        CAPABILITY_SET_VALUES,
        CAPABILITY_TENSOR_VALUES,
        CAPABILITY_METAOBJECT_VALUES,
        CAPABILITY_INFINITY_VALUE,
        CAPABILITY_BIG_INT_VALUES,
    ];

    fn capabilities(names: &[&str]) -> Capabilities {
        Capabilities::new(wire::ServerInfoResponse {
            capabilities: names.iter().map(|name| (*name).to_owned()).collect(),
            ..Default::default()
        })
    }

    fn metres(magnitude: Magnitude) -> Quantity {
        Quantity {
            magnitude,
            unit: "m".to_owned(),
            unit_term: Some(UnitTerm {
                scale_num: 1.0,
                scale_den: 1.0,
                factors: vec![UnitFactor {
                    unit_id: "SI::m".to_owned(),
                    exponent: 1.0,
                }],
            }),
        }
    }

    fn sent(value: &Value) -> Kind {
        value_to_wire(value, &capabilities(ALL))
            .unwrap_or_else(|error| panic!("{value:?} was refused: {error}"))
            .kind
            .expect("an encoded value has a kind")
    }

    fn refused(value: &Value) -> String {
        match value_to_wire(value, &capabilities(ALL)) {
            Err(Error::UnsupportedValue(message)) => message,
            other => panic!("{value:?} should be refused, got {other:?}"),
        }
    }

    #[test]
    fn every_sendable_value_round_trips_through_the_wire() {
        let values = vec![
            Value::Integer(3),
            Value::Real(2.5),
            Value::Boolean(true),
            Value::Text("x".to_owned()),
            Value::InstanceRef(7),
            Value::Null,
            Value::Infinity,
            Value::Complex(Complex {
                real: 1.0,
                imaginary: -2.0,
            }),
            Value::Quantity(metres(Magnitude::Integer(4))),
            Value::Quantity(metres(Magnitude::Real(4.5))),
            Value::Sequence(vec![
                Value::Integer(1),
                Value::Sequence(vec![Value::Real(2.0)]),
            ]),
            Value::Array(
                Array::new(
                    vec![2, 2],
                    vec![
                        Value::Integer(1),
                        Value::Integer(2),
                        Value::Integer(3),
                        Value::Integer(4),
                    ],
                )
                .unwrap(),
            ),
            Value::Vector(Vector {
                components: vec![Magnitude::Integer(1), Magnitude::Real(2.0)],
            }),
            Value::VectorQuantity(
                VectorQuantity::new(vec![
                    metres(Magnitude::Real(1.0)),
                    metres(Magnitude::Real(2.0)),
                ])
                .unwrap(),
            ),
            Value::MeasurementRef(MeasurementRef {
                unit: "m".to_owned(),
                unit_term: metres(Magnitude::Integer(1)).unit_term.unwrap(),
                unit_id: Some("SI::m".to_owned()),
            }),
            Value::Function(Function {
                calc_id: "P::f".to_owned(),
                self_id: Some(4),
            }),
            Value::Set(Set::new(vec![Value::Integer(1), Value::Text("a".to_owned())]).unwrap()),
            Value::TensorQuantity(
                TensorQuantity::new(
                    vec![1, 2],
                    vec![metres(Magnitude::Real(1.0)), metres(Magnitude::Real(2.0))],
                )
                .unwrap(),
            ),
            Value::Metaobject(Metaobject {
                element_id: "P::a".to_owned(),
                metaclass_id: "SysML::PartUsage".to_owned(),
            }),
            Value::EnumLiteral(EnumLiteral {
                literal_id: "P::Color::red".to_owned(),
                enumeration_id: "P::Color".to_owned(),
                name: "red".to_owned(),
                value: Some(Box::new(Value::Integer(0))),
            }),
        ];
        for value in values {
            let wire = value_to_wire(&value, &capabilities(ALL)).unwrap();
            let back = Value::try_from(wire).unwrap();
            assert!(back.same_value(&value), "{value:?} came back as {back:?}");
        }
    }

    #[test]
    fn a_quantity_keeps_an_integer_magnitude_integral() {
        let Kind::Quantity(quantity) = sent(&Value::Quantity(metres(Magnitude::Integer(4)))) else {
            panic!("not a quantity");
        };
        assert_eq!(
            quantity.magnitude,
            Some(wire::quantity::Magnitude::IntMagnitude(4))
        );
    }

    #[test]
    fn a_dimensionless_quantity_reduces_to_one_and_a_unit_without_reduction_is_refused() {
        let Kind::Quantity(quantity) = sent(&Value::Quantity(Quantity {
            magnitude: Magnitude::Real(2.0),
            unit: String::new(),
            unit_term: None,
        })) else {
            panic!("not a quantity");
        };
        let term = quantity.unit_term.unwrap();
        assert_eq!((term.scale_num, term.scale_den), (1.0, 1.0));
        assert!(refused(&Value::Quantity(Quantity {
            magnitude: Magnitude::Real(2.0),
            unit: "km".to_owned(),
            unit_term: None,
        }))
        .contains("km"));
    }

    #[test]
    fn a_scale_that_reduces_to_nothing_is_refused() {
        let mut quantity = metres(Magnitude::Real(1.0));
        quantity.unit_term.as_mut().unwrap().scale_den = 0.0;
        refused(&Value::Quantity(quantity));
        refused(&Value::MeasurementRef(MeasurementRef {
            unit: "m".to_owned(),
            unit_term: UnitTerm {
                scale_num: f64::NAN,
                scale_den: 1.0,
                factors: Vec::new(),
            },
            unit_id: None,
        }));
    }

    #[test]
    fn an_absent_closure_object_is_sent_as_zero_and_object_zero_is_refused() {
        let Kind::Function(function) = sent(&Value::Function(Function {
            calc_id: "P::f".to_owned(),
            self_id: None,
        })) else {
            panic!("not a function");
        };
        assert_eq!(function.self_id, 0);
        refused(&Value::Function(Function {
            calc_id: "P::f".to_owned(),
            self_id: Some(0),
        }));
    }

    #[test]
    fn values_naming_nothing_are_refused() {
        refused(&Value::Function(Function {
            calc_id: String::new(),
            self_id: None,
        }));
        refused(&Value::Metaobject(Metaobject {
            element_id: String::new(),
            metaclass_id: "SysML::PartUsage".to_owned(),
        }));
        refused(&Value::EnumLiteral(EnumLiteral {
            literal_id: String::new(),
            enumeration_id: "P::Color".to_owned(),
            name: "red".to_owned(),
            value: None,
        }));
    }

    #[test]
    fn an_empty_vector_is_sent() {
        assert_eq!(
            sent(&Value::Vector(Vector {
                components: Vec::new()
            })),
            Kind::Vector(wire::Vector {
                components: Vec::new()
            })
        );
    }

    #[test]
    fn infinity_is_sent_asserted() {
        assert_eq!(sent(&Value::Infinity), Kind::Infinity(true));
    }

    #[test]
    fn unset_and_undetermined_are_answers_not_arguments() {
        assert!(refused(&Value::Unset).contains("unset"));
        refused(&Value::Undetermined(Undetermined {
            reason: "open".to_owned(),
            count_lower: "1".to_owned(),
            count_upper: "1".to_owned(),
        }));
    }

    #[test]
    fn an_enum_literal_needs_enum_values() {
        let literal = Value::EnumLiteral(EnumLiteral {
            literal_id: "P::Color::red".to_owned(),
            enumeration_id: "P::Color".to_owned(),
            name: "red".to_owned(),
            value: None,
        });
        match value_to_wire(&literal, &capabilities(&[])) {
            Err(Error::MissingCapability { capability, .. }) => {
                assert_eq!(capability, CAPABILITY_ENUM_VALUES)
            }
            other => panic!("expected a missing capability, got {other:?}"),
        }
    }

    #[test]
    fn a_nested_value_needs_the_capability_of_its_innermost_kind() {
        let nested = Value::Sequence(vec![Value::Sequence(vec![Value::Complex(Complex {
            real: 0.0,
            imaginary: 1.0,
        })])]);
        match value_to_wire(&nested, &capabilities(&[])) {
            Err(Error::MissingCapability { capability, remedy }) => {
                assert_eq!(capability, CAPABILITY_COMPLEX_VALUES);
                assert!(!remedy.is_empty());
            }
            other => panic!("expected a missing capability, got {other:?}"),
        }
    }

    #[test]
    fn every_gated_kind_names_its_capability() {
        let cases = [
            (
                Value::Array(Array::new(vec![1], vec![Value::Integer(1)]).unwrap()),
                CAPABILITY_STRUCTURED_VALUES,
            ),
            (
                Value::Set(Set::new(vec![Value::Integer(1)]).unwrap()),
                CAPABILITY_SET_VALUES,
            ),
            (
                Value::Function(Function {
                    calc_id: "P::f".to_owned(),
                    self_id: None,
                }),
                CAPABILITY_FUNCTION_VALUES,
            ),
            (
                Value::Metaobject(Metaobject {
                    element_id: "P::a".to_owned(),
                    metaclass_id: String::new(),
                }),
                CAPABILITY_METAOBJECT_VALUES,
            ),
            (Value::Infinity, CAPABILITY_INFINITY_VALUE),
        ];
        for (value, expected) in cases {
            match value_to_wire(&value, &capabilities(&[])) {
                Err(Error::MissingCapability { capability, .. }) => {
                    assert_eq!(capability, expected)
                }
                other => panic!("{value:?}: expected {expected}, got {other:?}"),
            }
        }
    }

    #[test]
    fn an_integer_beyond_int64_needs_big_int_values_bare_or_nested() {
        let wide = BigInteger::parse("1180591620717411303424").unwrap();
        let quantity = metres(Magnitude::BigInteger(wide.clone()));
        let without: Vec<&str> = ALL
            .iter()
            .copied()
            .filter(|name| *name != CAPABILITY_BIG_INT_VALUES)
            .collect();
        for value in [
            Value::BigInteger(wide.clone()),
            Value::Sequence(vec![
                Value::Integer(1),
                Value::Sequence(vec![Value::BigInteger(wide.clone())]),
            ]),
            Value::Set(Set::new(vec![Value::BigInteger(wide.clone())]).unwrap()),
            Value::Quantity(quantity.clone()),
            Value::Vector(Vector {
                components: vec![Magnitude::Integer(1), Magnitude::BigInteger(wide.clone())],
            }),
            Value::VectorQuantity(VectorQuantity::new(vec![quantity.clone()]).unwrap()),
            Value::TensorQuantity(TensorQuantity::new(vec![1], vec![quantity.clone()]).unwrap()),
        ] {
            match value_to_wire(&value, &capabilities(&without)) {
                Err(Error::MissingCapability { capability, .. }) => {
                    assert_eq!(capability, CAPABILITY_BIG_INT_VALUES, "{value:?}")
                }
                other => panic!("{value:?}: expected big_int_values, got {other:?}"),
            }
            assert!(
                value_to_wire(&value, &capabilities(ALL)).is_ok(),
                "{value:?}"
            );
        }
        assert_eq!(
            value_to_wire(&Value::Integer(i64::MAX), &capabilities(&without))
                .unwrap()
                .kind,
            Some(Kind::IntValue(i64::MAX))
        );
    }

    #[test]
    fn a_value_nested_too_deep_is_refused_rather_than_overflowing() {
        let mut value = Value::Integer(1);
        for _ in 0..=MAX_DEPTH + 1 {
            value = Value::Sequence(vec![value]);
        }
        assert!(refused(&value).contains("nests"));
    }
}
