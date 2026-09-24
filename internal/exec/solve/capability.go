package solve

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// Capability is one thing this layer needs of a backend beyond parsing SMT-LIB2:
// a theory feature, a script-level facility, or a solver's own extension.
type Capability int

const (
	// CapModels is `:produce-models` with `(get-value …)`, which every answer
	// carrying an assignment needs.
	CapModels Capability = iota
	// CapUnsatCores is `:produce-unsat-cores` with `:named` assertions and
	// `(get-unsat-core)`, which explaining a conflict needs.
	CapUnsatCores
	// CapIncremental is more than one `(check-sat)` in one dialogue, which
	// enumerating configurations needs.
	CapIncremental
	// CapDatatypes is `(declare-datatypes …)` with nullary constructors, which
	// enumerations and variation points are declared as.
	CapDatatypes
	// CapStrings is the String sort with equality, from the strings theory.
	CapStrings
	// CapIntegerDivision is `div` and `mod` from the Ints theory.
	CapIntegerDivision
	// CapNonlinearArith is a product or quotient of two non-literal terms.
	CapNonlinearArith
	// CapMixedArith is the mixed integer and real logics AUFLIRA and AUFNIRA,
	// which a query over both Int and Real is set to.
	CapMixedArith
	// CapNonStandardLogic is `(set-logic ALL)`, which no SMT-LIB logic name
	// covers and which datatypes and strings are set to.
	CapNonStandardLogic
	// CapOptimization is `(maximize …)`/`(minimize …)` with `(get-objectives)`,
	// a solver extension rather than SMT-LIB 2.6.
	CapOptimization
	// CapOptimizationPriority is `:opt.priority`, a z3 extension for ordering
	// several objectives.
	CapOptimizationPriority
)

// AllCapabilities are every capability, in declaration order: what a probe
// reports on and what a compatibility table lists.
var AllCapabilities = []Capability{
	CapModels, CapUnsatCores, CapIncremental, CapDatatypes, CapStrings,
	CapIntegerDivision, CapNonlinearArith, CapMixedArith, CapNonStandardLogic,
	CapOptimization, CapOptimizationPriority,
}

// capabilityNames are the short names a report and a message use.
var capabilityNames = map[Capability]string{
	CapModels:               "models",
	CapUnsatCores:           "unsat-cores",
	CapIncremental:          "incremental",
	CapDatatypes:            "datatypes",
	CapStrings:              "strings",
	CapIntegerDivision:      "integer-division",
	CapNonlinearArith:       "nonlinear-arithmetic",
	CapMixedArith:           "mixed-arithmetic",
	CapNonStandardLogic:     "non-standard-logic",
	CapOptimization:         "optimization",
	CapOptimizationPriority: "optimization-priority",
}

// capabilityFeatures say what SMT-LIB, or which extension, each capability is.
var capabilityFeatures = map[Capability]string{
	CapModels:               "SMT-LIB 2.6 model output: `:produce-models` with `(get-value …)`",
	CapUnsatCores:           "SMT-LIB 2.6 unsat cores: `:produce-unsat-cores` with `:named` assertions and `(get-unsat-core)`",
	CapIncremental:          "an incremental dialogue: more than one `(check-sat)` in one script",
	CapDatatypes:            "`(declare-datatypes …)` for enumerations and variation points",
	CapStrings:              "the String sort of the strings theory",
	CapIntegerDivision:      "`div` and `mod` from the Ints theory",
	CapNonlinearArith:       "nonlinear arithmetic: a product or quotient of two non-literal terms",
	CapMixedArith:           "the mixed integer and real logics AUFLIRA and AUFNIRA",
	CapNonStandardLogic:     "`(set-logic " + NonStandardLogic + ")`, which the SMT-LIB logic list does not define",
	CapOptimization:         "`(maximize …)`/`(minimize …)` with `(get-objectives)`, a solver extension",
	CapOptimizationPriority: "`:opt.priority` for ordering objectives, a z3 extension",
}

// String names the capability as a report names it.
func (c Capability) String() string {
	if name, ok := capabilityNames[c]; ok {
		return name
	}
	return "capability"
}

