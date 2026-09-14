package runtime

import (
	"math"
	"strings"
)

// censusPackages are the analysis-domain libraries and the function libraries
// they build on, with the representative invocation of each declaration.
var censusPackages = []censusPackage{
	{name: "AnalysisTooling", path: "Domain Libraries/Analysis/AnalysisTooling.sysml"},
	{name: "SampledFunctions", path: "Domain Libraries/Analysis/SampledFunctions.sysml", probes: sampledFunctionProbes},
	{name: "TradeStudies", path: "Domain Libraries/Analysis/TradeStudies.sysml", probes: tradeStudyProbes},
	{name: "StateSpaceRepresentation", path: "Domain Libraries/Analysis/StateSpaceRepresentation.sysml", probes: stateSpaceProbes},
	{name: "VectorFunctions", path: "Kernel Libraries/Kernel Function Library/VectorFunctions.kerml", probes: vectorFunctionProbes},
	{name: "OccurrenceFunctions", path: "Kernel Libraries/Kernel Function Library/OccurrenceFunctions.kerml", probes: occurrenceFunctionProbes},
}

// probeModel wraps the declarations of a probe in the package `test`, importing
// the library packages it uses.
func probeModel(imports string, body string) string {
	model := "package test {\n\tprivate import ScalarValues::*;\n"
	for _, pkg := range strings.Fields(imports) {
		model += "\tprivate import " + pkg + "::*;\n"
	}
	return model + body + "}\n"
}

func expectReal(v float64) ExpectedValue { return ExpectedValue{Type: "Real", Value: v} }

func expectBool(v bool) ExpectedValue { return ExpectedValue{Type: "Boolean", Value: v} }

func expectInt(v int64) ExpectedValue { return ExpectedValue{Type: "Integer", Value: float64(v)} }

func expectReals(vs ...float64) []ExpectedValue {
	elements := make([]ExpectedValue, len(vs))
	for i, v := range vs {
		elements[i] = expectReal(v)
	}
	return elements
}

func expectVector(vs ...float64) ExpectedValue {
	return ExpectedValue{Type: "Vector", Elements: expectReals(vs...)}
}

func expectSequence(vs ...float64) ExpectedValue {
	return ExpectedValue{Type: "Sequence", Elements: expectReals(vs...)}
}

// sampledPairs is a sampled function of two samples, (1 → 2) and (3 → 4).
const sampledPairs = `	attribute s : SampledFunction = new SampledFunction(
		samples = (new SamplePair(1.0, 2.0), new SamplePair(3.0, 4.0)));
`

var sampledFunctionProbes = []libraryProbe{
	{
		decl:  "SampledFunctions::Domain",
		model: probeModel("SampledFunctions", sampledPairs+"\tattribute r = Domain(s);\n"),
		read:  "test::r",
		want:  expectSequence(1.0, 3.0),
	},
	{
		decl:  "SampledFunctions::Range",
		model: probeModel("SampledFunctions", sampledPairs+"\tattribute r = Range(s);\n"),
		read:  "test::r",
		want:  expectSequence(2.0, 4.0),
	},
	{
		decl: "SampledFunctions::Sample",
		model: probeModel("SampledFunctions", `	calc def Sq { in v : Real; return : Real = v * v; }
	attribute r = Range(Sample(Sq, (1.0, 2.0, 3.0)));
`),
		read: "test::r",
		want: expectSequence(1.0, 4.0, 9.0),
	},
	{
		decl: "SampledFunctions::Interpolate",
		model: probeModel("SampledFunctions", sampledPairs+`	calc def First :> Interpolate { return :>> result = fn.samples#(1).rangeValue; }
	attribute r = First(s, 9.0);
`),
		read: "test::r",
		want: expectReal(2.0),
	},
	{
		// The library's Linear formula: f = (2-1)/(1-3) = -0.5, 4 + f*(2-4) = 5.
		decl:  "SampledFunctions::interpolateLinear",
		model: probeModel("SampledFunctions", sampledPairs+"\tattribute r = interpolateLinear(s, 2.0);\n"),
		read:  "test::r",
		want:  expectReal(5.0),
	},
}

