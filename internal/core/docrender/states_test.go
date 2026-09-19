package docrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// stateFixtureDocument evaluates the lamp report over a session that drove lamp1 on
// at 0 s, dimmed at 1 s, boosted at 2 s and lamp2 on at 2 s, off at 2.5 s; clock at 3 s.
func stateFixtureDocument(t *testing.T) *docir.Document {
	t.Helper()
	fixture := loadRenderFixture(t, filepath.Join("testdata", "state_report.sysml"))
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	ctx.SetTrace(runtime.NewTraceRecorder())
	var roots []queryexec.Root
	instantiate := func(name string) *runtime.Instance {
		inst, err := ctx.Instantiate(fixture.symbol(t, "Lamps::"+name))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", name, err)
		}
		roots = append(roots, queryexec.Root{Label: name, Object: inst})
		return inst
	}
	lamp1, lamp2 := instantiate("lamp1"), instantiate("lamp2")
	instantiate("panel")
	send := func(inst *runtime.Instance, signal string, args map[string]runtime.Value) {
		msg, err := ctx.SignalMessage(fixture.symbol(t, "Lamps::"+signal), args, inst)
		if err != nil {
			t.Fatalf("send %s: %v", signal, err)
		}
		ctx.PostMessage(msg)
	}
	advance := func(seconds float64) {
		if _, err := ctx.Advance(seconds); err != nil {
			t.Fatalf("advance %v: %v", seconds, err)
		}
	}
	level := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	send(lamp1, "Toggle", nil)
	advance(1)
	send(lamp1, "Dim", map[string]runtime.Value{"level": level})
	advance(1)
	send(lamp1, "Boost", nil)
	send(lamp2, "Toggle", nil)
	advance(0.5)
	send(lamp2, "Toggle", nil)
	advance(0.5)
	report := symbols.PreferDeclared(fixture.index.LookupQualified("Lamps::LampReport"))
	if len(report) != 1 {
		t.Fatalf("lookup Lamps::LampReport: got %d symbols", len(report))
	}
	plan, err := docplan.Compile(fixture.index, fixture.model, fixture.resolver, report[0])
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}
	document, err := docir.Evaluate(plan,
		queryexec.Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx, Roots: roots},
		queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document: %v", err)
	}
	return document
}

// TestMarkdownStateReportGolden locks the Markdown of a document over a session's
// states and trace: active leaves, the objects in `on`, one lamp's events, bare rows.
func TestMarkdownStateReportGolden(t *testing.T) {
	got, err := Markdown(stateFixtureDocument(t), MarkdownOptions{})
	if err != nil {
		t.Fatalf("render document: %v", err)
	}
	golden := filepath.Join("testdata", "state_report.golden.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("rendered Markdown differs from %s (run with -update after intentional changes)\ngot:\n%s", golden, got)
	}
}

// TestHTMLStateReport checks the HTML over states and events: rows carry the object,
// machine and state path or the event's kind and instant; bare items read as summaries.
func TestHTMLStateReport(t *testing.T) {
	got, err := HTML(stateFixtureDocument(t), HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatalf("render document as HTML: %v", err)
	}
	for _, want := range []string{
		`<tr class="sysml-row" data-object="#1" data-machine="lp" data-state="on.dim" data-region="light" data-element="Lamps::LampMachine::on::light::dim" data-element-kind="stateUsage">`,
		`<tr class="sysml-row" data-object="#3" data-machine="lp" data-state="off" data-element="Lamps::LampMachine::off" data-element-kind="stateUsage">`,
		`<td class="sysml-cell" data-column="enclosing" data-value-kind="string"><span class="sysml-value" data-value-kind="string">on</span></td>`,
		`<tr class="sysml-row" data-object="#1" data-element="Lamps::lamp1" data-element-kind="partUsage">`,
		`<tr class="sysml-row" data-object="#1" data-event-kind="accept" data-time="1" data-element="Lamps::LampMachine" data-element-kind="stateDef">`,
		`<tr class="sysml-row" data-object="#1" data-event-kind="send" data-time="2" data-element="Lamps::LampMachine" data-element-kind="stateDef">`,
		`<td class="sysml-cell" data-column="time" data-value-kind="quantity"><span class="sysml-value" data-value-kind="quantity" data-magnitude="1" data-unit="s">1 [s]</span></td>`,
		`<td class="sysml-cell" data-column="payload" data-value-kind="string"><span class="sysml-value" data-value-kind="string">level = 3</span></td>`,
		`<td class="sysml-cell" data-column="target" data-value-kind="object"><span class="sysml-value sysml-object" data-value-kind="object" data-object="#5" data-element="Lamps::panel" data-element-kind="partUsage">panel</span></td>`,
		`<li class="sysml-item" data-object="#1" data-event-kind="entry" data-time="0" data-element="Lamps::LampMachine" data-element-kind="stateDef">t=0 lamp1.lp: enter: on</li>`,
		`<li class="sysml-item" data-object="#1" data-machine="lp" data-state="on.fast" data-region="fan" data-element="Lamps::LampMachine::on::fan::fast" data-element-kind="stateUsage">lamp1.lp in on.fast</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
}
