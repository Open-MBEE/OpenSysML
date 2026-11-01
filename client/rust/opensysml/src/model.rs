//! Name lookup over a model and the model-scoped forms of the service's operations.

use std::collections::{BTreeMap, HashMap, HashSet, VecDeque};
use std::path::Path;

use crate::capabilities::{upgrade_remedy, CAPABILITY_QUERY};
use crate::conversion::{
    format_of_path, Conversion, ConvertOptions, ConvertSource, FORMAT_API_JSON, FORMAT_SYSML,
    FORMAT_TURTLE,
};
use crate::document::{DocumentForm, DocumentQueryResult, DocumentValue};
use crate::domain::{Model, Symbol, Value};
use crate::edit::Editor;
use crate::error::Error;
use crate::operations::{
    AnalysisOptions, RecordOptions, RunOptions, RunsOptions, SweepOptions, VerifyOptions,
};
use crate::query::{Constraint, Query, QueryElement};
use crate::results::{
    ActionRun, AnalysisResult, CalcResult, Exploration, Recording, RunsResult, Satisfaction,
    SelfCheckResult, StateRun, SweepRange, SweepTable, Validation, Verdict,
};

const PROPERTY_ID: &str = "@id";
const PROPERTY_NAME: &str = "name";
const PROPERTY_OWNER: &str = "owner";
const SUGGESTIONS: usize = 3;
const SUGGESTION_CUTOFF: f64 = 0.6;

impl Model {
    /// Convert the model to `to_format`, as [`crate::Connection::convert`] does.
    pub fn convert(
        &self,
        to_format: &str,
        tolerate_syntax_errors: bool,
    ) -> Result<Conversion, Error> {
        self.convert_with(
            to_format,
            &ConvertOptions {
                from_format: FORMAT_SYSML.to_owned(),
                tolerate_syntax_errors,
                ..Default::default()
            },
        )
    }
    /// Convert the model to `to_format` with explicit options, as
    /// [`crate::Connection::convert`] does for a model the service holds: `id_form`,
    /// `include_library_stubs` and `documents` are gated and sent as it gates and sends them.
    pub fn convert_with(
        &self,
        to_format: &str,
        options: &ConvertOptions,
    ) -> Result<Conversion, Error> {
        self.connection.convert(
            to_format,
            &ConvertSource::Model(self.hash().to_owned()),
            options,
        )
    }
    /// The model as SysML notation.
    pub fn to_sysml(&self, tolerate_syntax_errors: bool) -> Result<Conversion, Error> {
        self.convert(FORMAT_SYSML, tolerate_syntax_errors)
    }
    /// The model as RDF Turtle.
    pub fn to_turtle(&self) -> Result<Conversion, Error> {
        self.convert(FORMAT_TURTLE, false)
    }
    /// The model as SysML v2 API JSON.
    pub fn to_api_json(&self) -> Result<Conversion, Error> {
        self.convert(FORMAT_API_JSON, false)
    }
    /// Convert the model and write it to `path`, in `to_format` or the format its extension names.
    pub fn save(
        &self,
        path: impl AsRef<Path>,
        to_format: Option<&str>,
        tolerate_syntax_errors: bool,
    ) -> Result<Conversion, Error> {
        self.save_with(
            path,
            to_format,
            &ConvertOptions {
                from_format: FORMAT_SYSML.to_owned(),
                tolerate_syntax_errors,
                ..Default::default()
            },
        )
    }
    /// Convert the model with explicit options, as [`Model::convert_with`] does, and write
    /// it to `path`, in `to_format` or the format its extension names.
    pub fn save_with(
        &self,
        path: impl AsRef<Path>,
        to_format: Option<&str>,
        options: &ConvertOptions,
    ) -> Result<Conversion, Error> {
        let path = path.as_ref();
        let format = match to_format {
            Some(format) => format,
            None => format_of_path(path)?,
        };
        let conversion = self.convert_with(format, options)?;
        conversion.write(path)?;
        Ok(conversion)
    }

    /// An editor collecting source-preserving edits of this model.
    pub fn edit(&self) -> Editor {
        Editor::new(
            self.hash().to_owned(),
            self.connection.clone(),
            self.documents().len() > 1,
        )
    }

