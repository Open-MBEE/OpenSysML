// Package enginewire is the message set an external engine and the host exchange over
// standard input and output: JSON-RPC 2.0 objects, one per line. The types here are the
// wire's, shared by the host, the stand-in engine and the schema that publishes them.
package enginewire

import (
	"encoding/json"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/modelform"
)

// Version is the version of this message set.
const Version = 1

// JSONRPC is the value every message's jsonrpc member carries.
const JSONRPC = "2.0"

// Methods of the host's requests and notification, and of the engine's notification.
const (
	MethodDescribe = "describe"
	MethodCovers   = "covers"
	MethodRun      = "run"
	MethodCancel   = "cancel"
	MethodProgress = "progress"
)

// Error codes an engine answers a request with.
const (
	// CodeUnsupported: a construct met at run time and not refused in covers.
	CodeUnsupported = "unsupported"
	// CodeBudget: the engine stopped at a bound of its own.
	CodeBudget = "budget"
	// CodeInternal: the engine failed.
	CodeInternal = "internal"
)

// Message is one line of the protocol: a request (method, params, id), a notification
// (method, params, no id) or a response (id and result or error). Members are kept raw where
// their absence must be told from null.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is the error member of a response.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DescribeParams are the describe request's: the protocol versions the host serves.
type DescribeParams struct {
	Protocols []int `json:"protocols"`
}

// Description is the engine's answer to describe: its manifest entry as the engine itself
// states it, checked field by field against the file. Optional members are omitted when the
// engine leaves them to the manifest.
type Description struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Protocol   int      `json:"protocol"`
	Answers    []string `json:"answers"`
	Subjects   []string `json:"subjects,omitempty"`
	Bounds     []string `json:"bounds,omitempty"`
	Witness    string   `json:"witness,omitempty"`
	Model      []string `json:"model,omitempty"`
	Authority  string   `json:"authority,omitempty"`
	Concurrent *bool    `json:"concurrent,omitempty"`
}

// CoversParams are the covers request's: the question and the model.
type CoversParams struct {
	Question Question `json:"question"`
	Model    Model    `json:"model"`
}

// CoversResult is the engine's answer to covers.
type CoversResult struct {
	Covers bool   `json:"covers"`
	Reason string `json:"reason,omitempty"`
}

// RunParams are the run request's: the question, the model, the bounds the question fixes
// and the budget.
type RunParams struct {
	Question Question `json:"question"`
	Model    Model    `json:"model"`
	Bounds   []Bound  `json:"bounds"`
	Budget   Budget   `json:"budget"`
}

// CancelParams are the cancel notification's: the id of the run to stop.
type CancelParams struct {
	ID int64 `json:"id"`
}

// ProgressParams are the progress notification's: the run's id and how far it is.
type ProgressParams struct {
	ID    int64  `json:"id"`
	Runs  int64  `json:"runs,omitempty"`
	Depth int64  `json:"depth,omitempty"`
	Steps int64  `json:"steps,omitempty"`
	Text  string `json:"text,omitempty"`
}

// Question is the framework's question in the model's own names: no host pointer crosses.
type Question struct {
	// Kind is evaluate, outcomes, holds, sensitive, satisfiable, sweep or compute.
	Kind string `json:"kind"`
	// Subject is the element asked about, as the surface spelled it.
	Subject string `json:"subject"`
	// SubjectKind is the subject's declaration kind as a manifest's subjects spells it
	// (action, state, calc, …), absent when the model does not declare the subject once.
	SubjectKind string `json:"subjectKind,omitempty"`
	// Schedule is the scheduling policy the question states, as -schedule spells it.
	Schedule string `json:"schedule"`
	// ModelSeed is the seed the runs' modeled draws come from, apart from the schedule's;
	// absent leaves them to a `seed:<n>` schedule.
	ModelSeed *uint64 `json:"modelSeed,omitempty"`
	// Draws is the policy the runs' RandomFunctions draws resolve under, as -draws
	// spells it (min, max, average); absent draws at random.
	Draws string `json:"draws,omitempty"`
	// Free is what the question leaves open: "schedule", "inputs".
	Free []string `json:"free"`
	// Condition is the requirement or constraint a holds question asks about; absent asks
	// only that no schedule deadlocks.
	Condition *Condition `json:"condition,omitempty"`
	// Conditions are the condition sets of a satisfiable question, one per query.
	Conditions []ConditionSet `json:"conditions,omitempty"`
	// Bindings are the fixed values of the subject's inputs.
	Bindings []Value `json:"bindings,omitempty"`
	// Inputs are the free inputs with their declared types, units and domains.
	Inputs []FreeInput `json:"inputs,omitempty"`
	// Sweep is the domain of a sweep question.
	Sweep *Sweep `json:"sweep,omitempty"`
}