// engines are two alternatives a trade study chooses between.
const engines = `	part def Engine { attribute mass : Real; }
	part light : Engine { attribute :>> mass = 10.0; }
	part heavy : Engine { attribute :>> mass = 20.0; }
`

// byMass is a trade study over the two engines with the objective named.
func byMass(objective string) string {
	return engines + `	analysis study : TradeStudy {
		subject : Engine[1..*] = (heavy, light);
		objective : ` + objective + `;
		calc :>> evaluationFunction {
			in part e :>> alternative : Engine;
			return :>> result : Real = e.mass;
		}
		return part :>> selectedAlternative : Engine;
	}
`
}

var tradeStudyProbes = []libraryProbe{
	{
		decl: "TradeStudies::EvaluationFunction",
		model: probeModel("TradeStudies", engines+`	calc massOf : EvaluationFunction {
		in part e :>> alternative : Engine;
		return :>> result : Real = e.mass;
	}
	attribute r : Real = massOf(heavy);
`),
		read: "test::r",
		want: expectReal(20.0),
	},
	{
		decl: "TradeStudies::TradeStudyObjective",
		model: probeModel("TradeStudies", `	requirement def ExactlyTwenty :> TradeStudyObjective { attribute :>> best = 20.0; }
`+byMass("ExactlyTwenty")+"\tattribute r : Real = study.selectedAlternative.mass;\n"),
		read: "test::r",
		want: expectReal(20.0),
	},
	{
		decl:  "TradeStudies::MinimizeObjective",
		model: probeModel("TradeStudies", byMass("MinimizeObjective")+"\tattribute r : Real = study.selectedAlternative.mass;\n"),
		read:  "test::r",
		want:  expectReal(10.0),
	},
	{
		decl:  "TradeStudies::MaximizeObjective",
		model: probeModel("TradeStudies", byMass("MaximizeObjective")+"\tattribute r : Real = study.selectedAlternative.mass;\n"),
		read:  "test::r",
		want:  expectReal(20.0),
	},
	{
		decl:  "TradeStudies::TradeStudy",
		model: probeModel("TradeStudies", byMass("MinimizeObjective")+"\tattribute r : Boolean = study.selectedAlternative === light;\n"),
		read:  "test::r",
		want:  expectBool(true),
	},
	{
		decl:  "TradeStudies::TradeStudy::evaluationFunction",
		model: probeModel("TradeStudies", byMass("MinimizeObjective")+"\tattribute r : Real = study.evaluationFunction(heavy);\n"),
		read:  "test::r",
		want:  expectReal(20.0),
	},
}

// stateSpaceModel specializes the state-space library as a model does: a calc for
// each abstract calc definition and a dynamics of each kind over a two-axis position.
const stateSpaceModel = `	private import Quantities::*;
	private import SI::*;
	private import VectorFunctions::*;
	attribute def Force :> Input;
	attribute def Position :> StateSpace;
	attribute def Reading :> Output;
	attribute force : Force = 2 [N] * VectorOf((1.0, 2.0));
	attribute position : Position = 1 [m] * VectorOf((1.0, 1.0));
	calc def Hold :> GetNextState {
		in input : Input;
		in stateSpace : StateSpace;
		in timeStep : DurationValue;
		return : StateSpace = stateSpace;
	}
	calc def Echo :> GetOutput {
		in input : Input;
		in stateSpace : StateSpace;
		return : Output = stateSpace;
	}
	calc def Rate :> GetDerivative {
		in input : Input;
		in stateSpace : StateSpace;
		return : StateDerivative = stateSpace / 1 [s];
	}
	calc def Euler :> Integrate {
		in getDerivative : GetDerivative;
		in input : Input;
		in initialState : StateSpace;
		in timeInterval : DurationValue;
		return result : StateSpace = initialState + getDerivative(input, initialState) * timeInterval;
	}
	calc def Shift :> GetDifference {
		in input : Input;
		in stateSpace : StateSpace;
		return : StateSpace = stateSpace;
	}
	action def Plant :> StateSpaceDynamics {
		calc :>> getNextState : Hold;
		calc :>> getOutput : Echo;
		attribute :>> stateSpace = position;
	}
	action plant : Plant { in :>> input = force; }
	action def Damper :> ContinuousStateSpaceDynamics {
		calc :>> getDerivative : Rate;
		calc :>> getOutput : Echo;
		attribute :>> stateSpace = position;
	}
	action damper : Damper { in :>> input = force; }
	action def Spring :> DiscreteStateSpaceDynamics {
		calc :>> getDifference : Shift;
		calc :>> getOutput : Echo;
		attribute :>> stateSpace = position;
	}
	action spring : Spring { in :>> input = force; }
`

