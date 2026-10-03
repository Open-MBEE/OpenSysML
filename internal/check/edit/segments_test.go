package edit

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var observerMu sync.Mutex
var observerMismatches []string

func TestMain(m *testing.M) {
	applyObserver = func(model Model, ops []Operation, got *Result, gotErr error) {
		want, _, wantErr := apply(model, ops, true)
		if field := firstDifference(got, gotErr, want, wantErr); field != "" {
			observerMu.Lock()
			message := fmt.Sprintf("operations: %#v\nfirst difference: %s", ops, field)
			if field == "Result.Applied" {
				message += fmt.Sprintf("\ngot applied: %#v\nwant applied: %#v", got.Applied, want.Applied)
			}
			observerMismatches = append(observerMismatches, message)
			observerMu.Unlock()
		}
	}
	code := m.Run()
	observerMu.Lock()
	defer observerMu.Unlock()
	for _, mismatch := range observerMismatches {
		fmt.Fprintln(os.Stderr, mismatch)
	}
	if len(observerMismatches) > 0 {
		code = 1
	}
	os.Exit(code)
}

func firstDifference(got *Result, gotErr error, want *Result, wantErr error) string {
	if field := errorDifference(gotErr, wantErr); field != "" {
		return field
	}
	if (got == nil) != (want == nil) {
		return "Result nilness"
	}
	if got == nil {
		return ""
	}
	if !bytes.Equal(got.Content, want.Content) {
		return "Result.Content"
	}
	if !reflect.DeepEqual(got.Applied, want.Applied) {
		return "Result.Applied"
	}
	if !reflect.DeepEqual(got.Others, want.Others) {
		return "Result.Others"
	}
	return ""
}

func errorDifference(got, want error) string {
	if (got == nil) != (want == nil) {
		return "error nilness"
	}
	if got == nil {
		return ""
	}
	var gotEdit, wantEdit *Error
	if errors.As(got, &gotEdit) && errors.As(want, &wantEdit) {
		switch {
		case gotEdit.Failure != wantEdit.Failure:
			return "Error.Failure"
		case gotEdit.OperationIndex != wantEdit.OperationIndex:
			return "Error.OperationIndex"
		case gotEdit.Message != wantEdit.Message:
			return "Error.Message"
		case !reflect.DeepEqual(gotEdit.Diagnostics, wantEdit.Diagnostics):
			return "Error.Diagnostics"
		case !sameDiagnosed(gotEdit.Diagnosed, wantEdit.Diagnosed):
			return "Error.Diagnosed"
		default:
			return ""
		}
	}
	if got.Error() != want.Error() {
		return "error message"
	}
	return ""
}

func diagnosedText(err error) string {
	var editErr *Error
	if errors.As(err, &editErr) && editErr.Diagnosed != nil {
		return string(editErr.Diagnosed.Bytes())
	}
	return ""
}

func sameDiagnosed(got, want *source.SourceFile) bool {
	if (got == nil) != (want == nil) {
		return false
	}
	return got == nil || (got.Name() == want.Name() && bytes.Equal(got.Bytes(), want.Bytes()))
}

