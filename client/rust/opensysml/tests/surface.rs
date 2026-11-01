#![allow(missing_docs)]

//! The typed surface over a real service: each family of RPC through the typed API, no wire.

use std::collections::BTreeMap;
use std::env;

use opensysml::{
    AnalysisOptions, Connection, Constraint, ConvertOptions, ConvertSource, DocumentForm,
    DocumentValue, ElementRef, Error, FailureReason, IdForm, MemberOptions, MigrateOptions,
    MigrateSource, Model, Query, RenderViewCompartments, RenderViewPorts, RunOptions,
    SourceDocument, SourcesOptions, Status, SweepOptions, SweepRange, Value, VerifyOptions,
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

    calc def Add { in a : Integer; in b : Integer; return : Integer[1] = a + b; }
    calc def Sum { in a : Real; in b : Real; return : Real[1] = a + b; }

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
        Some(Value::Sequence(elements)) if elements.len() == 1 => real(elements.first()),
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
fn api_json_names_library_references_separately_and_opts_into_legacy_stubs() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let source = ConvertSource::Content(
        "package P { private import SI::*; attribute time = 20 [s]; }".to_owned(),
    );
    let options = ConvertOptions {
        from_format: "sysml".to_owned(),
        ..Default::default()
    };
    let plain = connection.convert("api-json", &source, &options).unwrap();
    let second = plain
        .library_references
        .iter()
        .find(|reference| reference.qualified_name == "SI::second")
        .expect("SI::second is referenced");
    assert_eq!(second.id, "0c539f7c-20a1-520d-bae0-f3f855517aea");
    assert_eq!(second.r#type, "AttributeUsage");
    let plain_elements: Vec<serde_json::Value> = serde_json::from_str(&plain.content).unwrap();
    assert!(plain_elements
        .iter()
        .all(|element| element["isLibraryElement"] != serde_json::Value::Bool(true)));
    let legacy = connection
        .convert(
            "api-json",
            &source,
            &ConvertOptions {
                include_library_stubs: true,
                ..options.clone()
            },
        )
        .unwrap();
    let legacy_elements: Vec<serde_json::Value> = serde_json::from_str(&legacy.content).unwrap();
    let stub = legacy_elements
        .iter()
        .find(|element| element["@id"] == second.id.as_str())
        .expect("the stub is written");
    assert_eq!(stub["qualifiedName"], "SI::second");
    assert_eq!(stub["isLibraryElement"], true);
    assert_eq!(legacy.library_references, plain.library_references);
    let notation = connection.convert("sysml", &source, &options).unwrap();
    assert!(notation.library_references.is_empty());
}

#[test]
fn a_held_model_converts_and_saves_with_explicit_options() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(
        &connection,
        "package P { private import SI::*; attribute time = 20 [s]; }",
    );
    let options = ConvertOptions {
        id_form: Some(IdForm::Uuid),
        include_library_stubs: true,
        ..Default::default()
    };
    let legacy = model.convert_with("api-json", &options).unwrap();
    assert_eq!(legacy.to_format, "api-json");
    let second = legacy
        .library_references
        .iter()
        .find(|reference| reference.qualified_name == "SI::second")
        .expect("SI::second is referenced");
    let elements: Vec<serde_json::Value> = serde_json::from_str(&legacy.content).unwrap();
    let stub = elements
        .iter()
        .find(|element| element["@id"] == second.id.as_str())
        .expect("the stub is written");
    assert_eq!(stub["isLibraryElement"], true);
    let ids: Vec<&str> = elements
        .iter()
        .filter_map(|element| element["@id"].as_str())
        .collect();
    assert!(
        !ids.contains(&"P__time"),
        "ids are uuids, not qualified: {ids:?}"
    );
    let direct = connection
        .convert(
            "api-json",
            &ConvertSource::Model(model.hash().to_owned()),
            &options,
        )
        .unwrap();
    assert_eq!(legacy.content, direct.content);
    let plain = model.to_api_json().unwrap();
    let plain_elements: Vec<serde_json::Value> = serde_json::from_str(&plain.content).unwrap();
    let plain_ids: Vec<&str> = plain_elements
        .iter()
        .filter_map(|element| element["@id"].as_str())
        .collect();
    assert!(plain_ids.contains(&"P__time"), "{plain_ids:?}");
    assert!(plain_elements
        .iter()
        .all(|element| element["isLibraryElement"] != serde_json::Value::Bool(true)));
    let dir = env::temp_dir().join(format!("opensysml-rust-save-{}", std::process::id()));
    std::fs::create_dir_all(&dir).unwrap();
    let path = dir.join("p.json");
    let saved = model.save_with(&path, None, &options).unwrap();
    assert_eq!(saved.to_format, "api-json");
    assert_eq!(std::fs::read_to_string(&path).unwrap(), legacy.content);
    let refused = model.save_with(dir.join("p.txt"), Some("sysml"), &options);
    assert!(
        matches!(&refused, Err(Error::Service { message, .. }) if message.contains("include_library_stubs requires to_format api-json")),
        "{refused:?}"
    );
    assert!(
        !dir.join("p.txt").exists(),
        "a refused conversion writes nothing"
    );
    let notation = model
        .save_with(dir.join("p.txt"), Some("sysml"), &ConvertOptions::default())
        .unwrap();
    assert_eq!(notation.to_format, "sysml");
    assert!(notation.library_references.is_empty());
    assert!(notation.content.contains("attribute time"));
    assert!(matches!(
        model.save_with(dir.join("p.txt"), None, &options),
        Err(Error::InvalidRequest(_))
    ));
    std::fs::remove_dir_all(&dir).unwrap();
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
        (77, 15, 3, 2)
    );
    assert_eq!(migrated.report.entries.len(), 97);
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
    let asciidoc = model
        .render_document("Observatory::MassReport", DocumentForm::AsciiDoc)
        .unwrap();
    assert!(asciidoc.starts_with("= "), "{asciidoc}");
}

