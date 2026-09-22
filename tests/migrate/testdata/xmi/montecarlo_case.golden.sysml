part def Sensor {
    attribute settleTime : ScalarValues::Real;
    attribute gain : ScalarValues::Real default = 1.5;
    attribute scale : ScalarValues::Real;
    attribute label : ScalarValues::String;
    attribute misses : ScalarValues::Integer;
    action def Settle {
        first start then 'start timer';
        action 'start timer' {
            assign this.settleTime := localClock.currentTime;
        }
        first 'start timer' then wait;
        action wait accept after RandomFunctions::uniform(2.0, 4.0) [SI::s];
        first wait then settle;
        action settle;
        first settle then 'stop timer';
        action 'stop timer' {
            assign this.settleTime := localClock.currentTime - this.settleTime;
        }
        first 'stop timer' then final;
        action final terminate;
    }
    perform action settle : Settle;
}
part def 'Settling Analysis' :> Sensor {
    /* not migrated: «BindingConnector» Connector (_bindMedian) — the connector binds the simulation tool's MonteCarloAnalysis::Median, a statistic Simulation::MonteCarlo has no counterpart for; applied stereotypes «BindingConnector» */
    /* not migrated: «BindingConnector» Connector (_bindN) — the connector binds the simulation tool's MonteCarloAnalysis::N to label, which is of String, and the statistic is of a number; applied stereotypes «BindingConnector» */
    bind gain = scale;
}
analysis def 'Settling Analysis Monte Carlo' :> Simulation::MonteCarlo {
    subject analysed : 'Settling Analysis';
    perform action run ::> analysed.settle;
    attribute :>> observed : ScalarValues::Real = analysed.settleTime;
    return Mean : ScalarValues::Real = mean;
    out Deviation : ScalarValues::Real[0..1] = deviation;
    out OutOfSpec : ScalarValues::Integer = outOfSpec;
}
individual part def 'settling analysis' :> 'Settling Analysis';
part def Stranger {
    attribute x : ScalarValues::Real;
    /* not migrated: «BindingConnector» Connector (_bindStranger) — the connector binds x to the simulation tool's MonteCarloAnalysis::Mean, a statistic of an analysis its owner does not inherit; applied stereotypes «BindingConnector» */
}
action def 'Settling Group' {
    @Simulation::Configuration {
        runs = 5;
        draws = Simulation::DrawPolicy::random;
        timeVariable = "simtime";
    }
    part target : 'settling analysis';
    perform action run ::> target.settle;
    /* results of the simulation tool: 1 snapshot(s) in Results standing for 5 run(s) analysing settleTime holding settleTime */
}
package Results {
    individual part def 'settling of 5 runs' :> 'Settling Analysis' {
        /* not migrated: Slot (_sMedian) — the slot holds the simulation tool's MonteCarloAnalysis::Median, a statistic Simulation::MonteCarlo has no counterpart for */
        attribute :>> settleTime = 3.1;
        analysis 'Monte Carlo' : 'Settling Analysis Monte Carlo' {
            subject :>> analysed : 'settling of 5 runs';
            out :>> runs = 5;
            out :>> mean = 3.1;
            out :>> deviation = 0.6;
            out :>> outOfSpec = 0;
        }
    }
}
