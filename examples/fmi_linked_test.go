//go:build !sysml_re && !sysml_nofmi

package examples

import (
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	engineset "github.com/Open-MBEE/OpenSysML/internal/exec/engines"
)

// TestSelfModelAnalysisFrameworkMatchesImplementation instantiates the modelled
// analysis framework and compares it with internal/exec/analysis: the engines the
// build's registry (engines.Default) holds and what each declares, the question kinds, the
// evidence scale, the selections, the budget and the jobs setting.
func TestSelfModelAnalysisFrameworkMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	framework := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "AnalysisFramework")
	parts := framework.parts()

	engines := engineset.Default().Engines()
	var names []string
	for _, e := range engines {
		names = append(names, e.Name())
	}
	registry := parts["registry"]
	if declared := registry.integer("engineCount"); declared != len(engines) {
		t.Errorf("pipeline.sysml says engineCount = %d, the default registry holds %d", declared, len(engines))
	}
	if declared, actual := registry.str("engineNames"), strings.Join(names, ", "); declared != actual {
		t.Errorf("pipeline.sysml says engineNames = %q, the default registry lists %q", declared, actual)
	}
	duplicate := analysis.NewRegistry()
	if err := duplicate.Register(engines[0]); err != nil {
		t.Fatalf("register %s once: %v", engines[0].Name(), err)
	}
	if err := duplicate.Register(engines[0]); (err != nil) != registry.boolean("refusesDuplicateNames") {
		t.Errorf("pipeline.sysml says refusesDuplicateNames = %v, registering twice gave %v", registry.boolean("refusesDuplicateNames"), err)
	}

	// Each modelled engine against what the registered one describes.
	declared := map[string]*modelInstance{}
	for _, part := range parts {
		if part.has("engineName") {
			declared[part.str("engineName")] = part
		}
	}
	for _, e := range engines {
		part, ok := declared[e.Name()]
		if !ok {
			t.Errorf("the default registry holds %s, which AnalysisFramework does not model", e.Name())
			continue
		}
		delete(declared, e.Name())
		desc := e.Describe()
		var kinds []string
		for _, k := range desc.Questions {
			kinds = append(kinds, k.String())
		}
		facts := []struct {
			attribute string
			got, want any
		}{
			{"answers", part.str("answers"), strings.Join(kinds, ", ")},
			{"bounds", part.str("bounds"), strings.Join(desc.Bounds, ", ")},
			{"authority", part.str("authority"), desc.Authority.String()},
			{"replays", part.boolean("replays"), desc.Replays},
			{"needsProcess", part.boolean("needsProcess"), desc.Process != ""},
		}
		for _, fact := range facts {
			if fact.got != fact.want {
				t.Errorf("pipeline.sysml models %s.%s = %v, the engine declares %v", e.Name(), fact.attribute, fact.got, fact.want)
			}
		}
	}
	for name := range declared {
		t.Errorf("AnalysisFramework models engine %s, which the default registry does not hold", name)
	}

	// The question kinds and freedoms, in declaration order.
	questions := parts["questions"]
	var kinds []string
	for k := analysis.Evaluate; k.String() != "unknown"; k++ {
		kinds = append(kinds, k.String())
	}
	if declared, actual := questions.str("kinds"), strings.Join(kinds, ", "); declared != actual {
		t.Errorf("pipeline.sysml says kinds = %q, analysis.Kind spells %q", declared, actual)
	}
	if declared := questions.integer("kindCount"); declared != len(kinds) {
		t.Errorf("pipeline.sysml says kindCount = %d, analysis.Kind has %d", declared, len(kinds))
	}
	freedoms := []string{analysis.FreeNothing.String(), analysis.FreeSchedule.String(), analysis.FreeInputs.String()}
	if declared, actual := questions.str("freedoms"), strings.Join(freedoms, ", "); declared != actual {
		t.Errorf("pipeline.sysml says freedoms = %q, analysis.Freedom spells %q", declared, actual)
	}

	// The evidence scale: every strength and claim, weakest first.
	evidence := parts["evidence"]
	var strengths []string
	for s := analysis.NotCovered; s.String() != "unknown"; s++ {
		strengths = append(strengths, s.String())
	}
	if declared, actual := evidence.str("strengths"), strings.Join(strengths, ", "); declared != actual {
		t.Errorf("pipeline.sysml says strengths = %q, analysis.Strength spells %q", declared, actual)
	}
	if declared := evidence.integer("strengthCount"); declared != len(strengths) {
		t.Errorf("pipeline.sysml says strengthCount = %d, analysis.Strength has %d", declared, len(strengths))
	}
	var claims []string
	for c := analysis.ClaimNone; c.String() != "unknown"; c++ {
		claims = append(claims, c.String())
	}
	if declared, actual := evidence.str("claims"), strings.Join(claims, ", "); declared != actual {
		t.Errorf("pipeline.sysml says claims = %q, analysis.Claim spells %q", declared, actual)
	}
	if declared := evidence.integer("claimCount"); declared != len(claims) {
		t.Errorf("pipeline.sysml says claimCount = %d, analysis.Claim has %d", declared, len(claims))
	}

	// The selections a question can be put with, and the default.
	dispatch := parts["dispatch"]
	if declared, actual := dispatch.str("defaultSelection"), analysis.ParseSelection("").String(); declared != actual {
		t.Errorf("pipeline.sysml says defaultSelection = %q, an unset selection is %q", declared, actual)
	}
	selections := []string{analysis.Auto().String(), analysis.All().String(), "<engine>"}
	if declared, actual := dispatch.str("selections"), strings.Join(selections, ", "); declared != actual {
		t.Errorf("pipeline.sysml says selections = %q, the implementation spells %q", declared, actual)
	}
	if named := analysis.ParseSelection(names[0]); named.Mode != analysis.SelectNamed || named.String() != names[0] {
		t.Errorf("a selection naming %s parses as %+v", names[0], named)
	}
	// The candidates each kind of question has in the default registry, in name order.
	declaring := map[analysis.Kind]int{}
	for _, e := range engines {
		for _, kind := range e.Describe().Questions {
			declaring[kind]++
		}
	}
	var perKind []string
	for k := analysis.Evaluate; k.String() != "unknown"; k++ {
		if declaring[k] > 0 {
			perKind = append(perKind, k.String()+": "+strconv.Itoa(declaring[k]))
		}
	}
	sort.Strings(perKind)
	if declared, actual := dispatch.str("candidatesPerKind"), strings.Join(perKind, ", "); declared != actual {
		t.Errorf("pipeline.sysml says candidatesPerKind = %q, the default registry has %q", declared, actual)
	}

	// The budget's fields and the jobs setting behind one of them.
	budget := parts["budget"]
	budgetType := reflect.TypeOf(analysis.Budget{})
	var fields []string
	for i := 0; i < budgetType.NumField(); i++ {
		fields = append(fields, budgetType.Field(i).Name)
	}
	if declared, actual := budget.str("fields"), strings.Join(fields, ", "); declared != actual {
		t.Errorf("pipeline.sysml says fields = %q, analysis.Budget has %q", declared, actual)
	}
	if declared := budget.integer("fieldCount"); declared != len(fields) {
		t.Errorf("pipeline.sysml says fieldCount = %d, analysis.Budget has %d", declared, len(fields))
	}
	if declared := budget.str("jobsEnvVar"); declared != analysis.JobsEnvVar {
		t.Errorf("pipeline.sysml says jobsEnvVar = %q, analysis reads %q", declared, analysis.JobsEnvVar)
	}
	jobs := analysis.DefaultJobs()
	if declared, actual := budget.boolean("defaultJobsAtMostCpuCount"), jobs >= 1 && jobs <= goruntime.NumCPU(); declared != actual {
		t.Errorf("pipeline.sysml says defaultJobsAtMostCpuCount = %v, DefaultJobs is %d of %d CPUs: %v", declared, jobs, goruntime.NumCPU(), actual)
	}
	rejects := true
	for _, text := range []string{"0", "-1", "two", ""} {
		if _, err := analysis.ParseJobs(analysis.JobsEnvVar, text); err == nil {
			rejects = false
		}
	}
	if _, err := analysis.ParseJobs(analysis.JobsEnvVar, "2"); err != nil {
		t.Errorf("ParseJobs rejects 2: %v", err)
	}
	if declared := budget.boolean("rejectsNonPositiveJobs"); declared != rejects {
		t.Errorf("pipeline.sysml says rejectsNonPositiveJobs = %v, ParseJobs rejects them: %v", declared, rejects)
	}

	// The jobs flag and command the budget names, against the surfaces that define them.
	if !strings.Contains(readGoPackage(t, filepath.Join("..", "cmd", "sysml")), `"`+strings.TrimPrefix(budget.str("jobsFlag"), "-")+`"`) {
		t.Errorf("pipeline.sysml says jobsFlag = %q, cmd/sysml defines no such flag", budget.str("jobsFlag"))
	}
	if !strings.Contains(readGoPackage(t, filepath.Join("..", "internal", "frontend", "repl")), `"`+budget.str("jobsCommand")+`"`) {
		t.Errorf("pipeline.sysml says jobsCommand = %q, internal/frontend/repl defines no such command", budget.str("jobsCommand"))
	}
}
