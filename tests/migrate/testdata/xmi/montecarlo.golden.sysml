part def Timer {
    attribute t : ScalarValues::Real;
    attribute u : ScalarValues::Real;
    attribute p : ScalarValues::Real default = 0.5;
    action def Tick {
        first start then three;
        action three {
            out result = 3.0;
        }
        first three then 'set t';
        action 'set t' {
            in value;
            assign this.t := value;
        }
        first 'set t' then final;
        action final terminate;
        flow three.result to 'set t'.value;
    }
    perform action tick : Tick;
}
part def 'Timer Analysis' :> Timer {
    /* not migrated: «BindingConnector» Connector (_bind) — the connector binds t to the simulation tool's MonteCarloAnalysis::Mean, the statistic it computes of t over the runs, which v2 has no analysis pattern for; the migration results read the statistic from the result snapshots; applied stereotypes «BindingConnector» */
}
individual part def 'timer analysis' :> 'Timer Analysis';
part def 'Unbound Analysis' :> Timer;
individual part def 'unbound analysis' :> 'Unbound Analysis';
action def 'Group 0' {
    @Simulation::Configuration {
        runs = 3;
        draws = Simulation::DrawPolicy::average;
    }
    part target : 'timer analysis';
    perform action run ::> target.tick;
    /* results of the simulation tool: 5 snapshot(s) in Results standing for 8 run(s) analysing t holding p, t */
}
action def 'Group 1' {
    @Simulation::Configuration {
        draws = Simulation::DrawPolicy::average;
    }
    part target : 'timer analysis';
    perform action run ::> target.tick;
    /* results of the simulation tool: 0 snapshot(s) in Empty analysing t */
}
action def 'Group 2' {
    @Simulation::Configuration {
        runs = 1;
        draws = Simulation::DrawPolicy::average;
    }
    part target : 'unbound analysis';
    perform action run ::> target.tick;
    /* results of the simulation tool: 1 snapshot(s) in Unbound Results holding t */
}
package Results {
    individual part def 'run 1' :> 'Timer Analysis' {
        attribute :>> t = 2.0;
        attribute :>> p = 0.5;
        /* not migrated: Slot (_rawN) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_rawMean) — the slot holds the simulation tool's MonteCarloAnalysis::Mean statistic of the runs, which is no value of the instance; the migration results read it */
    }
    individual part def 'analysis of 4 runs' :> 'Timer Analysis' {
        attribute :>> t = 3.5;
        /* not migrated: Slot (_sumN) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_sumMean) — the slot holds the simulation tool's MonteCarloAnalysis::Mean statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_sumDev) — the slot holds the simulation tool's MonteCarloAnalysis::Deviation statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_sumOut) — the slot holds the simulation tool's MonteCarloAnalysis::OutOfSpec statistic of the runs, which is no value of the instance; the migration results read it */
    }
    individual part def 'analysis of u' :> 'Timer Analysis' {
        attribute :>> u = 9.0;
        /* not migrated: Slot (_forN) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_forMean) — the slot holds the simulation tool's MonteCarloAnalysis::Mean statistic of the runs, which is no value of the instance; the migration results read it */
    }
    individual part def 'half an analysis' :> 'Timer Analysis' {
        attribute :>> t = 6.0;
        /* not migrated: Slot (_halfN) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
    }
    individual part def 'analysis of two and a half runs' :> 'Timer Analysis' {
        attribute :>> t = 4.0;
        /* not migrated: Slot (_oddN) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_oddMean) — the slot holds the simulation tool's MonteCarloAnalysis::Mean statistic of the runs, which is no value of the instance; the migration results read it */
    }
    individual part def 'analysis of nothing held' :> 'Timer Analysis' {
        /* not migrated: Slot (_strayN) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
        /* not migrated: Slot (_strayMean) — the slot holds the simulation tool's MonteCarloAnalysis::Mean statistic of the runs, which is no value of the instance; the migration results read it */
    }
}
package Empty;
package 'Unbound Results' {
    individual part def 'run 1' :> 'Unbound Analysis' {
        attribute :>> t = 1.0;
        /* not migrated: Slot (_ub1N) — the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it */
    }
}
