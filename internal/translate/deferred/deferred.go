// Package deferred writes the standard SysML v2 encoding of a deferred signal:
// an ordered buffer item, a do action whose accept loop keeps each occurrence
// while the state is active, and an exit action that sends each kept
// occurrence to self. The SysML v1 migrator and the PSSM referee write it
// through this one package, so the two never spell the encoding apart.
package deferred

// Writer is the sink an encoding is written into: a line at the current
// indentation, a brace-delimited block whose body is one level deeper, text
// already rendered at the current indentation, and a name the encoding made
// up for the block being written, which a sink with no use for it ignores.
type Writer interface {
	Line(s string)
	Block(header string, body func())
	Raw(text string)
	MadeUp(name string)
}

// Loop is one accept loop of a kept signal: from the object itself when Via
// is "", via the port Via names otherwise. Its names are spelled as written;
// Accept names the signal as the accept's payload type, qualified past the
// payload's own name where that would shadow it.
type Loop struct {
	Receive, Keep, Payload string
	Accept, Via            string
}

// Signal is one signal a state keeps: the buffer item its occurrences fill,
// the accept loops that fill it, and the flush loop's members, every name
// spelled as written.
type Signal struct {
	// Ref names the signal as the buffer's type; Kind is the usage keyword
	// the buffer is declared with, `item` when "", `attribute` for a signal
	// that is an attribute definition.
	Ref, Kind   string
	Buffer      string
	Item, Clear string
	Loops       []Loop
}

// Encoding is a state's deferred signals with the members encoding them.
type Encoding struct {
	// Buffer and Flush name the do and exit actions; Split names the fork that
	// runs the accept loops beside each other and the state's own do behavior.
	Buffer, Split, Flush string
	// Including refers to SequenceFunctions::including from the state's scope.
	Including string
	Signals   []Signal
}

// Own is a state's own do or exit behavior rendered as the nested action the
// encoding runs beside its loops or ahead of its flush.
type Own struct {
	// Text is the rendering, at the body's indentation; "" for no behavior.
	Text string
	// Written reports whether Text declares the nested action the encoding
	// then runs: a rendering of comments alone does not.
	Written bool
	// Run names the nested action; MadeUp is that name when the rendering
	// made it up for a behavior its source left anonymous, "" otherwise.
	Run, MadeUp string
}

// LoopCount is how many accept loops the encoding writes in all.
func (e *Encoding) LoopCount() int {
	n := 0
	for _, k := range e.Signals {
		n += len(k.Loops)
	}
	return n
}

// Items writes the ordered buffer item of each kept signal.
func (e *Encoding) Items(w Writer) {
	for _, k := range e.Signals {
		kind := k.Kind
		if kind == "" {
			kind = "item"
		}
		w.Line(kind + " " + k.Buffer + " : " + k.Ref + "[*] ordered;")
		w.MadeUp(k.Buffer)
	}
}

// Do writes the do action that keeps each deferred signal while the state is
// active: one endless accept loop per route, run beside the state's own do
// behavior when own renders one. own is called inside the action's body, so
// its text is rendered at that indentation.
func (e *Encoding) Do(w Writer, own func() Own) {
	w.Block("do action "+e.Buffer, func() {
		var run Own
		if own != nil {
			run = own()
		}
		if run.Written || e.LoopCount() > 1 {
			w.Line("first start then " + e.Split + ";")
			w.Line("fork " + e.Split + ";")
			w.MadeUp(e.Split)
			if run.Written {
				w.Line("then " + run.Run + ";")
			}
			for _, k := range e.Signals {
				for _, l := range k.Loops {
					w.Line("then " + l.Receive + ";")
				}
			}
		} else {
			w.Line("first start then " + e.Signals[0].Loops[0].Receive + ";")
		}
		w.Raw(run.Text)
		if run.MadeUp != "" {
			w.MadeUp(run.MadeUp)
		}
		for _, k := range e.Signals {
			for _, l := range k.Loops {
				via := ""
				if l.Via != "" {
					via = " via " + l.Via
				}
				w.Line("action " + l.Receive + " accept " + l.Payload + " : " + l.Accept + via + ";")
				w.Line("then action " + l.Keep + " { assign " + k.Buffer + " := " + e.Including + "(" + k.Buffer + ", " + l.Receive + "." + l.Payload + "); }")
				w.Line("then " + l.Receive + ";")
				w.MadeUp(l.Receive)
				w.MadeUp(l.Keep)
			}
		}
	})
	w.MadeUp(e.Buffer)
}

// Exit writes the exit action that sends each kept occurrence to the state's
// own object once the state exits, after the state's own exit behavior when
// own renders one, and empties each buffer so a later visit replays only what
// it kept. own is called inside the action's body.
func (e *Encoding) Exit(w Writer, own func() Own) {
	w.Block("exit action "+e.Flush, func() {
		prefix := ""
		if own != nil {
			run := own()
			w.Raw(run.Text)
			if run.MadeUp != "" {
				w.MadeUp(run.MadeUp)
			}
			if run.Written {
				prefix = "then "
			}
		}
		for _, k := range e.Signals {
			w.Line(prefix + "for " + k.Item + " in " + k.Buffer + " { send " + k.Item + " to self; }")
			w.Line("then action " + k.Clear + " { assign " + k.Buffer + " := (); }")
			w.MadeUp(k.Clear)
			prefix = "then "
		}
	})
	w.MadeUp(e.Flush)
}