    /// Run a SysML v2 API & Services query over the model.
    pub fn query(&self, query: &Query) -> Result<Vec<QueryElement>, Error> {
        self.connection.query(self.hash(), query)
    }
    /// Select elements with OSLC Query 3.0 parameter text.
    pub fn query_oslc(&self, oslc: &str) -> Result<Vec<QueryElement>, Error> {
        self.connection.query_oslc(self.hash(), oslc)
    }
    /// Run a named document query the model declares.
    pub fn run_document_query<K: AsRef<str>>(
        &self,
        query_id: &str,
        bindings: &[(K, Vec<DocumentValue>)],
    ) -> Result<DocumentQueryResult, Error> {
        self.connection
            .run_document_query(self.hash(), query_id, bindings)
    }
    /// Render a named document the model declares.
    pub fn render_document(&self, document_id: &str, form: DocumentForm) -> Result<String, Error> {
        self.connection
            .render_document(self.hash(), document_id, form)
    }
    /// Render a named view with minimal ports.
    pub fn render_view(&self, view_name: &str) -> Result<crate::document::RenderedView, Error> {
        self.connection.render_view(self.hash(), view_name)
    }
    /// Render a named view with the requested port selection.
    pub fn render_view_with_ports(
        &self,
        view_name: &str,
        ports: crate::document::RenderViewPorts,
    ) -> Result<crate::document::RenderedView, Error> {
        self.connection
            .render_view_with_ports(self.hash(), view_name, ports)
    }
    /// Render a named view showing the named optional notation elements besides
    /// those its rendering shows.
    pub fn render_view_showing(
        &self,
        view_name: &str,
        ports: crate::document::RenderViewPorts,
        show: &[&str],
    ) -> Result<crate::document::RenderedView, Error> {
        self.connection
            .render_view_showing(self.hash(), view_name, ports, show)
    }
    /// Render a named view as `render_view_showing` does, drawing `depth` levels of
    /// nested elements below each exposed element when `depth` is above 0.
    pub fn render_view_bounded(
        &self,
        view_name: &str,
        ports: crate::document::RenderViewPorts,
        show: &[&str],
        depth: u32,
    ) -> Result<crate::document::RenderedView, Error> {
        self.connection
            .render_view_bounded(self.hash(), view_name, ports, show, depth)
    }
    /// Render a named view as `render_view_bounded` does, folding a tree's owned
    /// members into compartment rows of their owner as `compartments` says.
    pub fn render_view_folded(
        &self,
        view_name: &str,
        ports: crate::document::RenderViewPorts,
        show: &[&str],
        depth: u32,
        compartments: crate::document::RenderViewCompartments,
    ) -> Result<crate::document::RenderedView, Error> {
        self.connection
            .render_view_folded(self.hash(), view_name, ports, show, depth, compartments)
    }

    /// Search every schedule of an action for a violated property, a deadlock, a
    /// failure or a divergent value, as `sysml -check-property` does.
    pub fn check_action(
        &self,
        action_id: &str,
        inputs: &BTreeMap<String, Value>,
        options: &crate::CheckOptions,
    ) -> Result<crate::CheckResult, Error> {
        self.connection
            .check_action(self.hash(), action_id, inputs, options)
    }

    /// Search every schedule of a state machine; see [`Model::check_action`].
    pub fn check_state_machine(
        &self,
        state_machine_id: &str,
        options: &crate::CheckOptions,
    ) -> Result<crate::CheckResult, Error> {
        self.connection
            .check_state_machine(self.hash(), state_machine_id, options)
    }

    /// Export the lowered graph of an action or state machine as `graphs:1` JSON.
    pub fn export_graphs(&self, subject: &str) -> Result<crate::Graphs, Error> {
        self.connection.export_graphs(self.hash(), subject)
    }

    /// Sets the feature values data files assign into this model.
    pub fn import_data(
        &self,
        imports: &[crate::DataImport],
        options: &crate::DataImportOptions,
    ) -> Result<crate::ImportResult, Error> {
        self.connection.import_data(self.hash(), imports, options)
    }

    /// Runs the configurations a results sidecar indexes against this model
    /// and compares the runs with the tool's.
    pub fn compare_results(
        &self,
        results: &str,
        options: &crate::CompareOptions,
    ) -> Result<crate::Comparison, Error> {
        self.connection
            .compare_results(self.hash(), results, options)
    }