// Feature says what SMT-LIB feature, or which extension, the capability is.
func (c Capability) Feature() string {
	if feature, ok := capabilityFeatures[c]; ok {
		return feature
	}
	return c.String()
}

// capState is what a check established about one capability: that the backend
// has it, that it refused it, or that the check settled neither.
type capState int

const (
	// capUnknown is a check the backend neither answered nor rejected: it gave up,
	// stopped, or ran out of time. Nothing was learned, so nothing is refused on
	// its account and the operation runs and reports what happens.
	capUnknown capState = iota
	capSupported
	capRefused
)

// Capabilities is what one backend supports of the subset this layer needs,
// either probed by running it or declared for a backend already known.
type Capabilities struct {
	// Solver names the backend the answers are about.
	Solver string

	// Probed reports that the answers came from running the executable rather
	// than from a declaration.
	Probed bool

	// Elapsed is how long probing took, zero for declared capabilities.
	Elapsed time.Duration

	states  map[Capability]capState
	details map[Capability]string
}

// DeclaredCapabilities states what a backend supports without probing it, which
// is how a caller that already knows a backend avoids the probe.
func DeclaredCapabilities(solver string, supported ...Capability) *Capabilities {
	caps := &Capabilities{Solver: solver, states: map[Capability]capState{}, details: map[Capability]string{}}
	for _, c := range supported {
		caps.states[c] = capSupported
	}
	for _, c := range AllCapabilities {
		if caps.states[c] != capSupported {
			caps.states[c], caps.details[c] = capRefused, "it was declared not to support this"
		}
	}
	return caps
}

// Supports reports whether the backend was found to have the capability.
func (c *Capabilities) Supports(capability Capability) bool {
	return c.state(capability) == capSupported
}

// Refuses reports whether the backend rejected the capability. Only a refusal
// stops a query: a check that settled nothing does not.
func (c *Capabilities) Refuses(capability Capability) bool {
	return c.state(capability) == capRefused
}

// Undetermined reports that the check neither established the capability nor was
// refused, so what the backend does with the feature is unknown.
func (c *Capabilities) Undetermined(capability Capability) bool {
	return c.state(capability) == capUnknown
}

// state is what was established about the capability.
func (c *Capabilities) state(capability Capability) capState {
	if c == nil {
		return capUnknown
	}
	return c.states[capability]
}

// Detail is what the backend said when it refused the capability or left the
// check undecided, empty for one it supports.
func (c *Capabilities) Detail(capability Capability) string {
	if c == nil {
		return ""
	}
	return c.details[capability]
}

// Missing are those of the capabilities the backend refused, in the order given,
// with duplicates dropped.
func (c *Capabilities) Missing(needed []Capability) []Capability {
	var out []Capability
	seen := map[Capability]bool{}
	for _, capability := range needed {
		if seen[capability] || !c.Refuses(capability) {
			continue
		}
		seen[capability] = true
		out = append(out, capability)
	}
	return out
}

// Supported are the capabilities the backend has, in AllCapabilities order.
func (c *Capabilities) Supported() []Capability {
	var out []Capability
	for _, capability := range AllCapabilities {
		if c.Supports(capability) {
			out = append(out, capability)
		}
	}
	return out
}

// probe is one capability check: a self-contained script, and the replies a
// backend supporting it gives. Each check runs in its own process, since a
// strict solver stops reading after rejecting a command.
type probe struct {
	capability Capability
	script     string
	want       []reply
}

// reply is what one answer must be for a check to count as supported.
type reply int

const (
	// replySat, replyUnsat and replyDecided require that verdict; replyDecided
	// accepts unknown too, for a check whose arithmetic a solver may give up on.
	replySat reply = iota
	replyUnsat
	replyDecided
	// replyList requires a non-empty list, as a model, a core or the objectives
	// are.
	replyList
)