// metres is a two-axis vector quantity in metres.
func metres(vs ...float64) ExpectedValue {
	elements := make([]ExpectedValue, len(vs))
	for i, v := range vs {
		elements[i] = ExpectedValue{Type: "Quantity", Value: v, Unit: "m"}
	}
	return ExpectedValue{Type: "VectorQuantity", Elements: elements}
}

// stateSpaceProbes invoke each declaration as the library declares it, with no
// state-space runner supplying the abstract dynamics.
var stateSpaceProbes = []libraryProbe{
	stateSpaceRead("GetNextState", "Hold(force, position, 1 [s])", metres(1.0, 1.0)),
	stateSpaceRead("GetOutput", "Echo(force, position)", metres(1.0, 1.0)),
	stateSpaceAction("StateSpaceEventDef"),
	stateSpaceAction("ZeroCrossingEventDef"),
	stateSpacePerform("StateSpaceDynamics", "plant"),
	stateSpaceRead("StateSpaceDynamics::getNextState", "plant.getNextState(force, position, 1 [s])", metres(1.0, 1.0)),
	stateSpaceRead("StateSpaceDynamics::getOutput", "plant.getOutput(force, position)", metres(1.0, 1.0)),
	stateSpaceRead("GetDerivative", "Rate(force, position)", ExpectedValue{Type: "VectorQuantity", Elements: []ExpectedValue{
		{Type: "Quantity", Value: 1.0, Unit: "SI::'m/s'"}, {Type: "Quantity", Value: 1.0, Unit: "SI::'m/s'"}}}),
	stateSpaceRead("Integrate", "Euler(Rate, force, position, 2 [s])", metres(3.0, 3.0)),
	stateSpacePerform("ContinuousStateSpaceDynamics", "damper"),
	stateSpaceRead("ContinuousStateSpaceDynamics::getDerivative", "damper.getDerivative(force, position)", ExpectedValue{
		Type: "VectorQuantity", Elements: []ExpectedValue{
			{Type: "Quantity", Value: 1.0, Unit: "SI::'m/s'"}, {Type: "Quantity", Value: 1.0, Unit: "SI::'m/s'"}}}),
	// The library leaves Integrate abstract; an Euler step over one second is the value checked.
	stateSpaceRead("ContinuousStateSpaceDynamics::getNextState", "damper.getNextState(force, position, 1 [s])", metres(2.0, 2.0)),
	stateSpaceRead("ContinuousStateSpaceDynamics::getNextState::integrate", "damper.getNextState.integrate.result", metres(2.0, 2.0)),
	stateSpaceRead("GetDifference", "Shift(force, position)", metres(1.0, 1.0)),
	stateSpacePerform("DiscreteStateSpaceDynamics", "spring"),
	stateSpaceRead("DiscreteStateSpaceDynamics::getDifference", "spring.getDifference(force, position)", metres(1.0, 1.0)),
	stateSpaceRead("DiscreteStateSpaceDynamics::getNextState", "spring.getNextState(force, position, 1 [s])", metres(2.0, 2.0)),
}

