package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// engineStandinWire names the directory the stand-in writes every line it exchanges to.
const engineStandinWire = "ENGINE_STANDIN_WIRE"

// engineSchemaPath is the JSON Schema the reference publishes for the message set.
var engineSchemaPath = filepath.Join("..", "..", "..", "docs", "reference", "engine-protocol.schema.json")

// engineSchema is the published schema compiled at one of its definitions.
type engineSchema struct {
	host, engine *jsonschema.Schema
}

func loadEngineSchema(t *testing.T) engineSchema {
	t.Helper()
	f, err := os.Open(engineSchemaPath)
	if err != nil {
		t.Fatalf("the reference publishes no schema: %v", err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	const url = "engine-protocol.schema.json"
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	compile := func(def string) *jsonschema.Schema {
		s, err := c.Compile(url + "#/$defs/" + def)
		if err != nil {
			t.Fatalf("compile %s: %v", def, err)
		}
		return s
	}
	return engineSchema{host: compile("hostMessage"), engine: compile("engineMessage")}
}

// wireLine is one line of a capture: who wrote it and what.
type wireLine struct {
	host bool
	text string
}

// captured reads every line the stand-ins under dir exchanged.
func captured(t *testing.T, dir string) []wireLine {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.wire"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var lines []wireLine
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			switch {
			case strings.HasPrefix(line, "< "):
				lines = append(lines, wireLine{host: true, text: line[2:]})
			case strings.HasPrefix(line, "> "):
				lines = append(lines, wireLine{text: line[2:]})
			default:
				t.Fatalf("%s: line %q is neither read nor written", file, line)
			}
		}
	}
	if len(lines) == 0 {
		t.Fatalf("no stand-in wrote to %s", dir)
	}
	return lines
}

// validate checks one line against the side's schema; a line that is not JSON fails too.
func (s engineSchema) validate(line wireLine) error {
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(line.text))
	if err != nil {
		return err
	}
	if line.host {
		return s.host.Validate(v)
	}
	return s.engine.Validate(v)
}

// methods counts the methods and the members of the results the lines carry, so a test can
// assert which paths a capture walked.
func methods(t *testing.T, lines []wireLine) map[string]int {
	t.Helper()
	seen := map[string]int{}
	for _, line := range lines {
		var m struct {
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *enginewire.Error
		}
		if err := json.Unmarshal([]byte(line.text), &m); err != nil {
			continue
		}
		if m.Method != "" {
			seen[m.Method]++
		}
		if m.Error != nil {
			seen["error:"+m.Error.Code]++
		}
		if len(m.Result) > 0 {
			var members map[string]json.RawMessage
			if err := json.Unmarshal(m.Result, &members); err != nil {
				t.Fatalf("result %s is no object", m.Result)
			}
			for k := range members {
				seen["result."+k]++
			}
		}
	}
	return seen
}

// Every line the host and the stand-in exchange, walking every protocol path an engine
// answers on, validates against the schema the reference publishes: the handshake, covers
// both ways, runs with every witness shape, progress, cancel, and each error code.
func TestSchemaValidatesEveryStandinMessage(t *testing.T) {
	schema := loadEngineSchema(t)
	dir := t.TempDir()
	t.Setenv(engineStandinWire, dir)
	f := parseFixture(t)
	race := f.checked(t, "race")
	holds := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}})
	notOne := runtime.CheckProperty{Name: "x", Holds: func(_ *runtime.Context, inv *runtime.Invocation) (bool, error) {
		return inv.Actions[0].Results()["x"].Const.Int != 1, nil
	}}
	violable := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{notOne}})
	sensitive := checkQuestion(t, f, Sensitive, &CheckAsk{Start: race.start})
	sat := Question{Kind: Satisfiable, Subject: "test::Tank", Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{intQuery("C", 2, 5)}, Ask: (*solve.Solver).Solve}}

	answers := func(t *testing.T, result string, witness WitnessKind, q Question, budget Budget) Result {
		t.Helper()
		r := standinRegistry(t, result, witness)
		return standinAnswers(t, r, f.building(), q, budget)
	}
	t.Run("violated", func(t *testing.T) {
		t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds","sensitive","outcomes","satisfiable"],"bounds":["depth","steps"],"model":["sources","graphs:1"]}`)
		r := standinRegistryEntry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`,"at":1},"bounds":[{"name":"depth","limit":10,"reached":true}],"elapsed":3}`, WitnessSchedule, func(e *EngineEntry) {
			e.Model = []ModelForm{FormSources, GraphsForm(1)}
			e.Bounds = []string{"depth", "steps"}
		})
		if result := standinAnswers(t, r, f.building(), violable, Budget{Depth: 10}); result.Claim != ClaimViolated {
			t.Fatalf("result %+v", result)
		}
	})
	t.Run("sensitive", func(t *testing.T) {
		if result := answers(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne, raceEndsThree)+`,"feature":"x"}}`, WitnessSchedule, sensitive, Budget{}); result.Claim != ClaimSensitive {
			t.Fatalf("result %+v", result)
		}
	})
	t.Run("executions", func(t *testing.T) {
		if result := answers(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceEndsOne)+`},{"schedules":`+schedules(raceEndsTwo)+`}],"reason":"two runs"}`, WitnessSchedule, holds, Budget{Jobs: 8}); result.Strength != Observed {
			t.Fatalf("result %+v", result)
		}
	})
	t.Run("assignment", func(t *testing.T) {
		if result := answers(t, `{"claim":"satisfiable","strength":"witnessed","witness":{"inputs":[{"name":"test::C::i","value":3}]},"values":[{"name":"test::C::i","value":3,"unit":"m"}]}`, WitnessAssignment, sat, Budget{}); result.Claim != ClaimSatisfiable {
			t.Fatalf("result %+v", result)
		}
	})
	t.Run("refused", func(t *testing.T) {
		t.Setenv(engineStandinCovers, `{"covers":false,"reason":"no loops"}`)
		plan, err := standinRegistry(t, `{"claim":"none","strength":"not covered"}`, WitnessSchedule).AnswerWith(context.Background(), f.building(), holds, Budget{}, Only("standin"))
		if err != nil || plan.Steps[0].Refusal == nil {
			t.Fatalf("plan %+v (%v), want the stand-in's refusal", plan, err)
		}
	})
	for _, code := range []string{enginewire.CodeUnsupported, enginewire.CodeBudget, enginewire.CodeInternal} {
		t.Run(code, func(t *testing.T) {
			t.Setenv(engineStandinMode, "error-"+code)
			notCovered(t, answers(t, "", WitnessSchedule, holds, Budget{}), code)
		})
	}
	t.Run("cancel", func(t *testing.T) {
		t.Setenv(engineStandinMode, "cancel-answers")
		t.Setenv(engineStandinProgress, "5")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		plan, err := standinRegistry(t, "", WitnessSchedule).AnswerWith(ctx, f.building(), holds, Budget{}, Only("standin"))
		if err == nil && !strings.Contains(plan.Result.Reason, "cancelled") {
			t.Fatalf("plan %+v, want the cancelled run's answer or the deadline", plan)
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want the plan's deadline", err)
		}
	})

	lines := captured(t, dir)
	for _, line := range lines {
		if err := schema.validate(line); err != nil {
			side := "engine"
			if line.host {
				side = "host"
			}
			t.Errorf("%s line %s\n%v", side, line.text, err)
		}
	}
	seen := methods(t, lines)
	for _, want := range []string{
		"describe", "covers", "run", "cancel", "progress",
		"result.name", "result.covers", "result.reason", "result.claim", "result.witness", "result.executions",
		"result.bounds", "result.values", "result.elapsed",
		"error:unsupported", "error:budget", "error:internal",
	} {
		if seen[want] == 0 {
			t.Errorf("no line carries %s; the paths walked were %v", want, seen)
		}
	}
	hosts := 0
	for _, line := range lines {
		if line.host && strings.Contains(line.text, `"graphs":{"version":1`) {
			hosts++
		}
	}
	if hosts == 0 {
		t.Error("no request carried the graphs:1 form")
	}
}