// probes are the capability checks, one per capability. Each asks the smallest
// script that exercises the feature the layer actually emits.
var probes = []probe{{
	capability: CapModels,
	script:     "(set-option :produce-models true)\n(set-logic QF_LIA)\n(declare-const x Int)\n(assert (> x 1))\n(check-sat)\n(get-value (x))\n",
	want:       []reply{replySat, replyList},
}, {
	capability: CapUnsatCores,
	script: "(set-option :produce-unsat-cores true)\n(set-logic QF_LIA)\n(declare-const x Int)\n" +
		"(assert (! (> x 1) :named " + CoreLabel(0) + "))\n(assert (! (< x 0) :named " + CoreLabel(1) + "))\n" +
		"(check-sat)\n(get-unsat-core)\n",
	want: []reply{replyUnsat, replyList},
}, {
	capability: CapIncremental,
	script:     "(set-logic QF_LIA)\n(declare-const x Int)\n(assert (> x 1))\n(check-sat)\n(assert (< x 0))\n(check-sat)\n",
	want:       []reply{replySat, replyUnsat},
}, {
	capability: CapDatatypes,
	// The logic is the one a datatype query is written with, so the check reports
	// on the script the writer emits rather than on a friendlier one.
	script: "(set-option :produce-models true)\n(set-logic " + NonStandardLogic + ")\n" +
		"(declare-datatypes ((F 0)) (((m) (p))))\n(declare-const f F)\n(assert (= f m))\n(check-sat)\n(get-value (f))\n",
	want: []reply{replySat, replyList},
}, {
	capability: CapStrings,
	script: "(set-option :produce-models true)\n(set-logic " + NonStandardLogic + ")\n" +
		"(declare-const s String)\n(assert (= s \"hi\"))\n(check-sat)\n(get-value (s))\n",
	want: []reply{replySat, replyList},
}, {
	capability: CapIntegerDivision,
	script:     "(set-logic QF_LIA)\n(declare-const x Int)\n(assert (= (div x 2) 3))\n(assert (= (mod x 3) 1))\n(check-sat)\n",
	want:       []reply{replySat},
}, {
	capability: CapNonlinearArith,
	script:     "(set-logic QF_NIA)\n(declare-const x Int)\n(declare-const y Int)\n(assert (= (* x y) 6))\n(check-sat)\n",
	want:       []reply{replyDecided},
}, {
	capability: CapNonlinearArith,
	// The nonlinear real logic, which a query over reals is set to instead.
	script: "(set-logic QF_NRA)\n(declare-const x Real)\n(declare-const y Real)\n" +
		"(assert (= (* x y) 6.0))\n(check-sat)\n",
	want: []reply{replyDecided},
}, {
	capability: CapNonlinearArith,
	// Division by a variable, which is what makes such a query nonlinear.
	script: "(set-logic QF_NIA)\n(declare-const x Int)\n(declare-const d Int)\n" +
		"(assert (and (> d 0) (= (div x d) 3)))\n(check-sat)\n",
	want: []reply{replyDecided},
}, {
	capability: CapMixedArith,
	script: "(set-logic AUFLIRA)\n(declare-const i Int)\n(declare-const r Real)\n" +
		"(assert (> (+ (to_real i) r) 1.0))\n(check-sat)\n",
	want: []reply{replyDecided},
}, {
	capability: CapMixedArith,
	// The nonlinear mixed logic, which a nonlinear query over both is set to.
	script: "(set-logic AUFNIRA)\n(declare-const i Int)\n(declare-const r Real)\n" +
		"(assert (> (* (to_real i) r) 1.0))\n(check-sat)\n",
	want: []reply{replyDecided},
}, {
	capability: CapNonStandardLogic,
	script:     "(set-logic " + NonStandardLogic + ")\n(declare-const x Int)\n(assert (> x 1))\n(check-sat)\n",
	want:       []reply{replySat},
}, {
	capability: CapOptimization,
	script: "(set-logic QF_LIA)\n(declare-const x Int)\n(assert (and (> x 1) (< x 5)))\n" +
		"(maximize x)\n(check-sat)\n(get-objectives)\n",
	want: []reply{replySat, replyList},
}, {
	capability: CapOptimizationPriority,
	script: "(set-option :opt.priority lex)\n(set-logic QF_LIA)\n(declare-const x Int)\n" +
		"(assert (and (> x 1) (< x 5)))\n(maximize x)\n(check-sat)\n",
	want: []reply{replySat},
}}

