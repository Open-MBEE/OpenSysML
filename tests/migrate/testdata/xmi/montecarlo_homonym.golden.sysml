part def MonteCarloAnalysis {
    attribute N : ScalarValues::Integer;
    attribute Mean : ScalarValues::Real;
    attribute Deviation : ScalarValues::Real;
    attribute OutOfSpec : ScalarValues::Integer;
}
part def Gauge {
    attribute reading : ScalarValues::Real;
}
part def 'Gauge Analysis' :> Gauge, MonteCarloAnalysis {
    bind reading = Mean;
}
individual part def 'gauge analysis' :> 'Gauge Analysis';
action def 'Gauge Group' {
    @Simulation::Configuration {
        runs = 1;
        draws = Simulation::DrawPolicy::average;
    }
    part target : 'gauge analysis';
    /* results of the simulation tool: 1 snapshot(s) in Results holding Mean, N, reading */
}
package Results {
    individual part def 'run 1' :> 'Gauge Analysis' {
        attribute :>> reading = 2.5;
        attribute :>> N = 4;
        attribute :>> Mean = 2.5;
    }
}