#[test]
fn a_rendered_view_keeps_ports_edges_and_origins() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(
        &connection,
        include_str!("../../../../conformance/fixtures/views.sysml"),
    );
    let rendered = model.render_view("RenderViewDemo::connections").unwrap();
    assert_eq!(rendered.kind, "interconnection");
    assert_eq!(rendered.edges.len(), 1);
    assert!(!rendered.edges[0].from_port.is_empty());
    assert!(!rendered.edges[0].to_port.is_empty());
    assert!(rendered.nodes.iter().all(|node| node.origin.is_some()));

    let full = model
        .render_view_with_ports("RenderViewDemo::connections", RenderViewPorts::Full)
        .unwrap();
    assert!(full
        .nodes
        .iter()
        .flat_map(|node| &node.ports)
        .any(|port| port.name == "spare"));
}

#[test]
fn rendered_views_fold_members_into_compartment_rows_on_request() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(
        &connection,
        include_str!("../../../../conformance/fixtures/views.sysml"),
    );
    let boxes = model.render_view("#tree:RenderViewDemo::Assembly").unwrap();
    assert!(boxes.nodes.iter().all(|node| node.compartment.is_empty()));
    let folded = model
        .render_view_folded(
            "#tree:RenderViewDemo::Assembly",
            RenderViewPorts::Minimal,
            &[],
            0,
            RenderViewCompartments::Most,
        )
        .unwrap();
    let sender = folded
        .nodes
        .iter()
        .find(|node| node.name == "sender")
        .expect("sender is drawn");
    assert_eq!(sender.compartment, "parts");
    assert!(folded.nodes.len() < boxes.nodes.len());
}