func quickModel(name, text string) Model {
	sf := source.New(name, []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument(name, root)
	return Model{Source: sf, Root: root, Index: idx, ParseDiags: p.Diagnostics}
}

type segmentModel struct {
	name              string
	text              string
	batchOwner        string
	partOwner         string
	partType          string
	importOwners      []string
	inheritedFeatures []string
	localTargets      []string
	valueTargets      []string
	stateOwner        string
	substateOwner     string
	calculationOwner  string
}

var segmentModels = []segmentModel{
	{
		name: "flat-import", batchOwner: "P::v0", partOwner: "P::v0", partType: "P::Vehicle",
		importOwners:      []string{"P::v0", "P::v1", "P::v2", "P::v3"},
		inheritedFeatures: []string{"mass", "power", "cost"},
		valueTargets:      []string{"P::Vehicle::mass", "P::Vehicle::power"},
		text: `package P {
    private import ScalarValues::*;
    part def Vehicle {
        attribute mass : Real;
        attribute power : Real;
        attribute cost : Real;
    }
    part v0 : Vehicle;
    part v1 : Vehicle;
    part v2 : Vehicle;
    part v3 : Vehicle;
}
`,
	},
	{
		name: "nested-import", batchOwner: "P::v0::battery", partOwner: "P::v0", partType: "P::Vehicle",
		importOwners:      []string{"P::v0::battery", "P::v1::battery", "P::v2::battery", "P::v3::battery"},
		inheritedFeatures: []string{"capacity", "voltage", "charge", "temp", "cycles"},
		valueTargets:      []string{"P::Battery::capacity", "P::Vehicle::mass"},
		text: `package P {
    private import ScalarValues::*;
    part def Battery {
        attribute capacity : Real;
        attribute voltage : Real;
        attribute charge : Real;
        attribute temp : Real;
        attribute cycles : Real;
    }
    part def Vehicle {
        attribute mass : Real;
        part battery : Battery;
    }
    part v0 : Vehicle { part :>> battery; }
    part v1 : Vehicle { part :>> battery; }
    part v2 : Vehicle { part :>> battery; }
    part v3 : Vehicle { part :>> battery; }
}
`,
	},
	{
		name: "mixed-import", batchOwner: "P::v0", partOwner: "P::v0", partType: "P::Vehicle",
		importOwners:      []string{"P::v0", "P::v1", "P::v2", "P::v3"},
		inheritedFeatures: []string{"mass", "power", "cost"},
		localTargets:      []string{"P::v0::local", "P::v1::local", "P::v2::local", "P::v3::local"},
		valueTargets:      []string{"P::v0::local", "P::Vehicle::mass"},
		text: `package P {
    private import ScalarValues::*;
    part def Vehicle {
        attribute mass : Real;
        attribute power : Real;
        attribute cost : Real;
    }
    part v0 : Vehicle { attribute local : Real = 3.0; }
    part v1 : Vehicle { attribute local : Real = 4.0; }
    part v2 : Vehicle { attribute local : Real = 5.0; }
    part v3 : Vehicle { attribute local : Real = 6.0; }
}
`,
	},
	{
		name: "nested-owner", batchOwner: "P::Vehicle", partOwner: "P::v0", partType: "P::Vehicle",
		valueTargets: []string{"P::v0::mass", "P::Vehicle::mass"},
		text: `package P {
    part def Vehicle {
        attribute mass : Real = 1.0;
        part engine : Engine { attribute power : Real = 2.0; }
    }
    part def Engine;
    part v0 : Vehicle { attribute mass : Real = 3.0; attribute local : Real = 4.0; }
    part v1 : Vehicle { attribute mass : Real = 5.0; }
}
`,
	},
	{
		name: "tab-indented", batchOwner: "P::v0", partOwner: "P::v0", partType: "P::Vehicle",
		valueTargets: []string{"P::v0::mass", "P::Vehicle::mass"},
		text:         "package P {\n\tpart def Vehicle { attribute mass : Real = 1.0; }\n\tpart v0 : Vehicle { attribute mass : Real = 2.0; }\n\tpart v1 : Vehicle { attribute mass : Real = 3.0; }\n}\n",
	},
	{
		name: "one-line-owner", batchOwner: "P::v0", partOwner: "P::v0", partType: "P::Vehicle",
		valueTargets: []string{"P::v0::mass", "P::Vehicle::mass"},
		text:         `package P { part def Vehicle { attribute mass : Real = 1.0; } part v0 : Vehicle { attribute mass : Real = 2.0; } part v1 : Vehicle { attribute mass : Real = 3.0; } }`,
	},
	{
		name: "trailing-comments", batchOwner: "P::v0", partOwner: "P::v0", partType: "P::Vehicle",
		valueTargets: []string{"P::v0::mass", "P::Vehicle::mass"},
		text: `package P {
    part def Vehicle { attribute mass : Real = 1.0; } // vehicle
    part v0 : Vehicle { attribute mass : Real = 2.0; } // v0
    part v1 : Vehicle { attribute mass : Real = 3.0; } // v1
}
`,
	},
	{
		name: "calc-state-substate", batchOwner: "P::Vehicle", partOwner: "P::v0", partType: "P::Vehicle",
		valueTargets: []string{"P::v0::mass", "P::Vehicle::mass"},
		stateOwner:   "P::S", substateOwner: "P::S::sub", calculationOwner: "P::C",
		text: `package P {
    calc def C { return : Real = 1.0; }
    state def S { entry action enter; state sub; }
    part def Vehicle { attribute mass : Real = 1.0; }
    part v0 : Vehicle { attribute mass : Real = 2.0; }
    part v1 : Vehicle { attribute mass : Real = 3.0; }
}
`,
	},
	{
		name: "root-members", batchOwner: "Vehicle", partOwner: "v0", partType: "Vehicle",
		valueTargets: []string{"v0::mass", "Vehicle::mass"},
		text: `part def Vehicle { attribute mass : Real = 1.0; }
part v0 : Vehicle { attribute mass : Real = 2.0; }
part v1 : Vehicle { attribute mass : Real = 3.0; }
`,
	},
	{
		name: "bodyless-definition", batchOwner: "P::Empty", partOwner: "P::v0", partType: "P::Vehicle",
		valueTargets: []string{"P::v0::mass", "P::Vehicle::mass"},
		text: `package P {
    part def Empty;
    part def Vehicle { attribute mass : Real = 1.0; }
    part v0 : Vehicle { attribute mass : Real = 2.0; }
    part v1 : Vehicle { attribute mass : Real = 3.0; }
}
`,
	},
}

func TestApplySegmentsMatchSequential(t *testing.T) {
	seeds := []int64{82491827, 51739211, 10935197}
	models := make([]Model, len(segmentModels))
	for i, spec := range segmentModels {
		models[i] = quickModel(spec.name+".sysml", spec.text)
	}
	multiByModel := make([]int, len(segmentModels))
	var totalCases, totalSegments, multiCases int
	for seedIndex, seed := range seeds {
		seedRNG := rand.New(rand.NewSource(seed))
		var seedSegments, seedMulti int
		for caseIndex := 0; caseIndex < 1000; caseIndex++ {
			caseSeed := seedRNG.Int63()
			caseRNG := rand.New(rand.NewSource(caseSeed))
			modelIndex := (caseIndex + seedIndex) % len(models)
			model := models[modelIndex]
			spec := segmentModels[modelIndex]
			batchSize := 2 + caseRNG.Intn(11)
			var ops []Operation
			if caseIndex%4 != 0 {
				ops = batchingOperations(spec, caseRNG, caseIndex)
			} else {
				ops = randomizedOperations(spec, model, caseRNG, caseIndex, batchSize)
			}
			for len(ops) < batchSize {
				ops = append(ops, randomSegmentOperation(spec, model, caseRNG, caseIndex, len(ops)))
			}
			if len(ops) > batchSize {
				ops = ops[:batchSize]
			}
			got, starts, gotErr := apply(model, ops, false)
			want, _, wantErr := apply(model, ops, true)
			if field := firstDifference(got, gotErr, want, wantErr); field != "" {
				t.Fatalf("seed=%d case=%d model=%s field=%s gotErr=%#v wantErr=%#v\ngot text:\n%s\nwant text:\n%s\nops=%#v",
					caseSeed, caseIndex, spec.name, field, gotErr, wantErr,
					diagnosedText(gotErr), diagnosedText(wantErr), ops)
			}
			totalSegments += len(starts)
			seedSegments += len(starts)
			if hasMultiOperationSegment(starts, ops, got, gotErr) {
				multiCases++
				seedMulti++
				multiByModel[modelIndex]++
			}
			totalCases++
		}
		t.Logf("seed=%d cases=%d segments=%d multi-operation cases=%d", seed, 1000, seedSegments, seedMulti)
	}
	if multiCases < totalCases/2 {
		t.Fatalf("multi-operation cases = %d of %d, want at least half", multiCases, totalCases)
	}
	for i, count := range multiByModel {
		if count == 0 {
			t.Errorf("base model %q produced no multi-operation case", segmentModels[i].name)
		}
	}
	t.Logf("cases=%d segments=%d multi-operation cases=%d models=%d", totalCases, totalSegments, multiCases, len(segmentModels))
}

func batchingOperations(spec segmentModel, rng *rand.Rand, caseIndex int) []Operation {
	count := 2 + rng.Intn(3)
	adds := make([]Operation, 0, count)
	if len(spec.importOwners) > 0 {
		ownerIndex := rng.Intn(len(spec.importOwners))
		owner := spec.importOwners[ownerIndex]
		features := append([]string(nil), spec.inheritedFeatures...)
		perm := rng.Perm(len(features))
		for i := 0; i < count && i < len(features); i++ {
			adds = append(adds, Operation{
				Kind: OpAddMember, Owner: owner, MemberKind: "attribute",
				Redefines: []string{features[perm[i]]}, Value: fmt.Sprintf("%d.0", caseIndex+i),
			})
		}
		if caseIndex%29 == 0 && len(features) > 0 {
			adds = []Operation{
				{Kind: OpAddMember, Owner: owner, MemberKind: "attribute", Redefines: []string{features[0]}, Value: "1.0"},
				{Kind: OpAddMember, Owner: owner, MemberKind: "attribute", Redefines: []string{features[0]}, Value: "2.0"},
			}
		}
		if len(spec.localTargets) > 0 {
			localIndex := ownerIndex % len(spec.localTargets)
			set := Operation{Kind: OpSetValue, Target: spec.localTargets[localIndex], Value: fmt.Sprintf("%d.0", caseIndex+7)}
			insertAt := caseIndex % (len(adds) + 1)
			ops := make([]Operation, 0, len(adds)+1)
			ops = append(ops, adds[:insertAt]...)
			ops = append(ops, set)
			ops = append(ops, adds[insertAt:]...)
			return ops
		}
		return adds
	}
	for i := 0; i < count; i++ {
		adds = append(adds, Operation{
			Kind: OpAddMember, Owner: spec.batchOwner, MemberKind: "attribute",
			MemberName: fmt.Sprintf("batch%d_%d", caseIndex, i), Type: "Real",
		})
	}
	return adds
}

func randomizedOperations(spec segmentModel, model Model, rng *rand.Rand, caseIndex, batchSize int) []Operation {
	ops := make([]Operation, 0, batchSize)
	if caseIndex%8 == 0 {
		name := fmt.Sprintf("child%d", caseIndex)
		owner := spec.partOwner + "::" + name
		ops = append(ops,
			Operation{Kind: OpAddMember, Owner: spec.partOwner, MemberKind: "part", MemberName: name, Type: spec.partType},
			Operation{Kind: OpAddMember, Owner: owner, MemberKind: "attribute", MemberName: "x", Type: "Real", Value: "1.0"},
			Operation{Kind: OpSetValue, Target: owner + "::x", Value: "2.0"},
		)
	}
	for len(ops) < batchSize {
		ops = append(ops, randomSegmentOperation(spec, model, rng, caseIndex, len(ops)))
	}
	return ops
}

func randomSegmentOperation(spec segmentModel, model Model, rng *rand.Rand, caseIndex, opIndex int) Operation {
	values := []string{"1.0", "2.0", "1 // c", "1\t+ 2", `"abc`, "/* x"}
	memberValues := []string{"1.0", "2.0", "3.0", "1\t+ 2"}
	choice := rng.Intn(100)
	switch {
	case choice < 32:
		targets := spec.valueTargets
		if len(targets) == 0 {
			targets = []string{spec.batchOwner + "::missing"}
		}
		return Operation{Kind: OpSetValue, Target: targets[rng.Intn(len(targets))], Value: values[rng.Intn(len(values))]}
	case choice < 53 && len(spec.importOwners) > 0:
		owner := spec.importOwners[rng.Intn(len(spec.importOwners))]
		feature := spec.inheritedFeatures[rng.Intn(len(spec.inheritedFeatures))]
		return Operation{
			Kind: OpAddMember, Owner: owner, MemberKind: "attribute",
			Redefines: []string{feature}, Value: memberValues[rng.Intn(len(memberValues))],
			Doc: []string{"", "", "generated\tfeature"}[rng.Intn(3)],
		}
	case choice < 73:
		owner := spec.batchOwner
		name := fmt.Sprintf("generated%d_%d", caseIndex, opIndex)
		return Operation{
			Kind: OpAddMember, Owner: owner, MemberKind: "attribute",
			MemberName: name, Type: "Real", Value: memberValues[rng.Intn(len(memberValues))],
			Doc: []string{"", "", "generated\tfeature"}[rng.Intn(3)],
		}
	case choice < 77:
		return Operation{Kind: OpAddMember, Owner: "", MemberKind: "part def", MemberName: fmt.Sprintf("Root%d_%d", caseIndex, opIndex)}
	case choice < 81 && spec.stateOwner != "":
		kind := []string{"entry action", "do action", "exit action"}[rng.Intn(3)]
		return Operation{Kind: OpAddMember, Owner: spec.stateOwner, MemberKind: kind, MemberName: fmt.Sprintf("action%d_%d", caseIndex, opIndex)}
	case choice < 85 && spec.substateOwner != "":
		return Operation{Kind: OpAddMember, Owner: spec.substateOwner, MemberKind: "entry action", MemberName: fmt.Sprintf("nestedEntry%d_%d", caseIndex, opIndex)}
	case choice < 89 && spec.calculationOwner != "":
		return Operation{Kind: OpAddMember, Owner: spec.calculationOwner, MemberKind: "return", Type: "Real", Value: "1.0"}
	case choice < 92:
		return Operation{Kind: OpSetValue, Target: "missing::target", Value: "1.0"}
	case choice < 95:
		return Operation{Kind: OpAddMember, Owner: "missing::owner", MemberKind: "attribute", MemberName: "bad", Type: "Real"}
	default:
		for _, target := range spec.valueTargets {
			if decls := model.declared(target); len(decls) > 0 && decls[0].Decl.Span().Len > 0 {
				return SetLayoutAt(decls[0].Decl.Span(), "", &semantics.Layout{X: float64(caseIndex), Y: float64(opIndex)})
			}
		}
		return Operation{Kind: OpSetValue, Target: spec.valueTargets[0], Value: "1.0"}
	}
}

func hasMultiOperationSegment(starts []int, ops []Operation, result *Result, err error) bool {
	end := len(ops)
	var editErr *Error
	if errors.As(err, &editErr) && editErr.OperationIndex >= 0 {
		end = editErr.OperationIndex
	}
	for i, start := range starts {
		segmentEnd := end
		if i+1 < len(starts) && starts[i+1] < segmentEnd {
			segmentEnd = starts[i+1]
		}
		if segmentEnd-start >= 2 {
			return true
		}
	}
	if len(starts) == 1 && starts[0] == 0 && len(ops) > 1 && result != nil && err == nil {
		return true
	}
	return false
}

func TestApplyDoesNotMutateOperations(t *testing.T) {
	model := quickModel("slice.sysml", segmentModels[0].text)
	decl := model.declared("P::v0")[0].Decl.Span()
	ops := []Operation{
		{Kind: OpSetValue, Target: "P::Vehicle::mass", Value: "8.0"},
		SetLayoutAt(decl, "", &semantics.Layout{X: 1, Y: 2}),
	}
	before := append([]Operation(nil), ops...)
	_, _ = Apply(model, ops)
	if !reflect.DeepEqual(ops, before) {
		t.Fatalf("Apply mutated operations:\ngot %#v\nwant %#v", ops, before)
	}
}

func assertSegments(t *testing.T, model Model, ops []Operation, wantStarts []int) (*Result, error) {
	t.Helper()
	got, starts, gotErr := apply(model, ops, false)
	want, sequentialStarts, wantErr := apply(model, ops, true)
	if field := firstDifference(got, gotErr, want, wantErr); field != "" {
		t.Fatalf("sequential mismatch at %s: got %v, want %v", field, gotErr, wantErr)
	}
	if !reflect.DeepEqual(starts, wantStarts) {
		t.Fatalf("segment starts = %v, want %v (error: %v)", starts, wantStarts, gotErr)
	}
	wantSequentialStarts := make([]int, 0, len(ops))
	if !needsSequential(ops) {
		wantSequentialStarts = append(wantSequentialStarts, 0)
	} else {
		limit := len(ops)
		var editErr *Error
		if errors.As(wantErr, &editErr) && editErr.OperationIndex >= 0 && editErr.OperationIndex+1 < limit {
			limit = editErr.OperationIndex + 1
		}
		for i := 0; i < limit; i++ {
			wantSequentialStarts = append(wantSequentialStarts, i)
		}
	}
	if !reflect.DeepEqual(sequentialStarts, wantSequentialStarts) {
		t.Fatalf("sequential starts = %v, want %v", sequentialStarts, wantSequentialStarts)
	}
	return got, gotErr
}

func compactModel() Model {
	return quickModel("segments.sysml", `package P {
    part def Vehicle {
        attribute mass : Real = 1.0;
    }
    part v0 : Vehicle {
        attribute mass : Real = 2.0;
    }
    part v1 : Vehicle {
        attribute mass : Real = 3.0;
    }
}
`)
}

func TestSegmentsSplitOnWritesAndAllowBenignWrites(t *testing.T) {
	model := compactModel()
	same := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "9.0"},
	}
	assertSegments(t, model, same, []int{0})

	overlap := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "9.0"},
		{Kind: OpAddMember, Owner: "P::v1", MemberKind: "attribute", MemberName: "extra", Type: "Real", Value: "1.0"},
	}
	assertSegments(t, model, overlap, []int{0, 1})

	benign := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "extra", Type: "Real", Value: "1.0"},
	}
	assertSegments(t, model, benign, []int{0})

	nonBenign := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "1.0 +\n            2.0"},
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "extra", Type: "Real", Value: "1.0"},
	}
	assertSegments(t, model, nonBenign, []int{0, 1})

	oneLine := quickModel("line-touch.sysml", `package P { part def Vehicle { attribute mass : Real; } part v0 : Vehicle { attribute local : Real = 2.0; } }`)
	lineThreshold := []Operation{
		{Kind: OpSetValue, Target: "P::v0::local", Value: "8.0"},
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "extra", Type: "Real"},
	}
	assertSegments(t, oneLine, lineThreshold, []int{0, 1})
}

