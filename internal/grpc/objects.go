package grpc

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/objref"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// HeldObjectsEnvVar names the variable bounding the objects one cached model
// holds, nested objects counted: once the model holds that many, Instantiate is
// refused until the model leaves the cache, which releases them all.
const HeldObjectsEnvVar = "OPENSYSML_GRPC_MAX_HELD_OBJECTS"

// DefaultMaxHeldObjects is the bound HeldObjectsEnvVar takes when unset.
const DefaultMaxHeldObjects = 10000

// maxHeldObjectsFromEnv returns the positive integer HeldObjectsEnvVar holds, or
// DefaultMaxHeldObjects when it is unset or empty. An unusable value is an error
// naming the variable, rather than a silently kept default.
func maxHeldObjectsFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv(HeldObjectsEnvVar))
	if raw == "" {
		return DefaultMaxHeldObjects, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("held objects bound must be a positive integer, got %q (%s)", raw, HeldObjectsEnvVar)
	}
	return n, nil
}

// heldObjects is the population of objects Instantiate created for one cached
// model: a runtime outliving the requests, and the names its roots were
// instantiated under. Requests hold mu while they use the runtime.
type heldObjects struct {
	mu    sync.Mutex
	rt    *runtime.Context
	idx   *symbols.Index
	named map[string]*runtime.Instance // fqn -> the object the name denotes now
	// displaced are objects a later Instantiate of their name displaced, still
	// roots of their own reached by id.
	displaced []*runtime.Instance
}

// objects is the population held for cached, built by the first Instantiate
// over a runtime of its own so a request's worker never carries objects.
func (s *Service) objects(cached *CachedModel) *heldObjects {
	cached.objectsMu.Lock()
	defer cached.objectsMu.Unlock()
	if cached.objects == nil {
		model, _ := cached.Semantics()
		cached.objects = &heldObjects{
			rt:    s.newRuntimeContext(model),
			idx:   cached.Index,
			named: make(map[string]*runtime.Instance),
		}
	}
	return cached.objects
}

// lock takes exclusive use of the population and its runtime, and returns the
// function releasing it.
func (h *heldObjects) lock() func() {
	h.mu.Lock()
	return h.mu.Unlock
}

// hold records inst as the object the symbol it was instantiated from denotes;
// the object the name denoted before stays held, reached by id.
func (h *heldObjects) hold(sym *symbols.Symbol, inst *runtime.Instance) {
	fqn := h.idx.GetFQN(sym)
	if previous, ok := h.named[fqn]; ok && previous != nil && previous.ID != inst.ID {
		h.displaced = append(h.displaced, previous)
	}
	h.named[fqn] = inst
}

// empty reports whether no Instantiate has created an object for the model.
func (h *heldObjects) empty() bool {
	return len(h.named) == 0 && len(h.displaced) == 0
}

// room refuses another Instantiate once the model holds limit objects, so a
// population never outgrows the bound by more than one object graph.
func (h *heldObjects) room(limit int) error {
	if held := h.rt.InstanceCount(); held >= limit {
		return statusErrorf(connect.CodeResourceExhausted,
			"the model holds %d objects, the most %s allows (%d); raise it to hold more (the objects are released when the model leaves the cache)",
			held, HeldObjectsEnvVar, limit)
	}
	return nil
}

// roots are the objects a query enumerates from, each under the label it is
// reported by: named ones by qualified name in name order, displaced ones by id.
func (h *heldObjects) roots() []queryexec.Root {
	roots := make([]queryexec.Root, 0, len(h.named)+len(h.displaced))
	for fqn, inst := range h.named {
		roots = append(roots, queryexec.Root{Label: lexer.QualifiedNameText(fqn), Object: inst})
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Label < roots[j].Label })
	displaced := slices.Clone(h.displaced)
	sort.Slice(displaced, func(i, j int) bool { return displaced[i].ID < displaced[j].ID })
	for _, inst := range displaced {
		roots = append(roots, queryexec.Root{Label: fmt.Sprintf("#%d", inst.ID), Object: inst})
	}
	return roots
}