#[test]
fn exported_graphs_are_the_canonical_form_of_a_behavior() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(
        &connection,
        include_str!("../../../../conformance/fixtures/behavior.sysml"),
    );
    let graphs = model.export_graphs("Test::race").unwrap();
    assert_eq!(graphs.version, 1);
    assert_eq!(graphs.subject, "Test::race");
    assert!(
        graphs
            .content
            .starts_with("{\"version\":1,\"subject\":\"Test::race\""),
        "{}",
        graphs.content
    );
    assert!(graphs.content.ends_with("}\n"));
    assert!(graphs.content.contains("\"actions\""));
    assert!(model.export_graphs("Test::Missing").is_err());
    assert!(model.export_graphs("Test").is_err());
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

    let trace_for_action = RunOptions {
        trace: true,
        ..Default::default()
    };
    let traced = model
        .execute_action("Demo::addFive", &BTreeMap::new(), &trace_for_action)
        .expect("traced action run");
    let kinds: Vec<&str> = traced.trace.iter().map(|e| e.kind.as_str()).collect();
    assert!(
        kinds.contains(&"step") && kinds.contains(&"complete"),
        "{kinds:?}"
    );
    let assign = traced
        .trace
        .iter()
        .find(|e| e.kind == "assign")
        .expect("assign record");
    assert_eq!(assign.feature, "result");
    assert_eq!(assign.value.as_deref(), Some(&Value::Integer(5.into())));
    assert_eq!(traced.trace_dropped, 0);
    assert!(matches!(
        model.explore_action("Demo::race", &BTreeMap::new(), &trace_for_action),
        Err(Error::InvalidRequest(message)) if message.contains("one run")
    ));
    let events_without_performer = RunOptions {
        events: vec!["Go".to_owned()],
        ..Default::default()
    };
    assert!(matches!(
        model.execute_action("Demo::addFive", &BTreeMap::new(), &events_without_performer),
        Err(Error::InvalidRequest(message)) if message.contains("performer")
    ));
    assert!(matches!(
        model.explore_state::<&str>("Demo::Machine", &[], &trace_for_action),
        Err(Error::InvalidRequest(message)) if message.contains("one run")
    ));

    let state = model
        .execute_state::<&str>("Demo::Machine", &[], &RunOptions::default())
        .unwrap();
    assert!(
        state.states_visited.iter().any(|s| s.contains("Running")),
        "{:?}",
        state.states_visited
    );
    assert!(state.trace.is_empty());
    let traced = model
        .execute_state::<&str>(
            "Demo::Machine",
            &[],
            &RunOptions {
                trace: true,
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(
        traced.trace.first().map(|event| event.kind.as_str()),
        Some("entry")
    );
    assert_eq!(traced.trace_dropped, 0);
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

const RUNS_MODEL: &str = r#"package MC {
    private import ScalarValues::*;
    private import RandomFunctions::*;
    action def Settle { out t : Real; first start; then assign t := uniform(1.0, 5.0); then done; }
    part def Probe { attribute t : Real; action settle { first start; then assign t := uniform(1.0, 5.0); then done; } }
    individual def probe :> Probe;
    analysis def Mc :> Simulation::MonteCarlo {
        subject analysed : Probe;
        perform action run ::> analysed.settle;
        attribute :>> observed : Real = analysed.t;
        return Mean : Real[1] = mean;
    }
    part def P { attribute m : Real = 2.0; }
    part p : P;
    analysis def Cost { subject s : P; return total : Real[1] = s.m * 3.0; }
}"#;

#[test]
fn repeated_runs_table_the_observables_and_conclude_a_sample() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(&connection, RUNS_MODEL);
    let runs = model
        .run_runs(&opensysml::RunsOptions {
            action: Some("MC::Settle".to_owned()),
            runs: 4,
            seed: Some(7),
            observe: vec!["t".to_owned()],
            ..Default::default()
        })
        .unwrap();
    assert_eq!(runs.len(), 4);
    assert_eq!(runs.observables, vec!["t".to_owned()]);
    assert_eq!(runs.distributions.len(), 1);
    assert!(runs.distributions[0].numeric && runs.distributions[0].count == 4);
    assert_eq!(runs.seed, Some(7));
    let sample = model
        .run_runs(&opensysml::RunsOptions {
            analysis: Some("MC::Mc".to_owned()),
            subject: Some("MC::probe".to_owned()),
            runs: 3,
            seed: Some(1),
            ..Default::default()
        })
        .unwrap();
    assert_eq!(sample.len(), 3);
    assert!(sample.unconcluded.is_empty() && sample.output("Mean").is_some());
    let unseeded = model.run_runs(&opensysml::RunsOptions {
        action: Some("MC::Settle".to_owned()),
        runs: 1,
        ..Default::default()
    });
    assert!(
        matches!(
            unseeded,
            Err(opensysml::Error::Service {
                status: opensysml::Status::InvalidArgument,
                ..
            })
        ),
        "{unseeded:?}"
    );
    let unknown = model.run_runs(&opensysml::RunsOptions {
        action: Some("MC::Nope".to_owned()),
        runs: 1,
        draws: Some("min".to_owned()),
        ..Default::default()
    });
    assert!(
        matches!(unknown, Err(opensysml::Error::Execution { .. })),
        "{unknown:?}"
    );
}

