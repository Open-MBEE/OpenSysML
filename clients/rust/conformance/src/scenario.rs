use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};

use serde::Deserialize;
use serde_json::Value;

#[derive(Clone, Debug, Deserialize)]
pub struct SuiteFile {
    pub scenarios: Vec<Scenario>,
}

/// One fixture parsed alone, or several parsed together as one model, which
/// the v1 API's single-document parse cannot make.
#[derive(Clone, Debug, Deserialize, Eq, Hash, PartialEq)]
pub struct ModelSpec {
    #[serde(default)]
    pub fixture: String,
    #[serde(default)]
    pub fixtures: Vec<String>,
    #[serde(default)]
    pub language: String,
    #[serde(default)]
    pub strict_conformance: bool,
}

#[derive(Clone, Debug, Default, Deserialize)]
pub struct Expect {
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default)]
    pub status_message_contains: Option<String>,
    #[serde(default)]
    pub response: Option<Value>,
    #[serde(default)]
    pub contains: std::collections::BTreeMap<String, String>,
    #[serde(default)]
    pub contains_all: std::collections::BTreeMap<String, Vec<String>>,
    #[serde(default)]
    pub non_empty: Vec<String>,
    #[serde(default)]
    pub absent: Vec<String>,
    #[serde(default)]
    pub counts: std::collections::BTreeMap<String, usize>,
    #[serde(default)]
    pub min_counts: std::collections::BTreeMap<String, usize>,
}

#[derive(Clone, Debug, Deserialize)]
pub struct Scenario {
    pub id: String,
    pub rpc: String,
    #[serde(default)]
    pub requires_capabilities: Vec<String>,
    #[serde(default)]
    pub expect_without_capability: Option<Expect>,
    #[serde(default)]
    pub model: Option<ModelSpec>,
    #[serde(default)]
    pub request: Value,
    #[serde(default)]
    pub expect: Expect,
}

impl Scenario {
    pub fn method(&self) -> &str {
        self.rpc.rsplit('/').next().unwrap_or(&self.rpc)
    }
}

pub fn load_scenarios(dir: &Path) -> Result<Vec<Scenario>, String> {
    let mut paths = fs::read_dir(dir)
        .map_err(|error| format!("reading scenarios directory {}: {error}", dir.display()))?
        .map(|entry| entry.map(|item| item.path()))
        .collect::<Result<Vec<PathBuf>, _>>()
        .map_err(|error| format!("reading scenario entry: {error}"))?;
    paths.sort();
    let mut scenarios = Vec::new();
    let mut ids = HashSet::new();
    for path in paths
        .into_iter()
        .filter(|path| path.extension().is_some_and(|ext| ext == "json"))
    {
        let data = fs::read_to_string(&path)
            .map_err(|error| format!("reading {}: {error}", path.display()))?;
        let suite: SuiteFile = serde_json::from_str(&data)
            .map_err(|error| format!("parsing {}: {error}", path.display()))?;
        for scenario in suite.scenarios {
            if scenario.id.is_empty() || scenario.rpc.is_empty() {
                return Err(format!("{}: scenario needs id and rpc", path.display()));
            }
            if !ids.insert(scenario.id.clone()) {
                return Err(format!("duplicate scenario id {:?}", scenario.id));
            }
            scenarios.push(scenario);
        }
    }
    if scenarios.is_empty() {
        return Err(format!("no scenario files in {}", dir.display()));
    }
    Ok(scenarios)
}