// queryContext is the context the model's queries execute in: its runtime and
// the objects it holds, under the labels they are reported by.
func (h *heldObjects) queryContext() queryexec.Context {
	return queryexec.Context{
		Index:    h.idx,
		Resolver: h.rt.Resolver(),
		Model:    h.rt.Semantics(),
		Runtime:  h.rt,
		Roots:    h.roots(),
	}
}

// resolve is the object a request's DocumentObject binds and the label it is
// reported under: by path when one is written, else by id. A malformed
// reference is refused as written, whatever the model holds.
func (h *heldObjects) resolve(parameter string, ref *pb.DocumentObject) (*runtime.Instance, string, error) {
	if ref.GetPath() == "" && ref.GetInstanceId() == 0 {
		return nil, "", statusErrorf(connect.CodeInvalidArgument,
			"binding %s: an object is bound by instance_id or by path, and neither was given", parameter)
	}
	var parsed objref.Ref
	if ref.GetPath() != "" {
		var err error
		if parsed, err = objref.Parse(ref.GetPath()); err != nil {
			return nil, "", statusErrorf(connect.CodeInvalidArgument, "binding %s: %v", parameter, err)
		}
	} else if ref.GetInstanceId() < 0 {
		return nil, "", bindingError(parameter, notAnID(ref.GetInstanceId()))
	}
	if h.empty() {
		return nil, "", statusErrorf(connect.CodeNotFound,
			"binding %s: the model holds no objects (Instantiate creates one)", parameter)
	}
	var (
		inst  *runtime.Instance
		label string
		err   error
	)
	if ref.GetPath() != "" {
		inst, label, err = h.resolvePath(parsed)
	} else {
		inst, err = h.byID(ref.GetInstanceId())
		label = fmt.Sprintf("#%d", ref.GetInstanceId())
	}
	if err != nil {
		return nil, "", bindingError(parameter, err)
	}
	if id := ref.GetInstanceId(); id != 0 && ref.GetPath() != "" && inst.ID != id {
		return nil, "", statusErrorf(connect.CodeInvalidArgument,
			"binding %s: %s is object #%d, not #%d", parameter, label, inst.ID, id)
	}
	return inst, label, nil
}

// bindingError prefixes a typed resolution failure with the parameter bound.
func bindingError(parameter string, err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return statusErrorf(connectErr.Code(), "binding %s: %s", parameter, connectErr.Message())
	}
	return statusErrorf(connect.CodeInvalidArgument, "binding %s: %v", parameter, err)
}

// notAnID refuses an id no object can have.
func notAnID(id int64) error {
	return statusErrorf(connect.CodeInvalidArgument, "#%d is not an object id (ids count up from 1)", id)
}

// byID is the object with id among those the runtime holds.
func (h *heldObjects) byID(id int64) (*runtime.Instance, error) {
	if id <= 0 {
		return nil, notAnID(id)
	}
	if inst, ok := h.rt.Instance(id); ok {
		return inst, nil
	}
	return nil, h.unknownID(id)
}

// unknownIDListed bounds the ids an unknown-id error spells out.
const unknownIDListed = 20

// unknownID reports an id no held object has, and which ids are held.
func (h *heldObjects) unknownID(id int64) error {
	ids := h.rt.InstanceIDs()
	if len(ids) == 0 {
		return statusErrorf(connect.CodeNotFound,
			"no object #%d for this model: nothing materialized has that identity (no objects have been created)", id)
	}
	listed := make([]string, 0, len(ids))
	for _, known := range ids[:min(len(ids), unknownIDListed)] {
		listed = append(listed, fmt.Sprintf("#%d", known))
	}
	more := ""
	if len(ids) > unknownIDListed {
		more = fmt.Sprintf(", … (%d in all)", len(ids))
	}
	return statusErrorf(connect.CodeNotFound,
		"no object #%d for this model: nothing materialized has that identity (the objects are %s%s)", id, strings.Join(listed, ", "), more)
}