#[test]
fn the_self_check_counts_its_checks_and_refuses_an_unknown_rule_package() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(&connection, RUNS_MODEL);
    let check = model.self_check(&[]).unwrap();
    assert!(check.elements > 0 && check.checks > 0, "{check:?}");
    assert!(check.summary.starts_with("Self-model check: "));
    assert!(!check.applications.is_empty());
    let unknown = model.self_check(&["Nope".to_owned()]);
    assert!(
        matches!(unknown, Err(opensysml::Error::Execution { .. })),
        "{unknown:?}"
    );
}

#[test]
fn recording_a_run_and_a_sample_grows_the_model() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(&connection, RUNS_MODEL);
    let recording = model
        .record_run(
            "MC::Cost",
            &opensysml::RecordOptions {
                subject: Some("MC::p".to_owned()),
                ..Default::default()
            },
        )
        .unwrap();
    assert!(recording.run.is_some());
    assert_eq!(recording.package, "Records");
    assert_eq!(recording.records, vec!["Records::Cost_run1".to_owned()]);
    assert_eq!(recording.documents.len(), 1);
    assert!(!recording.model_hash.is_empty());
    let sample = model
        .record_run(
            "MC::Mc",
            &opensysml::RecordOptions {
                subject: Some("MC::probe".to_owned()),
                runs: 2,
                seed: Some(3),
                ..Default::default()
            },
        )
        .unwrap();
    assert!(sample.sample.is_some());
    assert_eq!(sample.records.len(), 3);
    let unknown = model.record_run("MC::Nope", &Default::default());
    assert!(
        matches!(unknown, Err(opensysml::Error::Execution { .. })),
        "{unknown:?}"
    );
}

const IMPORT_MODEL: &str = r#"package V {
    private import ScalarValues::*;
    part def Vehicle {
        attribute count : Integer default 1;
        attribute label : String default "plain";
    }
    part vehicle : Vehicle;
    part car : Vehicle {
        attribute :>> count = 2;
    }
}"#;

const PROBE_MODEL: &str = r#"package Cfg {
    private import ScalarValues::*;
    part def Probe {
        attribute total : Real = 1.0;
        action steady { first start; then assign total := 4.0; then done; }
    }
    action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
}"#;

const PROBE_RESULTS: &str = r#"{"source":"probe.xmi","configurations":[
 {"id":"_c1","name":"Cfg::'Group 1'","runs":2,"target":"target","behavior":"run","observables":["total"],
  "snapshots":[{"id":"_r1","values":{"total":4.0}},{"id":"_r2","values":{"total":6.0}}]},
 {"id":"_cn","name":"Cfg::Nothing","observables":[],"snapshots":[]}]}"#;