    /// The symbol a qualified or short name names; `None` when the model declares none.
    ///
    /// A short name finds the shallowest declaration of it.
    pub fn find(&self, name: &str) -> Result<Option<Symbol>, Error> {
        if let Some(root) = self
            .roots()
            .iter()
            .find(|r| r.name() == name || r.id() == name)
        {
            return Ok(Some(root.clone()));
        }
        if name.contains("::") {
            if let Some(symbol) = self.symbol_by_id(name)? {
                return Ok(Some(symbol));
            }
            return self.symbol_named(name);
        }
        if let Some(symbol) = self.symbol_named(name)? {
            return Ok(Some(symbol));
        }
        self.symbol_by_id(name)
    }

    /// The symbol a qualified name names; `None` when the model declares none.
    pub fn get(&self, fqn: &str) -> Result<Option<Symbol>, Error> {
        if let Some(root) = self.roots().iter().find(|r| r.id() == fqn) {
            return Ok(Some(root.clone()));
        }
        self.symbol_by_id(fqn)
    }

    /// The symbol a name names, else [`Error::SymbolNotFound`] with the model's nearest names.
    pub fn lookup(&self, name: &str) -> Result<Symbol, Error> {
        match self.find(name)? {
            Some(symbol) => Ok(symbol),
            None => Err(Error::SymbolNotFound {
                name: name.to_owned(),
                suggestions: self.near_names(name)?,
            }),
        }
    }

    /// Whether the model declares a symbol by that name.
    pub fn contains(&self, name: &str) -> Result<bool, Error> {
        Ok(self.find(name)?.is_some())
    }

    fn symbol_by_id(&self, fqn: &str) -> Result<Option<Symbol>, Error> {
        match self.connection.get_symbol(self.hash(), fqn) {
            Ok(symbol) if symbol.id() == fqn => Ok(Some(symbol)),
            Ok(_) | Err(Error::Model(_)) => Ok(None),
            Err(error) => Err(error),
        }
    }

    fn symbol_named(&self, name: &str) -> Result<Option<Symbol>, Error> {
        if !self.connection.capabilities().has(CAPABILITY_QUERY) {
            if self.roots().is_empty() {
                self.connection
                    .capabilities()
                    .require(CAPABILITY_QUERY, upgrade_remedy(CAPABILITY_QUERY))?;
            }
            return self.walk_to(name, None);
        }
        let named = self.query_where(Constraint::equals(PROPERTY_NAME, name))?;
        let mut ids: Vec<String> = named.iter().map(|e| e.id.clone()).collect();
        let mut depth = HashMap::new();
        if ids.len() > 1 || (!ids.is_empty() && !self.ok()) {
            depth = self.depths(&named)?;
            ids.sort_by_key(|id| depth[id]);
        }
        if !self.ok() {
            let limit = ids.first().map(|id| depth[id]);
            if let Some(symbol) = self.walk_to(name, limit)? {
                return Ok(Some(symbol));
            }
        }
        for fqn in ids {
            if let Some(symbol) = self.symbol_by_id(&fqn)? {
                return Ok(Some(symbol));
            }
        }
        Ok(None)
    }

    fn query_where(&self, filter: Constraint) -> Result<Vec<QueryElement>, Error> {
        self.query(&Query::new().select([PROPERTY_OWNER]).filter(filter))
    }

    fn depths(&self, elements: &[QueryElement]) -> Result<HashMap<String, usize>, Error> {
        let mut owner: HashMap<String, String> = self
            .roots()
            .iter()
            .map(|r| (r.id().to_owned(), String::new()))
            .collect();
        let owner_of = |e: &QueryElement| e.get(PROPERTY_OWNER).unwrap_or_default().to_owned();
        owner.extend(elements.iter().map(|e| (e.id.clone(), owner_of(e))));
        loop {
            let mut unknown: Vec<String> = owner
                .values()
                .filter(|fqn| !fqn.is_empty() && !owner.contains_key(*fqn))
                .cloned()
                .collect::<HashSet<_>>()
                .into_iter()
                .collect();
            if unknown.is_empty() {
                break;
            }
            unknown.sort();
            for element in self.query_where(Constraint::one_of(PROPERTY_ID, unknown.clone()))? {
                owner.insert(element.id.clone(), owner_of(&element));
            }
            for fqn in unknown {
                owner.entry(fqn).or_default();
            }
        }
        let roots: HashSet<&str> = self.roots().iter().map(Symbol::id).collect();
        let depth = |id: &str| depth_below(&owner, &roots, id);
        Ok(elements
            .iter()
            .map(|e| (e.id.clone(), depth(&e.id)))
            .collect())
    }

