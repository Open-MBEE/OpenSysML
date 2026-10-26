# Airflow provider

[`airflow-provider-opensysml`](https://github.com/Open-MBEE/airflow-provider-opensysml) brings
a SysML v2 model into an [Apache Airflow](https://airflow.apache.org) 3 deployment as something
a DAG can depend on, ask, wait for and be generated from. It is a client of the `sysml-grpc`
service through the [Python client](python-api.md): every task starts a private service the way
the client does, or names a running one through an `opensysml` connection, and the questions it
asks are the service's own — `RunAnalysis`, `VerifyConstraint`, `VerifyRequirement`,
`VerifySatisfaction`, `ValidateInstance` and `ExportGraphs`. The provider lives in its own
repository and is released on its own `v*` tags; this page is the map of what it offers and
where it meets OpenSysML. Its README is the reference for each class.

## Installing

```sh
pip install airflow-provider-opensysml
```

installs the provider and, through it, `opensysml` 0.11.0 or later — the first release whose
client and service carry graph export and every question the provider asks. Until the
provider's first PyPI release, `pip install git+https://github.com/Open-MBEE/airflow-provider-opensysml`
installs it from the repository. The `opensysml` client resolves the service binary as the
[Python API](python-api.md) describes: `$OPENSYSML_BINARY`, then its cache, then a download of
the release it pins — so a worker with no override runs the released `sysml-grpc`, and an
`opensysml` connection with a `host` and `port` makes the task talk to a service that is already
running instead. The connection's `extra` may name the `version` the service must report and
the `require_capabilities` it must advertise.

## What the provider offers

| Piece | What it does |
|---|---|
| `sysml_model_asset(path, ...)` | An Airflow asset over a model file or directory (every `*.sysml`/`*.kerml` under it); its watcher, `SysMLModelChangedTrigger`, runs in the triggerer, digests the file *contents*, and emits one asset event per settled change, so a DAG scheduled on it runs when the model changes and not when a checkout rewrites identical bytes |
| `SysMLAnalysisOperator(model_path, case, ...)` | Runs an analysis or verification case as `sysml -analysis` does and returns its `outputs`, `verdicts`, `verifications`, `evaluations` and the engine's `standing`; fails when an objective or assertion does not hold, a verification's body is anything but `pass`, or an evaluation fails |
| `SysMLVerifyOperator(model_path, kind, element, ...)` | Asks one `constraint`, `requirement`, `satisfy` or `object` question and returns the verdict; `fail_on_verdict` decides whether a false answer fails the task |
| `SysMLRequirementSensor` | Holds a task until a requirement, constraint, satisfaction or object check holds, or until a `case` run with values bound to its `in` parameters passes; `deferrable=True` waits in the triggerer through `SysMLRequirementHoldsTrigger`, re-asking at each change of the model's files |
| `sysml_requirement_asset(path, element)` | An asset whose event is a requirement turning from not holding to holding; `SysMLRequirementSatisfiedTrigger` asks at each model change and never fires for a model that goes on holding it |
| `SysMLActionDag` | Turns an `action def` into the DAG itself from the `graphs:1` export (`sysml -graphs <action>`, or `Model.export_graphs` live through `from_model`): one task per action node, successions and flows as `>>` edges through forks, joins, merges and decisions; a `task_factory` such as `KubernetesPodFactory` decides what each step runs |
| Requirement gates | `satisfy <requirement> by <step>` statements of the model become `SysMLRequirementSensor` tasks upstream of the step they name (`gates_of_model`, `load_gates`, `RequirementGateFactory`) |
| `OpenSysMLHook` and the `opensysml` connection | How a task reaches the service: a named host and port, or a private service started for the task and stopped with it |

The operators and sensors take `model_path` (templated), `opensysml_conn_id` and `strict` — refuse
a model the service reports errors for, the default. A question the engine cannot decide raises
the client's error rather than a verdict.

## Where it meets OpenSysML

- **Verdicts are the service's.** The provider adds no evaluation of its own: what a task reports
  is what `sysml -analysis`, `sysml -verify` and `sysml -satisfy` report for the same model, so a
  requirement that holds in the REPL holds in the DAG.
- **The DAG is the model's graph.** `SysMLActionDag` consumes the same `graphs:1` form the
  [external engines](external-engines.md) receive, so the pipeline the document renders
  (`-render-document`) and the pipeline Airflow runs are one `action def`.
- **A run's facts reach the model through `in` parameters.** `VerifyRequirement` takes no
  bindings, so a requirement over values a run produces is stated with `in` parameters and asked
  through a verification case, with `named_arguments` templated from `dag_run.conf`.

The provider repository carries two worked examples: a verification DAG over the lander model of
[`examples/analysis-demo`](https://github.com/Open-MBEE/OpenSysML/tree/develop/examples/analysis-demo),
and a terrain-reconstruction pipeline whose `action def` is the DAG, whose `satisfy` statements are
its gates, and whose description the same model renders as a document.