func TestSegmentWriteIntervalsTouchAtBoundaries(t *testing.T) {
	if !intervalsTouch(source.Span{Offset: 4, Len: 3}, source.Span{Offset: 7, Len: 2}) {
		t.Fatal("writes meeting at an endpoint should touch")
	}
	if intervalsTouch(source.Span{Offset: 4, Len: 3}, source.Span{Offset: 8, Len: 2}) {
		t.Fatal("writes separated by a byte should not touch")
	}
	segment := &editSegment{nonBenign: []int{0}, writes: []*segmentWrite{
		{sp: splice{span: source.Span{Offset: 4, Len: 3}}},
	}}
	candidate := segmentCandidate{sp: splice{span: source.Span{Offset: 7, Len: 2}}}
	if segment.writeIndependent(candidate, nil) {
		t.Fatal("touching writes should not share a segment")
	}
}

func TestSegmentsSplitOnTabsAndLexicalClosure(t *testing.T) {
	model := compactModel()
	tabTransition := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "1\t"},
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "extra", Type: "Real", Value: "1.0"},
	}
	assertSegments(t, model, tabTransition, []int{0, 1})

	comment := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "1 // c"},
		{Kind: OpAddMember, Owner: "P::v1", MemberKind: "attribute", MemberName: "extra", Type: "Real", Value: "2.0"},
	}
	assertSegments(t, model, comment, []int{0, 1})
}