    fn walk_to(&self, name: &str, depth: Option<usize>) -> Result<Option<Symbol>, Error> {
        let mut found = None;
        self.walk(depth, |symbol| {
            if symbol.name() == name {
                found = Some(symbol.clone());
            }
            found.is_none()
        })?;
        Ok(found)
    }

    /// Visit every symbol below the roots breadth first, to `depth` levels, while `visit` says go on.
    fn walk(
        &self,
        depth: Option<usize>,
        mut visit: impl FnMut(&Symbol) -> bool,
    ) -> Result<(), Error> {
        let mut queue: VecDeque<(Symbol, usize)> =
            self.roots().iter().map(|r| (r.clone(), 0)).collect();
        while let Some((current, level)) = queue.pop_front() {
            if depth.is_some_and(|d| level >= d) {
                continue;
            }
            for child in current.children()? {
                if !visit(&child) {
                    return Ok(());
                }
                queue.push_back((child, level + 1));
            }
        }
        Ok(())
    }

    fn near_names(&self, name: &str) -> Result<Vec<String>, Error> {
        let mut declared = Vec::new();
        if self.connection.capabilities().has(CAPABILITY_QUERY) {
            for element in self.query(&Query::new().select([PROPERTY_NAME]))? {
                let short = element.get(PROPERTY_NAME).unwrap_or_default().to_owned();
                declared.push((element.id, short));
            }
        } else {
            self.walk(None, |child| {
                declared.push((child.id().to_owned(), child.name().to_owned()));
                true
            })?;
        }
        let mut candidates = Vec::new();
        for (fqn, short) in declared {
            if !short.is_empty() {
                candidates.push(short.clone());
            }
            if !fqn.is_empty() && fqn != short {
                candidates.push(fqn);
            }
        }
        Ok(close_matches(name, candidates))
    }

    /// Execute an action once, as [`crate::Connection::execute_action`] does.
    pub fn execute_action(
        &self,
        action_id: &str,
        inputs: &BTreeMap<String, Value>,
        options: &RunOptions,
    ) -> Result<ActionRun, Error> {
        self.connection
            .execute_action(self.hash(), action_id, inputs, options)
    }
    /// Explore an action, as [`crate::Connection::explore_action`] does.
    pub fn explore_action(
        &self,
        action_id: &str,
        inputs: &BTreeMap<String, Value>,
        options: &RunOptions,
    ) -> Result<Exploration, Error> {
        self.connection
            .explore_action(self.hash(), action_id, inputs, options)
    }
    /// Run a state machine once, as [`crate::Connection::execute_state`] does.
    pub fn execute_state<S: AsRef<str>>(
        &self,
        machine_id: &str,
        events: &[S],
        options: &RunOptions,
    ) -> Result<StateRun, Error> {
        self.connection
            .execute_state(self.hash(), machine_id, events, options)
    }
    /// Explore a state machine, as [`crate::Connection::explore_state`] does.
    pub fn explore_state<S: AsRef<str>>(
        &self,
        machine_id: &str,
        events: &[S],
        options: &RunOptions,
    ) -> Result<Exploration, Error> {
        self.connection
            .explore_state(self.hash(), machine_id, events, options)
    }
    /// Ask whether a constraint holds.
    pub fn verify_constraint(
        &self,
        constraint_id: &str,
        options: &VerifyOptions,
    ) -> Result<Verdict, Error> {
        self.connection
            .verify_constraint(self.hash(), constraint_id, options)
    }
    /// Ask whether a requirement holds.
    pub fn verify_requirement(
        &self,
        requirement_id: &str,
        options: &VerifyOptions,
    ) -> Result<Verdict, Error> {
        self.connection
            .verify_requirement(self.hash(), requirement_id, options)
    }
    /// Ask whether the satisfaction assertions within `scope`, or every one, hold.
    pub fn verify_satisfaction(
        &self,
        scope: Option<&str>,
        options: &VerifyOptions,
    ) -> Result<Satisfaction, Error> {
        self.connection
            .verify_satisfaction(self.hash(), scope, options)
    }
    /// Whether every satisfaction assertion within `scope`, or every one the model states, holds.
    pub fn satisfied(&self, scope: Option<&str>) -> Result<bool, Error> {
        Ok(self
            .verify_satisfaction(scope, &VerifyOptions::default())?
            .holds())
    }
    /// Check every assertion about an object of `part_id`.
    pub fn validate_instance(
        &self,
        part_id: &str,
        engine: Option<&str>,
    ) -> Result<Validation, Error> {
        self.connection
            .validate_instance(self.hash(), part_id, engine)
    }
    /// Invoke a calculation.
    pub fn calc(
        &self,
        calc_id: &str,
        arguments: &[Value],
        engine: Option<&str>,
    ) -> Result<CalcResult, Error> {
        self.connection
            .calc(self.hash(), calc_id, arguments, engine)
    }
    /// Run an analysis case.
    pub fn run_analysis(
        &self,
        case_id: &str,
        options: &AnalysisOptions,
    ) -> Result<AnalysisResult, Error> {
        self.connection.run_analysis(self.hash(), case_id, options)
    }
    /// Explore an analysis case.
    pub fn explore_analysis(
        &self,
        case_id: &str,
        options: &AnalysisOptions,
    ) -> Result<Exploration, Error> {
        self.connection
            .explore_analysis(self.hash(), case_id, options)
    }
    /// Sweep an analysis case or calc over `ranges`.
    pub fn run_sweep(
        &self,
        target_id: &str,
        ranges: &[SweepRange],
        options: &SweepOptions,
    ) -> Result<SweepTable, Error> {
        self.connection
            .run_sweep(self.hash(), target_id, ranges, options)
    }

