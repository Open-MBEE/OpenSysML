#![allow(missing_docs)]

//! The client read over one capable model — quantities, enumerations,
//! multiplicity, nesting and a feature left without a value — so that every
//! value shape a response can carry is decoded by a test rather than by hand.

use std::env;

use opensysml::{Connection, EvalOptions, Magnitude, Model, Value};

/// The same model the `capable_model` example tours.
const MODEL: &str = r#"
package Vehicles {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;

    enum def Color {
        enum red;
        enum green;
        enum blue;
    }

    part def Engine {
        attribute mass : MassValue;
        attribute power : PowerValue;
    }

    part def Wheel {
        attribute diameter : LengthValue;
    }

    part def Car {
        attribute color : Color;
        attribute name : String;
        attribute street : Boolean;
        attribute mass : MassValue;
        attribute unpainted : Color;
        part engine : Engine;
        part wheels : Wheel[4];
    }

    part sedan : Car {
        attribute redefines color = Color::blue;
        attribute redefines name = "Sedan";
        attribute redefines street = true;
        attribute redefines mass = 1600.0 [kg];
        part redefines engine {
            attribute redefines mass = 180.0 [kg];
            attribute redefines power = 90000.0 [W];
        }
    }

    calc def Doubled { in x : Real; return : Real = x * 2.0; }
}
"#;

fn model_or_skip() -> Option<Model> {
    let connection = match Connection::private() {
        Ok(connection) => connection,
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("required sysml-grpc service unavailable: {error}");
            }
            eprintln!("skipping service-backed Rust client test: {error}");
            return None;
        }
    };
    let model = connection
        .parse_content(MODEL, &Default::default())
        .unwrap_or_else(|error| panic!("parse failed: {error}"));
    assert_eq!(
        model
            .diagnostics()
            .iter()
            .map(|diagnostic| diagnostic.wire().message.clone())
            .collect::<Vec<_>>(),
        Vec::<String>::new(),
        "the capable model is expected to parse clean"
    );
    Some(model)
}

#[test]
fn evaluation_decodes_every_scalar_shape() {
    let Some(model) = model_or_skip() else {
        return;
    };
    let eval = |expression: &str| {
        model
            .eval(expression)
            .unwrap_or_else(|error| panic!("evaluating {expression} failed: {error}"))
    };
    assert_eq!(eval("1 + 2 * 3"), Value::Integer(7));
    assert_eq!(
        eval("Vehicles::sedan::name"),
        Value::Text("Sedan".to_owned())
    );
    assert_eq!(eval("Vehicles::sedan::street"), Value::Boolean(true));
    assert_eq!(eval("Vehicles::Doubled(21.0)"), Value::Real(42.0));

    let Value::EnumLiteral(literal) = eval("Vehicles::sedan::color") else {
        panic!("color is an enumeration literal");
    };
    assert_eq!(literal.literal_id, "Vehicles::Color::blue");
    assert_eq!(literal.enumeration_id, "Vehicles::Color");
}

/// A value expression is written in its own scope, so a qualified name reads it
/// with the units and enumerations imported there — the same answer the scope
/// the name is written in would give.
#[test]
fn a_qualified_value_reads_its_declaring_scope() {
    let Some(model) = model_or_skip() else {
        return;
    };
    let qualified = model
        .eval("Vehicles::sedan::mass")
        .unwrap_or_else(|error| panic!("qualified evaluation failed: {error}"));
    let scoped = model
        .evaluate(
            "mass",
            &EvalOptions {
                context: Some("Vehicles::sedan".to_owned()),
                subject: None,
            },
        )
        .unwrap_or_else(|error| panic!("scoped evaluation failed: {error}"))
        .result;
    assert_eq!(qualified, scoped);

    let Value::Quantity(quantity) = qualified else {
        panic!("mass is a quantity");
    };
    assert_eq!(quantity.magnitude, Magnitude::Real(1600.0));
    assert_eq!(quantity.unit, "kg");
    let term = quantity.unit_term.expect("a kilogram reduces to grams");
    assert_eq!(term.scale_num, 1000.0);
    assert_eq!(
        term.factors
            .iter()
            .map(|factor| (factor.unit_id.as_str(), factor.exponent))
            .collect::<Vec<_>>(),
        [("SI::gram", 1.0)]
    );
}

