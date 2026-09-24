// An analysis run's printed results are discarded unless recorded — `%record`
// and `-record-run` write them back on the `AnalysisRecords` library
// vocabulary, or, as here, by hand. These records are hand-written because the
// story needs values from before a model edit: a derived attribute on the
// record recomputes the run's output from the model *as it stands now*, so a
// record whose numbers the model has moved away from reports its own drift.
// See README.md for the commands that produced each recorded row.
package Landers {
    private import ScalarValues::*;

    part def Lander {
        attribute dryMass : Real; // kg, without fuel
        attribute fuel : Real; // kg loaded at the start of descent
        attribute burnRate : Real; // kg/s the engine consumes at full thrust
        attribute payload : Real; // kg of instruments carried
        attribute touchdownSpeed : Real; // m/s, from the descent profile
    }

    part scout : Lander {
        attribute :>> dryMass = 600.0;
        attribute :>> fuel = 250.0;
        attribute :>> burnRate = 3.0;
        attribute :>> payload = 40.0;
        attribute :>> touchdownSpeed = 1.2;
    }
    part hauler : Lander {
        attribute :>> dryMass = 1400.0;
        attribute :>> fuel = 900.0;
        attribute :>> burnRate = 8.0;
        attribute :>> payload = 300.0;
        attribute :>> touchdownSpeed = 1.9;
    }
    part relay : Lander {
        attribute :>> dryMass = 450.0;
        // The recorded relay run and trade study predate this edit: fuel was
        // 180.0 then, so relayRun reads stale and lightestRun silently disagrees.
        attribute :>> fuel = 210.0;
        attribute :>> burnRate = 2.5;
        attribute :>> payload = 25.0;
        attribute :>> touchdownSpeed = 1.4;
    }
}

package Descent {
    private import ScalarValues::*;
    private import Landers::*;

    // The requirement the analysis objective is typed by; the case binds its subject.
    requirement def FuelReserve {
        subject lander : Lander;
        in attribute fuelLeft : Real;
        attribute reserve : Real = 20.0;
        require constraint { fuelLeft >= reserve }
    }

    // Fuel a descent burn leaves: `then`-chained action steps feed each other
    // through parameter bindings, and the case's outputs are read afterwards.
    analysis def FuelBudget {
        subject lander : Lander;
        in attribute burnTime : Real = 40.0; // s of engine burn
        action burn {
            in rate : Real = lander.burnRate;
            in seconds : Real = burnTime;
            out used : Real;
            assign used := rate * seconds;
        }
        then action remaining {
            in loaded : Real = lander.fuel;
            in used : Real = burn.used;
            out left : Real;
            assign left := loaded - used;
        }

        out fuelUsed : Real = burn.used;
        out wetMass : Real = lander.dryMass + remaining.left;

        objective reserveHeld : FuelReserve {
            subject = lander;
            in fuelLeft = remaining.left;
        }

        return fuelLeft : Real = remaining.left;
    }

    analysis scoutBudget : FuelBudget {
        subject lander = scout;
    }
    analysis haulerBudget : FuelBudget {
        subject lander = hauler;
    }
    analysis relayBudget : FuelBudget {
        subject lander = relay;
    }

    // The touchdown requirement, stated of each candidate and satisfied by it.
    requirement def SoftLanding {
        subject lander : Lander;
        attribute limit : Real = 1.5;
        require constraint { lander.touchdownSpeed <= limit }
    }
    requirement scoutLandsSoftly : SoftLanding;
    satisfy scoutLandsSoftly by scout;
    requirement haulerLandsSoftly : SoftLanding;
    satisfy haulerLandsSoftly by hauler;
    requirement relayLandsSoftly : SoftLanding;
    satisfy relayLandsSoftly by relay;

    // Verifies the requirement from its objective; `PassIf` decides the verdict.
    verification def CheckScout {
        subject lander : Lander;
        objective {
            verify scoutLandsSoftly;
            require constraint { lander.touchdownSpeed <= 1.5 }
        }
        VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
    }
    verification def CheckHauler {
        subject lander : Lander;
        objective {
            verify haulerLandsSoftly;
            require constraint { lander.touchdownSpeed <= 1.5 }
        }
        VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
    }
    verification def CheckRelay {
        subject lander : Lander;
        objective {
            verify relayLandsSoftly;
            require constraint { lander.touchdownSpeed <= 1.5 }
        }
        VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
    }
    verification checkScout : CheckScout {
        subject lander = scout;
    }
    verification checkHauler : CheckHauler {
        subject lander = hauler;
    }
    verification checkRelay : CheckRelay {
        subject lander = relay;
    }
}