    /// Runs an action, or a Monte Carlo case, of this model `options.runs` times.
    pub fn run_runs(&self, options: &RunsOptions) -> Result<RunsResult, Error> {
        self.connection.run_runs(self.hash(), options)
    }

    /// Applies the reflective SysMLValidation constraints to every element of this model.
    pub fn self_check(&self, packages: &[String]) -> Result<SelfCheckResult, Error> {
        self.connection.self_check(self.hash(), packages)
    }

    /// Runs an analysis case of this model and records the runs into it.
    pub fn record_run(&self, case_id: &str, options: &RecordOptions) -> Result<Recording, Error> {
        self.connection.record_run(self.hash(), case_id, options)
    }
}

/// Ownership hops from `fqn` to a root; one more when its chain ends short of every root.
fn depth_below(owner: &HashMap<String, String>, roots: &HashSet<&str>, fqn: &str) -> usize {
    let mut current = fqn;
    let mut hops = 0;
    while let Some(next) = owner.get(current).filter(|o| !o.is_empty()) {
        current = next;
        hops += 1;
    }
    if roots.contains(current) {
        hops
    } else {
        hops + 1
    }
}

/// The candidates most like `word`, best first: at most three, each at least 60% similar.
fn close_matches(word: &str, candidates: Vec<String>) -> Vec<String> {
    let mut scored: Vec<(f64, String)> = candidates
        .into_iter()
        .collect::<std::collections::BTreeSet<_>>()
        .into_iter()
        .map(|candidate| (similarity(word, &candidate), candidate))
        .filter(|(score, _)| *score >= SUGGESTION_CUTOFF)
        .collect();
    scored.sort_by(|a, b| b.0.total_cmp(&a.0).then_with(|| a.1.cmp(&b.1)));
    scored
        .into_iter()
        .take(SUGGESTIONS)
        .map(|(_, c)| c)
        .collect()
}

fn similarity(a: &str, b: &str) -> f64 {
    let a: Vec<char> = a.chars().collect();
    let b: Vec<char> = b.chars().collect();
    let longest = a.len().max(b.len());
    if longest == 0 {
        return 1.0;
    }
    let mut row: Vec<usize> = (0..=b.len()).collect();
    for (i, ca) in a.iter().enumerate() {
        let mut diagonal = row[0];
        row[0] = i + 1;
        for (j, cb) in b.iter().enumerate() {
            let substituted = diagonal + usize::from(ca != cb);
            diagonal = row[j + 1];
            row[j + 1] = substituted.min(row[j] + 1).min(row[j + 1] + 1);
        }
    }
    1.0 - row[b.len()] as f64 / longest as f64
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn near_names_are_the_closest_few() {
        let candidates = ["Engine", "Engines", "Wheel", "Vehicles::Engine", "Car"]
            .map(String::from)
            .to_vec();
        assert_eq!(
            close_matches("Engin", candidates.clone()),
            ["Engine", "Engines"]
        );
        assert!(close_matches("Zzz", candidates).is_empty());
        assert_eq!(similarity("", ""), 1.0);
    }
}