// Condition is one condition by qualified name and as expression text.
type Condition struct {
	Name string `json:"name"`
	Text string `json:"text,omitempty"`
}

// ConditionSet is one query of a satisfiable question: the free features and the
// assertions over them.
type ConditionSet struct {
	Name       string      `json:"name"`
	Features   []FreeInput `json:"features"`
	Assertions []Condition `json:"assertions"`
	Pinned     []Pinned    `json:"pinned,omitempty"`
}

// Pinned is one feature the model fixes, its value as the notation writes it.
type Pinned struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// FreeInput is one free feature: its name, declared type, unit and domain.
type FreeInput struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Unit   string `json:"unit,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// Sweep is a sweep question's domain; a Monte Carlo states runs and no range, with a
// seed unless its runs draw nothing at random (a fixed Draws policy). Seed is present,
// zero included, whenever the rows are drawn from it.
type Sweep struct {
	Ranges  []Range `json:"ranges"`
	Sampled bool    `json:"sampled,omitempty"`
	Samples int64   `json:"samples,omitempty"`
	Seed    *uint64 `json:"seed,omitempty"`
	Runs    int64   `json:"runs,omitempty"`
}

// Range is one parameter's range of a sweep.
type Range struct {
	Parameter string          `json:"parameter"`
	Type      string          `json:"type"`
	Unit      string          `json:"unit,omitempty"`
	From      json.RawMessage `json:"from"`
	To        json.RawMessage `json:"to"`
	Step      json.RawMessage `json:"step,omitempty"`
}

// Value is one named value: a JSON number, boolean or string, and for a quantity its unit.
type Value struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
	Unit  string          `json:"unit,omitempty"`
}

// Bound is one bound taken, and whether it was reached.
type Bound struct {
	Name    string `json:"name"`
	Limit   int64  `json:"limit"`
	Reached bool   `json:"reached,omitempty"`
}

// Budget is what a run may spend: the deadline as an absolute RFC 3339 time, empty for
// none, and the counts; a zero count is the engine's own default. Memory is passed to
// honor and not enforced by the host.
type Budget struct {
	Deadline string `json:"deadline,omitempty"`
	Depth    int64  `json:"depth,omitempty"`
	Steps    int64  `json:"steps,omitempty"`
	Runs     int64  `json:"runs,omitempty"`
	Memory   int64  `json:"memory,omitempty"`
	Jobs     int64  `json:"jobs,omitempty"`
}

// Model is the model in the forms the entry asked for; sources is always present
// and graphs is the `graphs:<v>` form, modelform.Graphs as JSON.
type Model struct {
	Sources *modelform.Sources `json:"sources,omitempty"`
	Graphs  json.RawMessage    `json:"graphs,omitempty"`
}

// Result is the engine's answer to run: the framework's Result as JSON. Strength is what
// the engine claims; the host decides what is printed.
type Result struct {
	Claim      string    `json:"claim"`
	Strength   string    `json:"strength"`
	Bounds     []Bound   `json:"bounds,omitempty"`
	Witness    *Witness  `json:"witness,omitempty"`
	Executions []Witness `json:"executions,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Values     []Value   `json:"values,omitempty"`
	// Inputs is the engine's account of the initial state it ranged over or pinned; a
	// witness's inputs stand for it when absent.
	Inputs []Input `json:"inputs,omitempty"`
	// Assumptions name the constraints the engine assumed over the initial state.
	Assumptions []string `json:"assumptions,omitempty"`
	// Elapsed is the engine's own time in milliseconds.
	Elapsed int64 `json:"elapsed,omitempty"`
}

// Input is one feature of the initial state as a result accounts for it: free over its
// domain or pinned, with the value a witness chose spelled as notation.
type Input struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Free     bool   `json:"free,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Value    string `json:"value,omitempty"`
}

// Witness is one execution or assignment the host checks. A schedule witness has one
// schedule (two for sensitive) in the replay format, the move a violation is at, the
// feature two schedules diverge on, and the inputs; an assignment witness has inputs alone.
type Witness struct {
	// Schedules are replay files: `step N: <token>@<node> first of …` lines.
	Schedules []string `json:"schedules,omitempty"`
	// At is the move a violation is at; absent names the schedule's end.
	At *int `json:"at,omitempty"`
	// Feature is what two schedules of a sensitivity diverge on.
	Feature string `json:"feature,omitempty"`
	// Inputs are the values of the free features: for a schedule, those the run fixes
	// before its first move; for an assignment, the whole witness.
	Inputs []Value `json:"inputs,omitempty"`
}

// Assignment reports whether the witness is an assignment: inputs and no schedule.
func (w Witness) Assignment() bool { return len(w.Schedules) == 0 }