func TestSegmentsChainBodylessMemberInsertions(t *testing.T) {
	model := quickModel("chain.sysml", `package P {
    part def Battery;
    part def Vehicle {
        part battery : Battery;
    }
}
`)
	ops := []Operation{
		{Kind: OpAddMember, Owner: "P::Vehicle::battery", MemberKind: "attribute", MemberName: "alpha", Type: "Real"},
		{Kind: OpAddMember, Owner: "P::Vehicle::battery", MemberKind: "attribute", MemberName: "beta", Type: "Real"},
	}
	result, err := assertSegments(t, model, ops, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Content), "attribute alpha : Real;\n            attribute beta : Real;") {
		t.Fatalf("chained body is out of order:\n%s", result.Content)
	}

	bodyful := compactModel()
	bodyfulOps := []Operation{
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "alpha", Type: "Real"},
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "beta", Type: "Real"},
	}
	assertSegments(t, bodyful, bodyfulOps, []int{0})
}

func TestSegmentsBatchFlatImportedFeatures(t *testing.T) {
	model := loadContent(t, "flat-import.sysml", `package P {
    private import ScalarValues::*;
    part def Vehicle {
        attribute mass : Real;
    }
    part v0 : Vehicle;
    part v1 : Vehicle;
}
`)
	ops := []Operation{
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "mass", Type: "Real", Value: "1.0", Redefines: []string{"P::Vehicle::mass"}},
		{Kind: OpAddMember, Owner: "P::v1", MemberKind: "attribute", MemberName: "mass", Type: "Real", Value: "2.0", Redefines: []string{"P::Vehicle::mass"}},
	}
	_, err := assertSegments(t, model, ops, []int{0})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSegmentsRetryOperationsAfterBoundary(t *testing.T) {
	model := compactModel()
	ops := []Operation{
		{Kind: OpAddMember, Owner: "P", MemberKind: "part", MemberName: "newPart", Type: "Vehicle"},
		{Kind: OpAddMember, Owner: "P::newPart", MemberKind: "attribute", MemberName: "newValue", Type: "Real"},
		{Kind: OpSetValue, Target: "P::newPart::newValue", Value: "2.0"},
	}
	if _, err := assertSegments(t, model, ops, []int{0, 1, 2}); err != nil {
		t.Fatalf("operations into a newly declared owner failed: %v", err)
	}

	calculation := []Operation{
		{Kind: OpAddMember, Owner: "P", MemberKind: "calc def", MemberName: "NewCalc"},
		{Kind: OpAddMember, Owner: "P::NewCalc", MemberKind: "return", Type: "Real", Value: "1.0"},
	}
	assertSegments(t, model, calculation, []int{0, 1})

	refused := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
		{Kind: OpSetValue, Target: "P::missing::mass", Value: "9.0"},
	}
	_, err := assertSegments(t, model, refused, []int{0})
	if got := editError(t, err); got.OperationIndex != 1 {
		t.Fatalf("refused operation index = %d, want 1", got.OperationIndex)
	}
}