#[test]
fn import_data_sets_the_values_and_answers_the_edited_model() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(&connection, IMPORT_MODEL);
    let imports = vec![
        opensysml::DataImport {
            name: "fleet.csv".to_owned(),
            content: "element,count,label\nV::vehicle,7,fast\nV::car,9,\n".to_owned(),
            format: None,
            map: None,
        },
        opensysml::DataImport {
            name: "labels.txt".to_owned(),
            content: r#"[{"element":"V::car","label":"slow"}]"#.to_owned(),
            format: Some("json".to_owned()),
            map: None,
        },
    ];
    let result = model
        .import_data(&imports, &opensysml::DataImportOptions::default())
        .unwrap();
    let descriptions: Vec<&str> = result
        .files
        .iter()
        .flat_map(|file| file.assignments.iter().map(|a| a.description.as_str()))
        .collect();
    assert_eq!(
        descriptions,
        vec![
            "V::vehicle::count = 7 (redefines V::Vehicle::count)",
            "V::vehicle::label = \"fast\" (redefines V::Vehicle::label)",
            "V::car::count = 9",
            "V::car::label = \"slow\" (redefines V::Vehicle::label)",
        ]
    );
    assert_eq!(result.files.len(), 2);
    assert_eq!((result.files[0].values, result.files[0].elements), (3, 2));
    assert_eq!(
        result.files[0].assignments[0].redefines,
        "V::Vehicle::count"
    );
    assert!(!result.files[0].assignments[0].where_.is_empty());
    assert!(result.content.contains("attribute :>> count = 7;"));
    assert!(result.content.contains("attribute :>> label = \"slow\";"));
    assert!(!result.model_hash.is_empty() && result.model_hash != model.hash());
    assert_eq!(result.documents.len(), 1);

    let dry = model
        .import_data(
            &[opensysml::DataImport {
                name: "fleet.tsv".to_owned(),
                content: "element\tcount\nV::vehicle\t7\n".to_owned(),
                format: None,
                map: None,
            }],
            &opensysml::DataImportOptions {
                dry_run: true,
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(dry.files.len(), 1);
    assert_eq!(dry.files[0].assignments.len(), 1);
    assert!(dry.model_hash.is_empty() && dry.content.is_empty() && dry.documents.is_empty());

    let values = model
        .import_data(
            &[opensysml::DataImport {
                name: "fleet.tsv".to_owned(),
                content: "element\tcount\nV::vehicle\t7\n".to_owned(),
                format: None,
                map: None,
            }],
            &opensysml::DataImportOptions {
                import_as: Some("values".to_owned()),
                dry_run: true,
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(values.files[0].assignments.len(), 1);
    let unknown_form = model.import_data(
        &[opensysml::DataImport {
            name: "fleet.tsv".to_owned(),
            content: "element\tcount\nV::vehicle\t7\n".to_owned(),
            format: None,
            map: None,
        }],
        &opensysml::DataImportOptions {
            import_as: Some("rows".to_owned()),
            dry_run: true,
            ..Default::default()
        },
    );
    assert!(
        matches!(
            &unknown_form,
            Err(Error::Service { status: Status::InvalidArgument, message, .. }) if message.contains("rows")
        ),
        "{unknown_form:?}"
    );
    let refused = model.import_data(
        &[opensysml::DataImport {
            name: "fleet.csv".to_owned(),
            content: "element,count\nV::car,9\nV::Nowhere,1\n".to_owned(),
            format: None,
            map: None,
        }],
        &opensysml::DataImportOptions::default(),
    );
    assert!(
        matches!(&refused, Err(opensysml::Error::Execution { message, .. }) if message.contains("V::Nowhere")),
        "{refused:?}"
    );
    let empty = model.import_data(&[], &opensysml::DataImportOptions::default());
    assert!(
        matches!(
            empty,
            Err(opensysml::Error::Service {
                status: opensysml::Status::InvalidArgument,
                ..
            })
        ),
        "{empty:?}"
    );
}

#[test]
fn compare_results_tables_the_tool_beside_the_runs() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(&connection, PROBE_MODEL);
    let comparison = model
        .compare_results(
            PROBE_RESULTS,
            &opensysml::CompareOptions {
                seed: Some(7),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(comparison.source, "probe.xmi");
    assert!(!comparison.holds());
    assert_eq!(comparison.configurations.len(), 2);
    let group = &comparison.configurations[0];
    assert_eq!(group.name, "Cfg::'Group 1'");
    assert_eq!(group.status, "holds");
    assert_eq!((group.runs, group.completed, group.stored_runs), (2, 2, 2));
    assert_eq!(group.seed, Some(7));
    assert_eq!(group.rows.len(), 2);
    assert_eq!(group.observables.len(), 1);
    let total = &group.observables[0];
    assert_eq!(
        (total.stored.as_str(), total.feature.as_str()),
        ("total", "target.total")
    );
    assert!(total.compared);
    let tool = total.tool.as_ref().unwrap();
    let run = total.run.as_ref().unwrap();
    assert_eq!((tool.count, tool.mean, run.mean), (2, 5.0, 4.0));
    assert_eq!(total.difference.as_ref().unwrap().mean, "-20.0%");
    assert!(!group.lines.is_empty());
    let nothing = &comparison.configurations[1];
    assert_eq!(nothing.status, "unresolved");
    assert!(!nothing.unresolved.is_empty());

    let selected = model
        .compare_results(
            PROBE_RESULTS,
            &opensysml::CompareOptions {
                actions: vec!["Group 1".to_owned()],
                runs: 1,
                seed: Some(3),
                ..Default::default()
            },
        )
        .unwrap();
    assert_eq!(selected.configurations.len(), 1);
    assert_eq!(selected.configurations[0].runs, 1);
    assert!(selected.holds());

    let negative = model.compare_results(
        PROBE_RESULTS,
        &opensysml::CompareOptions {
            runs: -1,
            ..Default::default()
        },
    );
    assert!(
        matches!(
            negative,
            Err(opensysml::Error::Service {
                status: opensysml::Status::InvalidArgument,
                ..
            })
        ),
        "{negative:?}"
    );
    let malformed = model.compare_results("not json", &opensysml::CompareOptions::default());
    assert!(
        matches!(
            malformed,
            Err(opensysml::Error::Service {
                status: opensysml::Status::InvalidArgument,
                ..
            })
        ),
        "{malformed:?}"
    );
}

#[test]
fn a_checked_behavior_witnesses_its_violation_and_divergence() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let model = parsed(
        &connection,
        include_str!("../../../../conformance/fixtures/tank.sysml"),
    );
    let options = opensysml::CheckOptions {
        properties: vec!["Plant::Tank::low".to_owned()],
        diverge: vec!["this.level".to_owned()],
        witness: true,
        performer: Some("Plant::tank".to_owned()),
        ..Default::default()
    };
    let result = model
        .check_action("Plant::Tank::fill", &Default::default(), &options)
        .unwrap();
    assert_eq!(result.verdict, "fails");
    assert!(!result.holds());
    assert!(!result.lines.is_empty());
    let search = result.search.as_ref().expect("an explicit-state search");
    assert_eq!((search.states, search.moves, search.max_depth), (11, 10, 6));
    assert_eq!(result.violations.len(), 1, "{:?}", result.violations);
    let violation = &result.violations[0];
    assert_eq!(
        (
            violation.kind.as_str(),
            violation.property.as_str(),
            violation.depth
        ),
        ("property", "Plant::Tank::low", 3)
    );
    let witness = violation.witness.as_ref().expect("a witness");
    assert!(
        witness.schedule.starts_with("replay"),
        "{}",
        witness.schedule
    );
    assert!(!witness.file_name.is_empty() && witness.file_content.contains('\n'));
    assert_eq!(result.divergences.len(), 1);
    assert_eq!(result.divergences[0].feature, "this.level");
    let values: Vec<&str> = result.divergences[0]
        .values
        .iter()
        .map(|v| v.value.as_str())
        .collect();
    assert_eq!(values, ["1", "2"]);
    assert_eq!(result.standing.engine, "check");

    let held = model
        .check_action(
            "Plant::Tank::fill",
            &Default::default(),
            &opensysml::CheckOptions {
                properties: vec!["Plant::Tank::capped".to_owned()],
                diverge: vec!["this.capacity".to_owned()],
                performer: Some("Plant::tank".to_owned()),
                ..Default::default()
            },
        )
        .unwrap();
    assert!(held.holds(), "{:?}", held.lines);
    assert!(held.violations.is_empty() && held.divergences.is_empty());

    let missing = model
        .check_action(
            "Plant::Tank::drain",
            &Default::default(),
            &Default::default(),
        )
        .unwrap_err();
    assert!(
        matches!(missing, opensysml::Error::Execution { .. }),
        "{missing:?}"
    );
    let negative = model
        .check_action(
            "Plant::Tank::fill",
            &Default::default(),
            &opensysml::CheckOptions {
                depth: -1,
                ..Default::default()
            },
        )
        .unwrap_err();
    assert!(
        matches!(&negative, opensysml::Error::Service { message, .. } if message.contains("check_depth")),
        "{negative:?}"
    );
}

/// Start a service of this checkout with the external-tool demo's manifests registered, so a
/// dry run has a tool to preview; the registry refuses a manifest directory others may write.
#[cfg(unix)]
fn service_with_demo_tools() -> Option<(std::process::Child, Connection, std::path::PathBuf)> {
    use std::io::{BufRead, BufReader};
    use std::os::unix::fs::{DirBuilderExt, PermissionsExt};
    use std::path::{Path, PathBuf};
    use std::process::{Command, Stdio};

    let checkout = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..");
    let binary = env::var("OPENSYSML_GRPC_BINARY")
        .map(PathBuf::from)
        .unwrap_or_else(|_| checkout.join("bin/sysml-grpc"));
    if !binary.is_file() {
        eprintln!("skipping: no sysml-grpc at {}", binary.display());
        return None;
    }
    let tools = env::temp_dir().join(format!("opensysml-rust-tools-{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&tools);
    std::fs::DirBuilder::new()
        .mode(0o700)
        .create(&tools)
        .unwrap();
    let demo = checkout.join("examples/external-tool-demo");
    for name in ["thermal.json", "thermal.py"] {
        let copied = tools.join(name);
        std::fs::copy(demo.join("tools").join(name), &copied).unwrap();
        std::fs::set_permissions(&copied, std::fs::Permissions::from_mode(0o644)).unwrap();
    }
    let mut child = Command::new(binary)
        .args(["-port", "0", "-health-port", "0", "-report-address"])
        .env("OPENSYSML_TOOLS", &tools)
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .unwrap();
    let mut address = String::new();
    BufReader::new(child.stdout.take().unwrap())
        .read_line(&mut address)
        .unwrap();
    let (host, port) = address.trim().rsplit_once(':').unwrap();
    let connection = Connection::external(
        host.trim_start_matches('[').trim_end_matches(']'),
        port.parse().unwrap(),
    )
    .unwrap();
    Some((child, connection, tools))
}

#[cfg(unix)]
#[test]
fn a_tool_dry_run_previews_the_tool_call_instead_of_running_it() {
    if service_or_skip().is_none() {
        return;
    }
    let Some((mut child, connection, tools)) = service_with_demo_tools() else {
        return;
    };
    let source = std::fs::read_to_string(
        std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../examples/external-tool-demo/thermal.sysml"),
    )
    .unwrap();
    let outcome = std::panic::catch_unwind(|| {
        let model = parsed(&connection, &source);
        let result = model
            .run_analysis(
                "ThermalDemo::heating",
                &AnalysisOptions {
                    tool_dry_run: true,
                    ..Default::default()
                },
            )
            .unwrap();
        assert!(result.outputs.is_empty());
        let preview = result
            .tool_dry_run
            .expect("the result carries the tool preview");
        assert_eq!(preview.tool, "ThermalSolver");
        assert_eq!(preview.version, "1.0");
        assert!(preview.action.contains("Solve"), "{}", preview.action);
        assert!(
            preview.args.join(" ").contains("--mass 12.5"),
            "{:?}",
            preview.args
        );
        assert!(!preview.lines.is_empty());
        let real = model
            .run_analysis("ThermalDemo::heating", &AnalysisOptions::default())
            .map(|result| result.tool_dry_run.is_none());
        assert!(matches!(real, Ok(true) | Err(_)), "{real:?}");
    });
    let _ = child.kill();
    let _ = child.wait();
    let _ = std::fs::remove_dir_all(&tools);
    if let Err(panic) = outcome {
        std::panic::resume_unwind(panic);
    }
}