// ProbeTimeout is how long one capability check is given. The checks are tiny, so
// a backend that does not answer in this long is refusing rather than working.
const ProbeTimeout = 10 * time.Second

// Capabilities reports what this backend supports of the whole subset, probing
// each capability once per executable however many queries ask. Declared
// capabilities are returned as they are, unprobed.
func (s *Solver) Capabilities(ctx context.Context) (*Capabilities, error) {
	return s.capabilitiesFor(ctx, AllCapabilities)
}

// capabilitiesFor reports on just the capabilities named, so a query pays for no
// check it does not need and none is run twice.
func (s *Solver) capabilitiesFor(ctx context.Context, needed []Capability) (*Capabilities, error) {
	if s.Declared != nil {
		return s.Declared, nil
	}
	entry := capabilityEntry(s.cacheKey())
	caps := &Capabilities{
		Solver: s.Name, Probed: true,
		states: map[Capability]capState{}, details: map[Capability]string{},
	}
	started := time.Now()
	for _, capability := range needed {
		result, err := entry.probe(ctx, s, capability)
		if err != nil {
			return nil, err
		}
		caps.states[capability], caps.details[capability] = result.state, result.detail
	}
	caps.Elapsed = time.Since(started)
	return caps, nil
}

// probeTimeout is how long one check is given: no longer than a query itself, so
// the timeout the operator set bounds the checks before it too.
func (s *Solver) probeTimeout() time.Duration {
	if s.Timeout > 0 && s.Timeout < ProbeTimeout {
		return s.Timeout
	}
	return ProbeTimeout
}

// msgItAnswered opens a refusal quoting what the solver replied.
const msgItAnswered = "it answered "

// runProbe runs one capability check: a backend that rejects the script refuses
// the capability, one that answers has it, and one that gives up or stops settles
// neither. Only a backend that could not be run at all is an error.
func (s *Solver) runProbe(ctx context.Context, p probe) (capabilityResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, s.probeTimeout())
	defer cancel()
	sess, err := s.start(runCtx)
	if err != nil {
		return capabilityResult{}, err
	}
	defer sess.close()
	if err := sess.send(p.script); err != nil {
		return capabilityResult{state: capUnknown, detail: "it stopped reading the check"}, nil
	}
	for i, want := range p.want {
		got, err := readSexpr(sess.out)
		// A backend acknowledging commands has not answered the check yet.
		for err == nil && got.Atom == "success" {
			got, err = readSexpr(sess.out)
		}
		if err != nil {
			if ctx.Err() != nil {
				return capabilityResult{}, ctx.Err()
			}
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				return undetermined(i, "it did not answer within "+s.probeTimeout().String()), nil
			}
			return undetermined(i, strings.TrimSuffix("it stopped without answering: "+sess.stderrText(), ": ")), nil
		}
		if msg, isErr := got.isError(); isErr {
			return refused(i, "it rejected the script: "+comment(msg)), nil
		}
		if got.Atom == "unsupported" {
			return refused(i, msgItAnswered+quoteReply(got)), nil
		}
		if !smtlibResponse(got) {
			// A reply SMT-LIB does not define says nothing about the feature: the
			// executable is not answering as a solver, which is a process failure.
			return capabilityResult{}, s.processError("capability check",
				msgItAnswered+quoteReply(got)+" rather than an SMT-LIB response",
				sess.stderrText(), nil)
		}
		if got.Atom == "unknown" && want != replyDecided {
			return undetermined(i, msgItAnswered+quoteReply(got)), nil
		}
		if !want.accepts(got) {
			return refused(i, msgItAnswered+quoteReply(got)), nil
		}
	}
	return capabilityResult{state: capSupported}, nil
}

// refused and undetermined are one check's outcome, saying which command of it the
// backend answered that way.
func refused(i int, detail string) capabilityResult {
	return capabilityResult{state: capRefused, detail: refusedAt(i, detail)}
}

func undetermined(i int, detail string) capabilityResult {
	return capabilityResult{state: capUnknown, detail: refusedAt(i, detail)}
}

// refusedAt says which command of a check the backend answered that way at.
func refusedAt(i int, detail string) string {
	if i == 0 {
		return detail
	}
	return "after answering the first command, " + detail
}