func TestSegmentsSplitAtDeclarationAndNameDependencies(t *testing.T) {
	model := compactModel()
	declarationModel := loadContent(t, "declaration.sysml", `package P {
    private import ScalarValues::*;
    part def Vehicle { attribute mass : Real; }
    part v0 : Vehicle { attribute mass : Real = 2.0; }
    part v1 : Vehicle { attribute mass : Real = 3.0; }
}
`)
	decl := declarationModel.declared("P::v0")[0].Decl.Span()
	ops := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
		SetLayoutAt(decl, "", &semantics.Layout{X: 1, Y: 2}),
		{Kind: OpSetValue, Target: "P::v1::mass", Value: "9.0"},
	}
	assertSegments(t, declarationModel, ops, []int{0, 1, 2})

	beforeDeclaration := []Operation{
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
		{Kind: OpAddMember, Owner: "P::v1", MemberKind: "attribute", MemberName: "newValue", Type: "Real"},
		SetLayoutAt(decl, "", &semantics.Layout{X: 3, Y: 4}),
	}
	assertSegments(t, declarationModel, beforeDeclaration, []int{0, 1, 2})

	redefines := []Operation{
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "override", Type: "Real", Value: "4.0", Redefines: []string{"P::v0::mass"}},
		{Kind: OpSetValue, Target: "P::v0::mass", Value: "5.0"},
	}
	assertSegments(t, model, redefines, []int{0, 1})

	ancestorModel := quickModel("ancestor-name.sysml", `package P {
    part def Child { attribute x : Real = 1.0; }
    part def Vehicle { attribute mass : Real; }
    part container : Vehicle {
        part mass : Child { attribute x : Real = 2.0; }
    }
}
`)
	ancestor := []Operation{
		{Kind: OpAddMember, Owner: "P::container", MemberKind: "attribute", Redefines: []string{"mass"}, Value: "3.0"},
		{Kind: OpSetValue, Target: "P::container::mass::x", Value: "4.0"},
	}
	assertSegments(t, ancestorModel, ancestor, []int{0, 1})

	taken := []Operation{
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "extra", Type: "Real"},
		{Kind: OpAddMember, Owner: "P::v0", MemberKind: "attribute", MemberName: "extra", Type: "Real"},
	}
	_, err := assertSegments(t, model, taken, []int{0, 1})
	if editError(t, err).OperationIndex != 1 {
		t.Fatalf("name collision index = %d, want 1", editError(t, err).OperationIndex)
	}
}