// stateSpaceRead evaluates an expression over stateSpaceModel and checks its value.
func stateSpaceRead(name, expr string, want ExpectedValue) libraryProbe {
	return libraryProbe{
		decl:  "StateSpaceRepresentation::" + name,
		model: probeModel("StateSpaceRepresentation", stateSpaceModel+"\tattribute r = "+expr+";\n"),
		read:  "test::r",
		want:  want,
	}
}

// stateSpacePerform performs one of stateSpaceModel's dynamics and reads its output.
func stateSpacePerform(name, usage string) libraryProbe {
	return libraryProbe{
		decl:   "StateSpaceRepresentation::" + name,
		model:  probeModel("StateSpaceRepresentation", stateSpaceModel),
		action: "test::" + usage,
		output: "output",
		want:   metres(1.0, 1.0),
	}
}

// stateSpaceAction performs an event definition as declared; it has no body and
// no output, so no value it could answer passes.
func stateSpaceAction(name string) libraryProbe {
	return libraryProbe{
		decl:   "StateSpaceRepresentation::" + name,
		model:  probeModel("StateSpaceRepresentation", "\taction run : "+name+";\n"),
		action: "test::run",
		output: "output",
		want:   metres(0.0),
	}
}

var vectorFunctionProbes = []libraryProbe{
	vectorProbe("isZeroVector", "isZeroVector(CartesianVectorOf((0.0, 0.0)))", expectBool(true)),
	vectorProbe("+", "VectorFunctions::'+'((1.0, 2.0), (3.0, 4.0))", expectVector(4.0, 6.0)),
	vectorProbe("-", "VectorFunctions::'-'((1.0, 2.0), (3.0, 4.0))", expectVector(-2.0, -2.0)),
	vectorProbe("sum0", "sum0((CartesianVectorOf((1.0, 2.0)), CartesianVectorOf((3.0, 4.0))), CartesianVectorOf((0.0, 0.0)))", expectVector(4.0, 6.0)),
	vectorProbe("VectorOf", "VectorOf((1.0, 2.0, 3.0))", expectVector(1.0, 2.0, 3.0)),
	vectorProbe("scalarVectorMult", "scalarVectorMult(2.0, (1.0, 2.0))", expectVector(2.0, 4.0)),
	vectorProbe("vectorScalarMult", "vectorScalarMult((1.0, 2.0), 3.0)", expectVector(3.0, 6.0)),
	vectorProbe("vectorScalarDiv", "vectorScalarDiv((2.0, 4.0), 2.0)", expectVector(1.0, 2.0)),
	vectorProbe("inner", "inner((1.0, 2.0), (3.0, 4.0))", expectReal(11.0)),
	vectorProbe("norm", "norm((3.0, 4.0))", expectReal(5.0)),
	vectorProbe("angle", "angle((1.0, 0.0), (0.0, 1.0))", expectReal(math.Pi/2)),
	vectorProbe("CartesianVectorOf", "CartesianVectorOf((1.0, 2.0))", expectVector(1.0, 2.0)),
	vectorProbe("CartesianThreeVectorOf", "CartesianThreeVectorOf((1.0, 2.0, 3.0))", expectVector(1.0, 2.0, 3.0)),
	vectorProbe("isCartesianZeroVector", "isCartesianZeroVector(CartesianVectorOf((0.0, 1.0)))", expectBool(false)),
	vectorProbe("cartesian+", "'cartesian+'((1.0, 2.0), (3.0, 4.0))", expectVector(4.0, 6.0)),
	vectorProbe("cartesian-", "'cartesian-'((1.0, 2.0), (3.0, 4.0))", expectVector(-2.0, -2.0)),
	vectorProbe("cartesianScalarVectorMult", "cartesianScalarVectorMult(2.0, (1.0, 2.0))", expectVector(2.0, 4.0)),
	vectorProbe("cartesianVectorScalarMult", "cartesianVectorScalarMult((1.0, 2.0), 2.0)", expectVector(2.0, 4.0)),
	vectorProbe("cartesianInner", "cartesianInner((1.0, 2.0), (3.0, 4.0))", expectReal(11.0)),
	vectorProbe("cartesianNorm", "cartesianNorm((3.0, 4.0))", expectReal(5.0)),
	vectorProbe("cartesianAngle", "cartesianAngle((1.0, 0.0), (0.0, 1.0))", expectReal(math.Pi/2)),
	vectorProbe("sum", "sum((CartesianThreeVectorOf((1.0, 2.0, 3.0)), CartesianThreeVectorOf((4.0, 5.0, 6.0))))", expectVector(5.0, 7.0, 9.0)),
}