#[test]
fn nested_symbols_are_navigated_lazily() {
    let Some(model) = model_or_skip() else {
        return;
    };
    let car = model
        .symbol("Vehicles::Car")
        .unwrap_or_else(|error| panic!("symbol lookup failed: {error}"));
    assert_eq!(car.kind(), "partDef");
    let children = car
        .children()
        .unwrap_or_else(|error| panic!("children lookup failed: {error}"));
    assert_eq!(
        children
            .iter()
            .map(|child| child.name().to_owned())
            .collect::<Vec<_>>(),
        [
            "color",
            "name",
            "street",
            "mass",
            "unpainted",
            "engine",
            "wheels"
        ]
    );
}

#[test]
fn instantiation_decodes_values_multiplicity_and_nesting() {
    let Some(model) = model_or_skip() else {
        return;
    };
    let instantiation = model
        .instantiate("Vehicles::sedan")
        .unwrap_or_else(|error| panic!("instantiation failed: {error}"));
    let sedan = &instantiation.instance;

    // A feature declaring no value is unset rather than absent or null.
    assert_eq!(
        sedan
            .feature("unpainted")
            .and_then(|feature| feature.value()),
        Some(&Value::Unset)
    );

    // A multi-valued feature holds values, not one value.
    let wheels = sedan.feature("wheels").expect("wheels is a feature");
    assert_eq!(wheels.value(), None);
    assert_eq!(wheels.values().len(), 4);
    assert!(wheels
        .values()
        .iter()
        .all(|value| matches!(value, Value::InstanceRef(_))));

    // A nested object is reachable through the instances the response carries.
    let Some(Value::InstanceRef(engine_id)) =
        sedan.feature("engine").and_then(|feature| feature.value())
    else {
        panic!("engine holds one object");
    };
    let engine = instantiation
        .instances()
        .iter()
        .find(|instance| instance.id() == *engine_id)
        .expect("the engine is among the reachable instances");
    let Some(Value::Quantity(power)) = engine.feature("power").and_then(|feature| feature.value())
    else {
        panic!("engine power is a quantity");
    };
    assert_eq!(power.magnitude, Magnitude::Real(90000.0));
    assert_eq!(power.unit, "W");
}

/// Reading a feature materializes the object it holds, so an instantiation that
/// depended on map order would hand out different ids each time it was asked.
/// The service keeps every object it builds, so each instantiation is a new
/// object with ids of its own; the graph is the same up to that offset.
#[test]
fn instantiation_is_the_same_graph_every_time() {
    let Some(model) = model_or_skip() else {
        return;
    };
    let graph = || {
        let instances = model
            .instantiate("Vehicles::sedan")
            .unwrap_or_else(|error| panic!("instantiation failed: {error}"));
        let root = instances.instance.id();
        instances
            .instances()
            .iter()
            .map(|instance| (instance.id() - root, instance.type_symbol_id().to_owned()))
            .collect::<Vec<_>>()
    };
    let first = graph();
    assert_eq!(first.len(), 6);
    assert_eq!(first[0].0, 0);
    for _ in 0..4 {
        assert_eq!(graph(), first);
    }
}

/// The service holds each object it builds, so a second instantiation of the
/// same part is a second object, its ids fresh.
#[test]
fn instantiating_again_builds_a_new_object() {
    let Some(model) = model_or_skip() else {
        return;
    };
    let ids = || {
        model
            .instantiate("Vehicles::sedan")
            .unwrap_or_else(|error| panic!("instantiation failed: {error}"))
            .instances()
            .iter()
            .map(|instance| instance.id())
            .collect::<Vec<_>>()
    };
    let first = ids();
    let second = ids();
    let highest = first.iter().copied().max().expect("an instance");
    assert!(
        second.iter().all(|id| *id > highest),
        "{second:?} after {first:?}"
    );
}
