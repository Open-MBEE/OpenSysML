package main

import (
	"flag"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// The help-text placeholders and check flag names the help repeats.
const (
	callArg         = "<call>"
	nameArg         = "<name>"
	fileArg         = "<file>"
	featureArg      = "<feature>"
	formatArg       = "<format>"
	formArg         = "<form>"
	checkDepthFlag  = "check-depth"
	checkStatesFlag = "check-states"
	checkUnrollFlag = "check-unroll"
)

// doc describes the command for both the terminal help and the man page, so a
// mode documented for one is documented for the other.
func doc() usage.Doc {
	return withoutOmitted(usage.Doc{
		Command:    "sysml",
		ManSection: 1,
		Summary:    "run, check, convert and render SysML v2 and KerML models, and migrate SysML v1",
		Synopsis:   []string{"[options] [file...]"},
		Description: []string{
			"sysml loads the models it is given — a file, a directory to walk or a " +
				"glob — as a single model, so a declaration in one file resolves " +
				"against the others whichever order they were named in. With no " +
				"expression or check to carry out it opens an interactive prompt; " +
				"otherwise it does what was asked and exits on the verdict, which " +
				"is what lets a run gate a build.",
		},
		Options: optionGroups(),
		Sections: []usage.Section{{
			Title:         "Examples",
			BeforeOptions: true,
			Examples: []usage.Example{
				usage.Ex("sysml", "Start interactive REPL"),
				usage.Ex(`sysml -e "5 + 3"`, "Evaluate and exit"),
				usage.Ex(`sysml -e "expr" file.sysml`, "Load file, evaluate, and exit"),
				usage.Ex("sysml file.sysml", "Load file and start REPL"),
				usage.Ex("sysml -debug file.sysml", "Load file, reporting every diagnostic"),
				usage.Ex("sysml -trace file.sysml", "Load file, reporting each execution step"),
			},
		}, {
			Title: "Checking a model",
			Examples: []usage.Example{
				usage.Ex("sysml -constraint MassBudget model.sysml", "Evaluate one constraint and exit"),
				usage.Ex("sysml -requirement PowerMargin model.sysml", "Evaluate one requirement and exit"),
				usage.Ex("sysml -satisfy model.sysml", "Evaluate every satisfaction assertion"),
				usage.Ex("sysml -satisfy=Ctx model.sysml", "...only the ones Ctx states"),
				usage.Ex("sysml -instantiate p -constraint C model.sysml", "Check C against an object of p"),
				usage.Ex("sysml -validate model.sysml", "Report diagnostics only"),
				usage.Ex("sysml -validate -strict model.sysml", "...asking whether it is conforming SysML v2"),
				usage.Ex("sysml -self-check model.sysml", "Apply the SysMLValidation constraints"),
				usage.Ex("sysml -self-check-package Acme::Rules m.sysml", "...plus a rule package's constraints"),
				usage.Ex("sysml -instantiate car -validate=car m.sysml", "Check every assertion about an object"),
				usage.Ex(`sysml -calc "Fall(3, 4)" model.sysml`, "Invoke a calculation"),
				usage.Ex("sysml -analysis shipCost model.sysml", "Run an analysis case"),
				usage.Ex(`sysml -analysis "CostAnalysis ship" model.sysml`, "...on an object as its subject"),
				usage.Ex("sysml -analysis speedCheck model.sysml", "Run a verification case for its verdict"),
				usage.Ex(`sysml -run-query "Heavy root=scope" model.sysml`, "Execute a document query"),
				usage.Ex("sysml -action Drive model.sysml", "Run an action to completion"),
				usage.Ex("sysml -state Mission -advance 10 model.sysml", "Run a state machine for 10 time units"),
				usage.Ex("sysml -action Ping -state Sm -advance 5 m.sysml", "...an action and a machine on one clock"),
				usage.Ex("sysml -schedule explore -action Drive m.sysml", "Run every linearization; table the outcomes"),
				usage.Ex("sysml -satisfy -json model.sysml", "Report the verdicts as JSON"),
			},
			Paragraphs: []string{
				"Check flags may be combined. -validate=<object> checks every " +
					"assertion about an object -instantiate created and the objects it " +
					"holds: an object named, #<id>, or a path such as car.engine. " +
					"-self-check applies the OpenSysML SysMLValidation constraints to " +
					"each reflectively classified element; underived reflective features " +
					"are reported as unevaluated, not as violations. " +
					"-self-check-package adds the constraint defs of a package the loaded " +
					"files declare to the same walk, as -self-check-package " +
					"Acme::ModelingRules: a rule is a constraint def whose first in " +
					"parameter is typed by a SysML or KerML metaclass, and it applies to " +
					"every element that metaclass conforms to. " +
					"-requirement and -satisfy report beside each verdict the verdict of " +
					"every verification case verifying the requirement. -analysis takes " +
					"arguments for the case's inputs and an object as its subject, as " +
					"-analysis \"Pkg::Case(3.0) Pkg::part\"; a verification case also " +
					"reports the verdict its body produced.",
			},
		}, {
			Title: "Running actions and state machines",
			Paragraphs: []string{
				"-action runs an action to completion and -state a state machine, both " +
					"on one shared clock that -advance runs for the given simulated time " +
					"units; without -advance a state machine takes only its initial " +
					"transition. Either names a definition or usage, or one followed by " +
					"the object to perform it on, as -action \"Drive rover1\".",
				"-schedule is the policy every run resolves its choice points under — " +
					"concurrent tokens, overlapping guards, competing transitions: " +
					"declared, reverse (the default), seed:<n> for a reproducible " +
					"pseudo-random order, explore[:runs=N,depth=D] to run every " +
					"linearization within the budget and table the distinct outcomes, " +
					"or replay:<file> to follow a witness file's choice lines move for " +
					"move, then reverse.",
				"Under -schedule explore, -engine check, smt or all, each run creates " +
					"objects of its own before the behaviors start: one per -instantiate, " +
					"which a -state or -action named alone attaches to, and one per " +
					"-action or -state naming a definition or usage to create or a path " +
					"into one such as mission.rover. The declaration is created once per " +
					"run, so machines on sibling parts share it and its connectors.",
				"-jobs is how many runs of one check may go concurrently — the " +
					"linearizations of an exploration, the engines -engine all consults " +
					"— each on a worker of its own over the shared model, and how many " +
					"files of one load are parsed and validated at once; the result is " +
					"the same at any count. By default one per CPU, fewer where the memory " +
					"available leaves less than 512 MiB per worker.",
			},
		}, {
			Title: "Analysis engines",
			Examples: []usage.Example{
				usage.Ex("sysml -engines", "List engines, kind and status; nothing runs"),
				usage.Ex("sysml -engines -probe", "Also start each external engine once"),
				usage.Ex("sysml -engine explore -action Drive m.sysml", "The same as -schedule explore"),
				usage.Ex("sysml -engine all -requirement R m.sysml", "Every engine that covers the question"),
				usage.Ex("sysml -engine run -json -constraint C m.sysml", "One engine; the plan in the report"),
			},
			Paragraphs: []string{
				"Every check is a question put to an analysis engine: run executes the " +
					"model once under -schedule, explore runs every linearization, sweep " +
					"runs the rows of -sweep, and solve puts condition sets to an SMT " +
					"solver. -engine auto (the default) picks the engine of highest " +
					"authority that covers the question and falls back to the next when " +
					"it refuses or covers nothing; -engine <name> puts the question to " +
					"that engine alone, and its refusal is the answer; -engine all puts " +
					"it to every engine that covers it, one after another in name order, " +
					"and composes their answers: a witnessed violation stands over any " +
					"universal claim, and a contradiction is reported as a disagreement " +
					"decided in the interpreter's favor.",
				"Every verdict is followed by its standing: the claim, the strength of " +
					"the evidence — observed (one run), witnessed (a replayed execution), " +
					"bounded (every case within a budget), proved (every case) — and what " +
					"earned it, as `holds (observed: 1 run under reverse)`. A budget the " +
					"engine reached lowers the strength and is named. Under -json each " +
					"check carries the plan and one results[] entry per engine that " +
					"answered, with its engine, claim, strength, bounds and witness.",
				"-engines lists each engine's name, kind (built-in, tool, engine, policy, " +
					"sampler), protocol, authority, the questions it answers and whether " +
					"its process is found, with the manifest entry and resolved command " +
					"of each external one; -probe also starts each external engine once " +
					"and checks its describe against its manifest entry field by field. " +
					"An engine is named run, explore, check, smt, sweep, solve, " +
					"tool:<name> from OPENSYSML_TOOLS, or as an external engine from " +
					"OPENSYSML_ENGINES. -engine explore is -schedule explore; -engine " +
					"check searches every schedule of each -action for a violation, " +
					"deadlock, failure or divergence; -engine smt decides each " +
					"-check-property over every schedule and every value of the action's " +
					"free inputs through an SMT solver.",
			},
		}, {
			Title: "Checking every schedule",
			Examples: []usage.Example{
				usage.Ex("sysml -engine check -action Go -check-property Safe m.sysml", "Search every schedule for Safe"),
				usage.Ex("sysml -engine check -action Go -check-diverge speed m.sysml", "Is speed schedule-dependent?"),
				usage.Ex("sysml -engine smt -action Go -check-input t -check-property Safe m.sysml", ""),
				usage.Ex("sysml -engine check -action Go -check-witness out m.sysml", "A witness file per finding"),
			},
			Paragraphs: []string{
				"-check-property evaluates a constraint or requirement at every stable " +
					"state of the action, on the performing object when there is one, " +
					"and reports a schedule at which it is false. -check-diverge reports " +
					"a feature sensitive when schedules leave it with different final " +
					"values — check searching the schedules, smt asking the solver for " +
					"two that end it apart — named as x, step.out for a performed node's " +
					"output or this.level for the performing object's (not covered under " +
					"smt); a name nothing holds is refused, and without it every " +
					"attribute of the action and of its performing object is checked, " +
					"the action's own when it has none.",
				"Under smt every input the model leaves unbound is free and every bound " +
					"one is pinned; -check-input leaves a bound feature free in its " +
					"declared domain, and a name that is not a feature the action reads " +
					"is refused. -check-assume assumes a constraint or requirement over " +
					"the initial state; a set no initial state satisfies is reported not " +
					"covered, never proved.",
				"-check-witness writes a file for each violation and each divergent " +
					"value — the inputs the solver chose, the schedule's choices, a blank " +
					"line, then the run's trace — which -schedule replay:<file> and " +
					"%replay follow. A bound is named when it is hit: -check-depth is the " +
					"most moves one schedule may make before the search backtracks, or " +
					"the moves smt unrolls the action to; -check-states the most distinct " +
					"states the search may visit, the same figure -engine all gives an " +
					"exploration as its runs; -check-unroll the most iterations of one " +
					"loop smt unrolls; -check-timeout the time the plan may run for and " +
					"the time each solver query may take in place of " +
					"OPENSYSML_SMT_TIMEOUT — a search it stops is reported incomplete " +
					"with the states and depth reached, not as a verdict.",
			},
		}, {
			Title: "Sweeping and sampling a parameter",
			Examples: []usage.Example{
				usage.Ex(`sysml -calc Twice -sweep "n=1..8:2" m.sysml`, "One run per range value"),
				usage.Ex(`sysml -analysis C -sweep "m=1..3" -sweep "v=1..2" m.sysml`, "Every combination of the two"),
				usage.Ex(`sysml -analysis C -sweep "m=1..9" -samples 20 -seed 7 m.sysml`, "20 values drawn uniformly"),
				usage.Ex(`sysml -analysis C -sweep "m=1..3" -json m.sysml`, "Report the rows as JSON"),
			},
			Paragraphs: []string{
				"A sweep is orchestration over the ordinary run: each row is one " +
					"-analysis or -calc invocation with the swept parameter bound to " +
					"that row's value and every other argument as given, so nothing " +
					"about how a case executes changes. Name a single -analysis or " +
					"-calc; a parameter the case does not declare, one the invocation " +
					"already binds, a step of zero or one pointing away from the end " +
					"of the range is an error rather than a silent row.",
				"An endpoint and a step carry the syntax and the units an argument " +
					`carries ("mass=0.0 [SI::kg]..9.0 [SI::kg]:3.0 [SI::kg]"), and ` +
					"the end of the range is included where the step lands on it.",
				"The values are produced in the swept parameter's declared type, not " +
					"the literals': a Real parameter swept over 1..4:1 is bound to 1.0, " +
					"2.0, 3.0, 4.0 and an Integer one swept over 1.0..3.0:1.0 to 1, 2, 3, " +
					"while an endpoint or step no Integer parameter can take (:0.5) is " +
					"refused before any run, as is a range over a Boolean, String, " +
					"enumeration or non-scalar parameter. A parameter declaring no type " +
					"takes the range as written, and the table says so.",
				"A range between whole numbers steps by one where no :<step> is written; " +
					"one with a fractional endpoint needs one. A range read as reals takes " +
					"an Integer endpoint or step only where a Real holds it without rounding, " +
					"and steps only where the reals tell its rows apart. Rows come out in the order the " +
					"ranges were given, the first varying slowest, and a run that " +
					"failed is a row carrying its error rather than the end of the " +
					"table.",
				"-samples draws that many values uniformly from each range instead of " +
					"stepping through it — Integers inclusively for a parameter taking " +
					"Integers, reals in [<from>, <to>) for one taking reals — and needs " +
					"-seed: the same seed draws the same table on every platform. " +
					"Uniform is the only distribution " +
					"on offer — the bundled library states no probability " +
					"distributions — so a range must be written <from>..<to> rather " +
					"than as a named distribution.",
			},
		}, {
			Title: "Running an action many times",
			Examples: []usage.Example{
				usage.Ex(`sysml -action Acquire -runs 100 -seed 7 m.sysml`, "100 runs, every feature"),
				usage.Ex(`sysml -action Acquire -runs 100 -seed 7 -observe elapsed m.sysml`, "One observable"),
				usage.Ex(`sysml -action Acquire -seed 7 m.sysml`, "One run, its draws seeded"),
				usage.Ex(`sysml -action Acquire -draws max m.sysml`, "Durations at their max"),
				usage.Ex(`sysml -action A -runs 9 -seed 7 -draws average m.sysml`, "Mean durations"),
				usage.Ex(`sysml -analysis "Mc obj" -runs 100 -seed 7 m.sysml`, "A MonteCarlo analysis case"),
			},
			Paragraphs: []string{
				"-runs runs one -action to completion that many times, each run on a " +
					"fresh context whose modeled randomness — the weighted decisions " +
					"@Probability states and the draws of uniform, uniformInteger, " +
					"triangular and normal — is seeded from a seed of its own derived " +
					"from -seed, so the same seed makes the same table on every platform " +
					"and a run can be replayed alone. -schedule stays the second, " +
					"independent knob: it decides the concurrency choices, which carry " +
					"no probability, in every run alike.",
				"-draws is the third knob, the duration policy: random (the default) " +
					"draws every RandomFunctions call from the seed; min, max and average " +
					"resolve each call to the least, greatest or mean value of its " +
					"distribution instead — the midpoint of a uniform, the mean of a " +
					"triangular — so a run whose only randomness is durations completes " +
					"deterministically and needs no -seed. normal has no least or greatest " +
					"value, so a run that calls it under min or max stops with an error. " +
					"Weighted decisions are not durations: they draw from the seed under " +
					"every policy, and an unseeded run takes the most probable branch. " +
					"The policy is written into every witness as `draws by <policy>`, " +
					"and -schedule replay: follows it.",
				"The table has one row per run, numbered, with each -observe feature " +
					"of the action — `clock` is the simulation time the run completed " +
					"at, never a feature of that name — and without -observe every " +
					"feature the action holds and the clock. Below it each numeric observable is summarised over the " +
					"completed runs: min, mean, max, the nearest-rank p50 and p90, and " +
					"a histogram; a non-numeric one is counted by value. A feature the " +
					"action does not hold is refused.",
				"-runs also runs an -analysis that specializes Simulation::MonteCarlo, " +
					"the OpenSysML library's analysis of repeated runs: each run performs " +
					"the case's steps on a fresh subject, seeded as an action's run is, and " +
					"reads the value the case binds as observed; the table has one row per " +
					"run with that value, and beneath its distribution the case is concluded " +
					"once over the sample — runs, mean, deviation (the sample standard " +
					"deviation) and outOfSpec (the runs in which a check of the case did not " +
					"hold) bound, and the case's own outputs and checks evaluated over them. " +
					"Each run makes its objects from their declarations, so a subject named " +
					"by `#id` alone is refused. Run once, without -runs, such a case leaves " +
					"its statistics unbound. -observe belongs to an action's runs, not a case's.",
			},
		}, {
			Title: "Conversion",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -convert ttl", "SysML notation to RDF Turtle, on stdout"),
				usage.Ex("sysml model.ttl -convert sysml", "RDF Turtle to SysML notation"),
				usage.Ex("sysml model.sysml -convert ttl -o m.ttl", "Write the conversion to a file"),
				usage.Ex("sysml in.txt -convert ttl -from sysml", "Name the input format explicitly"),
				usage.Ex("sysml flexo://demo/main -convert sysml", "A Flexo branch as notation"),
				usage.Ex("sysml model.sysml -convert ttl -o flexo://demo/main", "Push the graph to the branch"),
			},
			Paragraphs: []string{
				"The input format is taken from the file extension (.sysml, .kerml, " +
					".ttl, .json, .fmu) unless -from names it: sysml, kerml, ttl, turtle, " +
					"rdf, api-json, or fmu for a Functional Mock-up Unit to import. " +
					"Converting to the format it is " +
					"already in rewrites the input: notation is reformatted, Turtle " +
					"is normalized.",
				"A SysML v1 model (.xmi, .uml, .mdzip) is refused: it is migrated, " +
					"not converted; see Migration.",
				"Either side may name a Flexo MMS project branch instead of a file: " +
					"http(s)://host[:port][/base]/projects/{project}/branches/{branch}, " +
					"or flexo://{project}/{branch}, both naming the endpoint " +
					"FLEXO_SYSMLV2_URL configures. A branch input is read as its head " +
					"commit's RDF graph; a branch -o takes the -convert ttl output as " +
					"the branch's whole model graph, conditional on the branch's etag, " +
					"and refuses a head the sync state says has moved. Both need the " +
					"bearer token in FLEXO_INTEROP_TOKEN and record the head commit " +
					"in the sync state (-sync-state, or <output>.sync.json on a read / " +
					"<model>.sync.json on a push).",
				// Printed rather than restated, so the help cannot drift from what a
				// conversion reports.
				convert.ExperimentalNotice,
				"Every run that converts RDF says so on stderr. " +
					"Saving to .sysml or .kerml is stable.",
			},
		}, {
			Title: "Migration",
			Examples: []usage.Example{
				usage.Ex("sysml Model.mdzip -migrate sysml -o Model.sysml", "A Cameo project as notation"),
				usage.Ex("sysml Model.mdzip -migrate sysml -o Model.sysml -migration-report Model.report.txt", ""),
				usage.Ex("sysml Model.xmi -migrate ttl -o Model.ttl", "A v1 XMI export as RDF Turtle"),
				usage.Ex("sysml Model.mdzip -migrate sysml -layout Model_mtip.xml", "Views laid out from MTIP"),
				usage.Ex("sysml Model.mdzip -migrate sysml -migration-results runs.json", "Index the tool's results"),
				usage.Ex("sysml Model.sysml -compare-results runs.json", "Then compare with the tool's"),
				usage.Ex("sysml export.xml -from xmi -migrate sysml", "Name the v1 form explicitly"),
			},
			Paragraphs: []string{
				"A SysML v1 model — UML XMI 2.5.1 with the SysML profile applied, an " +
					"Eclipse UML2 .uml file, or a Cameo/MagicDraw .mdzip archive — is " +
					"migrated to SysML v2, not converted: the migration is ledgered, " +
					"not lossless. Every v1 element gets one of three verdicts, mapped " +
					"(written as the v2 construct it corresponds to), approximated " +
					"(written as the nearest v2 construct, with a note saying what " +
					"differs) or unmapped (left behind, with the reason); profile, " +
					"library and notation content nothing refers to is skipped. The " +
					"one-line summary of the verdicts goes to stderr; -migration-report " +
					"writes the element-by-element ledger to a file, JSON when it ends " +
					"in .json, text otherwise. -convert refuses a v1 model for this reason.",
				"-from names the v1 form (xmi, uml or mdzip) when the extension does " +
					"not; -migrate writes sysml, kerml, ttl, turtle, rdf or api-json, " +
					"and refuses an input that is not v1, which -convert handles. " +
					"-layout names an MTIP export of the same project, whose diagram " +
					"geometry is written into the migrated views as DiagramLayout " +
					"metadata; -image-base-url resolves the relative <img src> of a " +
					"comment to the server serving it; -migration-results writes the " +
					"simulation tool's run configurations and the result snapshots it " +
					"stored as a JSON sidecar, which -compare-results on the migrated " +
					"model runs against; -strict writes only notation a pinned SysML v2 " +
					"production admits, reporting a construct whose only v2 form is an " +
					"OpenSysML extension as unmapped. The images a document's Image " +
					"blocks carry are written beside -o, so a migration that wrote any " +
					"needs -o a local file path.",
				convert.MigrationNotice,
				"Every run that migrates a v1 model says so on stderr.",
			},
		}, {
			Title: "Native compilation",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -compile Pkg::Fib -o fib", "Compile a calc def to a C executable"),
				usage.Ex("sysml model.sysml -compile Pkg::Fib -target go -o fib", "...via Go"),
				usage.Ex("sysml model.sysml -compile Pkg::Fib -source -o fib.c", "Write the generated source only"),
			},
			Paragraphs: []string{
				"The executable takes the calc's parameters as arguments and prints " +
					"its result; it computes what sysml -calc computes, or fails with " +
					"the same error. Only the scalar subset compiles (Integer, Real, " +
					"Boolean; see docs/project/native-compilation.md).",
			},
		}, {
			Title: "Syncing against a repository",
			Lead:  []string{"A dry run unless -sync-apply:"},
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -sync-diff repo.ttl", "Show the change set and exit"),
				usage.Ex("sysml model.sysml -sync-diff repo.ttl -sync-base last-seen.ttl", ""),
				usage.Ex("sysml model.sysml -sync-diff repo.ttl -sync-confirm-deletes", ""),
				usage.Ex("sysml model.sysml -sync-diff repo.ttl -sync-mint-ids -sync-annotate out.sysml", ""),
				usage.Ex("sysml model.sysml -sync-diff http://localhost:8083", "Against the live API; no writes"),
				usage.Ex("sysml model.sysml -sync-apply http://localhost:8083", "Write the change set as a commit"),
			},
			Paragraphs: []string{
				"The diff correlates elements by their effective id — an @ElementId " +
					"annotation, or the encoded qualified name — so a rename, move or " +
					"retype is an update, never a delete plus a create. " +
					"Repository-only elements are reported as deletes but applying " +
					"them needs -sync-confirm-deletes; conflicts — a declared id the " +
					"branch no longer has, or a repository change since the last-seen " +
					"commit — exit 1 and are never resolved silently.",
				"The last-seen commit is tool state in <model>.sync.json (or " +
					"-sync-state), never written into the notation. -sync-apply " +
					"refuses a change set the dry run would have flagged, sends each " +
					"update under its retained id, and records the resulting commit; " +
					"the token comes from " + syncTokenEnv + ".",
			},
		}, {
			Title: "Rendering a view",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -render Views::vehicleView", "ASCII text at a terminal"),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form markdown", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -o view.mmd", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form dot", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form mermaid -render-palette okabe-ito", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form dot -render-unplaced strip", "unpositioned nodes in a strip below"),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form mermaid -render-style cameo", "drawn in Mermaid's Cameo approximation"),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form dot -render-style cameo", "drawn as Cameo draws it"),
				usage.Ex("sysml model.sysml -render Views::loopView -render-form dot -render-ports full", "every port of every part, typed"),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form plantuml -o view.puml", ""),
				usage.Ex("sysml model.sysml -render Views::vehicleView -render-form d2 -o view.d2", ""),
				usage.Ex("sysml model.sysml -render Views::partsTable -render-form csv -o parts.csv", ""),
				usage.Ex("sysml types.sysml model.sysml -render Views::vehicleView", "several files, loaded as one model"),
				usage.Ex("sysml model.sysml -render-all rendered", ""),
			},
			Paragraphs: []string{
				"The rendering is the one the view's render member states, and a " +
					"containment tree where it states none. It is tool-defined " +
					"output: SysML v2 specifies the notation, not how a tool draws " +
					"it. Notices — an empty view, an element the rendering cannot " +
					"represent — go on stderr. Every file named is loaded as one " +
					"model, so a view may expose elements a sibling file declares. " +
					"-render-form names the form written — text, mermaid, markdown, dot, " +
					"plantuml, d2, csv or tsv; by default -render takes it from the destination and " +
					"-render-all writes each kind's machine form, one file per view named " +
					"by its qualified name with :: as . and every byte unsafe in a filename " +
					"(/, \\, :, ., %, control characters, what Windows reserves) as %XX; " +
					"a name past 255 bytes is cut and tagged ~ and a hash of the whole. " +
					"A graph-shaped rendering defaults to Mermaid. DOT writes tree, " +
					"interconnection, state, action, case, mixed, requirement, definition and " +
					"package renderings; PlantUML writes those kinds and sequence. D2 writes " +
					"tree, interconnection, state, action, sequence, requirement, definition " +
					"and package renderings; case and mixed renderings refuse D2 with a typed " +
					"error. Table and matrix renderings refuse graph forms with a typed error. Neither " +
					"Graphviz, PlantUML nor D2 is needed to write graph forms. A table or matrix is " +
					"written as a " +
					"Markdown table by default, and as comma- or tab-separated values with " +
					"-render-form csv or tsv: a header record of the columns, then a record " +
					"per row, quoted as RFC 4180 quotes a field. Graph forms are drawn in the " +
					"black-and-white style of the SysML v2 Pilot visualizer; -render-palette " +
					"fills the DOT, Mermaid, PlantUML and D2 nodes by keyword family from a colourblind-safe palette " +
					"(okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, " +
					"viridis or cividis), keeping black text legible on every fill. " +
					"-render-style names the look the DOT and Mermaid forms draw in: pilot (default), " +
					"the Pilot visualizer's, or cameo, the look of Cameo Systems Modeler — a " +
					"diagram frame with a header tab, Arial text, gradient fills, compartments " +
					"and the UML pseudo-state symbols — for a diagram migrated from Cameo " +
					"to keep its look. An interconnection or mixed rendering draws on " +
					"each part the ports its interconnection edges end at, each a small " +
					"square on the part's border " +
					"named beside it; -render-ports full draws every port a part has, " +
					"labelled name : Type. A DiagramLayout Style on a member colours it over " +
					"either look, and a Note is drawn beside the member it is about. " +
					"A view whose members carry DiagramLayout positions draws the placed " +
					"members and the edges between them in every graph form, and leaves a " +
					"member with no position undrawn: the DOT form pins each at its stated " +
					"place, so nothing lands on a positioned box, while Mermaid, PlantUML and D2 " +
					"lay the same members out themselves. -render-unplaced strip draws the " +
					"unplaced members too, in rows in a strip below a DOT drawing. " +
					"The same setting shapes the diagrams of -render-document and " +
					"-render-documents.",
			},
		}, {
			Title: "Rendering a document",
			Examples: []usage.Example{
				usage.Ex("sysml model.sysml -render-document Reports::MassReport", "Markdown on stdout"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -o report.md", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form pdf -o report.pdf", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -o report.html", ""),
				usage.Ex("sysml model.sysml -render-documents rendered", "every document, linked"),
				usage.Ex("sysml model.sysml -render-documents site -doc-form html -html-css theme.css", ""),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -html-theme report -o report.html", "a bundled theme"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -html-mermaid cdn -o report.html", "diagrams drawn in the browser"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form html -html-math cdn -o report.html", "formulas typeset in the browser"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -diagram-form dot -o report.md", "diagrams as Graphviz DOT"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -diagram-form plantuml -o report.md", "diagrams as PlantUML"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -diagram-form d2 -o report.md", "diagrams as D2"),
				usage.Ex("sysml model.sysml -render-document Reports::MassReport -doc-form pdf "+
					"-pdf-engine pandoc -doc-title-page -doc-toc -doc-number-sections -o report.pdf", ""),
				usage.Ex("sysml -html-default-css -o sysml-document.css", "the default stylesheet"),
				usage.Ex("sysml -html-default-css -html-theme modern -o modern.css", "a theme's full sheet"),
			},
			Paragraphs: []string{
				"A document is a part def specializing DocumentQueries::Document. Its " +
					"queries are bound in the model and run against it, and the " +
					"result is written as CommonMark-compatible Markdown. Its diagram " +
					"blocks are chosen per diagram: a view some DiagramLayout::Layout or Route " +
					"positions is drawn by Graphviz where it states, as inline SVG when dot " +
					"is installed and as a dot fence otherwise, and every other graph-shaped " +
					"view is Mermaid source; with Graphviz absent a positioned view falls " +
					"back to Mermaid under a notice saying so. -diagram-form mermaid, dot, " +
					"plantuml or d2 writes every graph-shaped one in that form instead, in " +
					"Markdown and HTML alike, while a table or matrix view stays a table. D2 " +
					"writes tree, interconnection, state, action, sequence, requirement, " +
					"definition and package renderings, and refuses case and mixed diagrams. Neither " +
					"Graphviz, PlantUML nor D2 is needed to write a fence.",
				"-doc-form html writes semantic HTML instead, carrying each element's " +
					"identity and kind, styled by a stylesheet in a cascade layer your " +
					"own CSS overrides without !important.",
				"-doc-form pdf lays that HTML out with a print stylesheet through an " +
					"external converter named by -pdf-engine — weasyprint (default) or " +
					"prince, or pandoc, which reads the Markdown instead — run as " +
					"a subprocess, never linked in; diagrams are pre-rendered to SVG " +
					"with mermaid-cli (mmdc). -html-theme, -html-css and " +
					"-html-no-default-css style the PDF as they style the page. None of " +
					"these tools is needed until PDF output is asked for; " +
					"scripts/download-doc-pdf-toolchain.sh provisions pinned copies.",
				"HTML output needs nothing external and loads nothing by default: -html-theme " +
					"picks one of the bundled looks (default, acm, ieee, modern, nasa, print, report; " +
					"acm, ieee and nasa follow those bodies' manuscript conventions), -html-css adds " +
					"your own stylesheets, -html-no-default-css drops the default one, " +
					"-html-fragment writes the document element alone to embed in a " +
					"page of yours, and -html-default-css writes the default sheet out " +
					"— or, with -html-theme, a theme's whole sheet — to start from. -html-mermaid " +
					"cdn has the page load a pinned Mermaid release from jsDelivr so a " +
					"browser draws the Mermaid diagrams, or names a URL of your own to load it from.",
				"Mathematics is LaTeX: a Span styled math is an inline formula, a Formula " +
					"block a display one. Markdown writes them between $ and $$ delimiters, HTML " +
					"in \\( \\) and \\[ \\] inside sysml-math elements, left as source until " +
					"-html-math cdn has the page load a pinned MathJax release from jsDelivr, or " +
					"a URL of your own; PDF typesets them with KaTeX (katex) as it pre-renders " +
					"diagrams with mermaid-cli.",
			},
		}, {
			Title: "Flag order",
			Paragraphs: []string{
				"Flags may be written before or after the model they apply to. A file " +
					"named like a flag is read as a file after --, which ends the " +
					"flags: sysml -trace -- -m.sysml",
			},
		}, {
			Title: "Reading from standard input",
			Examples: []usage.Example{
				usage.Ex("cat model.sysml | sysml -validate -", "A lone - names standard input"),
				usage.Ex("cat model.sysml | sysml - -convert ttl -from sysml", ""),
			},
			Paragraphs: []string{
				"What was read from standard input is called <stdin> in diagnostics, " +
					"and a file really named \"-\" is read by naming it ./- instead.",
			},
		}, {
			Title: "Profiling a run",
			Examples: []usage.Example{
				usage.Ex("sysml -validate -memstats model.sysml", "Report what the run cost, on stderr"),
				usage.Ex("sysml -validate -memprofile heap.out model.sysml", "Write a heap profile for go tool pprof"),
				usage.Ex("sysml -validate -cpuprofile cpu.out model.sysml", "Write a CPU profile for go tool pprof"),
			},
		}, {
			Title: "Exit status",
			Lead:  []string{"Every run that is not a prompt exits:"},
			Items: []usage.Item{
				usage.Entry("0", "It did what was asked."),
				usage.Entry("1", "The model answered false for a check."),
				usage.Entry("2", "What was asked could not be carried out at all — an unreadable "+
					"file, a model that did not analyse cleanly, an unresolved name, a "+
					"failed conversion."),
				usage.Entry("3", "Part of what was asked was carried out: a -render-documents set "+
					"in which some document could not be rendered. The others were "+
					"written, a page stating the error stands in for each that was not, "+
					"and each failure is reported with the document's qualified name."),
			},
		}, {
			Title: "Output streams",
			Paragraphs: []string{
				"What was asked for is reported on stdout and what went wrong on " +
					"stderr, prefixed \"sysml: \" unless it locates a finding in the " +
					"source.",
			},
		}, {
			Title:      "Environment",
			ManOnly:    true,
			Items:      append(append(append(usage.BudgetEnvironment(), usage.LoadJobsEnvironment()...), usage.ToolEnvironment()...), solverEnvironment()...),
			Paragraphs: []string{usage.LegacyPrefixNote, usage.BudgetScopeNote},
		}, {
			Title:   "Files",
			ManOnly: true,
			Items: []usage.Item{
				usage.Entry("$XDG_STATE_HOME/sysml/history", "Prompt history, when XDG_STATE_HOME is set and writable."),
				usage.Entry("~/.sysml_history", "Prompt history otherwise. When neither can be written the history is kept for the session only."),
			},
		}, {
			Title:   "Reporting bugs",
			ManOnly: true,
			Paragraphs: []string{
				"Report bugs at https://github.com/Open-MBEE/OpenSysML/issues.",
			},
		}},
		SeeAlso: []string{"sysml-lsp(1)", "sysml-grpc(1)", "https://github.com/Open-MBEE/OpenSysML"},
	})
}

