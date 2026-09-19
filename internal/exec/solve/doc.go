// Package solve translates the conditions a constraint, requirement or
// satisfaction assertion states into a solver-independent term IR and writes
// that IR as an SMT-LIB2 script.
//
// It also runs an external solver over that script — z3 or cvc5, found on PATH
// or named by OPENSYSML_SMT — as a process speaking SMT-LIB2 on standard input,
// so no library is linked in and releases stay pure Go. The verdicts sat, unsat
// and unknown stay distinct: a timeout or arithmetic the solver gave up on is
// unknown, and a solver that crashes, is absent, or replies unusably is a typed
// error rather than a verdict.
//
// The runtime evaluator remains the normative semantics; SysML v2 defines no
// solving semantics, so this is an advertised extension, not a conformance
// claim.
//
// The capability set — satisfiability, unsat-core conflict explanation, value
// synthesis and objective optimization over an SMT solver — follows the design of
// OpenMBEE's HMF ConstraintSolverService (Apache 2.0); see README Acknowledgements.
//
// # Solver compatibility
//
// OPENSYSML_SMT names any executable that speaks SMT-LIB2 on standard input, so
// what a backend must support is stated rather than assumed of z3.
//
// Logic selection emits the narrowest logic of the SMT-LIB 2.6 logic list
// (https://smt-lib.org/logics.shtml) that covers what a query actually uses:
// QF_UF for constants alone, QF_LIA/QF_NIA over Int, QF_LRA/QF_NRA over Real, and
// AUFLIRA/AUFNIRA for a query over both, the list defining no quantifier-free
// mixed logic to narrow to. Truncating integer division by a literal divisor stays
// in the linear logic — `div` and `mod` are the Ints theory's, which those logics
// include — and a variable divisor selects the nonlinear one it really needs.
// Datatypes and strings have no logic in the list, so a query using either sets
// the non-standard NonStandardLogic ("ALL"), which both verified backends accept;
// the script says so in a comment, and LogicChoice.Standard reports it. No logic is
// widened to dodge a hard case.
//
// What a backend is required to support is a Capability, and a Solver is probed
// once per executable (cached, one small script per form of the feature the writer
// emits — QF_NIA and QF_NRA for nonlinear, AUFLIRA and AUFNIRA for mixed — and only
// for the capabilities a query and operation actually need) or declared capable by
// the caller (Solver.Declared). A capability the backend rejects — an `(error …)`
// reply or `unsupported` — makes the request an UnsupportedCapabilityError naming
// the backend, the feature and the operation, refused before any query is run: no
// silent degrade and no fabricated verdict. A check the backend neither answered
// nor rejected settles nothing, so the query proceeds and its own verdict or
// SolverProcessError is reported.
//
// Three answers to a check are told apart, so a backend is never blamed for the
// wrong thing:
//   - `(error …)`, `unsupported`, or a defined reply that contradicts the check —
//     the backend refuses the capability, reported as UnsupportedCapabilityError.
//   - `unknown`, no reply, a closed pipe or the check's deadline — nothing was
//     established, so the query runs and answers for itself.
//   - a reply SMT-LIB does not define at all, such as `maybe` — the executable is
//     not answering as a solver, reported as SolverProcessError rather than as a
//     missing feature, which is also how the dialogue itself reports one.
//
// Probed against z3 4.8.12 and cvc5 1.3.4: both support models, unsat cores,
// incremental checks, datatypes, strings, div/mod, nonlinear and mixed arithmetic
// and the non-standard logic. cvc5 rejects `(maximize …)` as a parse error and
// answers `unsupported` to `:opt.priority`, both z3 extensions, so objective
// optimization is z3-only. internal/exec/solve's portability harness
// (portability_test.go, TestPortability) runs one query per feature against
// whatever OPENSYSML_SMT names and reports each as pass, refuse or fail, a
// rejected script being ours to fix in the writer.
//
// # Conflict explanation
//
// Explain answers an unsat verdict with the assertions that conflict. The script
// it writes (CoreScript) names each assertion, turns unsat cores on and asks for
// the core once the verdict is unsat; labels are the assertion's position, so a
// core reads back to the Assertion, and its Provenance, that produced it. Every
// role can appear in a core, a declared domain (RoleDomain) or a well-definedness
// guard (RoleDefined) included, and an inherited condition names the supertype
// that declared it.
//
// Minimality is established, not assumed. A solver's core is unsatisfiable but
// need not be irreducible, so reduction drops one member at a time, each round a
// fresh solver process, and Core.Minimal says every remaining member was shown to
// be needed: dropping any one left the rest satisfiable. Reduction is bounded, in
// the spirit of the runtime's step budgets, by DefaultMaxCoreMembers members and
// DefaultCoreBudget of wall time (OPENSYSML_SMT_CORE_BUDGET overrides it); a core
// too large, out of budget, or whose round the solver did not decide is reported
// as it stands with Minimal false and Core.Note saying why. A solver that refuses
// cores, names an assertion the query did not assert, answers unreadably or
// reports an empty core is a CoreError, never an empty or invented core.
//
// Conditions come from the evaluator's own collection
// (runtime.Context.ConditionsOf), keeping its order and its distinctions:
// `require` versus `assume`, negation, and a body meaning the conjunction of
// its conditions.
//
// # Differential agreement gate
//
// The translation is evidence-backed rather than asserted: for an element whose
// conditions translate, and for a concrete assignment of the features they read,
// the gate (differential_test.go and the corpus and randomized gates beside it)
// requires the query conjoined with that assignment to be sat exactly when the
// evaluator says the conditions hold, and unsat exactly when it says they do
// not. Every other outcome is classified, never averaged away: unknown is
// recorded, a typed evaluator error is no verdict, and ErrDivisionByZero is
// required to correspond to the guarded query being unsat for that assignment.
//
// It runs over the runtime conformance corpus, the bundled standard library, the
// OMG training corpus and deterministic randomized models, and reports how much
// of each it reached — translated, refused, agreed, disagreed, unknown — so
// coverage drift is reviewable. What it proves is that the translation is
// faithful to the evaluator on the cases it covered; it is not a conformance
// claim, and the evaluator remains normative where the two ever differ.
//
// # Translatable subset
//
// A condition is translatable when every part of it is:
//
//   - Boolean operators: `not`, `and`, `&`, `or`, `|`, `xor`, `implies`, and the
//     conditional expression `if c ? a else b`.
//   - Equality `==` and `!=` between two values of the same sort.
//   - Comparisons `<`, `<=`, `>`, `>=` between numbers of the same dimension.
//   - Arithmetic `+`, `-`, `*`, unary `-` and `+`.
//   - Division `/` and remainder `%`. Integer division truncates toward zero as
//     the evaluator does, encoded as ite(a >= 0, div(a, b), -div(-a, b)), and the
//     remainder as a - b*tdiv(a, b), which takes the dividend's sign. A literal
//     divisor keeps this linear; a variable divisor sets Query.Nonlinear, so
//     unknown is an expected verdict rather than a surprise.
//   - Division by zero, which SMT-LIB leaves underspecified while the evaluator
//     refuses it: a literal zero divisor refuses translation, and any other
//     divisor, integer or real, is asserted non-zero as a RoleDefined side
//     condition. That assertion constrains the whole query, so it is only made
//     where the division is always evaluated and read unnegated; a computed
//     divisor under `not`, `or`, `xor`, `implies`, a conditional branch or a
//     denied element refuses instead, since the evaluator may never divide there.
//   - Literals: boolean, integer, real, string.
//   - Quantity expressions (`450.0 [km/h]`), normalized to the base units their
//     unit reduces to through semantics.UnitTermOf — magnitudes are exact
//     rationals, so a scale factor introduces no rounding.
//   - References to scalar-valued features, resolved through the same names the
//     evaluator resolves: Boolean, String, Natural (declared non-negative),
//     Integer, Rational, Real and Number features, features typed by a quantity
//     value type, enumeration-typed features and variation points.
//   - Feature chains that ground in such a feature (`lander.verticalSpeed`).
//   - Enumeration literals and variants, as constructors of a finite datatype
//     sort declared per enumeration definition or variation point.
//
// A variable stands for the value a feature may take, constrained only by its
// sort: declared values are not asserted, so a query asks what the conditions
// permit rather than what one object holds.
//
// # Value synthesis
//
// TranslateWith (and ConstraintWith, RequirementWith, SatisfactionWith) takes a
// partial assignment: Pins fixing some features to the values the model already
// fixes, the rest left free for the solver to choose. A pin is read where the
// evaluator reads it — Fixed and FixedFor go through the runtime's objects,
// feature values and declared defaults — and carries its provenance: held by an
// object (PinHeld), declared by the model (PinDeclared) or chosen by the caller
// (PinChosen). Passing no pin translates exactly as before, so a query with no
// partial assignment is the same script it always was.
//
// A pin becomes an ordinary equality assertion in role RolePinned, asserted
// before the conditions and named in Query.Pinned with its assertion index, so it
// can appear in an unsat core like any other assertion: unsat under pins means no
// values exist consistent with what is already fixed, and the core says which
// fixed values conflict. Values are converted through the same machinery the
// translator uses — a quantity normalized to base units as an exact rational, an
// enumeration literal or variant as the datatype constructor the writer declares
// — and a value the subset cannot represent, or one whose dimension does not
// match its feature, is a PinError wrapping ErrNotPinnable, never a silent drop.
// Features read by the conditions but not readable as a value are reported as
// Unread rather than being fixed to something.
//
// Result.Model is one witness, not a canonical answer: a satisfiable query
// usually has many models and the solver may return any of them. Values are
// rendered in OpenSysML's terms where the sort allows (qualified feature names,
// declared units, enumeration and variant names) and flagged as the solver wrote
// them where it does not.
//
// # Variant configuration
//
// A variation point translates as a finite datatype sort (Sort.Variation), so
// Query.Variations are its variation variables. Query.FixValue chooses a variant
// (PinChosen), which Solve then checks like any other fixed value, and with none
// chosen Solve's model is a consistent selection. Configurations enumerates
// consistent selections: one fresh check-sat per solution, each asserting the
// negation of the complete previous assignment, built from the solver's own
// terms rather than from rendered text. Every variation variable is assigned in
// every solution, nested variation points and constrained variants included,
// since they are variables of the same query as any other condition.
//
// The enumeration is bounded, in the spirit of the runtime's step budgets, by
// DefaultMaxConfigurations solutions (OPENSYSML_SMT_MAX_CONFIGURATIONS overrides
// it). Result.Truncated says the enumeration was cut short and why: AtBound for
// the bound, Undecided for a solver that stopped deciding, with TimedOut when
// the run's deadline was what stopped it — a deadline reports the solutions
// already found rather than discarding them. Results are exhaustive only when a
// final check-sat answered unsat; nothing implies exhaustiveness that was not
// shown. A query reading no variation point is a NoVariationsError wrapping
// ErrNoVariations, not an empty enumeration.
//
// Known limitations: only variation points in the translatable subset are
// configured, so a variation whose variants carry collection-valued or otherwise
// untranslatable conditions refuses with ErrNotTranslatable; variants are
// configured as values of a variation point, not as objects, so nothing is
// materialized and features a variant would only have once bound are not
// constrained; and the enumeration order is the solver's, not a defined one.
//
// # Objective optimization
//
// Analysis and AnalysisWith translate an `analysis def` as an optimization query:
// what its conditions permit, and the objectives to improve within that.
// Solver.Optimize then asks a backend for each objective's optimum.
//
// SysML v2 states no direction, value or solving semantics for `objective`, so
// this layer states the contract it reads (an OpenSysML extension, not a
// conformance claim):
//
//   - Direction comes from the trade-study definition the objective is typed by:
//     TradeStudies::MinimizeObjective or MaximizeObjective, specializations
//     included. An objective typed by neither refuses with ErrNotOptimizable.
//   - The value to improve is the expression the objective's redefinition of
//     the library's `eval` calculation returns — `objective o :
//     MinimizeObjective { subject :>> selectedAlternative; in calc :>> eval {
//     expression } }` — which is the library's own extension point: it derives
//     `best` as `eval` over the alternatives. An objective giving the bound
//     `best` a value of its own instead (`attribute :>> best = expression;`) is
//     a validation error, and refuses here with ErrNotOptimizable pointing at
//     the `eval` spelling rather than being read. A value bound directly to the
//     objective is read too, where a model can write one.
//   - What is feasible is the case's own conditions (CaseConditionsOf: `require`,
//     `assume`, `assert`, `inv`, inherited ones included) together with the
//     conditions each objective states: its own body's, and the ones it inherits
//     from the model's own objective definitions, read where they are inherited.
//     Only the trade-study library's own conditions are left out, being about
//     choosing among alternatives rather than about which values are feasible. A
//     condition an inherited definition states over `best` bounds the value
//     improved, since the objective's `best` is asserted equal to what its `eval`
//     returns, as the library derives it (RoleDefined).
//     A case stating no condition is legitimately unbounded, not refused.
//   - Objectives, values and conditions are read through the runtime's own
//     surfaces (runtime.Context.ObjectivesOf), so what is optimized is what the
//     evaluator would evaluate; no declaration is re-parsed and no AST mutated.
//
// The objective term must be numeric and linear: an optimizer improves a linear
// objective, so a product or quotient of two computed values refuses with
// ErrNotOptimizable rather than being sent and misread. Divisor guards therefore
// sit in the conditions, where a computed divisor is asserted non-zero as usual.
//
// # Multiple objectives, and backend requirements
//
// Several objectives are optimized lexicographically in declaration order: each
// within what the ones before it already settled. The script says so itself with
// `(set-option :opt.priority lex)` rather than relying on a backend default —
// z3's `box` mode reports each objective's optimum separately and returns a model
// attaining only one of them, which would make "the assignments achieving the
// optimum" untrue.
//
// `(minimize e)`/`(maximize e)`, `(get-objectives)` and `:opt.priority` are
// solver extensions rather than SMT-LIB2; cvc5 implements none of them.
// Solver.Optimize settles them through the capability model below before sending
// a query — CapOptimization and CapOptimizationPriority, probed once per backend
// and cached — and reports a backend without them as NoOptimizationError, which
// wraps both ErrNoOptimization and the ErrUnsupportedCapability refusal it was
// settled by. Nothing is ever degraded to a plain check-sat and presented as an
// optimum.
//
// # What an optimum is, and is not
//
// Optimum.Status keeps the cases apart, and no case fabricates a number:
//
//   - OptimumAttained: the value reported is the optimum and an assignment
//     attains it. Both are checked here rather than taken on the backend's word:
//     the objective's value in the reported model is read back, and a further
//     check asks whether any assignment does lexicographically better — better in
//     an earlier objective, or equal there and better in a later one. Unsat is
//     what makes the answer an optimum.
//   - OptimumUnbounded: the conditions permit arbitrarily better values (`oo`).
//   - OptimumBounded: the objective approaches a bound no assignment attains, as
//     a strict inequality over the reals does. Backends report this as an
//     infinitesimal (`(+ 10.5 (* (- 1.0) epsilon))`) or an interval; the bound is
//     reported as a bound, never as an attained value.
//   - OptimumUnverified: the backend reported a value a better feasible value
//     refutes. z3 4.8.12 does this for open real suprema — maximizing x under
//     x < 10.5 reports 9.5 — which is why every optimum is verified rather than
//     trusted.
//   - OptimumUndecided: verification did not decide, or the answer was no number.
//
// Optimum.Feasible is always a value the reported assignment attains: a witness
// the conditions permit, not an optimum. A solver answering sat but refusing to
// report its objectives readably is an OptimumError wrapping ErrNoOptimum and
// ErrSolverProcess; unsat and unknown stay the verdicts they are, with no optima
// invented for them, and a query stating no objective is a NoObjectiveError
// wrapping ErrNoObjective rather than a satisfiability check.
//
// # Deliberately out of subset
//
// Everything else refuses with ErrNotTranslatable, and one refused conjunct
// fails the whole query, so no partial script exists:
//
//   - Collections and quantifiers: sequences, sets, `->select`, `->collect`,
//     `->forAll`, `->exists`, `->size`, indexing `#(i)`, ranges `a..b`, and
//     collection-valued features. Bounded expansion is not implemented.
//   - Invocations of any kind, calc usages included: a calc body may be
//     iterative or read state, and constant folding it is the evaluator's job.
//   - Euclidean `div`/`mod` themselves, as SMT-LIB defines them: only the
//     evaluator's truncating semantics are encoded.
//   - Real remainder `%`, which the evaluator answers by floating-point
//     remainder.
//   - `**` and `^`: exponentiation is outside linear and polynomial arithmetic
//     as encoded here.
//   - Classification and metadata operators: `hastype`, `istype`, `@`, `@@`,
//     `as`, `meta`, `all`, `===`, `!==`, `??`, `~`, and `null`.
//   - Complex numbers, string operations other than equality, and features whose
//     type determines no scalar sort.
//   - Comparing or adding magnitudes of different dimensions, which the
//     evaluator reports as incommensurable units.
//   - Unresolved names and feature chains that ground in nothing.
package solve