// resolvePath is the object a parsed reference denotes — `#3`, an instantiated
// name, or a path through feature values from either — and the label it is
// reported under.
func (h *heldObjects) resolvePath(ref objref.Ref) (*runtime.Instance, string, error) {
	walker := objref.Walker{Runtime: h.rt, Index: h.idx}
	if ref.ID > 0 {
		inst, err := h.byID(ref.ID)
		if err != nil {
			return nil, "", err
		}
		return h.walked(walker.Walk(inst, fmt.Sprintf("#%d", ref.ID), ref.Segments))
	}
	inst, fqn, rest, err := h.namedRoot(ref)
	if err != nil {
		return nil, "", err
	}
	return h.walked(walker.Walk(inst, lexer.QualifiedNameText(fqn), rest))
}

// walked types a walk's failure: the path is the caller's, so an invalid argument.
func (h *heldObjects) walked(inst *runtime.Instance, label string, err error) (*runtime.Instance, string, error) {
	if err != nil {
		return nil, "", statusErrorf(connect.CodeInvalidArgument, "%v", err)
	}
	return inst, label, nil
}

// namedRoot finds the object a name-rooted reference starts from: the longest
// run of leading segments naming an instantiated declaration, its qualified
// name, and the segments left to walk from it.
func (h *heldObjects) namedRoot(ref objref.Ref) (*runtime.Instance, string, []objref.Segment, error) {
	head := objref.Head(ref.Segments)
	noInstance := ""
	for i := head; i > 0; i-- {
		sym, err := h.lookup(ref.Segments[:i])
		if err != nil {
			return nil, "", nil, err
		}
		if sym == nil {
			continue
		}
		fqn := h.idx.GetFQN(sym)
		if inst, ok := h.named[fqn]; ok && inst != nil {
			return inst, fqn, ref.Segments[i:], nil
		}
		if i == head && head < len(ref.Segments) && objref.IsNamespace(sym) {
			shown := lexer.QualifiedNameText(fqn)
			return nil, "", nil, statusErrorf(connect.CodeInvalidArgument,
				"%q is not an object reference: %s is a %s, not an object: its member is written %s::%s",
				ref.Text, shown, objref.NamespaceKind(sym), shown, ref.Segments[head].Text)
		}
		if noInstance == "" {
			noInstance = fqn
		}
	}
	if noInstance != "" {
		if h.empty() {
			return nil, "", nil, statusErrorf(connect.CodeNotFound,
				"no instance of %q: the model holds no objects (Instantiate creates one)", lexer.QualifiedNameText(noInstance))
		}
		return nil, "", nil, statusErrorf(connect.CodeNotFound,
			"no instance of %q (use Instantiate first)", lexer.QualifiedNameText(noInstance))
	}
	return nil, "", nil, statusErrorf(connect.CodeNotFound, "symbol not found: %s", objref.JoinTyped(ref.Segments[:head]))
}

// lookup is the declaration segments name: by qualified name as every RPC
// reads one, or — for a lone name, as the REPL reads `car` — the one model
// declaration of that name among all declared. Nil when nothing is declared under it.
func (h *heldObjects) lookup(segments []objref.Segment) (*symbols.Symbol, error) {
	name := objref.JoinTyped(segments)
	if syms := lookupNamed(h.idx, name); len(syms) > 0 {
		return syms[0], nil
	}
	if len(segments) != 1 {
		return nil, nil
	}
	var found []*symbols.Symbol
	for _, fqn := range h.idx.FQNsEndingIn(segments[0].Name, math.MaxInt) {
		for _, sym := range h.idx.LookupQualified(fqn) {
			if !h.idx.Library(sym) {
				found = append(found, sym)
			}
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	}
	names := make([]string, 0, len(found))
	for _, sym := range found {
		names = append(names, lexer.QualifiedNameText(h.idx.GetFQN(sym)))
	}
	sort.Strings(names)
	return nil, statusErrorf(connect.CodeInvalidArgument,
		"%q is ambiguous: it names %s (write the qualified name)", name, strings.Join(names, ", "))
}
