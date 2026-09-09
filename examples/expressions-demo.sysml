// An instrument payload written so that each expression form — casts, `*`,
// `.metadata`, function values, sets, tensors, collection bodies — has
// something concrete to answer: see EXPRESSIONS-DEMO.md for the walkthrough.
package ExpressionsDemo {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;
    private import Quantities::*;
    private import MeasurementReferences::*;
    private import TensorCalculations::*;
    private import ControlFunctions::*;
    private import SequenceFunctions::*;
    private import CollectionFunctions::*;
    private import Collections::*;
    private import SampledFunctions::*;

    // --- The payload -------------------------------------------------------

    metadata def Heritage {
        attribute mission : String;
        attribute flown : Boolean default true;
    }

    part def Instrument {
        attribute mass : MassValue;
    }
    part def Camera :> Instrument;
    part def Spectrometer :> Instrument;

    part navCam : Camera {
        :>> mass = 4.0 [kg];
        @Heritage { mission = "Cassini"; }
    }
    part sciCam : Camera {
        :>> mass = 6.5 [kg];
    }
    part spectrometer : Spectrometer {
        :>> mass = 12.0 [kg];
        @Heritage { mission = "Rosetta"; flown = false; }
    }

    // --- Casts: `x as T` selects, it does not convert -------------------------

    calc def WholeReadings {
        in readings : Real[0..*];
        return : Integer[0..*] = readings as Integer;
    }

    calc def CamerasOf {
        in candidates : Instrument[0..*];
        return : Camera[0..*] = candidates as Camera;
    }

    part def PayloadCasts {
        attribute wholeOnly : Integer[0..*] = WholeReadings((1.0, 2.5, 3.0, 4.75));
        ref part cameras : Camera[0..*] = CamerasOf((navCam, spectrometer, sciCam));
        attribute cameraCount : Natural = size(cameras);
        ref part asInstrument : Instrument[0..1] = navCam as Instrument;
        ref part notACamera : Camera[0..1] = CamerasOf(spectrometer);
    }

    // --- `*`: the unbounded value ---------------------------------------------

    part def DownlinkBudget {
        attribute passLimit : Natural = *;
        attribute plannedPasses : Natural = 40;
        attribute withinLimit : Boolean = plannedPasses < passLimit;
        attribute limitIsUnbounded : Boolean = passLimit == *;
    }

    // --- `.metadata`: what annotates an element -------------------------------

    part def HeritageReport {
        ref navCamAnnotations [*] = navCam.metadata;
        attribute navCamMission : String =
            (navCam.metadata#(1) as Heritage).mission;
        attribute navCamFlown : Boolean =
            (navCam.metadata#(1) as Heritage).flown;
        attribute spectrometerFlown : Boolean =
            (spectrometer.metadata#(1) as Heritage).flown;
        ref sciCamAnnotations [*] = sciCam.metadata;
    }

    // --- Calculations as values -----------------------------------------------

    calc def Square { in v : Real; return : Real = v * v; }
    calc def Halve { in v : Real; return : Real = v / 2.0; }

    calc def Apply {
        in calc f { in v : Real; return : Real; }
        in a : Real;
        return : Real = f(a);
    }

    calc def Squared { in a : Real; return : Real = Apply(Square, a); }
    calc def Halved { in a : Real; return : Real = Apply(Halve, a); }

    calc def SquaresOf {
        in xs : Real[0..*];
        attribute sampled : SampledFunction = Sample(Square, xs);
        return : Real[0..*] = Range(sampled);
    }

    part def Amplifier {
        attribute gain : Real = 1.5;
        calc amplify { in v : Real; return : Real = v * gain; }
        ref transfer = amplify;
        attribute sameTransfer : Boolean = transfer == amplify;
        attribute atThree : Real = Apply(transfer, 3.0);
        attribute squaredAtThree : Real = Apply(Square, 3.0);
    }

    // --- Sets: unordered, no repeats ------------------------------------------

    part def RadioBands {
        attribute requested : Set { :>> elements = ("X", "Ka", "X", "S", "Ka"); }
        attribute licensed : Set { :>> elements = ("S", "X", "Ka"); }
        attribute distinctBands : Natural = size(requested);
        attribute sameBands : Boolean = requested == licensed;
        attribute hasKa : Boolean = contains(requested, "Ka");
        attribute asList : String[*] ordered = requested.elements;
    }

    // --- Tensors of rank three ------------------------------------------------

    attribute stressRef : TensorMeasurementReference {
        :>> dimensions = (2, 2, 2);
        :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
    }

    part def MountStress {
        attribute field : TensorQuantityValue =
            TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), stressRef);
        attribute rank = field.order;
        attribute dims : Positive[*] ordered = field.dimensions;
        attribute corner = field#(1, 1, 1);
        attribute opposite = field#(2, 2, 2);
        attribute doubled = 2 * field;
        attribute doubledCorner = doubled#(2, 1, 1);
    }

    // --- Collection bodies: the result has the body's type --------------------

    part def MassRollup {
        ref part instruments : Instrument[0..*] = (navCam, sciCam, spectrometer);
        attribute masses : MassValue[0..*] =
            instruments->collect { in i : Instrument; i.mass };
        attribute total : MassValue =
            masses->reduce { in a : MassValue; in b : MassValue; a + b };
        attribute heaviest : MassValue =
            masses->reduce { in a : MassValue; in b : MassValue; if a > b ? a else b };
        ref part heavy : Instrument[0..*] =
            instruments->select { in i : Instrument; i.mass > 5.0 [kg] };
        attribute heavyCount : Natural = size(heavy);
    }
}
