package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// testdata/xmi/heater_receptions.xmi: a reception with a method accepts its signal and performs the
// method with the payload bound, its optional and defaulted parameters left unbound; one without a
// method, or whose method requires a value the signal lacks, only accepts; unmigratable methods
// and signals are refused. An object of the block performs every reception from creation and
// accepts again after each signal, so nothing starts one and a repeated signal runs the method again.
// A signal arriving at a port of the block is accepted via that port as well as from the object.
func TestReceptionsAcceptAndPerformTheirMethod(t *testing.T) {
	r := migrateFixtureFile(t, "heater_receptions")
	for _, line := range []string{
		"action def SetLevel {",
		"first start then spread;",
		"fork spread;",
		"first spread then receive;",
		"action receive accept setLevel : Signals::SetLevel;",
		"first receive then run;",
		"action run : 'Apply Level' { in value = setLevel.value; }",
		"first run then receive;",
		"first spread then 'receive via rx';",
		"action 'receive via rx' accept 'setLevel via rx' : Signals::SetLevel via rx;",
		"first 'receive via rx' then 'run via rx';",
		"action 'run via rx' : 'Apply Level' { in value = 'setLevel via rx'.value; }",
		"first 'run via rx' then 'receive via rx';",
		"perform action setLevel : SetLevel;",
		"action def Stop {",
		"action receive accept stop : Signals::Stop;",
		"first receive then receive;",
		"action 'receive via rx' accept 'stop via rx' : Signals::Stop via rx;",
		"first 'receive via rx' then 'receive via rx';",
		"perform action stop : Stop;",
		"action def Reset {",
		"first start then receive;",
		"action receive accept reset : Signals::Reset;",
		"perform action reset : Reset;",
		"action def Boost {",
		"action receive accept boost : Signals::Boost;",
		"perform action boost : Boost;",
		"in slack : ScalarValues::Real[0..1];",
		"comment /* reception 'Away' */",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, bound := range []string{"in gain =", "in slack =", ": Boosting", "then done;", "via aux"} {
		if strings.Contains(string(r.Notation), bound) {
			t.Errorf("%q was written, though nothing in the fixture calls for it:\n%s", bound, r.Notation)
		}
	}
	wantNote(t, r, "_rcvSet", migrate.Mapped, "written as an action def accepting SetLevel and performing its method Heater::Apply Level, which its owner performs as setLevel from creation, accepting the signal again after each; the signal arrives at the port rx over the document's connectors or declarations, so the reception is also written accepting via each; nothing in the document declares or sends a signal to the port aux, so one arriving there is not accepted")
	wantNote(t, r, "_rpValue", migrate.Mapped, "stands for the signal's attribute value, which the accepted payload carries")
	wantNote(t, r, "_rpExtra", migrate.Unmapped, "the parameter extra matches no attribute of the signal")
	wantNote(t, r, "_rcvStop", migrate.Approximated, "the reception has no method, so it only accepts the signal")
	wantNote(t, r, "_rcvReset", migrate.Approximated, "the method Heater::Resetting has no action def to perform; the reception only accepts the signal; nothing in the document declares or sends a signal to the ports rx, aux, so one arriving there is not accepted")
	wantNote(t, r, "_rcvBoost", migrate.Approximated, "the method Heater::Boosting's parameter amount must hold a value that no attribute of the signal supplies; the reception only accepts the signal")
	wantNote(t, r, "_rcvAway", migrate.Unmapped, "signal")
	wantNote(t, r, "_apply", migrate.Mapped, "")

	h := newHeaterRun(t, r)
	if got := len(h.heater.PerformedActionsOf(h.sym("Heater::SetLevel"))); got != 1 {
		t.Fatalf("the object performs SetLevel %d time(s) from creation, want 1", got)
	}
	for _, level := range []float64{3.5, 7.25} {
		h.send(t, "Signals::SetLevel", map[string]runtime.Value{"value": realValue(level)})
		if got := h.level(t); got != level {
			t.Errorf("after SetLevel(value=%v) the method left level = %v", level, got)
		}
	}
	h.send(t, "Signals::Boost", nil)
	if got := h.level(t); got != 7.25 {
		t.Errorf("the refused method ran: level = %v", got)
	}
	h.send(t, "Signals::Stop", nil)
	if got := len(h.heater.PerformedActionsOf(h.sym("Heater::SetLevel"))); got != 1 {
		t.Errorf("after three signals the object performs SetLevel %d time(s), want the one it was created with", got)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Room")
	meta(t, s, "%action Thermostat::'Turn Up' #1.t")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : h.level"); !strings.Contains(out, "= 7.25") {
		t.Errorf("the heater's reception did not run its method on the signal sent through the room's connector:\n%s", out)
	}
}

// heaterRun is a runtime over the migrated heater holding one object of it,
// its receptions running as the object's own behaviors.
type heaterRun struct {
	t      *testing.T
	idx    *symbols.Index
	ctx    *runtime.Context
	heater *runtime.Instance
}

func newHeaterRun(t *testing.T, r *migrate.Result) *heaterRun {
	t.Helper()
	p := parser.New(source.New("heater.sysml", r.Notation))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse the migrated notation: %v", p.Diagnostics[0])
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("heater.sysml", root)
	idx.ExpandWildcardImports()
	res := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(passes.NewTypedModel(res), res), 10_000_000)
	h := &heaterRun{t: t, idx: idx, ctx: ctx}
	heater, err := ctx.Instantiate(h.sym("Heater"))
	if err != nil {
		t.Fatalf("instantiate Heater: %v", err)
	}
	h.heater = heater
	return h
}

func (h *heaterRun) sym(fqn string) *symbols.Symbol {
	h.t.Helper()
	syms := h.idx.LookupQualified(fqn)
	if len(syms) != 1 {
		h.t.Fatalf("%s names %d symbols, want one", fqn, len(syms))
	}
	return syms[0]
}

// send posts the signal to the heater and lets the clock dispatch it.
func (h *heaterRun) send(t *testing.T, signal string, args map[string]runtime.Value) {
	t.Helper()
	msg, err := h.ctx.SignalMessage(h.sym(signal), args, h.heater)
	if err != nil {
		t.Fatalf("send %s: %v", signal, err)
	}
	h.ctx.PostMessage(msg)
	if _, err := h.ctx.Advance(1); err != nil {
		t.Fatalf("dispatch %s: %v", signal, err)
	}
}

// level reads the heater's level attribute.
func (h *heaterRun) level(t *testing.T) float64 {
	t.Helper()
	fv, err := h.heater.GetFeatureValue(h.ctx, "level")
	if err != nil {
		t.Fatalf("read level: %v", err)
	}
	if fv.Value.Kind != runtime.ValConst || fv.Value.Const.Kind != semantics.ValReal {
		t.Fatalf("level holds %v, not a Real", fv.Value)
	}
	return fv.Value.Const.Real
}

func realValue(v float64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: v}}
}