// solverEnvironment describes the variables of the experimental solving
// extension, which only the prompt's solver commands read.
func solverEnvironment() []usage.Item {
	return []usage.Item{
		usage.Entry("OPENSYSML_SMT", "Executable the %check, %explain, %solve, %configure and %optimize commands drive as their SMT solver, speaking SMT-LIB2 on standard input (experimental). Unset looks for z3, then cvc5, on PATH."),
		usage.Entry("OPENSYSML_SMT_TIMEOUT", "How long one solver query may take, as a Go duration. Default 10s, after which the verdict is unknown; a check's -check-timeout takes its place for the smt engine's queries."),
		usage.Entry("OPENSYSML_SMT_CORE_BUDGET", "How long %explain may spend reducing an unsat core to a minimal one, as a Go duration. Default 30s."),
		usage.Entry("OPENSYSML_SMT_MAX_CONFIGURATIONS", "How many variant selections %configure ... all may report before saying the enumeration was cut short. Default 32."),
	}
}

// registerFlags declares the command's flags on fs; each flag's group and
// placeholder live in optionGroups, and a flag missing there fails the tests.
func registerFlags(fs *flag.FlagSet) {
	fs.BoolVar(&showHelp, "help", false, "Show this help and exit")
	fs.BoolVar(&showHelp, "h", false, "Show this help and exit")
	fs.BoolVar(&showVersion, "version", false, "Show the version and exit")
	fs.BoolVar(&showVersion, "v", false, "Show the version and exit")
	fs.BoolVar(&showMan, "man", false, "Write this command's manual page, in roff, to stdout and exit")

	fs.Var(&evalExprs, "eval", "Evaluate this expression, against the model when one is loaded, and exit (repeatable)")
	fs.Var(&evalExprs, "e", "Evaluate this expression, against the model when one is loaded, and exit (repeatable)")
	fs.StringVar(&queryText, "query", "", "Evaluate this OSLC Query text against the model and exit")

	fs.Var(&modelChecks.validate, "validate", "Report the model's diagnostics and exit, nonzero on an error; -validate=<object> checks instead every assertion about that object (repeatable)")
	fs.Var(&disabledLints, "disable-lint", "Leave this lint out of the model's diagnostics: undeclared-signal, port-type-mismatch, deferred-keeper-unmarked or rounded-real-literal, comma-separated or repeated")
	fs.Var(&enabledLints, "enable-lint", "Report this opt-in lint, off by default, in the model's diagnostics: rounded-real-literal, comma-separated or repeated; -disable-lint wins")
	fs.BoolVar(&noRecordCache, "no-record-cache", false, "Parse every file loaded and hold it loaded, reading no interface record from the record cache and writing none; default off, or OPENSYSML_RECORD_CACHE=0")
	fs.BoolVar(&strictMode, "strict", false, "Judge the model as conforming SysML v2: notation no pinned production admits is an error, not a warning; a SysML v1 migration writes none of it")
	fs.Var(&modelChecks.constraints, "constraint", "Evaluate this constraint and exit (repeatable)")
	fs.BoolVar(&modelChecks.selfCheck, "self-check", false, "Apply the SysMLValidation constraints to every reflectively classified model element and exit")
	fs.Var(&modelChecks.selfCheckPackages, "self-check-package", "Apply this package's constraint defs with the SysMLValidation ones, as -self-check-package Acme::ModelingRules; implies -self-check (repeatable)")
	fs.Var(&modelChecks.requirements, "requirement", "Evaluate this requirement, and every verification case verifying it, and exit (repeatable)")
	fs.Var(&modelChecks.satisfy, "satisfy", "Evaluate every satisfaction assertion, or with -satisfy=<name> those the named element states, and exit (repeatable)")
	fs.Var(&modelChecks.calcs, "calc", "Invoke this calculation and report its result, as -calc \"Fall(3, 4)\" (repeatable)")
	fs.Var(&modelChecks.analyses, "analysis", "Run this analysis or verification case and report its outputs and verdict, as -analysis \"Pkg::Case(3.0) Pkg::part\" (repeatable)")
	fs.Var(&modelChecks.toolDryRuns, "tool-dry-run", "Show what the external tool the case's or action's ToolExecution names would be given — manifest, executable, argv, environment, standard input and reply mapping — without starting it, and discards what the run did; repeatable")
	fs.Var(&modelChecks.records, "record-run", "Run this analysis case as -analysis does and record the run into the model as AnalysisRecords elements, one record per -sweep value or -runs run (repeatable)")
	fs.StringVar(&modelChecks.recordInto, "record-into", "", "Record -record-run runs into this package instead of a Records package beside the case's")
	fs.Var(&modelChecks.queries, "run-query", "Execute this document query and report its rows, as -run-query \"Heavy root=telescope\" (repeatable)")
	fs.Var(&modelChecks.instantiate, "instantiate", "Create an object of this definition or usage before the checks, so a verdict is about it (repeatable)")
	fs.BoolVar(&modelChecks.jsonOut, "json", false, "Report checks as one JSON document rather than as lines")

	fs.Var(&modelChecks.actions, "action", "Run this action to completion, as -action \"Drive rover1\" to run it on an object (repeatable)")
	fs.Var(&modelChecks.states, "state", "Run this state machine, as -state \"Mission rover1\" to run it on an object (repeatable)")
	fs.Var(&modelChecks.advance, "advance", "Simulated time units to run the -action and -state behaviors for, on one shared clock; without it a state machine takes only its initial transition under every schedule and engine")
	fs.Var(&schedule, "schedule", "Policy every run resolves its choice points under: declared, reverse (default), seed:<n>, explore[:runs=N,depth=D] or replay:<file>")
	fs.Var(&modelChecks.seed, "seed", "Seed the model's own draws — weighted decisions, random functions — and those of -samples and -runs; the same seed draws the same run or table")
	fs.Var(&modelChecks.draws, "draws", "How every run resolves the draws of RandomFunctions: random (default) draws from -seed; min, max and average take each call's least, greatest or mean value and need no seed; weighted decisions draw from -seed whatever the policy")
	fs.Var(&modelChecks.clockStep, "clock-step", "The step, in seconds, the clock of every run ticks by: a wait comes due at the first multiple of it not before the wait ends; 0 (default) is a continuous clock. With -compare-results, replaces every configuration's stepSize")
	fs.Var(&modelChecks.runs, "runs", "Run the -action, or the Simulation::MonteCarlo -analysis, this many times, each run seeded from -seed, and table the -observe features or the case's observed value; needs -seed unless -draws is min, max or average")
	fs.Var(&modelChecks.observe, "observe", "Report this feature of the -runs action, or clock for the time it completed at; default every feature it holds and the clock. With -compare-results, the stored observable to compare, or <observable>=<feature> to read it from another feature of the run (repeatable)")
	fs.Var(&modelChecks.sweeps, "sweep", "Run the -analysis or -calc once per value of this range, as -sweep \"n=1..8:2\"; several ranges run their cartesian product (repeatable)")
	fs.Var(&modelChecks.samples, "samples", "Draw this many values uniformly from each -sweep range instead of stepping through it; needs -seed")
	fs.Var(&jobsFlag, "jobs", "Runs of one check that may go concurrently, each on a worker of its own, and files of one load that are parsed and validated at once; the result is the same at any count. Default OPENSYSML_JOBS, else one per CPU, fewer where the memory available leaves less than 512 MiB per worker")

	fs.BoolVar(&listEngines, "engines", false, "List the analysis engines this build knows and their status, spawning nothing, and exit")
	fs.BoolVar(&probeEngines, "probe", false, "With -engines, also start each external engine once and report the outcome as its status")
	fs.Var(&engine, "engine", "Analysis engine every check is put to: auto (default), all, or one by name — run, explore, check, smt, sweep, solve, tool:<name> or an external engine")

	fs.Var(&modelChecks.checker.properties, "check-property", "Evaluate this constraint or requirement at every stable state of the action and report a schedule at which it is false (repeatable)")
	fs.Var(&modelChecks.checker.diverge, "check-diverge", "Report this feature sensitive when schedules leave it with different final values, as x, step.out or this.level; default every attribute (repeatable)")
	fs.Var(&modelChecks.checker.inputs, "check-input", "Under smt, leave this feature of the action free in its declared domain although the model binds it (repeatable)")
	fs.Var(&modelChecks.checker.assume, "check-assume", "Under smt, assume this constraint or requirement over the initial state of the action (repeatable)")
	fs.StringVar(&modelChecks.checker.witness, "check-witness", "", "Write a witness file for each violation and each divergent value to this directory, for -schedule replay:<file> and %replay to follow")
	modelChecks.checker.depth.flag, modelChecks.checker.states.flag, modelChecks.checker.unroll.flag = checkDepthFlag, checkStatesFlag, checkUnrollFlag
	fs.Var(&modelChecks.checker.depth, checkDepthFlag, "The most moves one schedule may make before the search backtracks, or the moves smt unrolls the action to (default 10000 for check, 40 for smt)")
	fs.Var(&modelChecks.checker.states, checkStatesFlag, "Under check, the most distinct states the search may visit (default 1000000)")
	fs.Var(&modelChecks.checker.unroll, checkUnrollFlag, "Under smt, the most iterations of one loop the solver unrolls before it stops (default 4)")
	fs.Var(&modelChecks.checker.timeout, "check-timeout", "The time the check's plan may run for, as 30s or 2m, and the time each smt solver query may take in place of OPENSYSML_SMT_TIMEOUT")

	fs.StringVar(&convertFormat, "convert", "", convertUsage())
	fs.StringVar(&fromFormat, "from", "", fromUsage())
	fs.Var(&dataImports, "import", "Set the feature values a CSV, TSV, JSON or JSON Lines file assigns, one row per element, before -convert writes the model; an element column names the element and each other column a feature, as `mass [kg]` (repeatable)")
	fs.StringVar(&importAs, "import-as", "values", "What -import makes of each row: values, which sets the features of the element the row names")
	fs.Var(&importMap, "import-map", "Read the -import before it onto elements and features as this JSON mapping file says")
	fs.Var(&importFormat, "import-format", "Read the -import before it as csv, tsv, json or jsonl rather than as its extension says")
	fs.BoolVar(&importDryRun, "import-dry-run", false, "Report the values -import would set, changing and writing nothing")
	fs.StringVar(&idForm, "id", "", "With -convert ttl or api-json, how derived element ids are spelled: qualified (default) derives each from its qualified name; uuid mints name-based uuids under each root package, as the library convention does")
	fs.StringVar(&outputPath, "output", "", outputUsage())
	fs.StringVar(&outputPath, "o", "", outputUsage())
	fs.StringVar(&modelChecks.compare, "compare-results", "", "Run every configuration this -migration-results file indexes — or those -action names — with its recorded runs and duration mode, or the -runs and -draws given, seeded from -seed, and table the tool's and OpenSysML's min, mean, p50, p90 and max of each observable with their relative difference")

	fs.StringVar(&renderView, "render", "", "Render this view of the model instead of running it, in the form its render member states; #<kind> renders every file loaded and #<kind>:<element> one element, kind being tree, interconnection, state, action, case, mixed, sequence, table or matrix, without a declared view")
	fs.StringVar(&renderAllDir, "render-all", "", "Render every declared view into this directory")
	fs.StringVar(&graphsSubject, "graphs", "", "Write the lowered graph of this action or state machine, and of every behavior it performs, as the canonical graphs:1 JSON an external analysis engine is sent, to stdout or -output")
	fs.StringVar(&renderForm, "render-form", "", "Form -render or -render-all writes: text, mermaid, markdown, dot, plantuml, d2, csv or tsv (csv and tsv for a table or matrix); D2 writes tree, interconnection, state, action, sequence, requirement, definition and package renderings, not case or mixed; default from the destination for -render, each kind's machine form for -render-all")
	fs.StringVar(&renderPalette, "render-palette", "", "Palette the dot, mermaid, plantuml or d2 form fills nodes from, by keyword family: okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, viridis or cividis; default black and white")
	fs.StringVar(&renderLink, "render-link", "", "Source link template for -render, -render-all, -render-document, -render-documents and -render-run. Placeholders: {file}, {line}, {col}, {qname}, {id}; {file} is the path as loaded (use absolute paths for vscode:// or file:// links)")
	fs.StringVar(&renderStyle, "render-style", "", "Drawing style of the dot or mermaid form: pilot (default), the Pilot visualizer's black and white, or cameo, the look of Cameo Systems Modeler; applies to -render, -render-all and document diagrams")
	fs.StringVar(&renderPorts, "render-ports", "", "How much of a part's ports -render or -render-all draws on an interconnection or mixed rendering: minimal (default), the ports its interconnection edges end at, each a small square on the part's border named beside it, or full, every port, labelled name : Type")
	fs.StringVar(&renderOverlay, "render-overlay", "", "What -render or -render-all draws over a requirement rendering's structure: verdicts runs the verification cases verifying each requirement and colours and labels it by their verdicts; default none, a purely structural drawing")
	fs.StringVar(&renderUnplaced, "render-unplaced", "", "Where a graph form of a view some Layout positions puts the nodes none does: omit (default) leaves them undrawn in every form, strip draws them, in rows below the dot drawing; applies to -render, -render-all and document diagrams")

	fs.StringVar(&renderDoc, "render-document", "", "Compile this document definition, run its queries and write the rendered document")
	fs.StringVar(&renderDocsDir, "render-documents", "", "Render every document definition, linked to one another, into this directory; a document that cannot be rendered gets a page stating why and the run exits 3")
	fs.StringVar(&docForm, "doc-form", "", docFormUsage())
	fs.StringVar(&diagramForm, "diagram-form", "", "Form the documents' graph-shaped diagrams are written in: mermaid, dot, plantuml or d2; D2 writes tree, interconnection, state, action, sequence, requirement, definition and package renderings, not case or mixed; unset, a positioned view is dot and any other mermaid; a table or matrix view is a table either way")
	fs.BoolVar(&docNumberFigures, "doc-number-figures", false, docNumberFiguresUsage())

	fs.BoolVar(&debugMode, "debug", false, "Report every diagnostic over the whole session buffer, with the pass that produced it")
	fs.BoolVar(&quietMode, "quiet", false, "Report errors only, suppressing warnings")
	fs.BoolVar(&traceMode, "trace", false, "Report each execution step: expression evaluation, calc invocation, action tokens, state transitions")
	fs.Var(&renderRuns, "render-run", "Render the trace of a run as timeline or sequence into path (`-` is stdout); form comes from -render-form or the path extension")
	fs.BoolVar(&memStats, "memstats", false, "Report on stderr what the run cost: wall time, memory allocated, memory taken from the OS")

	fs.Var(&deprecatedFlag{instead: "-to has been replaced by -convert, as `sysml model.sysml -convert ttl`"}, "to", "Replaced by -convert, which names the output format")

	declareFeatureFlags(fs)
}

