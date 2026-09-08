package runtime

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

// frame is one level of local bindings an evaluation reads: a calc invocation's
// parameter slots, a map of named values, or both.
type frame struct {
	slots *slotFrame
	vars  map[string]Value
	// aliases map the name of a redefined feature to the name of the feature
	// redefining it, which the frame binds it under (`in g :>> x` holds x as g).
	aliases map[string]string
	// perf is the action performance vars belongs to, if any, whose flow's nodes
	// the frame also answers for (action_frame.go).
	perf *actionFrame
	// owner is the calc whose parameters, locals and outputs the frame binds, so a
	// qualified name of one of its members (`MassCase::result`) reads the binding.
	owner *calcShape
	// performed is the action whose performance a snapshot copied its bindings from,
	// so the copy still answers for a run of that action without the live perf.
	performed *symbols.Symbol
	// run numbers the behavior run the frame binds (Context.newRun), 0 for bindings
	// that are no run's; a function closing over the run is identified by it.
	run int64
}

// canonical is the name aliases bind name under: its redefinition's, else its own.
func canonical(aliases map[string]string, name string) string {
	if alias, ok := aliases[name]; ok {
		return alias
	}
	return name
}

// aliasRedefined records that name now binds the feature redefined was known by.
// Aliases already pointing at redefined follow it to name.
func aliasRedefined(aliases map[string]string, redefined, name string) map[string]string {
	if redefined == "" || name == "" || redefined == name {
		return aliases
	}
	if aliases == nil {
		aliases = make(map[string]string)
	}
	for alias, target := range aliases {
		if target == redefined {
			aliases[alias] = name
		}
	}
	aliases[redefined] = name
	delete(aliases, name)
	return aliases
}

// mapFrame is a frame holding vars alone.
func mapFrame(vars map[string]Value) frame {
	return frame{vars: vars}
}

// ownedFrame is a frame holding the values a run of owner bound, by name.
func ownedFrame(owner *calcShape, vars map[string]Value) frame {
	return frame{vars: vars, owner: owner}
}

// runs reports whether the frame holds a run of behavior: an invocation or usage of
// that calc or one specializing it, or a performance of that action or one typed by it.
func (f frame) runs(ctx *Context, behavior *symbols.Symbol) bool {
	if f.owner != nil {
		return f.owner.qualifiedBy(ctx, behavior)
	}
	if performed := f.performs(); performed != nil {
		return ctx.isOrSpecializes(performed, behavior)
	}
	return false
}

// performs is the action the frame holds a performance of: the live one's, or
// the one a snapshot copied; nil for a frame of a calc run or of plain bindings.
func (f frame) performs() *symbols.Symbol {
	if f.perf != nil && f.perf.scope != nil {
		return f.perf.scope.Owner()
	}
	return f.performed
}

// withVars is the frame holding vars in place of its own, still answering for
// the same run and performance.
func (f frame) withVars(vars map[string]Value) frame {
	return frame{vars: vars, aliases: f.aliases, perf: f.perf, owner: f.owner, performed: f.performed, run: f.run}
}

// lookup finds name in the frame: a slot binding it, else the map.
func (f frame) lookup(name string) (Value, bool) {
	name = canonical(f.aliases, name)
	if f.slots != nil {
		if value, ok := f.slots.lookup(name); ok {
			return value, true
		}
	}
	value, ok := f.vars[name]
	return value, ok
}

// has reports whether the frame binds name.
func (f frame) has(name string) bool {
	_, ok := f.lookup(name)
	return ok
}

// set binds name: in its slot when the frame has one for it, else in the map.
func (f frame) set(name string, value Value) {
	name = canonical(f.aliases, name)
	if f.slots != nil && f.slots.set(name, value) {
		return
	}
	f.vars[name] = value
}

// bindParam binds parameter i, named name: by position in the slots, else by name.
func (f frame) bindParam(i int, name string, value Value) {
	if f.slots != nil {
		f.slots.bind(i, value)
		return
	}
	f.vars[name] = value
}

// each calls fn for every name the frame binds.
func (f frame) each(fn func(name string, value Value)) {
	if f.slots != nil {
		f.slots.each(fn)
	}
	for name, value := range f.vars {
		fn(name, value)
	}
}

// snapshot copies the frame's bindings, and the aliases they are read through,
// into storage of its own, unchanged by whatever later reuses the frame's. The
// copy still answers for the run it was taken from, though not for its flow's nodes.
func (f frame) snapshot() frame {
	vars := make(map[string]Value, f.width())
	f.each(func(name string, value Value) { vars[name] = value })
	out := ownedFrame(f.owner, vars)
	out.performed, out.run = f.performs(), f.run
	if len(f.aliases) > 0 {
		out.aliases = make(map[string]string, len(f.aliases))
		for name, alias := range f.aliases {
			out.aliases[name] = alias
		}
	}
	return out
}

// width is the number of names the frame binds.
func (f frame) width() int {
	n := len(f.vars)
	if f.slots != nil {
		n += f.slots.width()
	}
	return n
}

// slotFrame holds a calc invocation's parameters by position, the names coming
// from its shape, so binding and reading them index rather than hash.
type slotFrame struct {
	names  []string
	values []Value
	bound  []bool
}

// reset prepares the frame for a calc binding the named parameters, with none
// bound yet, reusing the storage it holds.
func (s *slotFrame) reset(names []string) {
	s.names = names
	n := len(names)
	if cap(s.values) < n {
		s.values = make([]Value, n)
		s.bound = make([]bool, n)
	} else {
		s.values = s.values[:n]
		s.bound = s.bound[:n]
		clear(s.values)
		clear(s.bound)
	}
}

// release drops what the frame holds, keeping its storage for the next reset.
func (s *slotFrame) release() {
	clear(s.values)
	clear(s.bound)
	s.names = nil
	s.values = s.values[:0]
	s.bound = s.bound[:0]
}

// bind binds the parameter in slot i.
func (s *slotFrame) bind(i int, value Value) {
	s.values[i] = value
	s.bound[i] = true
}

// lookup finds a bound parameter by name.
func (s *slotFrame) lookup(name string) (Value, bool) {
	for i, n := range s.names {
		if n == name && s.bound[i] {
			return s.values[i], true
		}
	}
	return Value{}, false
}

// set binds the parameter named name and reports whether there is one.
func (s *slotFrame) set(name string, value Value) bool {
	for i, n := range s.names {
		if n == name {
			s.bind(i, value)
			return true
		}
	}
	return false
}

// each calls fn for every bound parameter.
func (s *slotFrame) each(fn func(name string, value Value)) {
	for i, n := range s.names {
		if s.bound[i] {
			fn(n, s.values[i])
		}
	}
}

// width is the number of bound parameters.
func (s *slotFrame) width() int {
	n := 0
	for _, b := range s.bound {
		if b {
			n++
		}
	}
	return n
}
