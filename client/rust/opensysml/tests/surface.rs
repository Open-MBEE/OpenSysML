#![allow(missing_docs)]

//! The typed surface over a real service: each family of RPC through the typed API, no wire.

use std::collections::BTreeMap;
use std::env;

use opensysml::{
    AnalysisOptions, Connection, Constraint, ConvertOptions, ConvertSource, DocumentForm,
    DocumentValue, ElementRef, Error, FailureReason, IdForm, MemberOptions, MigrateOptions,
    MigrateSource, Model, Query, RunOptions, SourceDocument, SourcesOptions, SweepOptions,
    SweepRange, Value, VerifyOptions,
};

const DEMO: &str = r#"
package Demo {
    private import ScalarValues::*;

    part def Vehicle {
        attribute mass : Real default = 1500.0;
        constraint massPositive { mass > 0.0 }
        constraint massLight { mass < 100.0 }
        assert constraint massBound { mass < 100.0 }
    }
    part vehicle : Vehicle;

    calc def Add { in a : Integer; in b : Integer; return : Integer = a + b; }
    calc def Sum { in a : Real; in b : Real; return : Real = a + b; }

    part def Ship {
        attribute cost : Real default = 30.0;
        attribute other : Real default = 7.0;
    }
    analysis def CostAnalysis {
        subject s : Ship;
        in limit : Real = 20.0;
        out total : Real = Sum(s.cost, s.other);
        objective affordable { require constraint { total <= limit } }
    }
    part barge : Ship;

    action addFive {
        attribute result : Integer = 0;
        first start;
        action inner { assign result := result + 5; }
        done;
        succession first start then inner;
        succession first inner then done;
    }
    action race {
        attribute winner : Integer = 0;
        first start;
        fork split;
        action left { assign winner := 1; }
        action right { assign winner := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }
    state Machine {
        entry; then init;
        state init;
        state Running;
        succession first init then Running;
        succession first Running then done;
    }
}
"#;

const DOCUMENT: &str = r#"
package Observatory {
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    private import ScalarValues::*;

    part def Subsystem { attribute mass : Real; }
    part telescope {
        part optics : Subsystem { attribute redefines mass = 8.5; }
        part mount : Subsystem { attribute redefines mass = 15.0; }
    }
    calc def Subsystems :> Query {
        in root : Element;
        WhereType(source = Descendants(source = root, maxDepth = 3), type = "PartUsage")
    }
    calc def SubsystemTable :> Query {
        in root : Element;
        Project(
            source = OrderBy(source = Subsystems(root = root), property = "name",
                direction = "ascending", missing = "last", multiple = "error"),
            properties = ("name", "mass")
        )
    }
    part def MassReport :> Document {
        attribute redefines title = "Mass Report";
        part masses : Table {
            calc rows : SubsystemTable { in root = telescope; }
        }
    }
}
"#;

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

fn parsed(connection: &Connection, content: &str) -> Model {
    let model = connection
        .parse_content(content, &Default::default())
        .unwrap_or_else(|error| panic!("parse failed: {error}"));
    assert!(model.ok(), "{:?}", model.diagnostics());
    model
}

fn demo() -> Option<Model> {
    service_or_skip().map(|connection| parsed(&connection, DEMO))
}

fn real(value: Option<&Value>) -> f64 {
    match value {
        Some(Value::Real(v)) => *v,
        other => panic!("expected a real, got {other:?}"),
    }
}

#[test]
fn several_documents_parse_as_one_model_and_edit_by_name() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = connection
        .parse_sources(
            &[
                SourceDocument::inline("a.sysml", "package A { part def Engine; }"),
                SourceDocument::inline(
                    "b.sysml",
                    "package B { private import A::*; part engine : Engine; }",
                ),
            ],
            &SourcesOptions {
                strict: true,
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(
        model.documents(),
        &["a.sysml".to_owned(), "b.sysml".to_owned()]
    );
    assert!(model.contains("B::engine").unwrap());
    assert_eq!(model.get("A::Engine").unwrap().unwrap().kind(), "partDef");

    let mut editor = model.edit();
    editor.in_document("b.sysml");
    editor.add_member("B", "part", "spare", MemberOptions::new().typed("Engine"));
    let result = editor.apply().unwrap();
    let b = result
        .documents
        .iter()
        .find(|document| document.name == "b.sysml")
        .expect("the edited document is answered");
    assert!(b.content.contains("spare"), "{}", b.content);
}

#[test]
fn duplicate_document_names_are_refused_before_sending() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let refused = connection.parse_sources(
        &[
            SourceDocument::inline("a.sysml", "package A;"),
            SourceDocument::inline("a.sysml", "package B;"),
        ],
        &Default::default(),
    );
    assert!(
        matches!(refused, Err(Error::InvalidRequest(_))),
        "{refused:?}"
    );
}

#[test]
fn a_model_converts_to_notation_and_turtle() {
    let Some(model) = demo() else {
        return;
    };
    let notation = model.to_sysml(false).unwrap();
    assert!(notation.content.contains("part def Vehicle"));
    assert!(!notation.experimental);
    let turtle = model
        .connection()
        .convert(
            "ttl",
            &ConvertSource::Content(DEMO.to_owned()),
            &ConvertOptions {
                from_format: "sysml".to_owned(),
                id_form: Some(IdForm::Uuid),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(turtle.to_format, "ttl");
    assert!(!turtle.content.is_empty());
}

#[test]
fn a_model_of_several_documents_converts_only_the_named_ones() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    if !connection.capabilities().has("convert_documents") {
        return;
    }
    let model = connection
        .parse_sources(
            &[
                SourceDocument::inline("lib.sysml", "package Lib { part def Engine; }"),
                SourceDocument::inline(
                    "app.sysml",
                    "package App { private import Lib::*; part engine : Engine; }",
                ),
            ],
            &Default::default(),
        )
        .unwrap();
    let app = connection
        .convert(
            "api-json",
            &ConvertSource::Model(model.hash().to_owned()),
            &ConvertOptions {
                documents: vec!["app.sysml".to_owned()],
                ..Default::default()
            },
        )
        .unwrap();
    let elements: Vec<serde_json::Value> = serde_json::from_str(&app.content).unwrap();
    let written: Vec<&str> = elements
        .iter()
        .filter_map(|element| element["@id"].as_str())
        .collect();
    assert!(written.contains(&"App__engine"), "{written:?}");
    assert!(!written.contains(&"Lib__Engine"), "{written:?}");
    let typed = elements.iter().any(|element| {
        element["@type"] == "FeatureTyping" && element["type"]["@id"] == "Lib__Engine"
    });
    assert!(typed, "App::engine is not typed by Lib__Engine");
}

#[test]
fn a_v1_model_is_migrated_and_refused_by_convert() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let vehicle = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../conformance/fixtures/vehicle.xmi");
    let migrated = connection
        .migrate(
            "sysml",
            &MigrateSource::File(vehicle.clone()),
            &MigrateOptions {
                report: true,
                ..Default::default()
            },
        )
        .unwrap();
    assert!(
        migrated.content.contains("part def Vehicle"),
        "{}",
        migrated.content
    );
    assert_eq!(migrated.from_format, "xmi");
    assert!(migrated.experimental);
    assert!(!migrated.experimental_notice.is_empty());
    assert_eq!(
        (
            migrated.report.mapped,
            migrated.report.approximated,
            migrated.report.unmapped,
            migrated.report.skipped
        ),
        (77, 13, 3, 2)
    );
    assert_eq!(migrated.report.entries.len(), 95);
    assert_eq!(migrated.report.by_verdict("unmapped").len(), 3);
    assert!(migrated.report.text.contains("unmapped"));
    assert!(migrated
        .source_path
        .as_deref()
        .is_some_and(|path| path.is_absolute()));

    let inline = connection
        .migrate(
            "ttl",
            &MigrateSource::Content(std::fs::read(&vehicle).unwrap()),
            &MigrateOptions {
                from_format: " XMI ".to_owned(),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(inline.to_format, "ttl");
    assert!(inline.source_path.is_none());
    assert!(inline.report.entries.is_empty());
    assert_eq!(inline.report.mapped, 77);

    let refused = connection.convert(
        "sysml",
        &ConvertSource::File(vehicle.clone()),
        &Default::default(),
    );
    assert!(
        matches!(&refused, Err(Error::InvalidRequest(message)) if message.contains("migrated, not converted")),
        "{refused:?}"
    );
    let refused = connection.migrate(
        "sysml",
        &MigrateSource::Content(b"package P;".to_vec()),
        &MigrateOptions {
            from_format: "sysml".to_owned(),
            ..Default::default()
        },
    );
    assert!(
        matches!(&refused, Err(Error::InvalidRequest(message)) if message.contains("converted, not migrated")),
        "{refused:?}"
    );
    let refused = connection.migrate(
        "sysml",
        &MigrateSource::Content(b"<xmi/>".to_vec()),
        &Default::default(),
    );
    assert!(
        matches!(&refused, Err(Error::InvalidRequest(message)) if message.contains("from_format")),
        "{refused:?}"
    );
}

#[test]
fn structured_and_oslc_queries_select_the_same_parts() {
    let Some(model) = demo() else {
        return;
    };
    let mut structured: Vec<String> = model
        .query(&Query::new().filter(Constraint::equals("@type", "PartUsage")))
        .unwrap()
        .into_iter()
        .map(|element| element.id)
        .collect();
    structured.sort();
    for part in ["Demo::barge", "Demo::vehicle"] {
        assert!(structured.iter().any(|id| id == part), "{structured:?}");
    }
    let mut oslc: Vec<String> = model
        .query_oslc(r#"oslc.where=rdf:type="PartUsage""#)
        .unwrap()
        .into_iter()
        .map(|element| element.id)
        .collect();
    oslc.sort();
    assert_eq!(oslc, structured);
}

#[test]
fn find_and_lookup_name_symbols_or_suggest_near_names() {
    let Some(model) = demo() else {
        return;
    };
    assert_eq!(
        model.find("Vehicle").unwrap().unwrap().id(),
        "Demo::Vehicle"
    );
    assert!(model.get("Vehicle").unwrap().is_none());
    match model.lookup("Vehicel") {
        Err(Error::SymbolNotFound { suggestions, .. }) => {
            assert!(
                suggestions.iter().any(|s| s.contains("Vehicle")),
                "{suggestions:?}"
            )
        }
        other => panic!("expected a symbol-not-found error, got {other:?}"),
    }
    let mass = model.lookup("Demo::Vehicle::mass").unwrap();
    let facts = mass.type_facts().expect("a typed attribute has type facts");
    assert_eq!(facts.primitive, "Real");
}

#[test]
fn a_document_query_answers_typed_rows_and_a_document_renders() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(&connection, DOCUMENT);
    let result = model
        .run_document_query(
            "Observatory::SubsystemTable",
            &[(
                "root",
                vec![DocumentValue::Element(ElementRef::new(
                    "Observatory::telescope",
                ))],
            )],
        )
        .unwrap();
    assert_eq!(result.columns, vec!["name", "mass"]);
    assert_eq!(result.rows.len(), 2);
    assert_eq!(
        result.rows[0].cells[0],
        vec![DocumentValue::Text("mount".to_owned())]
    );
    let markdown = model
        .render_document("Observatory::MassReport", DocumentForm::Markdown)
        .unwrap();
    assert!(markdown.contains("Mass Report"), "{markdown}");
    let html = model
        .render_document("Observatory::MassReport", DocumentForm::Html)
        .unwrap();
    assert!(html.contains("<"), "{html}");
}

#[test]
fn an_action_runs_explores_and_a_state_machine_steps() {
    let Some(model) = demo() else {
        return;
    };
    let mut inputs = BTreeMap::new();
    inputs.insert("result".to_owned(), Value::Integer(10));
    let run = model
        .execute_action("Demo::addFive", &inputs, &RunOptions::default())
        .unwrap();
    assert_eq!(run.outputs.get("result"), Some(&Value::Integer(15)));

    let explored = model
        .explore_action("Demo::race", &BTreeMap::new(), &RunOptions::default())
        .unwrap();
    assert!(explored.complete);
    let mut winners: Vec<_> = explored
        .outcomes
        .iter()
        .map(|outcome| outcome.outputs.get("winner").cloned())
        .collect();
    winners.sort_by_key(|w| format!("{w:?}"));
    assert_eq!(
        winners,
        vec![Some(Value::Integer(1)), Some(Value::Integer(2))]
    );

    let refused = model.execute_action(
        "Demo::race",
        &BTreeMap::new(),
        &RunOptions {
            schedule: Some("explore".to_owned()),
            ..Default::default()
        },
    );
    assert!(
        matches!(refused, Err(Error::InvalidRequest(_))),
        "{refused:?}"
    );

    let state = model
        .execute_state::<&str>("Demo::Machine", &[], &RunOptions::default())
        .unwrap();
    assert!(
        state.states_visited.iter().any(|s| s.contains("Running")),
        "{:?}",
        state.states_visited
    );
}

#[test]
fn verification_answers_typed_verdicts() {
    let Some(model) = demo() else {
        return;
    };
    let subject = VerifyOptions {
        subject: Some("Demo::vehicle".to_owned()),
        ..Default::default()
    };
    let holds = model
        .verify_constraint("Demo::Vehicle::massPositive", &subject)
        .unwrap();
    assert!(holds.holds && holds.evaluated());
    let fails = model
        .verify_constraint("Demo::Vehicle::massLight", &subject)
        .unwrap();
    assert!(!fails.holds && fails.evaluated());
    assert_eq!(fails.reason, FailureReason::Unspecified);

    let validation = model.validate_instance("Demo::vehicle", None).unwrap();
    assert!(!validation.valid());
    assert_eq!(validation.violated().count(), 1);

    let engines = model.connection().list_engines().unwrap();
    assert!(!engines.is_empty());
}

#[test]
fn a_calc_answers_its_value_and_a_wrong_kind_is_typed() {
    let Some(model) = demo() else {
        return;
    };
    let result = model
        .calc("Demo::Add", &[Value::Integer(2), Value::Integer(3)], None)
        .unwrap();
    assert_eq!(result.value, Some(Value::Integer(5)));
    let wrong = model.calc("Demo::vehicle", &[], None);
    assert!(matches!(wrong, Err(Error::WrongKind { .. })), "{wrong:?}");
}

#[test]
fn an_analysis_and_a_sweep_report_outputs_and_objectives() {
    let Some(model) = demo() else {
        return;
    };
    let analysis = model
        .run_analysis(
            "Demo::CostAnalysis",
            &AnalysisOptions {
                subject: Some("Demo::barge".to_owned()),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(real(analysis.output("total")), 37.0);
    assert!(!analysis.verdicts[0].holds);

    let table = model
        .run_sweep(
            "Demo::Sum",
            &[SweepRange::new("b", Value::Real(0.0), Value::Real(4.0)).step(Value::Real(2.0))],
            &SweepOptions {
                arguments: vec![Value::Real(1.0)],
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(table.parameters, vec!["b"]);
    let sums: Vec<f64> = table
        .rows
        .iter()
        .map(|row| real(row.output("result")))
        .collect();
    assert_eq!(sums, vec![1.0, 3.0, 5.0]);
}

#[test]
fn an_edit_rewrites_the_source_and_a_refusal_is_typed() {
    let Some(model) = demo() else {
        return;
    };
    let mut editor = model.edit();
    editor
        .rename("Demo::barge", "tug")
        .set_value("Demo::Vehicle::mass", "1200.0");
    let result = editor.apply().unwrap();
    assert!(result.content.contains("part tug"), "{}", result.content);
    assert!(result.content.contains("1200.0"), "{}", result.content);
    assert_eq!(result.applied.len(), 2);

    let mut editor = model.edit();
    editor.rename("Demo::nothing", "x");
    match editor.apply() {
        Err(Error::Edit(error)) => assert!(error.failure.is_target_error(), "{error}"),
        other => panic!("expected an edit error, got {other:?}"),
    }
}