package Selection {
    private import ScalarValues::*;
    private import TradeStudies::*;
    private import Landers::*;

    // Lightest on the pad: every alternative is scored, and the objective picks the least.
    analysis lightest : TradeStudy {
        subject : Lander[1..*] = (scout, hauler, relay);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part l :>> alternative : Lander;
            return :>> result : Real = l.dryMass + l.fuel;
        }
        return part :>> selectedAlternative : Lander;
    }
}

package Records {
    private import ScalarValues::*;
    private import Landers::*;
    private import AnalysisRecords::*;

    // The record definitions live one nesting level down: queries walk the
    // package's direct members, and a `part def` matching `WhereType` would
    // surface its own unbound features.
    private package Vocab {
        // Shared definition for the demo's hand-written records: the library's
        // AnalysisRun plus `command` as a plain attribute — a document query's
        // `Project` cannot read an annotation's attribute values, so the
        // provenance table needs it declared.
        part def DemoRun :> AnalysisRecords::AnalysisRun {
            attribute command : String;
        }

        // A FuelBudget run written back: subject, input and outputs as
        // recorded, plus the output recomputed from the model as it stands
        // now. `drift` measures how far the model has moved since the record
        // was written, and `stale` names a record that no longer matches.
        // `lander` is the typed subject the formula reads; the library's
        // untyped `'subject'` ref is bound to the same part on each record.
        part def FuelBudgetRun :> DemoRun {
            ref part lander : Lander;
            attribute burnTime : Real;
            attribute fuelUsed : Real;
            attribute wetMass : Real;
            attribute fuelLeft : Real;
            attribute liveFuelLeft : Real = lander.fuel - burnTime * lander.burnRate;
            attribute drift : Real = liveFuelLeft - fuelLeft;
            attribute stale : Boolean = drift != 0.0;
        }

        // A TradeStudy run written back: the selection, plus the run's checks
        // and evaluations carried under the library's `verdicts` and
        // `evaluations`.
        part def TradeStudyRun :> DemoRun {
            attribute selected : String;
        }
    }
    private import Vocab::*;
    // One baseline run per candidate, at FuelBudget's default 40 s burn.
    part scoutRun : FuelBudgetRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:14:00Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget";
            kind = "run";
        }
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget";
        ref :>> 'subject' = Landers::scout;
        ref part :>> lander = scout;
        attribute :>> subjectName = "Landers::scout";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 120.0;
        attribute :>> wetMass = 730.0;
        attribute :>> fuelLeft = 130.0;
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "reserveHeld";
            attribute :>> status = "satisfied";
        }
    }
    part haulerRun : FuelBudgetRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:14:30Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::haulerBudget";
            kind = "run";
        }
        attribute :>> caseName = "Descent::haulerBudget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::haulerBudget";
        ref :>> 'subject' = Landers::hauler;
        ref part :>> lander = hauler;
        attribute :>> subjectName = "Landers::hauler";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 320.0;
        attribute :>> wetMass = 1980.0;
        attribute :>> fuelLeft = 580.0;
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "reserveHeld";
            attribute :>> status = "satisfied";
        }
    }
    part relayRun : FuelBudgetRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:15:00Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::relayBudget";
            kind = "run";
        }
        attribute :>> caseName = "Descent::relayBudget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::relayBudget";
        ref :>> 'subject' = Landers::relay;
        ref part :>> lander = relay;
        attribute :>> subjectName = "Landers::relay";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 100.0;
        attribute :>> wetMass = 530.0;
        attribute :>> fuelLeft = 80.0;
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "reserveHeld";
            attribute :>> status = "satisfied";
        }
    }

    // A sweep of scoutBudget over burnTime, one record per row the sweep printed.
    part scoutSweep40 : FuelBudgetRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:20:10Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
            kind = "sweep";
        }
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "sweep";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        ref :>> 'subject' = Landers::scout;
        ref part :>> lander = scout;
        attribute :>> subjectName = "Landers::scout";
        attribute :>> iteration = 1;
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 120.0;
        attribute :>> wetMass = 730.0;
        attribute :>> fuelLeft = 130.0;
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "reserveHeld";
            attribute :>> status = "satisfied";
        }
    }
    part scoutSweep60 : FuelBudgetRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:20:10Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
            kind = "sweep";
        }
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "sweep";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        ref :>> 'subject' = Landers::scout;
        ref part :>> lander = scout;
        attribute :>> subjectName = "Landers::scout";
        attribute :>> iteration = 2;
        attribute :>> burnTime = 60.0;
        attribute :>> fuelUsed = 180.0;
        attribute :>> wetMass = 670.0;
        attribute :>> fuelLeft = 70.0;
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "reserveHeld";
            attribute :>> status = "satisfied";
        }
    }
    part scoutSweep80 : FuelBudgetRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:20:10Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
            kind = "sweep";
        }
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "sweep";
        attribute :>> 'objective' = "not satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        ref :>> 'subject' = Landers::scout;
        ref part :>> lander = scout;
        attribute :>> subjectName = "Landers::scout";
        attribute :>> iteration = 3;
        attribute :>> burnTime = 80.0;
        attribute :>> fuelUsed = 240.0;
        attribute :>> wetMass = 610.0;
        attribute :>> fuelLeft = 10.0;
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "reserveHeld";
            attribute :>> status = "not satisfied";
        }
    }

    // The trade study run: the selection, its objective verdict, and each
    // alternative's evaluation — recorded before relay.fuel changed, so the
    // relay score here is the old 630.0.
    part lightestRun : TradeStudyRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2025-11-02T09:25:00Z";
            tool = "sysml";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Selection::lightest";
            kind = "trade";
        }
        attribute :>> caseName = "Selection::lightest";
        attribute :>> kind = "trade";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Selection::lightest";
        attribute :>> selected = "Landers::relay";
        part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "objective";
            attribute :>> name = "tradeStudyObjective";
            attribute :>> status = "satisfied";
        }
        part evaluation1 : AnalysisRecords::EvaluationRecord :> evaluations {
            attribute :>> 'function' = "Selection::lightest::evaluationFunction";
            attribute :>> alternative = "Landers::scout (object #1)";
            attribute :>> score = 850.0;
            attribute :>> result = "850.0";
            attribute :>> selected = false;
            attribute :>> tied = false;
        }
        part evaluation2 : AnalysisRecords::EvaluationRecord :> evaluations {
            attribute :>> 'function' = "Selection::lightest::evaluationFunction";
            attribute :>> alternative = "Landers::hauler (object #2)";
            attribute :>> score = 2300.0;
            attribute :>> result = "2300.0";
            attribute :>> selected = false;
            attribute :>> tied = false;
        }
        part evaluation3 : AnalysisRecords::EvaluationRecord :> evaluations {
            attribute :>> 'function' = "Selection::lightest::evaluationFunction";
            attribute :>> alternative = "Landers::relay (object #3)";
            attribute :>> score = 630.0;
            attribute :>> result = "630.0";
            attribute :>> selected = true;
            attribute :>> tied = false;
        }
    }
}