func TestSegmentsKeepExcludedOperationsAlone(t *testing.T) {
	model := quickModel("excluded.sysml", `package P {
    calc def C { }
    state def S {
        state sub;
    }
    part def Vehicle { attribute mass : Real = 1.0; }
    part v0 : Vehicle { attribute mass : Real = 2.0; }
    part v1 : Vehicle { attribute mass : Real = 3.0; }
}
`)
	tests := []struct {
		name string
		op   Operation
	}{
		{"root", Operation{Kind: OpAddMember, MemberKind: "part def", MemberName: "RootPart"}},
		{"calculation", Operation{Kind: OpAddMember, Owner: "P::C", MemberKind: "attribute", MemberName: "local", Type: "Real"}},
		{"substate", Operation{Kind: OpAddMember, Owner: "P::S::sub", MemberKind: "attribute", MemberName: "entered", Type: "Real"}},
		{"entry action", Operation{Kind: OpAddMember, Owner: "P::S", MemberKind: "entry action", MemberName: "entered"}},
		{"do action", Operation{Kind: OpAddMember, Owner: "P::S", MemberKind: "do action", MemberName: "doWork"}},
		{"exit action", Operation{Kind: OpAddMember, Owner: "P::S", MemberKind: "exit action", MemberName: "leave"}},
		{"return", Operation{Kind: OpAddMember, Owner: "P::C", MemberKind: "return", Type: "Real"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := []Operation{
				{Kind: OpSetValue, Target: "P::v0::mass", Value: "8.0"},
				tt.op,
				{Kind: OpSetValue, Target: "P::v1::mass", Value: "9.0"},
			}
			_, err := assertSegments(t, model, ops, []int{0, 1, 2})
			if err != nil {
				t.Fatalf("operation was refused instead of exercising the exclusion: %v", err)
			}
		})
	}
}
