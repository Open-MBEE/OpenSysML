#![allow(missing_docs)]

use std::env;
use std::io::BufRead;
use std::path::PathBuf;
use std::process::Command;
use std::thread;
use std::time::Duration;

use opensysml::{Complex, Connection, Error, EvalOptions, Function, Magnitude, Value, Vector};

fn service_or_skip() -> Option<Connection> {
    match Connection::private() {
        Ok(connection) => Some(connection),
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("required sysml-grpc service unavailable: {error}");
            }
            eprintln!("skipping service-backed Rust client test: {error}");
            None
        }
    }
}

#[test]
fn parse_eval_and_navigation() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = connection
        .parse_content("package Demo {}", &Default::default())
        .unwrap_or_else(|error| panic!("parse failed: {error}"));
    assert!(!model.hash().is_empty());
    assert_eq!(
        model
            .eval("2 + 2")
            .unwrap_or_else(|error| panic!("evaluation failed: {error}")),
        Value::Integer(4)
    );
    assert_eq!(
        model.root().expect("parsed model has a root").kind(),
        "RootNamespace"
    );
    let adopted = connection.model_by_hash(model.hash());
    assert_eq!(adopted.hash(), model.hash());
}

#[test]
fn missing_capability_is_legible() {
    let error = Error::MissingCapability {
        capability: "strict_conformance".to_owned(),
        remedy: "upgrade the service".to_owned(),
    };
    assert!(error.to_string().contains("strict_conformance"));
    assert!(error.to_string().contains("upgrade"));
}

#[test]
fn start_failure_is_legible() {
    let error = Error::ServiceStart("binary exited before reporting an address".to_owned());
    assert!(error.to_string().contains("service start failed"));
    assert!(error.to_string().contains("reporting an address"));
}

#[test]
fn unknown_model_hash_is_a_transport_service_error() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let result = connection.diagnostics("definitely-unknown-model-hash");
    assert!(matches!(
        result,
        Err(Error::Service {
            status: opensysml::Status::NotFound,
            ..
        })
    ));
}

#[test]
fn unknown_model_hash_symbol_is_a_transport_service_error() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let result = connection
        .model_by_hash("no-such-model")
        .symbol("Demo::missing");
    assert!(matches!(
        result,
        Err(Error::Service {
            status: opensysml::Status::NotFound,
            ..
        })
    ));
}

#[test]
fn unknown_model_hash_evaluation_is_a_transport_service_error() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let result = connection.model_by_hash("no-such-model").eval("2 + 2");
    assert!(matches!(
        result,
        Err(Error::Service {
            status: opensysml::Status::NotFound,
            ..
        })
    ));
}

#[test]
fn in_band_evaluation_errors_remain_model_errors() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = match connection.parse_content("package ErrorTest {}", &Default::default()) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    assert!(matches!(model.eval("not valid ((("), Err(Error::Model(_))));
}

#[test]
fn instantiate_decodes_feature_values() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = match connection.parse_content(
        "package Demo { part def Car { attribute mass : Integer = 3; } part car : Car; }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let instance = match model.instantiate("Demo::car") {
        Ok(instantiation) => instantiation.instance,
        Err(error) => panic!("instantiation failed: {error}"),
    };
    assert_eq!(instance.type_symbol_id(), "Demo::car");
    assert!(instance.feature("mass").is_some());
}

#[test]
fn a_complex_number_is_one_value_with_both_parts() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("complex_values"));
    let model = match connection.parse_content(
        "package C {
            private import ScalarValues::*;
            private import ComplexFunctions::*;
            part def Signal {
                attribute z : Complex = rect(1.5, -2.0);
                attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
            }
        }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let options = EvalOptions {
        context: Some("C::Signal".to_owned()),
        subject: None,
    };
    let evaluated = match model.evaluate("z", &options) {
        Ok(evaluation) => evaluation.result,
        Err(error) => panic!("evaluation failed: {error}"),
    };
    assert_eq!(
        evaluated,
        Value::Complex(Complex {
            real: 1.5,
            imaginary: -2.0
        })
    );
    let instance = match model.instantiate("C::Signal") {
        Ok(instantiation) => instantiation.instance,
        Err(error) => panic!("instantiation failed: {error}"),
    };
    let z = instance.feature("z").expect("z is materialized");
    assert_eq!(
        z.value(),
        Some(&Value::Complex(Complex {
            real: 1.5,
            imaginary: -2.0
        }))
    );
    let zs = instance.feature("zs").expect("zs is materialized");
    assert_eq!(
        zs.values(),
        [
            Value::Complex(Complex {
                real: 1.0,
                imaginary: 2.0
            }),
            Value::Complex(Complex {
                real: 3.0,
                imaginary: 4.0
            }),
        ]
    );
}

