//! A tour of the client over one capable model: parsing, diagnostics, symbol
//! navigation, evaluation and instantiation.
//!
//! ```text
//! cargo run --manifest-path clients/rust/Cargo.toml -p opensysml --example capable_model
//! ```

use std::error::Error;

use opensysml::{Connection, EvalOptions, Language, ParseOptions, Value};

/// A model with the shapes a client has to decode: quantities, enumerations,
/// multiplicity, nesting, a calculation, and a feature left without a value.
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

fn main() -> Result<(), Box<dyn Error>> {
    let connection = Connection::connect()?;
    println!(
        "sysml-grpc {} with {} capabilities",
        connection.server_info().wire().version,
        connection.capabilities().wire().capabilities.len()
    );

    let model = connection.parse_content(
        MODEL,
        &ParseOptions {
            language: Language::Sysml,
            strict_conformance: false,
        },
    )?;
    println!("model {}", model.hash());
    for diagnostic in model.diagnostics() {
        println!("  {}", diagnostic.wire().message);
    }

    // Symbols are navigated lazily: each level is a request of its own.
    let car = model.symbol("Vehicles::Car")?;
    println!("{} {}", car.kind(), car.name());
    for child in car.children()? {
        println!("  {} {}", child.kind(), child.name());
    }

    // Every expression is evaluated against the model, so a value declared in a
    // package reads the units and enumerations imported there.
    for expression in [
        "1 + 2 * 3",
        "Vehicles::sedan::name",
        "Vehicles::sedan::color",
        "Vehicles::sedan::mass",
        "Vehicles::sedan::engine::power",
        "Vehicles::Doubled(21.0)",
    ] {
        println!("{expression} => {:?}", model.eval(expression)?);
    }

    // A feature declaring no value has none, which is a model error rather than
    // a transport one.
    match model.eval("Vehicles::sedan::unpainted") {
        Ok(value) => println!("unpainted => {value:?}"),
        Err(error) => println!("unpainted => {error}"),
    }

    // An expression can also be evaluated in the scope of a symbol.
    let scoped = model.evaluate(
        "mass",
        &EvalOptions {
            context: Some("Vehicles::sedan".to_owned()),
            subject: None,
        },
    )?;
    println!("mass in Vehicles::sedan => {:?}", scoped.result);

    // Instantiation materializes the object graph. Single-valued features hold a
    // value, multi-valued ones hold values, and an unvalued one is Unset.
    let instantiation = model.instantiate("Vehicles::sedan")?;
    println!(
        "instantiated {} objects from {}",
        instantiation.instances().len(),
        instantiation.instance.type_symbol_id()
    );
    let mut features: Vec<_> = instantiation.instance.feature_values().iter().collect();
    features.sort_by_key(|(name, _)| name.to_owned());
    for (name, feature) in features {
        match (feature.value(), feature.values()) {
            (Some(Value::Unset), _) => println!("  {name} is unset"),
            (Some(value), _) => println!("  {name} = {value:?}"),
            (None, values) => println!("  {name} holds {} objects", values.len()),
        }
    }

    Ok(())
}