package Reporting {
    private import ScalarValues::*;
    private import DocumentQueries::*;
    private import KerML::Root::Element;

    // The records package's members walked by name, so a run `-record-run`
    // writes into the same `Records` package joins the query results.
    calc def AllRuns :> Query {
        Project(
            source = WhereType(
            source = WhereMetadata(
            source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1),
            'metadata' = "AnalysisRecords::RecordedRun"),
            type = "AnalysisRecords::AnalysisRun"),
            properties = ("name", "caseName", "kind", "subjectName", "objective"))
    }

    // Every saved fuel-budget run, sorted by burn time; the document groups it by subject.
    calc def FuelBudgetRuns :> Query {
        Project(
            source = OrderBy(
            source = WhereType(source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1), type = "FuelBudgetRun"),
            property = "burnTime", direction = "ascending", missing = "last", multiple = "error"),
            properties = ("name", "subjectName", "kind", "burnTime", "fuelUsed", "wetMass", "fuelLeft", "objective"))
    }

    // The sweep rows alone.
    calc def SweepRuns :> Query {
        Project(
            source = OrderBy(
            source = WhereFeature(
            source = WhereType(source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1), type = "FuelBudgetRun"),
            'feature' = "kind", operator = "=", value = "sweep"),
            property = "burnTime", direction = "ascending", missing = "last", multiple = "error"),
            properties = ("name", "caseName", "burnTime", "fuelUsed", "fuelLeft", "objective"))
    }

    // Records whose saved numbers no longer match the model as it stands.
    calc def StaleRuns :> Query {
        Project(
            source = WhereFeature(
            source = WhereType(source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1), type = "FuelBudgetRun"),
            'feature' = "stale", operator = "=", value = "true"),
            properties = ("name", "subjectName", "burnTime", "fuelLeft", "liveFuelLeft", "drift"))
    }

    // The recorded runs with their provenance annotation's command line.
    calc def Provenance :> Query {
        Project(
            source = WhereMetadata(source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1), 'metadata' = "AnalysisRecords::RecordedRun"),
            properties = ("name", "kind", "caseName", "command"))
    }

    // The trade-study record.
    calc def TradeRuns :> Query {
        Project(
            source = WhereType(source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 1), type = "TradeStudyRun"),
            properties = ("name", "caseName", "selected", "objective"))
    }

    // The evaluations nested under the trade-study record.
    calc def TradeEvaluations :> Query {
        Project(
            source = WhereType(source = Descendants(source = Named(qualifiedName = "Records"), maxDepth = 2), type = "EvaluationRecord"),
            properties = ("alternative", "score", "selected", "tied"))
    }

    // Assertions about the scout, checked live at render time.
    calc def ScoutVerdicts :> Query {
        in root : Element;
        Project(
            source = Verdicts(source = root),
            properties = ("kind", "name", "verdict", "reason"))
    }

    // The document itself.
    part def AnalysisReport :> Document {
        attribute redefines title = "Recorded analysis runs";

        part intro : Paragraph {
            attribute redefines text = "An analysis run's printed results are discarded unless they are recorded — by -record-run or by hand, in the vocabulary the AnalysisRecords library defines. Every row below is a declared record typed by a run definition on that vocabulary, annotated with the command that produced it — except the last table, whose verdicts are recomputed live.";
        }

        part every : Section {
            attribute redefines title = "Every recorded run";
            part runs : Table {
                attribute redefines caption = "Every @AnalysisRecords::RecordedRun-annotated AnalysisRun in the Records package";
                calc rows : AllRuns;
            }
        }

        part budgets : Section {
            attribute redefines title = "Recorded fuel budgets";
            part runs : Table {
                attribute redefines caption = "One record per run, grouped by subject";
                attribute redefines groupBy = "subjectName";
                calc rows : FuelBudgetRuns;
            }
        }

        part sweep : Section {
            attribute redefines title = "Sweep of scoutBudget";
            part runs : Table {
                attribute redefines caption = "One record per sweep row";
                calc rows : SweepRuns;
            }
        }

        part stale : Section {
            attribute redefines title = "Runs that no longer match the model";
            part runs : Table {
                attribute redefines caption = "Records whose saved fuelLeft differs from the value the model now derives";
                calc rows : StaleRuns;
            }
        }

        part provenance : Section {
            attribute redefines title = "Provenance";
            part runs : Table {
                attribute redefines caption = "Every element annotated @AnalysisRecords::RecordedRun";
                calc rows : Provenance;
            }
        }

        part trade : Section {
            attribute redefines title = "Trade study";
            part runs : Table {
                attribute redefines caption = "The recorded selection and verdict";
                calc rows : TradeRuns;
            }
            part evals : Table {
                attribute redefines caption = "Each alternative's evaluation, as recorded";
                calc rows : TradeEvaluations;
            }
        }

        part live : Section {
            attribute redefines title = "Live verdicts";
            part runs : Table {
                attribute redefines caption = "Assertions about the scout, evaluated at render time";
                calc rows : ScoutVerdicts {
                    in root = Landers::scout;
                }
            }
        }
    }
}
