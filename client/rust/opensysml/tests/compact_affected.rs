#![allow(missing_docs)]

//! The compact API JSON and check-diagnostics conversion options, and the affected documents a
//! parse relates to a base model, against a real service.

use std::env;

use opensysml::{Connection, ConvertOptions, ConvertSource, Error, SourceDocument, SourcesOptions};

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

const SOURCE: &str = "package P { attribute a : ScalarValues::Integer = 1; }\n";

fn sysml() -> ConvertOptions {
    ConvertOptions {
        from_format: "sysml".to_owned(),
        ..Default::default()
    }
}

#[test]
fn compact_api_json_trims_derived_properties_and_keeps_the_named_ones() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let source = ConvertSource::Content(SOURCE.to_owned());
    let compact = connection
        .convert(
            "api-json",
            &source,
            &ConvertOptions {
                compact: true,
                ..sysml()
            },
        )
        .unwrap();
    let document: serde_json::Value = serde_json::from_str(&compact.content).unwrap();
    assert_eq!(document["format"], "api-json-compact/1");
    let elements = |conversion: &opensysml::Conversion| -> Vec<serde_json::Value> {
        serde_json::from_str::<serde_json::Value>(&conversion.content).unwrap()["elements"]
            .as_array()
            .unwrap()
            .clone()
    };
    let has_owner =
        |elements: &[serde_json::Value]| elements.iter().any(|e| e.get("owner").is_some());
    assert!(has_owner(&elements(&compact)));
    let trimmed = connection
        .convert(
            "api-json",
            &source,
            &ConvertOptions {
                compact: true,
                omit_derived: true,
                ..sysml()
            },
        )
        .unwrap();
    assert!(!has_owner(&elements(&trimmed)));
    let kept = connection
        .convert(
            "api-json",
            &source,
            &ConvertOptions {
                compact: true,
                omit_derived: true,
                keep_derived: vec!["owner".to_owned()],
                ..sysml()
            },
        )
        .unwrap();
    assert!(has_owner(&elements(&kept)));
    assert!(kept.content.len() < compact.content.len());
    let refused = connection
        .convert(
            "api-json",
            &source,
            &ConvertOptions {
                omit_derived: true,
                ..sysml()
            },
        )
        .expect_err("omit_derived without compact is refused");
    assert!(
        matches!(
            &refused,
            Error::Service { status: opensysml::Status::InvalidArgument, message, .. }
                if message.contains("compact")
        ),
        "{refused}"
    );
    let array = connection.convert("api-json", &source, &sysml()).unwrap();
    assert!(serde_json::from_str::<serde_json::Value>(&array.content)
        .unwrap()
        .is_array());
}

#[test]
fn check_diagnostics_report_the_semantic_check_beside_the_conversion() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    let broken = ConvertSource::Content(
        "package Broken { part def Boat { part motor : Missing; } }\n".to_owned(),
    );
    let unchecked = connection.convert("api-json", &broken, &sysml()).unwrap();
    assert!(unchecked.diagnostics.is_empty());
    let checked = connection
        .convert(
            "api-json",
            &broken,
            &ConvertOptions {
                include_check_diagnostics: true,
                ..sysml()
            },
        )
        .unwrap();
    assert!(
        checked.diagnostics.iter().any(|d| d.severity == "error"),
        "{:?}",
        checked.diagnostics
    );
    let clean = connection
        .convert(
            "api-json",
            &ConvertSource::Content(SOURCE.to_owned()),
            &ConvertOptions {
                include_check_diagnostics: true,
                ..sysml()
            },
        )
        .unwrap();
    assert!(clean.diagnostics.iter().all(|d| d.severity != "error"));
}

#[test]
fn a_base_model_names_the_documents_an_edit_reaches() {
    let Some(connection) = service_or_skip() else {
        return;
    };
    // Names of this test's own: the service relates two models document by document once it
    // has seen their set more than once, and a model it first answered fresh, under another
    // test's names, cannot be a base.
    let library =
        "package Lib { part def Engine { attribute power : ScalarValues::Real = 120.0; } }\n";
    let top = "package Top { private import Lib::*; part def Car { part engine : Engine; } part car : Car; }\n";
    let documents = |content: &str| {
        vec![
            SourceDocument::inline("affected-lib.sysml", library),
            SourceDocument::inline("affected-top.sysml", content),
        ]
    };
    let defaults = SourcesOptions::default();
    connection
        .parse_sources(&documents(&format!("{top}// first\n")), &defaults)
        .unwrap();
    let base = connection
        .parse_sources(&documents(top), &defaults)
        .unwrap();
    assert!(base.affected().is_empty());
    let related = SourcesOptions {
        base_model_hash: base.hash().to_owned(),
        ..Default::default()
    };
    let same = connection.parse_sources(&documents(top), &related).unwrap();
    assert_eq!(same.hash(), base.hash());
    assert!(same.affected().is_empty());
    let edited = connection
        .parse_sources(&documents(&top.replace("part car", "part sedan")), &related)
        .unwrap();
    assert_ne!(edited.hash(), base.hash());
    assert_eq!(edited.affected(), ["affected-top.sysml"]);
    assert_eq!(
        edited.documents(),
        ["affected-lib.sysml", "affected-top.sysml"]
    );
}