#[test]
fn an_array_a_vector_and_a_vector_quantity_arrive_whole() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("structured_values"));
    let model = match connection.parse_content(
        "package S {
            private import ScalarValues::*;
            private import Collections::*;
            private import VectorValues::*;
            private import VectorFunctions::*;
            private import Quantities::*;
            private import SI::*;
            attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
            attribute v : CartesianVectorValue = VectorOf((3.0, 4.0));
            attribute d : VectorQuantityValue = VectorOf((3.0, 4.0)) [m];
        }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let eval = |expr: &str| match model.evaluate(expr, &EvalOptions::default()) {
        Ok(evaluation) => evaluation.result,
        Err(error) => panic!("evaluating {expr} failed: {error}"),
    };

    let Value::Array(grid) = eval("S::grid") else {
        panic!("S::grid should be an array");
    };
    assert_eq!(grid.dimensions(), [2, 3]);
    assert_eq!(
        grid.elements(),
        (1..=6).map(Value::Integer).collect::<Vec<_>>()
    );
    assert_eq!(grid.get(&[1, 2]), Some(&Value::Integer(6)));

    assert_eq!(
        eval("S::v"),
        Value::Vector(Vector {
            components: vec![Magnitude::Real(3.0), Magnitude::Real(4.0)],
        })
    );

    let Value::VectorQuantity(d) = eval("S::d") else {
        panic!("S::d should be a vector quantity");
    };
    assert_eq!(d.unit(), Some("m"));
    assert_eq!(
        d.components()
            .iter()
            .map(|component| component.magnitude)
            .collect::<Vec<_>>(),
        [Magnitude::Real(3.0), Magnitude::Real(4.0)]
    );
    let term = d.components()[0]
        .unit_term
        .as_ref()
        .expect("a metre reduces to itself");
    assert_eq!(term.factors[0].unit_id, "SI::metre");
    assert_eq!(term.factors[0].exponent, 1.0);
}

#[test]
fn the_service_advertises_the_verification_body_verdicts_it_reports() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("verification_verdicts"));
}

#[test]
fn the_service_advertises_the_schedule_of_its_execution_requests() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("schedule"));
}

#[test]
fn the_service_advertises_the_explore_schedule() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("schedule_explore"));
}

#[test]
fn the_service_advertises_the_final_clock_instant_of_its_execution_responses() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("final_time"));
}

#[test]
fn a_bare_measurement_reference_arrives_with_its_reduction_and_declaration() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("measurement_refs"));
    let model = match connection.parse_content(
        "package M {
            private import ScalarValues::*;
            private import Quantities::*;
            private import MeasurementReferences::*;
            private import SI::*;
            attribute q : ISQ::LengthValue = 3 [km];
            attribute u : MeasurementUnit = m;
            attribute speed = m / s;
        }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let eval = |expr: &str| match model.evaluate(expr, &EvalOptions::default()) {
        Ok(evaluation) => evaluation.result,
        Err(error) => panic!("evaluating {expr} failed: {error}"),
    };

    let Value::MeasurementRef(u) = eval("M::u") else {
        panic!("M::u should be a measurement reference");
    };
    assert_eq!(u.unit, "m");
    assert_eq!(u.unit_id.as_deref(), Some("SI::metre"));
    assert_eq!(u.unit_term.factors[0].unit_id, "SI::metre");

    let Value::MeasurementRef(km) = eval("M::q.mRef") else {
        panic!("M::q.mRef should be a measurement reference");
    };
    assert_eq!(km.unit, "km");
    assert_eq!(km.unit_id.as_deref(), Some("SI::kilometre"));
    assert_eq!(km.unit_term.scale_num, 1000.0);

    let Value::MeasurementRef(speed) = eval("M::speed") else {
        panic!("M::speed should be a measurement reference");
    };
    assert_eq!(speed.unit_id, None);
    assert_eq!(
        speed
            .unit_term
            .factors
            .iter()
            .map(|factor| (factor.unit_id.as_str(), factor.exponent))
            .collect::<Vec<_>>(),
        [("SI::metre", 1.0), ("SI::second", -1.0)]
    );
}

#[test]
fn a_calc_held_as_a_value_arrives_as_the_function_it_names() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("function_values"));
    let model = match connection.parse_content(
        "package Demo {
            private import ScalarValues::*;
            calc def Sq { in v : Real; return : Real = v * v; }
            calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
            calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
            attribute pick = Identity(Sq);
            attribute nine = Fn(Sq, 3.0);
            part def Scaler {
                attribute k : Real = 2.0;
                calc scale { in x : Real; return : Real = x * k; }
            }
            part holder : Scaler;
            attribute scaler = holder.scale;
        }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let eval = |expr: &str| match model.evaluate(expr, &EvalOptions::default()) {
        Ok(evaluation) => evaluation.result,
        Err(error) => panic!("evaluating {expr} failed: {error}"),
    };

    assert_eq!(
        eval("Demo::pick"),
        Value::Function(Function {
            calc_id: "Demo::Sq".to_owned(),
            self_id: None,
        })
    );
    assert_eq!(eval("Demo::nine"), Value::Real(9.0));
    let Value::Function(scale) = eval("Demo::scaler") else {
        panic!("Demo::scaler should be a function");
    };
    assert_eq!(scale.calc_id, "Demo::Scaler::scale");
    assert!(scale.self_id.is_some_and(|id| id > 0));
}