// optionGroups lays the flags out by task, each with the placeholder its
// argument is shown as, in the order the help and the man page list them.
func optionGroups() []usage.OptionGroup {
	return []usage.OptionGroup{{
		Title: "General",
		Options: []usage.Option{
			usage.Opt("help", "", "h"),
			usage.Opt("version", "", "v"),
			usage.Opt("man", ""),
		},
	}, {
		Title: "Evaluating",
		Options: []usage.Option{
			usage.Opt("eval", "<expr>", "e"),
			usage.Opt("query", "<text>"),
		},
	}, {
		Title: "Checking a model",
		Options: []usage.Option{
			usage.Opt("validate", "[=<object>]"),
			usage.Opt("strict", ""),
			usage.Opt("disable-lint", "<code>"),
			usage.Opt("enable-lint", "<code>"),
			usage.Opt("no-record-cache", ""),
			usage.Opt("constraint", nameArg),
			usage.Opt("self-check", ""),
			usage.Opt("self-check-package", "<package>"),
			usage.Opt("requirement", nameArg),
			usage.Opt("satisfy", "[=<name>]"),
			usage.Opt("calc", callArg),
			usage.Opt("analysis", callArg),
			usage.Opt("tool-dry-run", callArg),
			usage.Opt("record-run", callArg),
			usage.Opt("record-into", "<package>"),
			usage.Opt("run-query", "<query>"),
			usage.Opt("instantiate", nameArg),
			usage.Opt("json", ""),
		},
	}, {
		Title: "Running behaviors",
		Options: []usage.Option{
			usage.Opt("action", nameArg),
			usage.Opt("state", nameArg),
			usage.Opt("advance", "<time>"),
			usage.Opt("schedule", "<policy>"),
			usage.Opt("seed", "<n>"),
			usage.Opt("draws", "<policy>"),
			usage.Opt("clock-step", "<seconds>"),
			usage.Opt("runs", "<n>"),
			usage.Opt("observe", featureArg),
			usage.Opt("sweep", "<range>"),
			usage.Opt("samples", "<n>"),
			usage.Opt("jobs", "<n>"),
		},
	}, {
		Title: "Analysis engines",
		Options: []usage.Option{
			usage.Opt("engines", ""),
			usage.Opt("probe", ""),
			usage.Opt("engine", nameArg),
		},
	}, {
		Title: "Checking every schedule (-engine check, smt or all)",
		Options: []usage.Option{
			usage.Opt("check-property", nameArg),
			usage.Opt("check-diverge", featureArg),
			usage.Opt("check-input", featureArg),
			usage.Opt("check-assume", nameArg),
			usage.Opt("check-witness", "<dir>"),
			usage.Opt(checkDepthFlag, "<n>"),
			usage.Opt(checkStatesFlag, "<n>"),
			usage.Opt(checkUnrollFlag, "<n>"),
			usage.Opt("check-timeout", "<duration>"),
		},
	}, {
		Title: "Converting and migrating",
		Options: []usage.Option{
			usage.Opt("convert", formatArg),
			usage.Opt("migrate", formatArg),
			usage.Opt("from", formatArg),
			usage.Opt("import", fileArg),
			usage.Opt("import-as", "<shape>"),
			usage.Opt("import-map", fileArg),
			usage.Opt("import-format", formatArg),
			usage.Opt("import-dry-run", ""),
			usage.Opt("id", nameArg),
			usage.Opt("output", fileArg, "o"),
			usage.Opt("migration-report", fileArg),
			usage.Opt("migration-results", fileArg),
			usage.Opt("layout", fileArg),
			usage.Opt("image-base-url", "<url>"),
			usage.Opt("compare-results", fileArg),
		},
	}, {
		Title: "Compiling natively",
		Options: []usage.Option{
			usage.Opt("compile", "<calc>"),
			usage.Opt("target", "<backend>"),
			usage.Opt("source", ""),
		},
	}, {
		Title: "Rendering views",
		Options: []usage.Option{
			usage.Opt("render", "<view>"),
			usage.Opt("render-all", "<dir>"),
			usage.Opt("graphs", nameArg),
			usage.Opt("render-form", formArg),
			usage.Opt("render-palette", "<palette>"),
			usage.Opt("render-link", "<template>"),
			usage.Opt("render-unplaced", "<placement>"),
			usage.Opt("render-style", "<style>"),
			usage.Opt("render-ports", "<display>"),
			usage.Opt("render-overlay", "<overlay>"),
		},
	}, {
		Title: "Rendering documents",
		Options: []usage.Option{
			usage.Opt("render-document", nameArg),
			usage.Opt("render-documents", "<dir>"),
			usage.Opt("doc-form", formArg),
			usage.Opt("diagram-form", formArg),
			usage.Opt("doc-title-page", ""),
			usage.Opt("doc-toc", ""),
			usage.Opt("doc-number-sections", ""),
			usage.Opt("doc-number-figures", ""),
			usage.Opt("pdf-engine", "<converter>"),
		},
	}, {
		Title: "Styling HTML documents",
		Options: []usage.Option{
			usage.Opt("html-theme", "<theme>"),
			usage.Opt("html-css", "<file|url>"),
			usage.Opt("html-no-default-css", ""),
			usage.Opt("html-fragment", ""),
			usage.Opt("html-default-css", ""),
			usage.Opt("html-mermaid", "cdn|<url>"),
			usage.Opt("html-math", "cdn|<url>"),
		},
	}, {
		Title: "Syncing against a repository",
		Options: []usage.Option{
			usage.Opt("sync-diff", "<repo>"),
			usage.Opt("sync-apply", "<url>"),
			usage.Opt("sync-base", fileArg),
			usage.Opt("sync-state", fileArg),
			usage.Opt("sync-confirm-deletes", ""),
			usage.Opt("sync-mint-ids", ""),
			usage.Opt("sync-annotate", fileArg),
		},
	}, {
		Title: "Diagnostics and profiling",
		Options: []usage.Option{
			usage.Opt("debug", ""),
			usage.Opt("quiet", ""),
			usage.Opt("trace", ""),
			usage.Opt("render-run", "<kind>=<path>"),
			usage.Opt("cpuprofile", fileArg),
			usage.Opt("memprofile", fileArg),
			usage.Opt("memstats", ""),
		},
	}, {
		Title: "Deprecated",
		Options: []usage.Option{
			usage.Opt("to", formatArg),
			usage.Opt("pdf-title-page", ""),
			usage.Opt("pdf-toc", ""),
			usage.Opt("pdf-number-sections", ""),
		},
	}}
}

// docFlags is a flag set holding the command's flags, for rendering the help or
// the man page without a command line to parse.
func docFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("sysml", flag.ContinueOnError)
	registerFlags(fs)
	return fs
}