// smtlibResponse reports whether the reply is one SMT-LIB defines: a verdict, an
// acknowledgement, `unsupported`, or a list such as a model, core or objectives.
func smtlibResponse(got sexpr) bool {
	if got.IsList {
		return true
	}
	switch got.Atom {
	case "sat", "unsat", "unknown", "unsupported", "success":
		return true
	}
	return false
}

// accepts reports whether a reply is the answer a supporting backend gives.
func (r reply) accepts(got sexpr) bool {
	if r == replyList {
		return got.IsList && len(got.List) > 0
	}
	switch got.Atom {
	case "sat":
		return r == replySat || r == replyDecided
	case "unsat":
		return r == replyUnsat || r == replyDecided
	case "unknown":
		return r == replyDecided
	}
	return false
}

// capabilityCache holds the probe answer per backend, so one process probes an
// executable once however many queries it asks.
var capabilityCache = struct {
	mu      sync.Mutex
	entries map[string]*capabilityRecord
}{entries: map[string]*capabilityRecord{}}

// capabilityRecord is one backend's checks, each run at most once. A check that
// could not be run at all is not remembered, so the next ask retries it: the
// failure may be the environment's rather than the backend's answer.
type capabilityRecord struct {
	mu      sync.Mutex
	results map[Capability]capabilityResult
}

// capabilityResult is one remembered check: what it established, and what the
// backend said when that was not support.
type capabilityResult struct {
	state  capState
	detail string
}

// probe answers one capability for a backend from the record, running the check
// only the first time it is asked.
func (r *capabilityRecord) probe(ctx context.Context, s *Solver, capability Capability) (capabilityResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if result, ok := r.results[capability]; ok {
		return result, nil
	}
	checks := probesFor(capability)
	if len(checks) == 0 {
		return capabilityResult{detail: "no check is defined for it"}, nil
	}
	// Every emitted form is checked, and the first answer short of support is the
	// capability's: taking one form and refusing another is not support.
	result := capabilityResult{state: capSupported}
	for _, check := range checks {
		got, err := s.runProbe(ctx, check)
		if err != nil {
			return capabilityResult{}, err
		}
		if got.state != capSupported {
			result = got
			break
		}
	}
	r.results[capability] = result
	return result, nil
}

// probesFor are the checks for a capability, one per form of it that is emitted.
func probesFor(capability Capability) []probe {
	var out []probe
	for _, p := range probes {
		if p.capability == capability {
			out = append(out, p)
		}
	}
	return out
}

// capabilityEntry is the cache record for a backend, created on first ask.
func capabilityEntry(key string) *capabilityRecord {
	capabilityCache.mu.Lock()
	defer capabilityCache.mu.Unlock()
	entry, ok := capabilityCache.entries[key]
	if !ok {
		entry = &capabilityRecord{results: map[Capability]capabilityResult{}}
		capabilityCache.entries[key] = entry
	}
	return entry
}

// cacheKey identifies the backend a probe answers about: the executable, the
// arguments it is run with, and the environment added to it.
func (s *Solver) cacheKey() string {
	return strings.Join(append(append([]string{s.Path}, s.Args...), s.Env...), "\x00")
}

// require refuses a query the backend cannot answer, naming the missing feature
// and the backend, rather than degrading the question or guessing a verdict.
// extra are the capabilities the operation adds to the query's own.
func (s *Solver) require(ctx context.Context, q *Query, operation string, extra ...Capability) error {
	needed := append(q.Requires(), extra...)
	caps, err := s.capabilitiesFor(ctx, needed)
	if err != nil {
		return err
	}
	missing := caps.Missing(needed)
	if len(missing) == 0 {
		return nil
	}
	return &UnsupportedCapabilityError{
		Solver: caps.Solver, Operation: operation, Missing: missing,
		Detail: caps.Detail(missing[0]),
	}
}

// Unsupported reports whether the error is a backend refusing a capability, which
// is a reported refusal rather than a defect in the script.
func Unsupported(err error) bool {
	return errors.Is(err, ErrUnsupportedCapability)
}