#[test]
fn a_set_arrives_once_per_member_and_a_tensor_with_its_rank() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    assert!(connection.capabilities().has("set_values"));
    assert!(connection.capabilities().has("tensor_values"));
    let model = match connection.parse_content(
        "package W {
            private import ScalarValues::*;
            private import Collections::*;
            private import Quantities::*;
            private import MeasurementReferences::*;
            private import SI::*;
            private import TensorCalculations::*;
            attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
            attribute e : Set { :>> elements = (); }
            attribute cubeRef : TensorMeasurementReference {
                :>> dimensions = (2, 2, 2);
                :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
            }
            attribute cube : TensorQuantityValue =
                TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
        }",
        &Default::default(),
    ) {
        Ok(model) => model,
        Err(error) => panic!("parse failed: {error}"),
    };
    let eval = |expr: &str| match model.evaluate(expr, &EvalOptions::default()) {
        Ok(evaluation) => evaluation.result,
        Err(error) => panic!("evaluating {expr} failed: {error}"),
    };

    let Value::Set(s) = eval("W::s.elements") else {
        panic!("W::s.elements should be a set");
    };
    assert_eq!(s.len(), 3);
    assert_eq!(
        s.elements(),
        [Value::Integer(1), Value::Integer(2), Value::Integer(3)]
    );
    assert!(s.contains(&Value::Integer(3)));
    let Value::Set(e) = eval("W::e.elements") else {
        panic!("W::e.elements should be a set");
    };
    assert!(e.is_empty());

    let Value::TensorQuantity(cube) = eval("W::cube") else {
        panic!("W::cube should be a tensor quantity");
    };
    assert_eq!(cube.rank(), 3);
    assert_eq!(cube.dimensions(), [2, 2, 2]);
    assert_eq!(cube.unit(), Some("Pa"));
    assert_eq!(
        cube.components()
            .iter()
            .map(|component| component.magnitude)
            .collect::<Vec<_>>(),
        (1..=8)
            .map(|i| Magnitude::Real(f64::from(i)))
            .collect::<Vec<_>>()
    );
    assert_eq!(
        cube.get(&[1, 0, 1]).map(|q| q.magnitude),
        Some(Magnitude::Real(6.0))
    );
}

#[test]
fn blocking_calls_work_inside_a_runtime() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let runtime = match tokio::runtime::Builder::new_current_thread().build() {
        Ok(runtime) => runtime,
        Err(error) => panic!("runtime creation failed: {error}"),
    };
    let result = runtime.block_on(async move {
        connection.parse_content("package RuntimeTest {}", &Default::default())
    });
    assert!(
        result.is_ok(),
        "blocking client call inside runtime failed: {result:?}"
    );
}

#[cfg(unix)]
#[test]
fn killing_the_parent_leaves_no_private_service() {
    let Some(binary) = env::var_os("CARGO_BIN_EXE_child_probe")
        .map(PathBuf::from)
        .or_else(|| {
            std::env::current_exe().ok().and_then(|executable| {
                executable
                    .ancestors()
                    .map(|directory| {
                        directory.join("examples").join(if cfg!(windows) {
                            "child_probe.exe"
                        } else {
                            "child_probe"
                        })
                    })
                    .find(|candidate| candidate.is_file())
            })
        })
    else {
        eprintln!("skipping SIGKILL lifecycle test: child_probe was not built");
        return;
    };
    let mut probe = match Command::new(binary)
        .stdout(std::process::Stdio::piped())
        .spawn()
    {
        Ok(probe) => probe,
        Err(error) => {
            if env::var("OPENSYSML_REQUIRE_SERVICE").ok().as_deref() == Some("1") {
                panic!("could not start lifecycle probe: {error}");
            }
            eprintln!("skipping SIGKILL lifecycle test: {error}");
            return;
        }
    };
    let Some(stdout) = probe.stdout.take() else {
        panic!("lifecycle probe stdout was not piped");
    };
    let mut lines = std::io::BufReader::new(stdout).lines();
    let pid = match lines.next() {
        Some(Ok(line)) => match line.parse::<u32>() {
            Ok(pid) => pid,
            Err(error) => panic!("lifecycle probe returned invalid pid: {error}"),
        },
        Some(Err(error)) => panic!("could not read lifecycle probe: {error}"),
        None => panic!("lifecycle probe exited without reporting a pid"),
    };
    let kill_status = Command::new("kill")
        .args(["-KILL", &probe.id().to_string()])
        .status();
    assert!(
        matches!(kill_status, Ok(status) if status.success()),
        "could not SIGKILL lifecycle probe: {kill_status:?}"
    );
    let _ = probe.wait();
    for _ in 0..50 {
        if !PathBuf::from(format!("/proc/{pid}")).exists() {
            return;
        }
        thread::sleep(Duration::from_millis(100));
    }
    panic!("private service {pid} survived parent SIGKILL");
}
