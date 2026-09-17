// Package opensysml is the public Go API for OpenSysML: parse SysML v2 models,
// look up symbols, evaluate expressions, instantiate parts, run actions and
// state machines, verify constraints and requirements, evaluate calculations,
// query the model, render documents, convert notations and edit source. Every
// RPC the service answers is a method on Client.
//
// # Two implementations, one interface
//
// New answers in process: the engine is already linked into a Go binary that
// imports this module, so the default implementation calls it directly — no
// port, no child process, no serialization. Dial answers through a sysml-grpc
// service someone else runs, named by an explicit address; this package never
// starts one. Both implement Client identically, and the conformance suite in
// conformance/ runs through both to hold them to it.
//
//	client, err := opensysml.New()
//	// or: client, err := opensysml.Dial("localhost:50051")
//
// # Models of several documents
//
// ParseFile and ParseSource each parse one document, which reads no sibling
// file: a name imported from another file is an unresolved-reference diagnostic
// rather than an error. ParseFiles and ParseDocuments parse several documents as
// one model, so an import between them is satisfied and every symbol resolves
// across them:
//
//	model, err := client.ParseFiles(ctx, []string{"lib.sysml", "top.sysml"})
//
// Each document keeps its own name, so a diagnostic locates itself in the file
// it came from, and Model.Roots holds the root namespace of each in the order
// parsed. The rest of the API takes such a model as it takes any other, except
// that an operation rewriting one document's own notation — conversion from a
// model handle, and editing — is refused with CodeFailedPrecondition rather than
// applied to one document of several.
//
// # Sessions
//
// Every method on Client is request-scoped: ExecuteAction and ExecuteState run
// a whole behaviour and answer what it did. A Session is the other shape — a
// persistent, interactive run that keeps its clock, its scheduling policy and
// the objects it instantiated between calls, so a caller instantiates a part,
// sends its state machine a signal, performs an action on it and reads what
// changed, one step at a time:
//
//	session, err := opensysml.OpenSession(client, model)
//	hero, err := session.Instantiate("Play::hero")
//	acceptance, err := session.Accepts(hero, "Play::Go", nil)
//	_, err = session.Send(hero, "Play::Go", nil)
//	advanced, err := session.Advance(1)
//	performed, err := session.Perform(hero, "Play::Hero::pay", inputs)
//
// A Session is deliberately not part of the Client interface. Client is the
// set of RPCs the service answers, held to identical answers from New and Dial
// by the conformance suite; a session is state the engine holds between calls,
// which the service exposes no RPC for. Rather than a Dial that answers some
// methods and not others, OpenSession is a separate, in-process-only surface:
// it is opened from a Client, so a program reaches it through the same handle
// and model it uses for everything else, but only a client New returned can
// answer it, and a Dial client is refused with CodeUnimplemented. What a
// Session answers is facts — the transitions out of the active state and what
// fires them, whether a signal's guard holds now, the choice points a run made,
// whether an action's opening decision turned its caller away — never the
// engine's own graphs or objects, so the boundary the rest of the package keeps
// holds here too.
//
// # Answers, verdicts and refusals
//
// Operations that ask something of the model answer what it says rather than
// treating an unwelcome answer as an error. A constraint that does not hold is
// a Verdict with Holds false; only a verification that could not be evaluated
// at all is an error, a *VerifyError whose Reason classifies it. A set of edits
// either all apply or none do, and a refusal is an *EditError naming its kind.
//
// # Concurrency, contexts and lifetime
//
// A Client is safe for concurrent use from any number of goroutines. A call
// whose context is already done is refused with CodeCanceled or
// CodeDeadlineExceeded, and a context that ends while a call is running
// withholds its answer the same way — the engine, like a service, still runs
// the call it started. After Close every call is refused with CodeUnavailable,
// and closing twice is not an error.
//
// # Errors
//
// A call fails in one of two documented ways, and the difference is part of
// the API because it is part of the wire contract:
//
//   - A refused call is a *StatusError carrying the canonical gRPC status
//     code (rendered by its canonical name, as in "opensysml: NOT_FOUND: …"),
//     whichever implementation answered. errors.Is(err, CodeNotFound)
//     tests the code: an unknown model hash, for example, is CodeNotFound.
//   - A failure the service reports inside a successful answer — an
//     unparsable expression, an unknown symbol, a failed instantiation — is a
//     *FailureError, matched by errors.Is(err, ErrFailure), carrying the
//     service's message and any diagnostics.
//
// A panic in the engine does not cross this boundary: the in-process
// implementation reports it as a *StatusError with CodeInternal, the status
// the wire would answer for a crashed handler.
//
// # Ownership
//
// Everything a Client returns is a copy the caller owns. The in-process
// implementation has no serialization boundary, so this is a promise, not a
// consequence: no returned value aliases engine state, and mutating one
// changes nothing about the model, the cache or later answers.
//
// # Capabilities
//
// Negotiate on ServerInfo.Capabilities, not on versions. Capability-gated
// requests are refused when unavailable, while response-population capabilities
// omit the fields they name. ServerInfo.Has provides the client-side preflight.
//
// # Stability
//
// The module is v0, so the Go compatibility promise does not yet bind it.
// Within v0, the surface of this package is what OpenSysML commits to keeping
// compatible: see the package README for the statement. RDF conversion is
// experimental — a Conversion says so — and generated model-ergonomics types
// are deliberately absent, models being read through Symbol, Instance and
// Value.
//
// No operation shells out to an SMT solver: verification evaluates conditions
// with the same runtime Evaluate and Instantiate use. The solver-backed
// analyses belong to the REPL.
package opensysml