// A line the session refuses as a protocol break the schema refuses too, so the schema and
// the host agree on what an engine may write; the host's own lines stay valid throughout.
func TestSchemaRefusesWhatTheSessionRefuses(t *testing.T) {
	schema := loadEngineSchema(t)
	for _, mode := range []string{
		"not-json", "wrong-jsonrpc", "no-id", "null-id", "string-id", "result-and-error",
		"neither", "error-no-code", "engine-request", "unknown-notification",
	} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(engineStandinWire, dir)
			t.Setenv(engineStandinMode, mode)
			s := standinSession(t, standinEntry(t), 5*time.Second)
			if _, err := runOnce(context.Background(), s, nil); !errors.Is(err, ErrProtocol) {
				t.Fatalf("got %v", err)
			}
			s.close()
			refused := 0
			for _, line := range captured(t, dir) {
				err := schema.validate(line)
				switch {
				case line.host && err != nil:
					t.Errorf("host line %s\n%v", line.text, err)
				case !line.host && err != nil:
					refused++
				}
			}
			if refused == 0 {
				t.Error("the schema accepts every line of a session the host broke off")
			}
		})
	}
}

// The schema names every member of every wire type and no other, and requires exactly the
// members the types always write.
func TestSchemaMatchesTheWireTypes(t *testing.T) {
	data, err := os.ReadFile(engineSchemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	types := map[string]any{
		"describeParams": enginewire.DescribeParams{},
		"description":    enginewire.Description{},
		"coversParams":   enginewire.CoversParams{},
		"coversResult":   enginewire.CoversResult{},
		"runParams":      enginewire.RunParams{},
		"cancelParams":   enginewire.CancelParams{},
		"progressParams": enginewire.ProgressParams{},
		"question":       enginewire.Question{},
		"condition":      enginewire.Condition{},
		"conditionSet":   enginewire.ConditionSet{},
		"pinned":         enginewire.Pinned{},
		"freeInput":      enginewire.FreeInput{},
		"sweep":          enginewire.Sweep{},
		"range":          enginewire.Range{},
		"value":          enginewire.Value{},
		"bound":          enginewire.Bound{},
		"budget":         enginewire.Budget{},
		"model":          enginewire.Model{},
		"result":         enginewire.Result{},
		"input":          enginewire.Input{},
		"witness":        enginewire.Witness{},
		"error":          enginewire.Error{},
		"sources":        export.Sources{},
		"graphs":         export.Graphs{},
	}
	for def, v := range types {
		d, ok := schema.Defs[def]
		if !ok {
			t.Errorf("the schema has no definition %s", def)
			continue
		}
		var members, required []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			tag := rt.Field(i).Tag.Get("json")
			name, opts, _ := strings.Cut(tag, ",")
			members = append(members, name)
			if !strings.Contains(opts, "omitempty") {
				required = append(required, name)
			}
		}
		var properties []string
		for k := range d.Properties {
			properties = append(properties, k)
		}
		sort.Strings(members)
		sort.Strings(required)
		sort.Strings(properties)
		sort.Strings(d.Required)
		if !reflect.DeepEqual(members, properties) {
			t.Errorf("%s: the type writes %v, the schema names %v", def, members, properties)
		}
		if !reflect.DeepEqual(required, d.Required) {
			t.Errorf("%s: the type always writes %v, the schema requires %v", def, required, d.Required)
		}
	}
}