// vectorProbe reads one expression over VectorFunctions at package level.
func vectorProbe(name, expr string, want ExpectedValue) libraryProbe {
	return libraryProbe{
		decl:  "VectorFunctions::" + name,
		model: probeModel("VectorValues VectorFunctions", "\tattribute r = "+expr+";\n"),
		read:  "test::r",
		want:  want,
	}
}

// widgets is a part definition whose parts are the occurrences the probes act on.
const widgets = `	part def Widget { attribute n : Integer = 1; }
`

// robot performs one action whose body assigns r from the statements given,
// over its part `arm`, its group `spares` and the parts `spare` and `fresh`
// the action holds.
func robot(body string) string {
	return probeModel("OccurrenceFunctions", widgets+`	private import SequenceFunctions::size;
	part def Robot {
		part arm : Widget;
		part spares : Widget[0..*];
		attribute r : Boolean;
		attribute count : Integer;
		perform action go {
			part fresh : Widget;
			part spare : Widget;
			first start;
			then action work {
`+body+`			}
			then done;
		}
	}
`)
}

var occurrenceFunctionProbes = []libraryProbe{
	{
		decl: "OccurrenceFunctions::===",
		model: probeModel("OccurrenceFunctions", widgets+`	part bench {
		part a : Widget;
		part b : Widget;
		attribute r : Boolean = OccurrenceFunctions::'==='(a, a) and not OccurrenceFunctions::'==='(a, b);
	}
`),
		instantiate: "test::bench",
		slot:        "r",
		want:        expectBool(true),
	},
	{
		decl:        "OccurrenceFunctions::isDuring",
		model:       robot("\t\t\t\tassign r := isDuring(arm);\n"),
		instantiate: "test::Robot",
		slot:        "r",
		want:        expectBool(true),
	},
	{
		decl:        "OccurrenceFunctions::create",
		model:       robot("\t\t\t\tassign r := create(fresh) === fresh;\n"),
		instantiate: "test::Robot",
		slot:        "r",
		want:        expectBool(true),
	},
	{
		decl:        "OccurrenceFunctions::destroy",
		model:       robot("\t\t\t\tassign arm := destroy(arm);\n\t\t\t\tassign r := isDuring(arm);\n"),
		instantiate: "test::Robot",
		slot:        "r",
		want:        expectBool(false),
	},
	{
		decl:        "OccurrenceFunctions::addNew",
		model:       robot("\t\t\t\tassign spares := addNew(spares, spare);\n\t\t\t\tassign count := size(spares);\n"),
		instantiate: "test::Robot",
		slot:        "count",
		want:        expectInt(1),
	},
	{
		decl:        "OccurrenceFunctions::addNewAt",
		model:       robot("\t\t\t\tassign spares := addNewAt(spares, spare, 1);\n\t\t\t\tassign count := size(spares);\n"),
		instantiate: "test::Robot",
		slot:        "count",
		want:        expectInt(1),
	},
	{
		decl: "OccurrenceFunctions::removeOld",
		model: robot(`				assign spares := addNew(spares, spare);
				perform action clean : removeOld { in group = spares; in occ = spare; }
				assign r := isDuring(spare);
`),
		instantiate: "test::Robot",
		slot:        "r",
		want:        expectBool(false),
	},
	{
		decl: "OccurrenceFunctions::removeOldAt",
		model: robot(`				assign spares := addNew(spares, spare);
				perform action clean : removeOldAt { in group = spares; in index = 1; }
				assign r := isDuring(spare);
`),
		instantiate: "test::Robot",
		slot:        "r",
		want:        expectBool(false),
	},
}
