// Analysis runs are printed, not saved: `-analysis` and `-sweep` report each
// run's inputs, outputs and verdicts and then discard them, so nothing a
// document query can read records that a run happened. This model shows the
// workaround — write each run back into the model yourself as a `part` usage
// typed by a result-record definition, annotated with provenance metadata —
// and the one thing the pattern cannot fake: a derived attribute on the
// record recomputes the run's output from the model *as it stands now*, so a
// record whose numbers the model has moved away from reports its own drift.
// See README.md for the commands that produced each recorded row.

package Landers {
    private import ScalarValues::*;

    part def Lander {
        attribute dryMass : Real;          // kg, without fuel
        attribute fuel : Real;             // kg loaded at the start of descent
        attribute burnRate : Real;         // kg/s the engine consumes at full thrust
        attribute payload : Real;          // kg of instruments carried
        attribute touchdownSpeed : Real;   // m/s, from the descent profile
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
        // The recorded relay run predates this edit: it was captured when fuel
        // was 180.0, so its record now reads stale.
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
        in attribute burnTime : Real = 40.0;   // s of engine burn

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

    // Provenance of a saved run: when, by what, at which revision, which command.
    metadata def RecordedRun {
        attribute runAt : String;
        attribute tool : String;
        attribute revision : String;
        attribute command : String;
    }

    // The vocabulary of a saved analysis run. `kind` is "run", "sweep" or
    // "trade"; `objective` records the verdict the run printed.
    // `command` is a plain attribute, not metadata, because a document
    // query's `Project` cannot read an annotation's attribute values — it is
    // what the provenance table shows for each record.
    part def AnalysisRun {
        attribute caseName : String;
        attribute kind : String;
        attribute 'objective' : String;
        attribute command : String;
    }

    // A FuelBudget run written back: subject, input and outputs as recorded,
    // plus the output recomputed from the model as it stands now. `drift`
    // measures how far the model has moved since the record was written, and
    // `stale` names a record that no longer matches.
    part def FuelBudgetRun :> AnalysisRun {
        ref part lander : Lander;
        attribute subjectName : String;
        attribute burnTime : Real;
        attribute fuelUsed : Real;
        attribute wetMass : Real;
        attribute fuelLeft : Real;
        attribute liveFuelLeft : Real = lander.fuel - burnTime * lander.burnRate;
        attribute drift : Real = liveFuelLeft - fuelLeft;
        attribute stale : Boolean = drift != 0.0;
    }

    // A TradeStudy run written back: the selection and every score it printed.
    part def TradeStudyRun :> AnalysisRun {
        attribute selected : String;
        attribute scoutScore : Real;
        attribute haulerScore : Real;
        attribute relayScore : Real;
    }
}

package Results {
    private import ScalarValues::*;
    private import Landers::*;
    private import Records::*;

    // The saved runs, held as subparts of one part so a query can walk them.
    part results {

    // One baseline run per candidate, at FuelBudget's default 40 s burn.
    part scoutRun : FuelBudgetRun {
        @RecordedRun {
            runAt = "2025-11-02T09:14:00Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget";
        }
        ref part :>> lander = scout;
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget";
        attribute :>> subjectName = "scout";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 120.0;
        attribute :>> wetMass = 730.0;
        attribute :>> fuelLeft = 130.0;
    }
    part haulerRun : FuelBudgetRun {
        @RecordedRun {
            runAt = "2025-11-02T09:14:30Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::haulerBudget";
        }
        ref part :>> lander = hauler;
        attribute :>> caseName = "Descent::haulerBudget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::haulerBudget";
        attribute :>> subjectName = "hauler";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 320.0;
        attribute :>> wetMass = 1980.0;
        attribute :>> fuelLeft = 580.0;
    }
    part relayRun : FuelBudgetRun {
        @RecordedRun {
            runAt = "2025-11-02T09:15:00Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::relayBudget";
        }
        ref part :>> lander = relay;
        attribute :>> caseName = "Descent::relayBudget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::relayBudget";
        attribute :>> subjectName = "relay";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 100.0;
        attribute :>> wetMass = 530.0;
        attribute :>> fuelLeft = 80.0;
    }

    // A sweep of scoutBudget over burnTime, one record per row the sweep printed.
    part scoutSweep40 : FuelBudgetRun {
        @RecordedRun {
            runAt = "2025-11-02T09:20:10Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        }
        ref part :>> lander = scout;
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "sweep";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        attribute :>> subjectName = "scout";
        attribute :>> burnTime = 40.0;
        attribute :>> fuelUsed = 120.0;
        attribute :>> wetMass = 730.0;
        attribute :>> fuelLeft = 130.0;
    }
    part scoutSweep60 : FuelBudgetRun {
        @RecordedRun {
            runAt = "2025-11-02T09:20:10Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        }
        ref part :>> lander = scout;
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "sweep";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        attribute :>> subjectName = "scout";
        attribute :>> burnTime = 60.0;
        attribute :>> fuelUsed = 180.0;
        attribute :>> wetMass = 670.0;
        attribute :>> fuelLeft = 70.0;
    }
    part scoutSweep80 : FuelBudgetRun {
        @RecordedRun {
            runAt = "2025-11-02T09:20:10Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        }
        ref part :>> lander = scout;
        attribute :>> caseName = "Descent::scoutBudget";
        attribute :>> kind = "sweep";
        attribute :>> 'objective' = "not satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget -sweep burnTime=40.0..80.0:20.0";
        attribute :>> subjectName = "scout";
        attribute :>> burnTime = 80.0;
        attribute :>> fuelUsed = 240.0;
        attribute :>> wetMass = 610.0;
        attribute :>> fuelLeft = 10.0;
    }

    // The trade study run: the selection and each alternative's score.
    part lightestRun : TradeStudyRun {
        @RecordedRun {
            runAt = "2025-11-02T09:25:00Z";
            tool = "sysml";
            revision = "v0.8";
            command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Selection::lightest";
        }
        attribute :>> caseName = "Selection::lightest";
        attribute :>> kind = "trade";
        attribute :>> 'objective' = "satisfied";
        attribute :>> command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Selection::lightest";
        attribute :>> selected = "relay";
        attribute :>> scoutScore = 850.0;
        attribute :>> haulerScore = 2300.0;
        attribute :>> relayScore = 630.0;
    }
    }
}

package Reporting {
    private import ScalarValues::*;
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    private import Results::*;

    // Every saved fuel-budget run, sorted by burn time; the document groups it by subject.
    calc def FuelBudgetRuns :> Query {
        in root : Element;
        Project(
            source = OrderBy(
                source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "FuelBudgetRun"),
                property = "burnTime", direction = "ascending", missing = "last", multiple = "error"),
            properties = ("name", "subjectName", "kind", "burnTime", "fuelUsed", "wetMass", "fuelLeft", "objective"))
    }

    // The sweep rows alone.
    calc def SweepRuns :> Query {
        in root : Element;
        Project(
            source = OrderBy(
                source = WhereFeature(
                    source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "FuelBudgetRun"),
                    'feature' = "kind", operator = "=", value = "sweep"),
                property = "burnTime", direction = "ascending", missing = "last", multiple = "error"),
            properties = ("name", "caseName", "burnTime", "fuelUsed", "fuelLeft", "objective"))
    }

    // Records whose saved numbers no longer match the model as it stands.
    calc def StaleRuns :> Query {
        in root : Element;
        Project(
            source = WhereFeature(
                source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "FuelBudgetRun"),
                'feature' = "stale", operator = "=", value = "true"),
            properties = ("name", "subjectName", "burnTime", "fuelLeft", "liveFuelLeft", "drift"))
    }

    // The recorded runs with their provenance annotation's command line.
    calc def Provenance :> Query {
        in root : Element;
        Project(
            source = WhereMetadata(source = Descendants(source = root, maxDepth = 1), 'metadata' = "Records::RecordedRun"),
            properties = ("name", "kind", "caseName", "command"))
    }

    // The trade-study record.
    calc def TradeRuns :> Query {
        in root : Element;
        Project(
            source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "TradeStudyRun"),
            properties = ("name", "caseName", "selected", "scoutScore", "haulerScore", "relayScore", "objective"))
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
            attribute redefines text = "Analysis runs are printed and discarded; nothing writes them back. Every row below is a declared record — a part typed by a run definition whose attributes hold the inputs, outputs and objective a run printed, annotated with the command that produced it — except the last table, whose verdicts are recomputed live.";
        }

        part budgets : Section {
            attribute redefines title = "Recorded fuel budgets";
            part runs : Table {
                attribute redefines caption = "One record per run, grouped by subject";
                attribute redefines groupBy = "subjectName";
                calc rows : FuelBudgetRuns {
                    in root = results;
                }
            }
        }

        part sweep : Section {
            attribute redefines title = "Sweep of scoutBudget";
            part runs : Table {
                attribute redefines caption = "One record per sweep row";
                calc rows : SweepRuns {
                    in root = results;
                }
            }
        }

        part stale : Section {
            attribute redefines title = "Runs that no longer match the model";
            part runs : Table {
                attribute redefines caption = "Records whose saved fuelLeft differs from the value the model now derives";
                calc rows : StaleRuns {
                    in root = results;
                }
            }
        }

        part provenance : Section {
            attribute redefines title = "Provenance";
            part runs : Table {
                attribute redefines caption = "Every element annotated @RecordedRun";
                calc rows : Provenance {
                    in root = results;
                }
            }
        }

        part trade : Section {
            attribute redefines title = "Trade study";
            part runs : Table {
                attribute redefines caption = "The recorded selection and scores";
                calc rows : TradeRuns {
                    in root = results;
                }
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
